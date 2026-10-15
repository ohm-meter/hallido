// Package mock liefert einen SpeakerController ohne echte Hardware.
//
// Damit laeuft und testet die gesamte Anwendung lokal, ohne Sonos-Geraet.
// Erreichbarkeit einzelner Lautsprecher ist konfigurierbar, sodass auch
// Fehlerpfade (Geraet offline, Gruppierung scheitert) getestet werden.
package mock

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

// PlayCall protokolliert einen Play-Aufruf fuer Test-Assertions.
type PlayCall struct {
	Coordinator string
	Source      speaker.Source
}

// Controller ist ein in-memory SpeakerController fuer Tests und lokalen Betrieb.
type Controller struct {
	mu sync.Mutex

	// reachable steuert, welche Lautsprecher als erreichbar gelten.
	// Fehlt ein Name, gilt er als nicht erreichbar.
	reachable map[string]bool

	// discovery ist die von ListSpeakers gelieferte Test-Liste (unabhaengig von Config).
	discovery []speaker.SpeakerInfo

	// transport haelt den simulierten AVTransport-Zustand je Coordinator.
	transport map[string]speaker.PlaybackState

	// nowPlaying haelt simulierte Metadaten je Box (fuer Tests/API).
	nowPlaying map[string]speaker.NowPlayingInfo

	// media merkt sich die zuletzt geladene Quelle je Coordinator -- wie
	// eine echte Box, die nach Stop/Pause per Play fortsetzen kann.
	media map[string]speaker.NowPlayingInfo

	// coordinatorOf bildet jede gruppierte Box auf ihren Coordinator ab;
	// fehlt ein Name, steht die Box allein.
	coordinatorOf map[string]string

	// ungroupRejects: Boxen, die Ungroup ablehnen (erreichbar, aber Fehler).
	ungroupRejects map[string]bool

	// Aufgezeichnete Aufrufe (fuer Tests).
	Volumes  map[string]int
	Groups   map[string][]string
	Plays    []PlayCall
	Stops    []string // Coordinatoren, auf denen Stop aufgerufen wurde
	Pauses   []string // Coordinatoren, auf denen Pause aufgerufen wurde
	Resumes  []string // Coordinatoren, auf denen Resume aufgerufen wurde
	Ungroups []string // Boxen, die per Ungroup ihre Gruppe verlassen haben
}

// New erstellt einen Mock; alle uebergebenen Namen gelten als erreichbar
// und erscheinen in der Discovery-Liste mit Test-IP/UID.
func New(reachableSpeakers ...string) *Controller {
	r := make(map[string]bool, len(reachableSpeakers))
	discovery := make([]speaker.SpeakerInfo, len(reachableSpeakers))
	for i, name := range reachableSpeakers {
		r[name] = true
		discovery[i] = defaultSpeakerInfo(name, i+1)
	}
	sort.Slice(discovery, func(i, j int) bool { return discovery[i].Name < discovery[j].Name })
	return &Controller{
		reachable:     r,
		discovery:     discovery,
		transport:     make(map[string]speaker.PlaybackState),
		nowPlaying:    make(map[string]speaker.NowPlayingInfo),
		media:         make(map[string]speaker.NowPlayingInfo),
		coordinatorOf: make(map[string]string),
		Volumes:       make(map[string]int),
		Groups:        make(map[string][]string),
	}
}

func defaultSpeakerInfo(name string, octet int) speaker.SpeakerInfo {
	return speaker.SpeakerInfo{
		Name:      name,
		IP:        fmt.Sprintf("192.168.1.%d", octet),
		UID:       fmt.Sprintf("RINCON_MOCK_%s", name),
		Reachable: true,
	}
}

// SetDiscovery ersetzt die Discovery-Liste (fuer Tests).
func (c *Controller) SetDiscovery(list []speaker.SpeakerInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.discovery = append([]speaker.SpeakerInfo(nil), list...)
	sort.Slice(c.discovery, func(i, j int) bool { return c.discovery[i].Name < c.discovery[j].Name })
}

// SetTransportState setzt den simulierten Wiedergabestatus (fuer Tests).
func (c *Controller) SetTransportState(coordinator string, state speaker.PlaybackState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.transport[coordinator] = state
}

// SetNowPlaying setzt simulierte Now-Playing-Daten (fuer Tests).
func (c *Controller) SetNowPlaying(name string, info speaker.NowPlayingInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	info.Speaker = name
	c.nowPlaying[name] = info
	c.media[name] = info
	if info.State != "" {
		c.transport[name] = info.State
	}
}

func (c *Controller) GetTransportState(_ context.Context, coordinator string) (speaker.PlaybackState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.reachable[coordinator] {
		return speaker.PlaybackUnknown, c.unreachable(coordinator)
	}
	if st, ok := c.transport[coordinator]; ok {
		return st, nil
	}
	return speaker.PlaybackStopped, nil
}

func (c *Controller) GetNowPlaying(_ context.Context, name string) (speaker.NowPlayingInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.reachable[name] {
		return speaker.NowPlayingInfo{}, c.unreachable(name)
	}
	if np, ok := c.nowPlaying[name]; ok {
		np.Speaker = name
		return np, nil
	}
	st := speaker.PlaybackStopped
	if s, ok := c.transport[name]; ok {
		st = s
	}
	return speaker.NowPlayingInfo{Speaker: name, State: st}, nil
}

// SetReachable schaltet die Erreichbarkeit eines Lautsprechers (fuer Tests).
func (c *Controller) SetReachable(name string, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reachable[name] = ok
}

func (c *Controller) unreachable(name string) *apperr.Error {
	return apperr.New(apperr.CodeSpeakerUnreachable,
		"Speaker is not responding").
		WithSpeaker(name).
		WithHint("Is the speaker powered on and on the same network? Check IP/discovery.")
}

func (c *Controller) Discover(_ context.Context) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var names []string
	for _, sp := range c.discovery {
		if c.reachable[sp.Name] {
			names = append(names, sp.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (c *Controller) ListSpeakers(_ context.Context) ([]speaker.SpeakerInfo, error) {
	return c.speakerList(), nil
}

func (c *Controller) RefreshSpeakers(_ context.Context) ([]speaker.SpeakerInfo, error) {
	return c.speakerList(), nil
}

func (c *Controller) speakerList() []speaker.SpeakerInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]speaker.SpeakerInfo, len(c.discovery))
	for i, sp := range c.discovery {
		out[i] = sp
		out[i].Reachable = c.reachable[sp.Name]
	}
	return out
}

func (c *Controller) EnsureSpeakers(_ context.Context, names []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	known := make(map[string]bool, len(c.discovery))
	var foundNames []string
	for _, sp := range c.discovery {
		known[sp.Name] = true
		foundNames = append(foundNames, sp.Name)
	}
	sort.Strings(foundNames)

	var missing []string
	for _, name := range names {
		if !known[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)

	return apperr.New(apperr.CodeSceneMisconfigured,
		"Scene contains unknown speaker names").
		WithHint(fmt.Sprintf("Found: %s. Missing: %s",
			strings.Join(foundNames, ", "),
			strings.Join(missing, ", ")))
}

func (c *Controller) Group(_ context.Context, coordinator string, members []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.reachable[coordinator] {
		return c.unreachable(coordinator)
	}
	for _, m := range members {
		if !c.reachable[m] {
			return apperr.New(apperr.CodeGroupingFailed,
				"Speaker could not be grouped").
				WithSpeaker(m).
				WithHint("Speaker unreachable -- grouping skipped.")
		}
		// Wie Sonos: x-rincon auf eine Box, die selbst nur Mitglied einer
		// Gruppe ist, scheitert mit UPnP 800.
		if co, member := c.coordinatorOf[coordinator]; member && co != coordinator && m != coordinator {
			return apperr.New(apperr.CodeGroupingFailed,
				"Speaker could not be grouped").
				WithSpeaker(m).
				WithHint("Device rejected SetAVTransportURI (HTTP 500): UPnP error code 800")
		}
	}
	c.Groups[coordinator] = append([]string(nil), members...)
	for _, m := range members {
		if m != coordinator {
			c.coordinatorOf[m] = coordinator
		}
	}
	delete(c.coordinatorOf, coordinator) // ein Coordinator fuehrt seine eigene Gruppe
	return nil
}

func (c *Controller) SetVolume(_ context.Context, name string, volume int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.reachable[name] {
		return c.unreachable(name)
	}
	c.Volumes[name] = volume
	return nil
}

func (c *Controller) Play(ctx context.Context, coordinator string, src speaker.Source) error {
	var service *speaker.FavoriteEntry
	if src.Type == speaker.SourceFavorite {
		favs, err := c.BrowseFavorites(ctx, coordinator)
		if err != nil {
			return err
		}
		for i, f := range favs {
			if !strings.EqualFold(f.Title, src.Name) {
				continue
			}
			if f.Integratable {
				src = speaker.Source{Type: speaker.SourceStream, URI: f.URI}
			} else if f.Playable {
				service = &favs[i]
			}
			break
		}
		if src.Type == speaker.SourceFavorite && service == nil {
			return apperr.New(apperr.CodePlaybackFailed,
				"Favorite not playable or not found").
				WithSpeaker(coordinator).
				WithHint("Check importable favorites via GET /api/v1/sonos/favorites.")
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.reachable[coordinator] {
		return c.unreachable(coordinator)
	}
	c.Plays = append(c.Plays, PlayCall{Coordinator: coordinator, Source: src})
	c.transport[coordinator] = speaker.PlaybackPlaying
	if service != nil {
		// Dienst-Favorit (TuneIn): Track ist der Pre-Roll, Quelle der Favorit.
		c.nowPlaying[coordinator] = speaker.NowPlayingInfo{
			State:       speaker.PlaybackPlaying,
			URI:         mockPreRollURI,
			Title:       service.Title,
			SourceURI:   service.URI,
			SourceTitle: service.Title,
		}
		c.media[coordinator] = c.nowPlaying[coordinator]
		return nil
	}
	uri := src.URI
	if strings.HasPrefix(uri, "https://") {
		uri = "x-rincon-mp3radio://" + strings.TrimPrefix(uri, "https://")
	} else if strings.HasPrefix(uri, "http://") {
		uri = "x-rincon-mp3radio://" + strings.TrimPrefix(uri, "http://")
	}
	c.nowPlaying[coordinator] = speaker.NowPlayingInfo{
		State: speaker.PlaybackPlaying,
		URI:   uri,
		Title: src.Name,
	}
	c.media[coordinator] = c.nowPlaying[coordinator]
	return nil
}

// Stop simuliert AVTransport Stop: Zustand stopped, Now-Playing geleert.
func (c *Controller) Stop(_ context.Context, coordinator string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.reachable[coordinator] {
		return c.unreachable(coordinator)
	}
	c.Stops = append(c.Stops, coordinator)
	c.transport[coordinator] = speaker.PlaybackStopped
	delete(c.nowPlaying, coordinator)
	return nil
}

// Pause simuliert AVTransport Pause: Zustand paused, Metadaten bleiben.
func (c *Controller) Pause(_ context.Context, coordinator string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.reachable[coordinator] {
		return c.unreachable(coordinator)
	}
	c.Pauses = append(c.Pauses, coordinator)
	c.transport[coordinator] = speaker.PlaybackPaused
	if np, ok := c.nowPlaying[coordinator]; ok {
		np.State = speaker.PlaybackPaused
		c.nowPlaying[coordinator] = np
	}
	return nil
}

// Resume simuliert AVTransport Play ohne neue URI: setzt die zuletzt
// geladene Quelle fort. Ohne Quelle lehnt die Box ab (wie Sonos mit 701).
func (c *Controller) Resume(_ context.Context, coordinator string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.reachable[coordinator] {
		return c.unreachable(coordinator)
	}
	np, ok := c.media[coordinator]
	if !ok {
		return apperr.New(apperr.CodePlaybackFailed,
			"Speaker cannot resume (no source loaded)").
			WithSpeaker(coordinator)
	}
	c.Resumes = append(c.Resumes, coordinator)
	np.Speaker = coordinator
	np.State = speaker.PlaybackPlaying
	c.nowPlaying[coordinator] = np
	c.transport[coordinator] = speaker.PlaybackPlaying
	return nil
}

// RejectUngroup laesst Ungroup auf name scheitern, obwohl die Box
// erreichbar ist (fuer Tests: Box lehnt ab).
func (c *Controller) RejectUngroup(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ungroupRejects == nil {
		c.ungroupRejects = map[string]bool{}
	}
	c.ungroupRejects[name] = true
}

// SetGroup legt eine Gruppe direkt fest, ohne Group-Aufruf zu protokollieren
// (fuer Tests: "in der Sonos-App gruppiert").
func (c *Controller) SetGroup(coordinator string, members ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.coordinatorOf, coordinator)
	for _, m := range members {
		if m != coordinator {
			c.coordinatorOf[m] = coordinator
		}
	}
}

// GetGroups baut die Topologie aus den erreichbaren Discovery-Boxen.
// Coordinator steht vorne, die uebrigen Mitglieder sortiert.
func (c *Controller) GetGroups(_ context.Context) ([]speaker.GroupInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	byCoord := map[string][]string{}
	for _, sp := range c.discovery {
		if !c.reachable[sp.Name] {
			continue
		}
		coord := sp.Name
		if co, ok := c.coordinatorOf[sp.Name]; ok && c.reachable[co] {
			coord = co
		}
		if coord != sp.Name {
			byCoord[coord] = append(byCoord[coord], sp.Name)
		} else if _, ok := byCoord[coord]; !ok {
			byCoord[coord] = nil
		}
	}
	if len(byCoord) == 0 && len(c.discovery) > 0 {
		return nil, apperr.New(apperr.CodeSpeakerUnreachable,
			"no speaker reachable for the group query")
	}
	out := make([]speaker.GroupInfo, 0, len(byCoord))
	for coord, members := range byCoord {
		sort.Strings(members)
		out = append(out, speaker.GroupInfo{
			Coordinator: coord,
			Members:     append([]string{coord}, members...),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Coordinator < out[j].Coordinator })
	return out, nil
}

// Ungroup simuliert BecomeCoordinatorOfStandaloneGroup: die Box steht danach
// allein und gestoppt. Verlaesst ein Coordinator die Gruppe, uebernimmt das
// alphabetisch erste verbleibende Mitglied (und spielt weiter).
func (c *Controller) Ungroup(_ context.Context, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.reachable[name] {
		return c.unreachable(name)
	}
	if c.ungroupRejects[name] {
		return apperr.New(apperr.CodePlaybackFailed,
			"Device rejected BecomeCoordinatorOfStandaloneGroup").
			WithSpeaker(name)
	}
	c.Ungroups = append(c.Ungroups, name)
	if _, member := c.coordinatorOf[name]; member {
		delete(c.coordinatorOf, name)
		c.transport[name] = speaker.PlaybackStopped
		delete(c.nowPlaying, name)
		return nil
	}
	var rest []string
	for m, co := range c.coordinatorOf {
		if co == name {
			rest = append(rest, m)
		}
	}
	if len(rest) == 0 {
		return nil
	}
	sort.Strings(rest)
	heir := rest[0]
	delete(c.coordinatorOf, heir)
	for _, m := range rest[1:] {
		c.coordinatorOf[m] = heir
	}
	if st, ok := c.transport[name]; ok {
		c.transport[heir] = st
	}
	if np, ok := c.nowPlaying[name]; ok {
		np.Speaker = heir
		c.nowPlaying[heir] = np
		c.media[heir] = np
	}
	c.transport[name] = speaker.PlaybackStopped
	delete(c.nowPlaying, name)
	return nil
}
