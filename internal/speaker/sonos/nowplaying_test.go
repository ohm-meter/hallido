package sonos

import (
	"context"
	"strings"
	"testing"
	"time"

	"sonos-radio-agent/internal/speaker"
)

func TestGetNowPlaying_SendsSOAPAndParsesMetadata(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	f.transportState = "PLAYING"
	f.currentURI = "x-sonos-spotify:spotify:track:abc"
	f.currentMeta = `&lt;DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"&gt;&lt;item&gt;&lt;dc:title&gt;Song&lt;/dc:title&gt;&lt;dc:creator&gt;Band&lt;/dc:creator&gt;&lt;upnp:album&gt;LP&lt;/upnp:album&gt;&lt;/item&gt;&lt;/DIDL-Lite&gt;`
	f.trackURI = "x-sonos-spotify:spotify:track:abc"
	f.trackMeta = f.currentMeta

	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_W"},
	})

	got, err := c.GetNowPlaying(context.Background(), "Wohnzimmer")
	if err != nil {
		t.Fatalf("GetNowPlaying: %v", err)
	}
	if got.State != speaker.PlaybackPlaying {
		t.Fatalf("state = %q, want playing", got.State)
	}
	if got.Title != "Song" || got.Artist != "Band" || got.Album != "LP" {
		t.Fatalf("metadata = %+v", got)
	}
	if got.URI != f.currentURI {
		t.Fatalf("uri = %q", got.URI)
	}
	if got.Raw != "" {
		t.Fatalf("raw should be empty, got %q", got.Raw)
	}
	if len(f.calls) != 3 {
		t.Fatalf("erwartet 3 SOAP-Aufrufe, bekam %d", len(f.calls))
	}
	for _, action := range []string{"GetTransportInfo", "GetMediaInfo", "GetPositionInfo"} {
		found := false
		for _, call := range f.calls {
			if strings.Contains(call.Action, action) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("fehlender SOAP-Aufruf %s", action)
		}
	}
}

func TestGetNowPlaying_RawOnBadMetadata(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	f.transportState = "PLAYING"
	f.currentMeta = "broken-meta"
	f.trackMeta = "broken-meta"

	c := controllerFor(map[string]*device{
		"Kueche": {name: "Kueche", baseURL: f.srv.URL, uid: "RINCON_K"},
	})

	got, err := c.GetNowPlaying(context.Background(), "Kueche")
	if err != nil {
		t.Fatalf("GetNowPlaying: %v", err)
	}
	if got.Raw != "broken-meta" {
		t.Fatalf("raw = %q, want broken-meta", got.Raw)
	}
}

func TestGetNowPlaying_SoftFailOnTransportHTTP500(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	f.transportFault = true
	c := controllerFor(map[string]*device{
		"Fernseher": {name: "Fernseher", baseURL: f.srv.URL, uid: "RINCON_TV"},
	})

	got, err := c.GetNowPlaying(context.Background(), "Fernseher")
	if err != nil {
		t.Fatalf("GetNowPlaying: erwartet kein harter Fehler, bekam %v", err)
	}
	if got.State != speaker.PlaybackUnknown {
		t.Fatalf("state = %q, want unknown", got.State)
	}
	if got.Speaker != "Fernseher" {
		t.Fatalf("speaker = %q", got.Speaker)
	}
}

func noSSDP(t *testing.T) {
	t.Helper()
	old := ssdpSearchHook
	ssdpSearchHook = func(time.Duration) ([]string, error) { return nil, nil }
	t.Cleanup(func() { ssdpSearchHook = old })
}

func TestGetNowPlaying_SlaveUsesCoordinatorState(t *testing.T) {
	noSSDP(t)
	slave := newFakeSonos()
	defer slave.close()
	slave.transportState = "PLAYING"
	slave.currentURI = "x-rincon:RINCON_BAD"
	slave.trackURI = "x-rincon:RINCON_BAD"

	coord := newFakeSonos()
	defer coord.close()
	coord.transportState = "STOPPED"
	coord.currentURI = "x-rincon-mp3radio://oe3.example/stream"
	coord.trackURI = coord.currentURI

	c := controllerFor(map[string]*device{
		"OG: WC":  {name: "OG: WC", baseURL: slave.srv.URL, uid: "RINCON_WC"},
		"OG: Bad": {name: "OG: Bad", baseURL: coord.srv.URL, uid: "RINCON_BAD"},
	})

	got, err := c.GetNowPlaying(context.Background(), "OG: WC")
	if err != nil {
		t.Fatalf("GetNowPlaying: %v", err)
	}
	if got.State != speaker.PlaybackStopped {
		t.Fatalf("state = %q, want stopped (Coordinator)", got.State)
	}
	if got.URI != coord.currentURI {
		t.Fatalf("uri = %q, want Coordinator-URI", got.URI)
	}
	if got.Speaker != "OG: WC" {
		t.Fatalf("speaker = %q, want angefragte Box", got.Speaker)
	}
}

func TestGetNowPlaying_SlaveCoordinatorUnresolvableFallsBack(t *testing.T) {
	noSSDP(t)
	slave := newFakeSonos()
	defer slave.close()
	slave.transportState = "PLAYING"
	slave.currentURI = "x-rincon:RINCON_GONE"
	slave.trackURI = "x-rincon:RINCON_GONE"

	c := controllerFor(map[string]*device{
		"OG: WC": {name: "OG: WC", baseURL: slave.srv.URL, uid: "RINCON_WC"},
	})

	got, err := c.GetNowPlaying(context.Background(), "OG: WC")
	if err != nil {
		t.Fatalf("GetNowPlaying: %v", err)
	}
	if got.State != speaker.PlaybackPlaying || got.URI != "x-rincon:RINCON_GONE" {
		t.Fatalf("erwartet Slave-Verhalten, bekam %+v", got)
	}
}

func TestGetNowPlaying_SlaveCoordinatorErrorFallsBack(t *testing.T) {
	noSSDP(t)
	slave := newFakeSonos()
	defer slave.close()
	slave.transportState = "PLAYING"
	slave.currentURI = "x-rincon:RINCON_BAD"
	slave.trackURI = "x-rincon:RINCON_BAD"

	coord := newFakeSonos()
	coord.close() // Coordinator nicht erreichbar

	c := controllerFor(map[string]*device{
		"OG: WC":  {name: "OG: WC", baseURL: slave.srv.URL, uid: "RINCON_WC"},
		"OG: Bad": {name: "OG: Bad", baseURL: coord.srv.URL, uid: "RINCON_BAD"},
	})

	got, err := c.GetNowPlaying(context.Background(), "OG: WC")
	if err != nil {
		t.Fatalf("GetNowPlaying: %v", err)
	}
	if got.State != speaker.PlaybackPlaying {
		t.Fatalf("state = %q, want playing (Fallback)", got.State)
	}
}

func TestGetNowPlaying_RadioURIIsNotTreatedAsSlave(t *testing.T) {
	noSSDP(t)
	f := newFakeSonos()
	defer f.close()
	f.transportState = "PLAYING"
	f.currentURI = "x-rincon-mp3radio://oe3.example/stream"
	f.trackURI = f.currentURI

	c := controllerFor(map[string]*device{
		"OG: Bad": {name: "OG: Bad", baseURL: f.srv.URL, uid: "RINCON_BAD"},
	})
	got, err := c.GetNowPlaying(context.Background(), "OG: Bad")
	if err != nil {
		t.Fatalf("GetNowPlaying: %v", err)
	}
	if got.State != speaker.PlaybackPlaying || len(f.calls) != 3 {
		t.Fatalf("state=%q calls=%d", got.State, len(f.calls))
	}
}
