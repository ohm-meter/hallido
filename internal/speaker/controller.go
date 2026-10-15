// Package speaker definiert die Abstraktion ueber steuerbaren
// Lautsprechern. Hinter dem SpeakerController-Interface steckt heute der
// Sonos-UPnP-Adapter (Inkrement 2) und der Mock (fuer Tests); kuenftig
// koennten andere Marken folgen, ohne den Rest der Anwendung zu beruehren.
package speaker

import "context"

// SourceType unterscheidet, wie eine Audioquelle adressiert wird.
type SourceType string

const (
	// SourceStream: direkte Stream-URL (z.B. ORF/FM4), kein App-Setup noetig.
	SourceStream SourceType = "stream"
	// SourceFavorite: ein in der Sonos-App gespeicherter Favorit per Name
	// (z. B. TuneIn-Sender, ADR 0018).
	SourceFavorite SourceType = "favorite"
)

// Source beschreibt eine abspielbare Audioquelle.
type Source struct {
	Type SourceType `json:"type"`
	// URI: bei SourceStream die Stream-URL; bei SourceFavorite optional die
	// Favoriten-URI (x-sonosapi-stream:…) fuer den Status-Abgleich.
	URI  string `json:"uri,omitempty"`
	Name string `json:"name,omitempty"` // bei SourceFavorite: Titel wie in der Sonos-App
}

// PlaybackState beschreibt den AVTransport-Zustand eines Coordinators.
type PlaybackState string

const (
	PlaybackPlaying PlaybackState = "playing"
	PlaybackStopped PlaybackState = "stopped"
	PlaybackPaused  PlaybackState = "paused"
	PlaybackUnknown PlaybackState = "unknown"
)

// SpeakerInfo beschreibt einen per Discovery gefundenen Lautsprecher.
type SpeakerInfo struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	UID       string `json:"uid"`
	Reachable bool   `json:"reachable"`
}

// NowPlayingInfo beschreibt die aktuelle Wiedergabe auf einer Box (best effort).
// Felder title/artist/album kommen aus UPnP-Metadaten; raw nur wenn Parsing scheitert.
type NowPlayingInfo struct {
	Speaker string        `json:"speaker"`
	State   PlaybackState `json:"state"`
	Title   string        `json:"title,omitempty"`
	Artist  string        `json:"artist,omitempty"`
	Album   string        `json:"album,omitempty"`
	URI     string        `json:"uri,omitempty"`
	// SourceURI/SourceTitle: geladene Quelle (GetMediaInfo) — bei Dienst-
	// Favoriten (TuneIn) die Favoriten-URI und der Sendername, waehrend URI/
	// Title den aktuellen Track (Pre-Roll, Stream) beschreiben.
	SourceURI    string `json:"source_uri,omitempty"`
	SourceTitle  string `json:"source_title,omitempty"`
	Raw          string `json:"raw,omitempty"`
	Integratable bool   `json:"integratable,omitempty"`
}

// GroupInfo beschreibt eine aktuelle Sonos-Gruppe (Zone Group): der
// Coordinator fuehrt die Wiedergabe, alle Members (inkl. Coordinator) folgen.
// Eine einzelne Box ist eine Gruppe mit genau einem Mitglied.
type GroupInfo struct {
	Coordinator string   `json:"coordinator"`
	Members     []string `json:"members"`
}

// GroupOf liefert die Gruppe, in der name gerade Mitglied ist.
func GroupOf(groups []GroupInfo, name string) (GroupInfo, bool) {
	for _, g := range groups {
		for _, m := range g.Members {
			if m == name {
				return g, true
			}
		}
	}
	return GroupInfo{}, false
}

// SpeakerController steuert physische Lautsprecher. Implementierungen muessen
// nebenlaeufig sicher sein (die API ruft sie aus mehreren Goroutinen auf).
//
// Jede Methode bekommt einen Context fuer Timeout/Abbruch -- Geraete koennen
// offline sein, daher darf keine Methode unbegrenzt blockieren.
type SpeakerController interface {
	// Discover liefert die Namen aktuell erreichbarer Lautsprecher.
	Discover(ctx context.Context) ([]string, error)

	// ListSpeakers liefert bekannte Boxen ohne Netzwerk-Discovery (Cache + Config).
	ListSpeakers(ctx context.Context) ([]SpeakerInfo, error)

	// RefreshSpeakers fuehrt SSDP/Fixed-IP-Discovery aus und liefert die Liste.
	// Bei Teilerfolg koennen Eintraege trotzdem zurueckkommen; der Fehler beschreibt
	// dann das Discovery-Problem (z. B. SSDP blockiert).
	RefreshSpeakers(ctx context.Context) ([]SpeakerInfo, error)

	// EnsureSpeakers aktualisiert nur die genannten Boxen (Fixed-IP/SSDP) und
	// prueft, ob alle Namen danach bekannt sind. Schneller als ListSpeakers
	// bei grosser Config — fuer Validierung vor Szenen-Play.
	EnsureSpeakers(ctx context.Context, names []string) error

	// Group laesst members synchron mit coordinator spielen.
	Group(ctx context.Context, coordinator string, members []string) error

	// SetVolume setzt die absolute Lautstaerke (0-100) eines Lautsprechers.
	SetVolume(ctx context.Context, name string, volume int) error

	// Play startet src auf coordinator; die gesamte Gruppe folgt.
	Play(ctx context.Context, coordinator string, src Source) error

	// Stop beendet die Wiedergabe auf coordinator (AVTransport Stop); die
	// Gruppe folgt. Ist die Box bereits gestoppt, ist das kein Fehler.
	Stop(ctx context.Context, coordinator string) error

	// Pause pausiert die Wiedergabe auf coordinator (AVTransport Pause).
	// Bei Live-Streams ist Stop meist sinnvoller (kein Puffer-Rueckstand).
	Pause(ctx context.Context, coordinator string) error

	// Resume setzt die Wiedergabe auf coordinator fort (AVTransport Play
	// ohne neue URI). Ohne geladene Quelle lehnt die Box ab (playback_failed).
	Resume(ctx context.Context, coordinator string) error

	// GetGroups liefert die aktuelle Gruppen-Topologie (ZoneGroupTopology),
	// unabhaengig davon, wer gruppiert hat (Szene, Sonos-App, ...).
	// Unsichtbare/gebundene Boxen (Sub, Satelliten) sind nicht enthalten.
	GetGroups(ctx context.Context) ([]GroupInfo, error)

	// Ungroup loest name aus seiner Gruppe; die Box spielt danach allein
	// (und ist gestoppt). Wird ein Coordinator geloest, uebernimmt ein
	// anderes Mitglied die Gruppe. Eine allein stehende Box ist kein Fehler.
	Ungroup(ctx context.Context, name string) error

	// GetTransportState liefert den Wiedergabestatus des Coordinators
	// (AVTransport GetTransportInfo).
	GetTransportState(ctx context.Context, coordinator string) (PlaybackState, error)

	// GetNowPlaying liefert Transportzustand plus Metadaten der laufenden Quelle
	// (AVTransport GetMediaInfo / GetPositionInfo). Funktioniert fuer alles, was
	// die Box bereits abspielt — auch Spotify/Sonos Radio, solange die App spielt.
	GetNowPlaying(ctx context.Context, name string) (NowPlayingInfo, error)
}
