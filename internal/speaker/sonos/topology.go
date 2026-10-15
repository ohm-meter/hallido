package sonos

import (
	"context"
	"encoding/xml"
	"errors"
	"html"
	"regexp"
	"strings"
	"time"
)

// hiddenCacheTTL begrenzt, wie lange ein Topologie-Ergebnis im Pfad pro Box
// (refreshDevicesFor) wiederverwendet wird, damit Szenenstarts keine eigene
// GetZoneGroupState-Abfrage pro Box ausloesen.
const hiddenCacheTTL = 30 * time.Second

var (
	reSatelliteUUID    = regexp.MustCompile(`(?i)<Satellite\b[^>]*\bUUID="([^"]+)"`)
	reInvisibleUUID    = regexp.MustCompile(`(?i)\bUUID="([^"]+)"[^>]*\bInvisible="1"`)
	reInvisibleUUIDAlt = regexp.MustCompile(`(?i)\bInvisible="1"[^>]*\bUUID="([^"]+)"`)
)

type getZoneGroupStateResponse struct {
	ZoneGroupState string `xml:"Body>GetZoneGroupStateResponse>ZoneGroupState"`
}

type getInvisibleResponse struct {
	CurrentInvisible string `xml:"Body>GetInvisibleResponse>CurrentInvisible"`
}

// filterBondedInvisible entfernt gebundene HT-Satelliten und unsichtbare Boxen
// aus der Discovery-Liste. Primaer per GetZoneGroupState, sonst GetInvisible pro Geraet.
func (c *Controller) filterBondedInvisible(ctx context.Context, found map[string]*device) {
	c.filterBonded(ctx, found, false)
}

// filterBonded wie filterBondedInvisible; useCache erlaubt, ein frisches
// Topologie-Ergebnis (hiddenCacheTTL) wiederzuverwenden.
func (c *Controller) filterBonded(ctx context.Context, found map[string]*device, useCache bool) {
	if len(found) == 0 {
		return
	}

	var hidden map[string]bool
	if useCache {
		c.mu.RLock()
		if c.hiddenSet != nil && time.Since(c.hiddenAt) < hiddenCacheTTL {
			hidden = c.hiddenSet
		}
		c.mu.RUnlock()
	}
	if hidden == nil {
		hidden = c.hiddenUUIDsViaTopology(ctx, found)
		if hidden != nil {
			c.mu.Lock()
			c.hiddenSet, c.hiddenAt = hidden, time.Now()
			c.mu.Unlock()
		}
	}
	if hidden == nil {
		c.log.Warn("zone topology not readable, checking devices individually via GetInvisible")
		hidden = c.hiddenUUIDsViaGetInvisible(ctx, found)
		if hidden == nil {
			c.log.Warn("invisible devices could not be determined, all discovery hits stay visible")
			return
		}
	}

	c.mu.Lock()
	if c.bonded == nil {
		c.bonded = map[string]bool{}
	}
	for name, d := range found {
		if hidden[d.uid] {
			delete(found, name)
			c.bonded[name] = true
		} else {
			delete(c.bonded, name)
		}
	}
	c.mu.Unlock()
}

// hiddenUUIDsViaTopology fragt GetZoneGroupState auf einem erreichbaren Geraet ab.
// nil = Aufruf oder Parsing fehlgeschlagen.
func (c *Controller) hiddenUUIDsViaTopology(ctx context.Context, found map[string]*device) map[string]bool {
	for _, d := range found {
		raw, err := c.soapCall(ctx, d, zoneGroupTopology, "GetZoneGroupState", nil)
		if err != nil {
			continue
		}
		state, err := parseZoneGroupStateSOAP(raw)
		if err != nil {
			c.log.Warn("zone group state parse failed", "speaker", d.name, "err", err)
			continue
		}
		return hiddenUUIDsFromZoneGroupState(state)
	}
	return nil
}

// hiddenUUIDsViaGetInvisible prueft jedes Geraet per DeviceProperties.GetInvisible.
// nil = kein einziges Geraet antwortete erfolgreich.
func (c *Controller) hiddenUUIDsViaGetInvisible(ctx context.Context, found map[string]*device) map[string]bool {
	hidden := map[string]bool{}
	var anyOK bool
	for _, d := range found {
		invis, ok := c.deviceInvisible(ctx, d)
		if !ok {
			continue
		}
		anyOK = true
		if invis {
			hidden[d.uid] = true
		}
	}
	if !anyOK {
		return nil
	}
	return hidden
}

func (c *Controller) deviceInvisible(ctx context.Context, d *device) (bool, bool) {
	raw, err := c.soapCall(ctx, d, deviceProperties, "GetInvisible", []arg{
		{"InstanceID", "0"},
	})
	if err != nil {
		return false, false
	}
	var resp getInvisibleResponse
	if err := xml.Unmarshal(raw, &resp); err != nil {
		return false, false
	}
	return strings.TrimSpace(resp.CurrentInvisible) == "1", true
}

func parseZoneGroupStateSOAP(body []byte) (string, error) {
	var resp getZoneGroupStateResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	state := strings.TrimSpace(resp.ZoneGroupState)
	if state == "" {
		return "", errEmptyZoneGroupState
	}
	return html.UnescapeString(state), nil
}

var errEmptyZoneGroupState = errors.New("empty ZoneGroupState")

// hiddenUUIDsFromZoneGroupState sammelt UUIDs unsichtbarer und gebundener HT-Geraete.
func hiddenUUIDsFromZoneGroupState(state string) map[string]bool {
	hidden := map[string]bool{}
	for _, re := range []*regexp.Regexp{reSatelliteUUID, reInvisibleUUID, reInvisibleUUIDAlt} {
		for _, m := range re.FindAllStringSubmatch(state, -1) {
			if len(m) > 1 && m[1] != "" {
				hidden[m[1]] = true
			}
		}
	}
	return hidden
}
