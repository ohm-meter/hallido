package sonos

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

const okEnvelope = `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body></s:Body></s:Envelope>`

// slowSonos antwortet je Aktion nach einer konfigurierbaren Verzoegerung; die
// Verzoegerung gilt nur fuer die ersten n Aufrufe der Aktion.
type slowSonos struct {
	mu     sync.Mutex
	srv    *httptest.Server
	counts map[string]int
	slow   map[string]int // Aktion -> Anzahl verzoegerter Aufrufe
	state  string         // Antwort auf GetTransportInfo
}

func newSlowSonos(delay time.Duration, slow map[string]int) *slowSonos {
	f := &slowSonos{counts: map[string]int{}, slow: slow}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		act := r.Header.Get("SOAPACTION")
		name := act[strings.LastIndex(act, "#")+1 : len(act)-1]
		f.mu.Lock()
		f.counts[name]++
		n := f.counts[name]
		state := f.state
		if state == "" {
			// Ohne Vorgabe: Box spielt, sobald Play angekommen ist.
			state = "STOPPED"
			if f.counts["Play"] > 0 {
				state = "PLAYING"
			}
		}
		f.mu.Unlock()
		if n <= slow[name] {
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}
		if name == "GetTransportInfo" {
			io.WriteString(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetTransportInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><CurrentTransportState>`+state+`</CurrentTransportState></u:GetTransportInfoResponse></s:Body></s:Envelope>`)
			return
		}
		io.WriteString(w, okEnvelope)
	}))
	return f
}

func (f *slowSonos) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[name]
}

func coldController(f *slowSonos) *Controller {
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_W"},
	})
	c.client.Timeout = 100 * time.Millisecond
	c.playClient.Timeout = 100 * time.Millisecond
	c.retryPause = time.Millisecond
	return c
}

var coldSrc = speaker.Source{Type: speaker.SourceStream, URI: "https://example.com/fm4"}

func TestPlay_RetriesSlowSetAVTransportURI(t *testing.T) {
	f := newSlowSonos(time.Second, map[string]int{"SetAVTransportURI": 1})
	defer f.srv.Close()
	c := coldController(f)

	if err := c.Play(context.Background(), "Wohnzimmer", coldSrc); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if got := f.count("SetAVTransportURI"); got != 2 {
		t.Errorf("SetAVTransportURI-Aufrufe = %d, want 2", got)
	}
	if got := f.count("Play"); got != 1 {
		t.Errorf("Play-Aufrufe = %d, want 1", got)
	}
}

func TestPlay_SlowPlayStillPlayingIsSuccess(t *testing.T) {
	// Play antwortet zu spaet, die Box spielt aber: kein zweites Play.
	f := newSlowSonos(time.Second, map[string]int{"Play": 2})
	f.state = "PLAYING"
	defer f.srv.Close()
	c := coldController(f)

	if err := c.Play(context.Background(), "Wohnzimmer", coldSrc); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if got := f.count("Play"); got != 1 {
		t.Errorf("Play-Aufrufe = %d, want 1", got)
	}
}

func TestPlay_SlowPlayTransitioningIsNotRepeated(t *testing.T) {
	// Coordinator einer grossen Gruppe puffert noch (Hardware 2026-10-15):
	// kein zweites Play, die Pruefung nach Play entscheidet.
	f := newSlowSonos(time.Second, map[string]int{"Play": 1})
	f.state = "TRANSITIONING"
	defer f.srv.Close()
	c := coldController(f)

	err := c.Play(context.Background(), "Wohnzimmer", coldSrc)
	if got := f.count("Play"); got != 1 {
		t.Errorf("Play-Aufrufe = %d, want 1", got)
	}
	// bleibt TRANSITIONING -> Pruefung nach Play meldet playback_failed
	if got := apperr.As(err).Code; got != apperr.CodePlaybackFailed {
		t.Errorf("code = %q, want playback_failed (err=%v)", got, err)
	}
}

func TestPlay_SlowPlayWithinPlayTimeout(t *testing.T) {
	// Play braucht laenger als das allgemeine SOAP-Zeitlimit, aber weniger als
	// playSOAPTimeout: ein einziger Aufruf, keine Warnung, kein Fehler.
	f := newSlowSonos(300*time.Millisecond, map[string]int{"Play": 1})
	defer f.srv.Close()
	c := coldController(f)
	c.playClient.Timeout = 2 * time.Second

	if err := c.Play(context.Background(), "Wohnzimmer", coldSrc); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if got := f.count("Play"); got != 1 {
		t.Errorf("Play-Aufrufe = %d, want 1", got)
	}
}

func TestPlay_SlowPlayNotPlayingStaysUnreachable(t *testing.T) {
	f := newSlowSonos(time.Second, map[string]int{"Play": 2})
	f.state = "STOPPED"
	defer f.srv.Close()
	c := coldController(f)

	err := c.Play(context.Background(), "Wohnzimmer", coldSrc)
	if got := apperr.As(err).Code; got != apperr.CodeSpeakerUnreachable {
		t.Fatalf("code = %q, want %q (err=%v)", got, apperr.CodeSpeakerUnreachable, err)
	}
}

func TestPlay_PersistentTimeoutRetriesOnlyOnce(t *testing.T) {
	f := newSlowSonos(time.Second, map[string]int{"SetAVTransportURI": 99})
	defer f.srv.Close()
	c := coldController(f)

	err := c.Play(context.Background(), "Wohnzimmer", coldSrc)
	if got := apperr.As(err).Code; got != apperr.CodeSpeakerUnreachable {
		t.Fatalf("code = %q, want speaker_unreachable", got)
	}
	if got := f.count("SetAVTransportURI"); got != 2 {
		t.Errorf("SetAVTransportURI-Aufrufe = %d, want 2", got)
	}
	if f.count("Play") != 0 {
		t.Error("Play darf nach gescheitertem SetAVTransportURI nicht laufen")
	}
}

func TestPlay_RejectionIsNotRetried(t *testing.T) {
	f := newFakeSonos()
	f.fault = true
	f.faultCode = "714"
	defer f.close()
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_W"},
	})
	err := c.Play(context.Background(), "Wohnzimmer", coldSrc)
	if got := apperr.As(err).Code; got != apperr.CodePlaybackFailed {
		t.Fatalf("code = %q, want playback_failed", got)
	}
	if len(f.calls) != 1 {
		t.Errorf("aufrufe = %d, want 1 (keine Wiederholung bei Ablehnung)", len(f.calls))
	}
}

func TestPlay_NoRetryWhenContextExpired(t *testing.T) {
	f := newSlowSonos(time.Second, map[string]int{"SetAVTransportURI": 99})
	defer f.srv.Close()
	c := coldController(f)
	c.client.Timeout = 5 * time.Second
	c.playClient.Timeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := c.Play(ctx, "Wohnzimmer", coldSrc); err == nil {
		t.Fatal("erwartet Fehler")
	}
	if got := f.count("SetAVTransportURI"); got != 1 {
		t.Errorf("SetAVTransportURI-Aufrufe = %d, want 1", got)
	}
}
