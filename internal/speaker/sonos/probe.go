package sonos

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"
)

// streamKind ist das per Probe erkannte Format eines Internetradio-Streams.
type streamKind string

const (
	kindUnknown streamKind = ""    // nicht ermittelbar -> bisheriges Verhalten
	kindMP3     streamKind = "mp3" // audio/mpeg
	kindAAC     streamKind = "aac" // audio/aac, audio/aacp, ADTS
	kindHLS     streamKind = "hls" // m3u8-Playlist
)

// streamProbeTimeout begrenzt die Probe vor dem Abspielen (Redirects inklusive).
const streamProbeTimeout = 3 * time.Second

// streamProbeTTL: so lange gilt ein erfolgreiches Probe-Ergebnis.
const streamProbeTTL = 10 * time.Minute

// sonosUserAgent ahmt den User-Agent einer Box nach. Manche Sender (z. B.
// Superfly/streaMonkey) liefern an Sonos-Geraete nichts aus; die zweite Probe
// mit diesem User-Agent macht das nach einem gescheiterten Play sichtbar.
const sonosUserAgent = "Linux UPnP/1.0 Sonos/86.10-80260 (ZPS1)"

// probeUserAgent ist der User-Agent der normalen Probe: leer = Go-Standard
// ("Go-http-client/…"). Superfly (streaMonkey) bedient nach dem Redirect nur
// bestimmte Clients — curl und Go ja, eigene Kennungen, "Mozilla/5.0" und
// Sonos nicht (Pi-Test 2026-10-15). Eine eigene Kennung liess die Probe
// dort ins Leere laufen.
const probeUserAgent = ""

// streamInfo ist das Ergebnis einer Stream-Probe.
type streamInfo struct {
	kind        streamKind
	contentType string // Content-Type der letzten Antwort (ohne Parameter)
	finalURL    string // URL nach Redirects
	status      int    // HTTP-Status der letzten Antwort, 0 = keine Antwort
	err         error  // Netzwerkfehler/Timeout (status == 0)
}

// reachable: der Server hat mit 2xx geantwortet.
func (i streamInfo) reachable() bool { return i.status >= 200 && i.status < 300 }

// describe liefert eine kurze Beschreibung fuer Hinweise/Logs.
func (i streamInfo) describe() string {
	switch {
	case i.status == 0 && i.err != nil:
		return "Stream probe without response"
	case !i.reachable():
		return fmt.Sprintf("Stream responds with HTTP %d", i.status)
	case i.contentType != "":
		return "detected: " + i.contentType
	default:
		return "Content-Type unknown"
	}
}

// streamProber ermittelt per kurzem GET Content-Type bzw. Codec eines Streams
// und cacht erfolgreiche Ergebnisse.
type streamProber struct {
	client *http.Client
	ttl    time.Duration
	now    func() time.Time

	mu    sync.Mutex
	cache map[string]cachedProbe
}

type cachedProbe struct {
	info streamInfo
	at   time.Time
}

func newStreamProber() *streamProber {
	return &streamProber{
		client: &http.Client{Timeout: streamProbeTimeout},
		ttl:    streamProbeTTL,
		now:    time.Now,
		cache:  map[string]cachedProbe{},
	}
}

// probe liefert das (gecachte) Probe-Ergebnis fuer rawURL.
func (p *streamProber) probe(ctx context.Context, rawURL string) streamInfo {
	p.mu.Lock()
	if c, ok := p.cache[rawURL]; ok && p.now().Sub(c.at) < p.ttl {
		p.mu.Unlock()
		return c.info
	}
	p.mu.Unlock()

	info := p.fetch(ctx, rawURL, probeUserAgent)
	if info.reachable() {
		p.mu.Lock()
		p.cache[rawURL] = cachedProbe{info: info, at: p.now()}
		p.mu.Unlock()
	}
	return info
}

// fetch fuehrt die eigentliche Probe aus (ungecacht): GET, Redirects folgen,
// nach den Headern und hoechstens 512 Byte abbrechen.
func (p *streamProber) fetch(ctx context.Context, rawURL, userAgent string) streamInfo {
	ctx, cancel := context.WithTimeout(ctx, streamProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return streamInfo{err: err}
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	req.Header.Set("Accept", "*/*")
	resp, err := p.client.Do(req)
	if err != nil {
		return streamInfo{err: err}
	}
	defer resp.Body.Close()

	info := streamInfo{status: resp.StatusCode, finalURL: resp.Request.URL.String()}
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		if mt, _, err := mime.ParseMediaType(ct); err == nil {
			info.contentType = strings.ToLower(mt)
		} else {
			info.contentType = strings.ToLower(strings.TrimSpace(ct))
		}
	}
	if !info.reachable() {
		return info
	}
	info.kind = kindFromContentType(info.contentType)
	if info.kind == kindUnknown && strings.HasSuffix(strings.ToLower(resp.Request.URL.Path), ".m3u8") {
		info.kind = kindHLS
	}
	if info.kind == kindUnknown {
		// Unklarer Content-Type (octet-stream, text/html …): erste Bytes ansehen.
		head := make([]byte, 512)
		n, _ := io.ReadAtLeast(resp.Body, head, 4)
		info.kind = kindFromBytes(head[:n])
	}
	return info
}

// kindFromContentType ordnet gaengige Radio-MIME-Typen zu.
func kindFromContentType(ct string) streamKind {
	switch ct {
	case "audio/mpeg", "audio/mp3", "audio/mpeg3", "audio/x-mpeg", "audio/x-mp3":
		return kindMP3
	case "audio/aac", "audio/aacp", "audio/x-aac", "audio/x-aacp", "audio/mp4", "audio/x-m4a", "audio/m4a":
		return kindAAC
	case "application/vnd.apple.mpegurl", "application/x-mpegurl", "audio/mpegurl", "audio/x-mpegurl":
		return kindHLS
	}
	return kindUnknown
}

// kindFromBytes erkennt ADTS-AAC, MP3 (ID3/Frame-Sync) und m3u8 am Anfang.
func kindFromBytes(b []byte) streamKind {
	switch {
	case len(b) < 2:
		return kindUnknown
	case bytes.HasPrefix(b, []byte("#EXTM3U")):
		return kindHLS
	case bytes.HasPrefix(b, []byte("ID3")):
		return kindMP3
	case b[0] == 0xFF && b[1]&0xF6 == 0xF0:
		// ADTS: Sync 0xFFF, Layer-Bits 00.
		return kindAAC
	case b[0] == 0xFF && b[1]&0xE0 == 0xE0:
		return kindMP3
	}
	return kindUnknown
}

// blocksSonos prueft nach einem gescheiterten Play, ob der Sender Anfragen mit
// Sonos-User-Agent nicht beantwortet (Timeout oder Fehlerstatus).
func (p *streamProber) blocksSonos(ctx context.Context, rawURL string) bool {
	return !p.fetch(ctx, rawURL, sonosUserAgent).reachable()
}
