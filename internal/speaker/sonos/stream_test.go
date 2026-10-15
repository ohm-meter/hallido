package sonos

import (
	"strings"
	"testing"
)

func TestPrepareStreamPlayback_HTTPS(t *testing.T) {
	uri := "https://orf-live.ors-shoutcast.at/fm4-q2a"
	gotURI, gotMeta := prepareStreamPlayback(uri, kindUnknown)

	wantURI := "x-rincon-mp3radio://orf-live.ors-shoutcast.at/fm4-q2a"
	if gotURI != wantURI {
		t.Errorf("transportURI: got %q, want %q", gotURI, wantURI)
	}
	if !strings.Contains(gotMeta, "object.item.audioItem.audioBroadcast") {
		t.Errorf("fehlende upnp:class im metadata: %s", gotMeta)
	}
	if !strings.Contains(gotMeta, "<dc:title>fm4-q2a</dc:title>") {
		t.Errorf("titel aus pfad fehlt: %s", gotMeta)
	}
	if !strings.Contains(gotMeta, tuneInServiceUDN) {
		t.Errorf("cdudn fehlt: %s", gotMeta)
	}
}

func TestPrepareStreamPlayback_AlreadyRadioPrefix(t *testing.T) {
	uri := "x-rincon-mp3radio://ice.example.com/stream"
	gotURI, gotMeta := prepareStreamPlayback(uri, kindUnknown)
	if gotURI != uri {
		t.Errorf("transportURI soll unveraendert bleiben: got %q", gotURI)
	}
	if gotMeta == "" {
		t.Fatal("metadata darf nicht leer sein")
	}
}

func TestRadioStreamMetaData_EscapesTitle(t *testing.T) {
	meta := radioStreamMetaData(`FM4 & <Test>`)
	if strings.Contains(meta, "FM4 & <Test>") {
		t.Errorf("titel nicht escaped: %s", meta)
	}
	if !strings.Contains(meta, "FM4 &amp; &lt;Test&gt;") {
		t.Errorf("erwartete escaped-form fehlt: %s", meta)
	}
}

func TestStreamTitle(t *testing.T) {
	tests := []struct {
		uri  string
		want string
	}{
		{"https://example.com/radio/fm4", "fm4"},
		{"https://stream.example.com/", "stream.example.com"},
		{"not-a-url", "Radio"},
	}
	for _, tc := range tests {
		if got := streamTitle(tc.uri); got != tc.want {
			t.Errorf("streamTitle(%q) = %q, want %q", tc.uri, got, tc.want)
		}
	}
}
