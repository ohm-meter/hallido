// Package sonos implementiert den SpeakerController gegen echte Sonos-Geraete
// ueber deren lokale UPnP/SOAP-Schnittstelle -- vollstaendig im LAN, ohne
// Sonos-Cloud.
//
// Steuerung laeuft je Geraet (Port 1400) ueber zwei UPnP-Services:
//   - RenderingControl: Lautstaerke (SetVolume)
//   - AVTransport:      Wiedergabe (SetAVTransportURI + Play) und Gruppierung
//
// Gruppierung in Sonos funktioniert, indem die AVTransport-Quelle eines
// Mitglieds auf "x-rincon:<coordinator-uid>" gesetzt wird; das Mitglied folgt
// dann dem Coordinator.
//
// Geraete werden per SSDP entdeckt; alternativ liefern feste IPs aus der
// Config die Geraete, falls Multicast im Netz nicht zuverlaessig ist.
package sonos

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

// device ist ein entdeckter Sonos-Lautsprecher.
type device struct {
	name    string // Raumname (wie in der Sonos-App)
	baseURL string // http://IP:1400
	uid     string // RINCON_... (fuer Gruppierung via x-rincon)
}

// Controller steuert Sonos-Geraete. Alle Methoden sind nebenlaeufig sicher.
type Controller struct {
	client     *http.Client
	retryPause time.Duration
	log        *slog.Logger

	// prober ermittelt vor dem Abspielen das Stream-Format (nil = keine Probe).
	prober *streamProber
	// playVerify* steuern die Pruefung nach Play: bis playVerifyWindow muss
	// der Coordinator PLAYING melden; STOPPED gilt erst nach playVerifyGrace
	// (oder nach TRANSITIONING) als Fehlschlag.
	playVerifyWindow   time.Duration
	playVerifyInterval time.Duration
	playVerifyGrace    time.Duration
	// favoriteVerifyWindow: wie playVerifyWindow, fuer Dienst-Favoriten (TuneIn
	// braucht 3-4 s bis PLAYING).
	favoriteVerifyWindow time.Duration
	overrides            map[string]string // Raumname -> feste IP (aus Config)

	discMu  sync.Mutex // serialisiert volle Discovery-Laeufe (resolve-Stuerme)
	ovMu    sync.RWMutex
	mu      sync.RWMutex
	devices map[string]*device // Raumname -> device

	offMu   sync.Mutex
	offline map[string]bool // Raumname -> Fixed-IP-Probe zuletzt fehlgeschlagen

	// failed merkt Boxen, deren letzte Probe fehlschlug (offline, oder die
	// Config-IP zeigt auf einen unsichtbaren Paar-Partner/Satelliten). Nie
	// geprobte Boxen fehlen hier und gelten weiter als erreichbar (lazy
	// Discovery vertraut der Config). Geschuetzt durch mu.
	failed map[string]bool

	bonded map[string]bool // Raumnamen, die Discovery als gebondet/unsichtbar aussortiert hat

	// hiddenSet/hiddenAt: kurzlebiger Cache der unsichtbaren UUIDs aus der
	// Topologie fuer den Pfad pro Box. Geschuetzt durch mu.
	hiddenSet map[string]bool
	hiddenAt  time.Time
}

// Sicherstellen, dass der Controller die Interfaces erfuellt.
var (
	_ speaker.SpeakerController = (*Controller)(nil)
	_ speaker.FixedIPReloader   = (*Controller)(nil)
)

// New erstellt den Sonos-Controller. overrides bildet Raumnamen auf feste IPs
// ab (leer = reine SSDP-Discovery).
func New(overrides map[string]string, log *slog.Logger) *Controller {
	return &Controller{
		client:     &http.Client{Timeout: soapTimeout},
		retryPause: retryPause,
		log:        log,
		overrides:  cloneOverrides(overrides),
		devices:    map[string]*device{},
		offline:    map[string]bool{},
		failed:     map[string]bool{},

		prober:             newStreamProber(),
		playVerifyWindow:   playVerifyWindow,
		playVerifyInterval: playVerifyInterval,
		playVerifyGrace:    playVerifyGrace,

		favoriteVerifyWindow: favoriteVerifyWindow,
	}
}

// noteProbeFailed loggt den Wechsel online -> offline einer Fixed-IP-Box als
// WARN; weitere Fehlschlaege derselben Box nur als DEBUG (kein Polling-Rauschen).
func (c *Controller) noteProbeFailed(name, ip string, err error) {
	c.offMu.Lock()
	already := c.offline[name]
	c.offline[name] = true
	c.offMu.Unlock()
	if already {
		c.log.Debug("fixed-ip speaker still unreachable", "speaker", name, "ip", ip, "err", err)
		return
	}
	c.log.Warn("fixed-ip speaker unreachable", "speaker", name, "ip", ip, "err", err)
}

// noteProbeOK loggt den Wechsel offline -> online einmalig als INFO.
func (c *Controller) noteProbeOK(name, ip string) {
	c.offMu.Lock()
	was := c.offline[name]
	delete(c.offline, name)
	c.offMu.Unlock()
	if was {
		c.log.Info("fixed-ip speaker reachable again", "speaker", name, "ip", ip)
	}
}

// ReloadFixedIPs ersetzt feste IPs aus der Config und leert den Geraete-Cache,
// damit die naechste Discovery die neuen Adressen nutzt.
func (c *Controller) ReloadFixedIPs(overrides map[string]string) {
	c.ovMu.Lock()
	c.overrides = cloneOverrides(overrides)
	c.ovMu.Unlock()
	c.mu.Lock()
	c.devices = map[string]*device{}
	c.failed = map[string]bool{}
	c.hiddenSet = nil
	c.mu.Unlock()
	c.offMu.Lock()
	c.offline = map[string]bool{}
	c.offMu.Unlock()
}

func cloneOverrides(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (c *Controller) fixedOverrides() map[string]string {
	c.ovMu.RLock()
	defer c.ovMu.RUnlock()
	return c.overrides
}

// Discover aktualisiert die Geraeteliste (feste IPs zuerst, dann SSDP) und
// liefert die gefundenen Raumnamen sortiert zurueck.
func (c *Controller) Discover(ctx context.Context) ([]string, error) {
	names, err := c.refreshDevices(ctx)
	if err != nil {
		return nil, err
	}
	return names, nil
}

// ListSpeakers liefert bekannte Boxen ohne SSDP (Cache + feste Config-IPs).
func (c *Controller) ListSpeakers(_ context.Context) ([]speaker.SpeakerInfo, error) {
	infos := c.buildSpeakerInfos()
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

// RefreshSpeakers aktualisiert per SSDP/Fixed-IP und liefert die Boxenliste.
func (c *Controller) RefreshSpeakers(ctx context.Context) ([]speaker.SpeakerInfo, error) {
	_, err := c.refreshDevices(ctx)
	infos := c.buildSpeakerInfos()
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, err
}

// buildSpeakerInfos merged Discovery-Cache und Config-Overrides (ohne Netzwerk).
func (c *Controller) buildSpeakerInfos() []speaker.SpeakerInfo {
	c.mu.RLock()
	infos := make([]speaker.SpeakerInfo, 0, len(c.devices))
	seen := make(map[string]bool, len(c.devices))
	for _, d := range c.devices {
		infos = append(infos, speaker.SpeakerInfo{
			Name:      d.name,
			IP:        ipFromBaseURL(d.baseURL),
			UID:       d.uid,
			Reachable: !c.failed[d.name],
		})
		seen[d.name] = true
	}
	defer c.mu.RUnlock()

	for name, ip := range c.fixedOverrides() {
		if seen[name] {
			continue
		}
		infos = append(infos, speaker.SpeakerInfo{
			Name:      name,
			IP:        ip,
			Reachable: ip != "" && !c.failed[name],
		})
	}
	return infos
}

// refreshDevices aktualisiert c.devices und liefert sortierte Namen.
func (c *Controller) refreshDevices(ctx context.Context) ([]string, error) {
	// 1. Feste IPs parallel mit kurzem Timeout — blockieren SSDP nicht.
	found := c.probeFixedIPs(ctx)

	// 2. SSDP-Discovery fuer alle uebrigen Boxen (unabhaengig vom Fixed-IP-Ergebnis).
	locations, err := runSSDPSearch(ssdpSearchTimeout)
	if err != nil {
		c.log.Warn("ssdp discovery failed", "err", err)
	}
	for name, d := range fetchDevicesFromLocations(ctx, c.client, locations) {
		if _, exists := found[name]; !exists {
			found[name] = d
		}
	}

	c.filterBondedInvisible(ctx, found)

	// Ein leeres Ergebnis (z. B. flackerndes Multicast) darf den Cache nicht
	// leeren; sonst gilt jede bekannte Box bis zum naechsten Erfolg als fehlend.
	// Gefundene Boxen ueberschreiben, bisher bekannte bleiben erhalten.
	c.mu.Lock()
	for name, d := range found {
		c.devices[name] = d
	}
	// Probe-Ergebnis fuer jede Config-IP festhalten: nicht aufgeloest (Probe
	// fehlgeschlagen oder als unsichtbar gefiltert) = nicht erreichbar.
	for name, ip := range c.fixedOverrides() {
		if ip == "" {
			continue
		}
		if _, ok := found[name]; ok {
			delete(c.failed, name)
		} else {
			c.failed[name] = true
		}
	}
	names := make([]string, 0, len(c.devices))
	for n := range c.devices {
		names = append(names, n)
	}
	c.mu.Unlock()
	sort.Strings(names)

	if len(found) == 0 {
		return nil, apperr.New(apperr.CodeSpeakerUnreachable,
			"No Sonos speakers found").
			WithHint("SSDP multicast may be blocked -- add fixed IPs under \"speakers\" in the config.")
	}
	return names, nil
}

// EnsureSpeakers aktualisiert nur die genannten Boxen und prueft deren Presence.
// "Unbekannt" (scene_misconfigured) sind nur Namen, die weder in Discovery noch
// als Config-IP existieren. Eine konfigurierte, aber offline Box ist bekannt:
// sie scheitert spaeter pro Box (Teilausfall), nicht die ganze Szene.
func (c *Controller) EnsureSpeakers(ctx context.Context, names []string) error {
	if err := c.refreshDevicesFor(ctx, names); err != nil {
		return err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	overrides := c.fixedOverrides()
	var missing []string
	for _, name := range names {
		if _, ok := c.devices[name]; ok {
			continue
		}
		if ip, ok := overrides[name]; ok && ip != "" && !c.bonded[name] {
			continue // in Config bekannt, nur offline (gebondete Partner bleiben ein Konfigurationsfehler)
		}
		missing = append(missing, name)
	}
	if len(missing) == 0 {
		return nil
	}

	foundNames := make([]string, 0, len(c.devices))
	for name := range c.devices {
		foundNames = append(foundNames, name)
	}
	sort.Strings(foundNames)
	sort.Strings(missing)

	hint := fmt.Sprintf("Found: %s. Missing: %s",
		strings.Join(foundNames, ", "),
		strings.Join(missing, ", "))
	for _, name := range missing {
		if c.bonded[name] {
			hint += ". " + name + ": " + hintBonded
		}
	}
	return apperr.New(apperr.CodeSceneMisconfigured,
		"Scene contains unknown speaker names").
		WithHint(hint)
}

// refreshDevicesFor aktualisiert nur die genannten Boxen in c.devices.
// Feste Config-IPs zuerst; fehlende Namen danach per SSDP.
func (c *Controller) refreshDevicesFor(ctx context.Context, names []string) error {
	want := make(map[string]bool, len(names))
	for _, name := range names {
		want[name] = true
	}

	overrides := c.fixedOverrides()
	var needSSDP []string
	probed := map[string]*device{}
	for _, name := range names {
		if ip, ok := overrides[name]; ok && ip != "" {
			probeCtx, cancel := context.WithTimeout(ctx, fixedIPProbeTimeout)
			d, err := fetchDevice(probeCtx, c.client, baseURLFromIP(ip))
			cancel()
			if err != nil {
				c.noteProbeFailed(name, ip, err)
				c.mu.Lock()
				c.failed[name] = true
				c.mu.Unlock()
				needSSDP = append(needSSDP, name)
				continue
			}
			c.noteProbeOK(name, ip)
			d.name = name
			probed[name] = d
			continue
		}

		c.mu.RLock()
		_, cached := c.devices[name]
		c.mu.RUnlock()
		if !cached {
			needSSDP = append(needSSDP, name)
		}
	}

	// Config-IPs koennen auf unsichtbare Paar-Partner/Satelliten zeigen; die
	// duerfen nie als Device uebernommen (und damit nie gruppiert) werden.
	c.filterBonded(ctx, probed, true)
	c.mu.Lock()
	for name := range overrides {
		if !want[name] || overrides[name] == "" {
			continue
		}
		if d, ok := probed[name]; ok {
			c.devices[name] = d
			delete(c.failed, name)
		} else if c.bonded[name] {
			delete(c.devices, name)
			c.failed[name] = true
		}
	}
	c.mu.Unlock()

	if len(needSSDP) == 0 {
		return nil
	}

	locations, err := runSSDPSearch(ssdpSearchTimeout)
	if err != nil {
		c.log.Warn("ssdp discovery failed", "err", err)
	}
	for name, d := range fetchDevicesFromLocations(ctx, c.client, locations) {
		if !want[name] {
			continue
		}
		c.mu.Lock()
		c.devices[name] = d
		delete(c.failed, name)
		c.mu.Unlock()
	}
	return nil
}

func ipFromBaseURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// resolve liefert ein device per Name. Ist es unbekannt, wird einmal neu
// entdeckt, bevor aufgegeben wird (Boxen koennen verzoegert ins Netz kommen).
func (c *Controller) resolve(ctx context.Context, name string) (*device, error) {
	c.mu.RLock()
	d, ok := c.devices[name]
	c.mu.RUnlock()
	if ok {
		return d, nil
	}

	c.discMu.Lock()
	// Ein paralleler resolve hat die Box evtl. gerade entdeckt.
	c.mu.RLock()
	d, ok = c.devices[name]
	c.mu.RUnlock()
	if ok {
		c.discMu.Unlock()
		return d, nil
	}
	_, derr := c.Discover(ctx)
	c.discMu.Unlock()

	c.mu.RLock()
	d, ok = c.devices[name]
	c.mu.RUnlock()
	if !ok && derr != nil {
		return nil, derr
	}
	if !ok {
		hint := "Name exactly as in the Sonos app? Speaker online?"
		c.mu.RLock()
		wasBonded := c.bonded[name]
		c.mu.RUnlock()
		if wasBonded {
			hint = hintBonded
		} else if ip, ok := c.fixedOverrides()[name]; ok && ip != "" {
			hint += fmt.Sprintf(" Fixed IP %s returns no device description.", ip)
		} else {
			hint += " If needed, add a fixed IP under \"speakers\" in the config."
		}
		return nil, apperr.New(apperr.CodeSpeakerUnreachable,
			"Speaker not in discovery").
			WithSpeaker(name).
			WithHint(hint)
	}
	return d, nil
}

// SetVolume setzt die absolute Lautstaerke (0-100) eines Lautsprechers.
func (c *Controller) SetVolume(ctx context.Context, name string, volume int) error {
	d, err := c.resolve(ctx, name)
	if err != nil {
		return err
	}
	_, err = c.soapCall(ctx, d, renderingControl, "SetVolume", []arg{
		{"InstanceID", "0"},
		{"Channel", "Master"},
		{"DesiredVolume", strconv.Itoa(volume)},
	})
	return err
}

// Group laesst members synchron mit coordinator spielen.
func (c *Controller) Group(ctx context.Context, coordinator string, members []string) error {
	coord, err := c.resolve(ctx, coordinator)
	if err != nil {
		return err
	}
	for _, m := range members {
		md, err := c.resolve(ctx, m)
		if err != nil {
			return err
		}
		if _, err := c.soapCall(ctx, md, avTransport, "SetAVTransportURI", []arg{
			{"InstanceID", "0"},
			{"CurrentURI", "x-rincon:" + coord.uid},
			{"CurrentURIMetaData", ""},
		}); err != nil {
			return apperr.New(apperr.CodeGroupingFailed,
				"Speaker could not be grouped").
				WithSpeaker(m).
				WithHint(c.groupFailureHint(ctx, coord, md, err))
		}
	}
	return nil
}

const hintBonded = "Speaker is bonded/invisible (surround or stereo pair partner) and cannot be controlled individually — " +
	"change the config IP to the visible player or remove the entry. The other speakers keep playing."

// groupFailureHint unterscheidet die Ursache eines fehlgeschlagenen
// Gruppierens: Box nicht erreichbar (aus/Netzwerk), Box gebondet/unsichtbar
// oder SOAP-Ablehnung durch ein erreichbares, sichtbares Geraet.
func (c *Controller) groupFailureHint(ctx context.Context, coord, member *device, cause error) string {
	ae := apperr.As(cause)
	if ae.Code == apperr.CodeSpeakerUnreachable {
		hint := "Speaker unreachable (powered off or network)."
		if ae.Hint != "" {
			hint += " " + ae.Hint
		}
		return hint + " The other speakers keep playing."
	}
	if hidden := c.hiddenUUIDsViaTopology(ctx, map[string]*device{coord.name: coord}); hidden[member.uid] {
		return hintBonded
	}
	if strings.Contains(ae.Message, "UPnP error code 800") {
		return fmt.Sprintf("Speaker rejected the grouping (%s): %s is currently only a member of another group "+
			"and cannot take on speakers. The other speakers keep playing.", ae.Message, coord.name)
	}
	return fmt.Sprintf("Speaker reachable but rejected the grouping (%s). "+
		"Possibly bonded/stereo pair partner or without Wi-Fi grouping capability (e.g. Bluetooth mode) — "+
		"check config IP/room name. The other speakers keep playing.", ae.Message)
}

// Play startet src auf coordinator; die gesamte Gruppe folgt.
func (c *Controller) Play(ctx context.Context, coordinator string, src speaker.Source) error {
	coord, err := c.resolve(ctx, coordinator)
	if err != nil {
		return err
	}

	switch src.Type {
	case speaker.SourceStream:
		return c.setURIAndPlay(ctx, coord, src.URI)

	case speaker.SourceFavorite:
		return c.playFavorite(ctx, coord, src.Name)

	default:
		return apperr.New(apperr.CodeBadRequest,
			"unknown source type: "+string(src.Type)).
			WithSpeaker(coordinator)
	}
}

// soapTimeout ist das HTTP-Zeitlimit pro SOAP-Aufruf. Direkt nach einem
// Agent-Neustart (kalte Verbindung, Pi unter Last) braucht SetAVTransportURI
// gelegentlich mehr als 5 s (Issue #9).
const soapTimeout = 10 * time.Second

// retryPause ist die Wartezeit vor der einmaligen Wiederholung eines
// zeitueberschrittenen Aufrufs.
const retryPause = 300 * time.Millisecond

// verifyTimeout begrenzt eine einzelne Statusabfrage nach Play.
const verifyTimeout = 3 * time.Second

// Pruefung nach Play: so lange darf der Coordinator brauchen, um PLAYING zu
// melden (auf echter Hardware 1-4 s fuer funktionierende Streams), in diesem
// Takt wird gefragt, und erst nach playVerifyGrace gilt STOPPED als Fehlschlag.
const (
	playVerifyWindow   = 6 * time.Second
	playVerifyInterval = 500 * time.Millisecond
	playVerifyGrace    = 1500 * time.Millisecond

	favoriteVerifyWindow = 10 * time.Second
)

// soapRetry ruft idempotente Aktionen (SetAVTransportURI, Play) hoechstens
// zweimal auf, wenn die Box nicht antwortet. Abgelehnte Aktionen (HTTP 500)
// und abgelaufene Contexts werden nicht wiederholt.
func (c *Controller) soapRetry(ctx context.Context, d *device, svc service, action string, args []arg) error {
	_, err := c.soapCall(ctx, d, svc, action, args)
	if err == nil || ctx.Err() != nil || apperr.As(err).Code != apperr.CodeSpeakerUnreachable {
		return err
	}
	c.log.Warn("soap timeout, retrying once", "speaker", d.name, "action", action)
	select {
	case <-ctx.Done():
		return err
	case <-time.After(c.retryPause):
	}
	_, err = c.soapCall(ctx, d, svc, action, args)
	return err
}

// setURIAndPlay setzt eine Stream-URI (als Sonos-Radio) auf dem Coordinator
// und startet die Wiedergabe. Vorher ermittelt eine kurze Probe das
// Stream-Format (MP3 -> x-rincon-mp3radio, AAC -> aac://); ein Stream, der mit
// HTTP-Fehler antwortet, wird gar nicht erst an die Box gegeben.
//
// SetAVTransportURI und Play werden bei Zeitueberschreitung einmal
// wiederholt; scheitert auch Play danach, zaehlt ein Folgeabruf des
// Transportstatus: spielt die Box bereits, war der Befehl angekommen und nur
// die Antwort zu spaet. Nach erfolgreichem Play muss der Coordinator binnen
// playVerifyWindow PLAYING melden, sonst gilt die Wiedergabe als gescheitert
// (Sonos quittiert Play auch fuer Streams, die es nicht abspielen kann).
func (c *Controller) setURIAndPlay(ctx context.Context, coord *device, rawURI string) error {
	info := c.probeStream(ctx, rawURI)
	if info.status != 0 && !info.reachable() {
		return apperr.New(apperr.CodePlaybackFailed, "Stream unreachable").
			WithSpeaker(coord.name).
			WithHint(fmt.Sprintf("Stream responds with HTTP %d — check the URL in a browser/VLC or choose another endpoint of the station.", info.status))
	}
	transportURI, metaData := prepareStreamPlayback(rawURI, info.kind)
	c.log.Debug("stream prepared", "speaker", coord.name, "uri", rawURI,
		"transport_uri", transportURI, "content_type", info.contentType, "kind", string(info.kind))

	if err := c.soapRetry(ctx, coord, avTransport, "SetAVTransportURI", []arg{
		{"InstanceID", "0"},
		{"CurrentURI", transportURI},
		{"CurrentURIMetaData", metaData},
	}); err != nil {
		return err
	}
	err := c.soapRetry(ctx, coord, avTransport, "Play", []arg{
		{"InstanceID", "0"},
		{"Speed", "1"},
	})
	if err != nil && apperr.As(err).Code == apperr.CodeSpeakerUnreachable && c.isPlaying(ctx, coord) {
		c.log.Warn("play without response, speaker is playing though", "speaker", coord.name)
		return nil
	}
	if err != nil {
		return err
	}
	return c.verifyPlaying(ctx, coord, rawURI, info)
}

// probeStream fragt das Stream-Format nur fuer http(s)-URLs ab.
func (c *Controller) probeStream(ctx context.Context, rawURI string) streamInfo {
	if c.prober == nil || !isHTTPURL(rawURI) {
		return streamInfo{}
	}
	info := c.prober.probe(ctx, rawURI)
	if info.err != nil {
		c.log.Debug("stream probe without response", "uri", rawURI, "err", info.err)
	}
	return info
}

// verifyPlaying prueft nach Play, ob der Coordinator PLAYING meldet. Bleibt
// er gestoppt (oder haengt er in TRANSITIONING), liefert es playback_failed
// mit Hinweis auf Format/Erreichbarkeit des Streams.
func (c *Controller) verifyPlaying(ctx context.Context, coord *device, rawURI string, info streamInfo) error {
	state, started := c.waitPlaying(ctx, coord, c.playVerifyWindow)
	if started {
		return nil
	}
	return c.playNotStarted(ctx, coord, rawURI, info, state)
}

// waitPlaying pollt den Transportstatus des Coordinators bis zu window lang,
// bis er PLAYING meldet (started). STOPPED gilt erst nach playVerifyGrace oder
// nach TRANSITIONING als Fehlschlag; state ist der letzte gelesene Status. Ist
// der Status gar nicht lesbar, gilt Play als erfolgreich (wie bisher).
func (c *Controller) waitPlaying(ctx context.Context, coord *device, window time.Duration) (state string, started bool) {
	start := time.Now()
	deadline := start.Add(window)
	last := ""
	sawTransition := false
	for {
		vctx, cancel := context.WithTimeout(ctx, verifyTimeout)
		body, err := c.soapCall(vctx, coord, avTransport, "GetTransportInfo", []arg{{"InstanceID", "0"}})
		cancel()
		if err == nil {
			state := strings.ToUpper(strings.TrimSpace(extractTransportState(body)))
			if state != "" {
				last = state
			}
			switch state {
			case "PLAYING":
				return state, true
			case "TRANSITIONING":
				sawTransition = true
			case "STOPPED", "NO_MEDIA_PRESENT", "PAUSED_PLAYBACK":
				if sawTransition || time.Since(start) >= c.playVerifyGrace {
					return state, false
				}
			}
		}
		if !time.Now().Before(deadline) {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(c.playVerifyInterval):
		}
		if ctx.Err() != nil {
			break // Aufrufer bricht ab: mit letztem Stand entscheiden
		}
	}
	if last == "" {
		c.log.Warn("playback not verifiable: transport state unreadable", "speaker", coord.name)
		return "", true
	}
	return last, false
}

// playNotStarted baut den Fehler fuer eine Wiedergabe, die nach Play nicht
// anlaeuft, inkl. erkanntem Content-Type und ggf. Sonos-Sperre des Senders.
func (c *Controller) playNotStarted(ctx context.Context, coord *device, rawURI string, info streamInfo, state string) error {
	secs := c.playVerifyWindow.Seconds()
	hint := "Stream format not supported by Sonos or stream unreachable for the speaker"
	var details []string
	if info.contentType != "" || info.status != 0 || info.err != nil {
		details = append(details, info.describe())
	}
	details = append(details, fmt.Sprintf("speaker state: %s", state))
	hint += " (" + strings.Join(details, ", ") + ")."
	if state == "TRANSITIONING" {
		hint += fmt.Sprintf(" Speaker still buffering after %.0f s — stream may be very slow.", secs)
	}
	switch {
	case info.kind == kindHLS:
		hint += " Sonos does not play HLS playlists (m3u8) this way — use the MP3/AAC endpoint of the station."
	case c.prober != nil && info.reachable() && c.prober.blocksSonos(context.WithoutCancel(ctx), rawURI):
		hint += " The station does not answer requests from Sonos devices (block on the Sonos user agent) — " +
			"save the station as a favorite (TuneIn) in the Sonos app and create a favorite scene " +
			"(GUI: New scene → From Sonos favorites) or choose another stream endpoint."
	default:
		hint += " Check the stream URL in a browser/VLC; if needed choose another (MP3) endpoint of the station."
	}
	c.log.Warn("playback does not start", "speaker", coord.name, "uri", rawURI,
		"state", state, "content_type", info.contentType, "kind", string(info.kind))
	return apperr.New(apperr.CodePlaybackFailed, "Speaker does not start playback").
		WithSpeaker(coord.name).
		WithHint(hint)
}

// isPlaying fragt den Transportstatus mit kurzem, vom Aufrufer unabhaengigem
// Zeitlimit ab (der Play-Context kann bereits knapp sein).
func (c *Controller) isPlaying(ctx context.Context, d *device) bool {
	vctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), verifyTimeout)
	defer cancel()
	body, err := c.soapCall(vctx, d, avTransport, "GetTransportInfo", []arg{{"InstanceID", "0"}})
	if err != nil {
		return false
	}
	return parseTransportState(body) == speaker.PlaybackPlaying
}
