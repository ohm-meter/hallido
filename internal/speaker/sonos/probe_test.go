package sonos

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

// adtsFrame beginnt wie ein ADTS-AAC-Frame (Sync 0xFFF1).
var adtsFrame = []byte{0xFF, 0xF1, 0x5C, 0x80, 0x2E, 0xFF, 0xFC, 0x21}

// streamServer simuliert Internetradio: Endpunkte liefern Header (und erste
// Bytes) und halten die Verbindung wie ein Live-Stream offen.
type streamServer struct {
	srv  *httptest.Server
	hits atomic.Int32
	// blockSonos: Anfragen mit "Sonos" im User-Agent bekommen keine Antwort.
	blockSonos bool
}

func newStreamServer(t *testing.T) *streamServer {
	t.Helper()
	s := &streamServer{}
	mux := http.NewServeMux()
	live := func(ct string, body []byte) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if s.blockSonos && strings.Contains(strings.ToLower(r.UserAgent()), "sonos") {
				<-r.Context().Done()
				return
			}
			if ct != "" {
				w.Header().Set("Content-Type", ct)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
			w.(http.Flusher).Flush()
			<-r.Context().Done() // Live-Stream endet nie
		}
	}
	mux.HandleFunc("/mp3", live("audio/mpeg", []byte("ID3\x04")))
	mux.HandleFunc("/aac", live("audio/aac", adtsFrame))
	mux.HandleFunc("/aacp", live("audio/aacp; charset=binary", adtsFrame))
	mux.HandleFunc("/octet", live("application/octet-stream", adtsFrame))
	mux.HandleFunc("/hls.m3u8", live("application/vnd.apple.mpegurl", []byte("#EXTM3U\n")))
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/aac", http.StatusFound)
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // nie Header -> Timeout
	})
	// alles andere: 404
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *streamServer) url(path string) string { return s.srv.URL + path }

func testProber() *streamProber {
	p := newStreamProber()
	p.client.Timeout = 500 * time.Millisecond
	return p
}

func TestStreamProbe_DetectsFormats(t *testing.T) {
	s := newStreamServer(t)
	p := testProber()
	tests := []struct {
		path string
		kind streamKind
		ct   string
	}{
		{"/mp3", kindMP3, "audio/mpeg"},
		{"/aac", kindAAC, "audio/aac"},
		{"/aacp", kindAAC, "audio/aacp"},
		{"/octet", kindAAC, "application/octet-stream"}, // per ADTS-Sync erkannt
		{"/hls.m3u8", kindHLS, "application/vnd.apple.mpegurl"},
	}
	for _, tc := range tests {
		info := p.probe(context.Background(), s.url(tc.path))
		if info.kind != tc.kind || info.contentType != tc.ct || !info.reachable() {
			t.Errorf("%s: kind=%q ct=%q status=%d err=%v, want kind=%q ct=%q",
				tc.path, info.kind, info.contentType, info.status, info.err, tc.kind, tc.ct)
		}
	}
}

func TestStreamProbe_FollowsRedirect(t *testing.T) {
	s := newStreamServer(t)
	info := testProber().probe(context.Background(), s.url("/redirect"))
	if info.kind != kindAAC {
		t.Fatalf("kind = %q, want aac", info.kind)
	}
	if info.finalURL != s.url("/aac") {
		t.Errorf("finalURL = %q, want %q", info.finalURL, s.url("/aac"))
	}
}

func TestStreamProbe_NotFound(t *testing.T) {
	s := newStreamServer(t)
	info := testProber().probe(context.Background(), s.url("/gibtsnicht"))
	if info.status != http.StatusNotFound || info.reachable() || info.kind != kindUnknown {
		t.Fatalf("status=%d kind=%q, want 404/unknown", info.status, info.kind)
	}
	if !strings.Contains(info.describe(), "HTTP 404") {
		t.Errorf("describe = %q", info.describe())
	}
}

func TestStreamProbe_TimeoutIsUnknown(t *testing.T) {
	s := newStreamServer(t)
	start := time.Now()
	info := testProber().probe(context.Background(), s.url("/slow"))
	if time.Since(start) > 2*time.Second {
		t.Errorf("probe dauerte %v, Timeout greift nicht", time.Since(start))
	}
	if info.status != 0 || info.err == nil || info.kind != kindUnknown {
		t.Fatalf("status=%d err=%v kind=%q, want Timeout ohne Status", info.status, info.err, info.kind)
	}
}

func TestStreamProbe_CachesSuccessOnly(t *testing.T) {
	s := newStreamServer(t)
	p := testProber()
	p.probe(context.Background(), s.url("/aac"))
	p.probe(context.Background(), s.url("/aac"))
	if got := s.hits.Load(); got != 1 {
		t.Errorf("hits nach 2 Proben = %d, want 1 (Cache)", got)
	}
	p.probe(context.Background(), s.url("/missing"))
	p.probe(context.Background(), s.url("/missing"))
	if got := s.hits.Load(); got != 3 {
		t.Errorf("hits = %d, want 3 (Fehler nicht cachen)", got)
	}
	// abgelaufener Eintrag wird neu geprobt
	p.now = func() time.Time { return time.Now().Add(streamProbeTTL + time.Second) }
	p.probe(context.Background(), s.url("/aac"))
	if got := s.hits.Load(); got != 4 {
		t.Errorf("hits nach TTL = %d, want 4", got)
	}
}

func TestStreamProbe_UsesGoDefaultUserAgent(t *testing.T) {
	var ua atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua.Store(r.UserAgent())
		w.Header().Set("Content-Type", "audio/aac")
	}))
	defer srv.Close()
	testProber().probe(context.Background(), srv.URL+"/s")
	if got, _ := ua.Load().(string); !strings.HasPrefix(got, "Go-http-client/") {
		t.Errorf("User-Agent = %q, want Go-Standard (Superfly bedient eigene Kennungen nicht)", got)
	}
}

func TestStreamProbe_BlocksSonos(t *testing.T) {
	s := newStreamServer(t)
	p := testProber()
	if p.blocksSonos(context.Background(), s.url("/aac")) {
		t.Error("ohne Sperre darf blocksSonos nicht anschlagen")
	}
	s.blockSonos = true
	if !p.blocksSonos(context.Background(), s.url("/aac")) {
		t.Error("Sonos-Sperre nicht erkannt")
	}
}

func TestKindFromBytes(t *testing.T) {
	tests := []struct {
		in   []byte
		want streamKind
	}{
		{adtsFrame, kindAAC},
		{[]byte{0xFF, 0xFB, 0x90, 0x64}, kindMP3}, // MPEG-1 Layer III
		{[]byte("ID3\x03"), kindMP3},
		{[]byte("#EXTM3U\n"), kindHLS},
		{[]byte("<html>"), kindUnknown},
		{nil, kindUnknown},
	}
	for _, tc := range tests {
		if got := kindFromBytes(tc.in); got != tc.want {
			t.Errorf("kindFromBytes(% x) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- Play mit Probe und Pruefung nach Play ---

// stateSonos ist ein SOAP-Mock, dessen Transportstatus nach Play einer
// vorgegebenen Folge folgt (letzter Wert bleibt stehen).
type stateSonos struct {
	mu      sync.Mutex
	srv     *httptest.Server
	actions []string
	bodies  []string
	states  []string // Antworten auf GetTransportInfo nach Play
	noState bool     // GetTransportInfo scheitert
}

func newStateSonos(t *testing.T, states ...string) *stateSonos {
	t.Helper()
	f := &stateSonos{states: states}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		act := r.Header.Get("SOAPACTION")
		name := act[strings.LastIndex(act, "#")+1 : len(act)-1]
		f.mu.Lock()
		f.actions = append(f.actions, name)
		f.bodies = append(f.bodies, string(raw))
		state := "STOPPED"
		if name == "GetTransportInfo" && len(f.states) > 0 {
			state = f.states[0]
			if len(f.states) > 1 {
				f.states = f.states[1:]
			}
		}
		noState := f.noState
		f.mu.Unlock()
		if name == "GetTransportInfo" {
			if noState {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetTransportInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><CurrentTransportState>` + state + `</CurrentTransportState></u:GetTransportInfoResponse></s:Body></s:Envelope>`))
			return
		}
		_, _ = w.Write([]byte(okEnvelope))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *stateSonos) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, a := range f.actions {
		if a == name {
			n++
		}
	}
	return n
}

func (f *stateSonos) setURIBody() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, a := range f.actions {
		if a == "SetAVTransportURI" {
			return f.bodies[i]
		}
	}
	return ""
}

func probedController(f *stateSonos, p *streamProber) *Controller {
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_W"},
	})
	c.prober = p
	return c
}

func playStream(c *Controller, uri string) error {
	return c.Play(context.Background(), "Wohnzimmer", speaker.Source{Type: speaker.SourceStream, URI: uri})
}

func TestPlay_AACStreamUsesAACScheme(t *testing.T) {
	s := newStreamServer(t)
	f := newStateSonos(t, "TRANSITIONING", "PLAYING")
	c := probedController(f, testProber())

	if err := playStream(c, s.url("/redirect")); err != nil {
		t.Fatalf("Play: %v", err)
	}
	want := "<CurrentURI>aac://" + s.url("/redirect") + "</CurrentURI>"
	if body := f.setURIBody(); !strings.Contains(body, want) {
		t.Errorf("SetAVTransportURI ohne aac://-Schema (Original-URL):\n%s", body)
	}
	if f.count("GetTransportInfo") < 2 {
		t.Errorf("Status nur %d-mal geprueft, erwartet TRANSITIONING -> PLAYING", f.count("GetTransportInfo"))
	}
}

func TestPlay_MP3StreamKeepsMP3Radio(t *testing.T) {
	s := newStreamServer(t)
	f := newStateSonos(t, "PLAYING")
	c := probedController(f, testProber())

	if err := playStream(c, s.url("/mp3")); err != nil {
		t.Fatalf("Play: %v", err)
	}
	want := "<CurrentURI>x-rincon-mp3radio://" + strings.TrimPrefix(s.url("/mp3"), "http://") + "</CurrentURI>"
	if body := f.setURIBody(); !strings.Contains(body, want) {
		t.Errorf("MP3 soll x-rincon-mp3radio bleiben:\n%s", body)
	}
}

func TestPlay_ProbeTimeoutFallsBackToMP3Radio(t *testing.T) {
	s := newStreamServer(t)
	f := newStateSonos(t, "PLAYING")
	c := probedController(f, testProber())

	if err := playStream(c, s.url("/slow")); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if body := f.setURIBody(); !strings.Contains(body, "<CurrentURI>x-rincon-mp3radio://") {
		t.Errorf("unbekanntes Format soll bisheriges Verhalten behalten:\n%s", body)
	}
}

func TestPlay_Stream404FailsBeforeSonos(t *testing.T) {
	s := newStreamServer(t)
	f := newStateSonos(t, "PLAYING")
	c := probedController(f, testProber())

	err := playStream(c, s.url("/weg"))
	ae := apperr.As(err)
	if ae.Code != apperr.CodePlaybackFailed || !strings.Contains(ae.Hint, "HTTP 404") {
		t.Fatalf("err = %v (hint %q), want playback_failed mit HTTP 404", err, ae.Hint)
	}
	if n := len(f.actions); n != 0 {
		t.Errorf("Box darf bei totem Stream nicht angesprochen werden, %d SOAP-Aufrufe", n)
	}
}

func TestPlay_StoppedAfterPlayIsPlaybackFailed(t *testing.T) {
	s := newStreamServer(t)
	f := newStateSonos(t, "TRANSITIONING", "STOPPED")
	c := probedController(f, testProber())

	err := playStream(c, s.url("/aac"))
	ae := apperr.As(err)
	if ae.Code != apperr.CodePlaybackFailed {
		t.Fatalf("code = %q, want playback_failed (err=%v)", ae.Code, err)
	}
	for _, want := range []string{"not supported", "audio/aac", "STOPPED"} {
		if !strings.Contains(ae.Hint, want) {
			t.Errorf("hint ohne %q: %s", want, ae.Hint)
		}
	}
	if ae.Speaker != "Wohnzimmer" {
		t.Errorf("speaker = %q", ae.Speaker)
	}
}

func TestPlay_SonosBlockedStreamNamesCause(t *testing.T) {
	s := newStreamServer(t)
	s.blockSonos = true
	f := newStateSonos(t, "STOPPED")
	c := probedController(f, testProber())

	ae := apperr.As(playStream(c, s.url("/aac")))
	if ae.Code != apperr.CodePlaybackFailed || !strings.Contains(ae.Hint, "Sonos user agent") {
		t.Fatalf("code=%q hint=%q, want Hinweis auf Sonos-Sperre", ae.Code, ae.Hint)
	}
}

func TestPlay_HLSHint(t *testing.T) {
	s := newStreamServer(t)
	f := newStateSonos(t, "STOPPED")
	c := probedController(f, testProber())

	ae := apperr.As(playStream(c, s.url("/hls.m3u8")))
	if ae.Code != apperr.CodePlaybackFailed || !strings.Contains(ae.Hint, "HLS") {
		t.Fatalf("code=%q hint=%q, want HLS-Hinweis", ae.Code, ae.Hint)
	}
	if body := f.setURIBody(); !strings.Contains(body, "x-rincon-mp3radio://") {
		t.Errorf("HLS soll als x-rincon-mp3radio versucht werden:\n%s", body)
	}
}

func TestPlay_StuckTransitioningFails(t *testing.T) {
	f := newStateSonos(t, "TRANSITIONING")
	c := probedController(f, nil)

	ae := apperr.As(playStream(c, "http://radio.invalid/stream"))
	if ae.Code != apperr.CodePlaybackFailed || !strings.Contains(ae.Hint, "buffering") {
		t.Fatalf("code=%q hint=%q, want playback_failed (still buffering)", ae.Code, ae.Hint)
	}
}

func TestPlay_BriefStoppedWithinGraceIsOK(t *testing.T) {
	// Direkt nach Play meldet die Box kurz noch STOPPED, dann PLAYING.
	f := newStateSonos(t, "STOPPED", "PLAYING")
	c := probedController(f, nil)
	c.playVerifyGrace = 200 * time.Millisecond

	if err := playStream(c, "http://radio.invalid/stream"); err != nil {
		t.Fatalf("Play: %v", err)
	}
}

func TestPlay_UnreadableStatusCountsAsSuccess(t *testing.T) {
	f := newStateSonos(t)
	f.noState = true
	c := probedController(f, nil)

	if err := playStream(c, "http://radio.invalid/stream"); err != nil {
		t.Fatalf("Status unlesbar darf Play nicht scheitern lassen: %v", err)
	}
}

func TestPrepareStreamPlayback_AAC(t *testing.T) {
	got, meta := prepareStreamPlayback("https://web.stream.superfly.fm/superfly-beach", kindAAC)
	if got != "aac://https://web.stream.superfly.fm/superfly-beach" {
		t.Errorf("transportURI = %q", got)
	}
	if !strings.Contains(meta, "audioBroadcast") {
		t.Errorf("metadata: %s", meta)
	}
	// bereits umhuellte URIs bleiben unveraendert
	if got, _ := prepareStreamPlayback("aac://http://x/y", kindAAC); got != "aac://http://x/y" {
		t.Errorf("aac:// doppelt umgewandelt: %q", got)
	}
}
