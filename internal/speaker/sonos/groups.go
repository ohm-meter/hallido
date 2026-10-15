package sonos

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/url"
	"sort"
	"strings"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

// zoneGroupXML bildet einen <ZoneGroup>-Eintrag aus GetZoneGroupState ab.
// Satellite-Elemente (HT-Surrounds) haengen unter ZoneGroupMember und werden
// hier bewusst nicht gelesen.
type zoneGroupXML struct {
	Coordinator string `xml:"Coordinator,attr"`
	ID          string `xml:"ID,attr"`
	Members     []struct {
		UUID         string `xml:"UUID,attr"`
		ZoneName     string `xml:"ZoneName,attr"`
		Location     string `xml:"Location,attr"`
		Invisible    string `xml:"Invisible,attr"`
		IsZoneBridge string `xml:"IsZoneBridge,attr"`
	} `xml:"ZoneGroupMember"`
}

// topologyMember ist eine sichtbare Box aus der Topologie.
type topologyMember struct {
	name    string
	uid     string
	baseURL string // aus Location, leer wenn unbekannt
}

// topologyGroup ist eine Gruppe mit sichtbaren Mitgliedern; coordinator ist
// zuerst einsortiert.
type topologyGroup struct {
	coordinator topologyMember
	members     []topologyMember
}

// parseZoneGroups liest alle ZoneGroup-Elemente aus einem (entescapten)
// ZoneGroupState. Funktioniert fuer beide Firmware-Varianten:
// <ZoneGroupState><ZoneGroups>... und direkt <ZoneGroups>...
func parseZoneGroups(state string) ([]topologyGroup, error) {
	dec := xml.NewDecoder(strings.NewReader(state))
	var out []topologyGroup
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "ZoneGroup" {
			continue
		}
		var zg zoneGroupXML
		if err := dec.DecodeElement(&zg, &se); err != nil {
			return nil, err
		}
		if g, ok := toTopologyGroup(zg); ok {
			out = append(out, g)
		}
	}
	return out, nil
}

func toTopologyGroup(zg zoneGroupXML) (topologyGroup, bool) {
	var g topologyGroup
	var coordFound bool
	seen := map[string]bool{}
	for _, m := range zg.Members {
		name := strings.TrimSpace(m.ZoneName)
		if name == "" || m.Invisible == "1" || m.IsZoneBridge == "1" || seen[name] {
			continue
		}
		seen[name] = true
		tm := topologyMember{name: name, uid: m.UUID, baseURL: topologyBaseURL(m.Location)}
		if m.UUID == zg.Coordinator {
			g.coordinator = tm
			coordFound = true
			continue
		}
		g.members = append(g.members, tm)
	}
	if !coordFound {
		return topologyGroup{}, false
	}
	sort.Slice(g.members, func(i, j int) bool { return g.members[i].name < g.members[j].name })
	g.members = append([]topologyMember{g.coordinator}, g.members...)
	return g, true
}

// topologyBaseURL macht aus dem Location-Attribut der Topologie
// ("http://IP:1400/xml/device_description.xml") die Basis-URL; leer, wenn
// das Attribut fehlt oder keine absolute URL ist.
func topologyBaseURL(loc string) string {
	u, err := url.Parse(strings.TrimSpace(loc))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// GetGroups liefert die aktuelle Gruppen-Topologie. Die Topologie ist im
// ganzen Haushalt gleich -- gefragt wird die erste Box, die antwortet.
// Boxen aus der Topologie, die noch nicht im Cache sind, werden ergaenzt
// (sonst muesste der naechste Befehl an sie eine Discovery ausloesen).
func (c *Controller) GetGroups(ctx context.Context) ([]speaker.GroupInfo, error) {
	groups, err := c.topology(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]speaker.GroupInfo, 0, len(groups))
	for _, g := range groups {
		gi := speaker.GroupInfo{Coordinator: g.coordinator.name}
		for _, m := range g.members {
			gi.Members = append(gi.Members, m.name)
		}
		out = append(out, gi)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Coordinator < out[j].Coordinator })
	return out, nil
}

func (c *Controller) topology(ctx context.Context) ([]topologyGroup, error) {
	candidates := c.cachedDevices()
	if len(candidates) == 0 {
		if _, err := c.Discover(ctx); err != nil {
			return nil, err
		}
		candidates = c.cachedDevices()
	}
	var lastErr error
	for _, d := range candidates {
		raw, err := c.soapCall(ctx, d, zoneGroupTopology, "GetZoneGroupState", nil)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				break
			}
			continue
		}
		state, err := parseZoneGroupStateSOAP(raw)
		if err != nil {
			lastErr = apperr.New(apperr.CodePlaybackFailed,
				"Group topology not readable: "+err.Error()).WithSpeaker(d.name)
			continue
		}
		groups, err := parseZoneGroups(state)
		if err != nil {
			lastErr = apperr.New(apperr.CodePlaybackFailed,
				"Group topology not readable: "+err.Error()).WithSpeaker(d.name)
			continue
		}
		c.learnFromTopology(groups)
		return groups, nil
	}
	if lastErr == nil {
		lastErr = apperr.New(apperr.CodeSpeakerUnreachable, "no Sonos speaker known for the group query")
	}
	return nil, lastErr
}

// cachedDevices liefert die bekannten Geraete in stabiler Reihenfolge.
func (c *Controller) cachedDevices() []*device {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]*device, 0, len(c.devices))
	for _, d := range c.devices {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func (c *Controller) learnFromTopology(groups []topologyGroup) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, g := range groups {
		for _, m := range g.members {
			if _, ok := c.devices[m.name]; ok || m.baseURL == "" || m.uid == "" {
				continue
			}
			c.devices[m.name] = &device{name: m.name, baseURL: m.baseURL, uid: m.uid}
		}
	}
}

// Resume setzt die Wiedergabe auf dem Coordinator fort (AVTransport Play
// mit der bereits geladenen Quelle). Laeuft die Box schon, ist das kein
// Fehler. Ohne Quelle antwortet Sonos mit UPnP 701 -> playback_failed.
func (c *Controller) Resume(ctx context.Context, coordinator string) error {
	d, err := c.resolve(ctx, coordinator)
	if err != nil {
		return err
	}
	_, err = c.soapCall(ctx, d, avTransport, "Play", []arg{
		{"InstanceID", "0"},
		{"Speed", "1"},
	})
	if err != nil && isTransitionNotAvailable(err) {
		return apperr.New(apperr.CodePlaybackFailed,
			"Speaker cannot resume (no source loaded)").
			WithSpeaker(coordinator).
			WithHint("After a stop without a source, or on a group member: restart the scene or address the coordinator.")
	}
	return err
}

// Ungroup loest die Box aus ihrer Gruppe
// (AVTransport BecomeCoordinatorOfStandaloneGroup). Ob die Box ueberhaupt
// gruppiert ist, prueft der Aufrufer per GetGroups.
func (c *Controller) Ungroup(ctx context.Context, name string) error {
	d, err := c.resolve(ctx, name)
	if err != nil {
		return err
	}
	_, err = c.soapCall(ctx, d, avTransport, "BecomeCoordinatorOfStandaloneGroup", []arg{
		{"InstanceID", "0"},
	})
	return err
}
