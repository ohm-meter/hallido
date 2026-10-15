package speaker

import "testing"

func TestCanonicalStreamKey(t *testing.T) {
	scene := "https://inselradiomallorca.stream06.radiohost.de/mallorcasunshineradio-live_aac-128"
	playing := "aac://http://inselradiomallorca.stream06.radiohost.de/mallorcasunshineradio-live_aac-128?token=abc"

	keyScene := CanonicalStreamKey(scene)
	keyPlay := CanonicalStreamKey(playing)
	if keyScene == "" || keyPlay == "" {
		t.Fatal("keys duerfen nicht leer sein")
	}
	if keyScene != keyPlay {
		t.Fatalf("keys sollten matchen: scene=%q play=%q", keyScene, keyPlay)
	}

	radio := "x-rincon-mp3radio://orf-live.ors-shoutcast.at/fm4-q2a"
	https := "https://orf-live.ors-shoutcast.at/fm4-q2a"
	if CanonicalStreamKey(radio) != CanonicalStreamKey(https) {
		t.Fatalf("radio-prefix und https sollten gleich normalisieren")
	}

	if CanonicalStreamKey("https://example.com/stream?q=1") != CanonicalStreamKey("http://example.com/stream") {
		t.Fatal("query und schema sollten ignoriert werden")
	}

	if CanonicalStreamKey("") != "" {
		t.Fatal("leerer input -> leerer key")
	}
}

func TestSourceMatchesPlayback_Stream(t *testing.T) {
	src := Source{Type: SourceStream, URI: "https://example.com/radio/live"}
	now := NowPlayingInfo{
		URI: "x-rincon-mp3radio://example.com/radio/live",
	}
	if !SourceMatchesPlayback(src, now) {
		t.Fatal("erwarte match fuer gleichen stream")
	}
	now.URI = "x-rincon-mp3radio://other.example/stream"
	if SourceMatchesPlayback(src, now) {
		t.Fatal("erwarte kein match fuer anderen stream")
	}
}

func TestSourceMatchesPlayback_Favorite(t *testing.T) {
	src := Source{Type: SourceFavorite, Name: "FM4"}
	if !SourceMatchesPlayback(src, NowPlayingInfo{Title: "FM4"}) {
		t.Fatal("exakter titel soll matchen")
	}
	if !SourceMatchesPlayback(src, NowPlayingInfo{Title: "FM4 — Live"}) {
		t.Fatal("titel mit suffix soll matchen")
	}
	if SourceMatchesPlayback(src, NowPlayingInfo{Title: "Oe3"}) {
		t.Fatal("falscher titel soll nicht matchen")
	}
}

func TestServiceStreamKey(t *testing.T) {
	fav := "x-sonosapi-stream:tunein%3A5216?sid=303&flags=8292&sn=9"
	box := "x-sonosapi-stream:tunein%3a5216?sid=303&flags=8232&sn=9"
	if ServiceStreamKey(fav) == "" || ServiceStreamKey(fav) != ServiceStreamKey(box) {
		t.Fatalf("Favorit und Box-URI sollen gleich normalisieren: %q vs %q", ServiceStreamKey(fav), ServiceStreamKey(box))
	}
	if ServiceStreamKey("x-sonosapi-stream:tunein%3a17315?sid=303") == ServiceStreamKey(box) {
		t.Fatal("verschiedene Sender duerfen nicht gleich sein")
	}
	for _, raw := range []string{"", "https://a.example/x", "x-sonosapi-radio:catalog%2fx?sid=201", "x-sonosapi-stream:?sid=1"} {
		if got := ServiceStreamKey(raw); got != "" {
			t.Errorf("%q: erwartet leer, bekam %q", raw, got)
		}
	}
}

func TestSourceMatchesPlayback_DienstFavorit(t *testing.T) {
	src := Source{Type: SourceFavorite, Name: "Radio Superfly", URI: "x-sonosapi-stream:tunein%3a17315?sid=303&flags=8232&sn=9"}
	preroll := NowPlayingInfo{
		URI:         "x-rincon-mp3radio://https://tunein-ondemand.cdnstream1.com/multi_bump_sonic_pre_pre.mp3",
		Title:       "multi_bump_sonic_pre_pre.mp3",
		SourceURI:   "x-sonosapi-stream:tunein%3a17315?sid=303&flags=8232&sn=9",
		SourceTitle: "Radio Superfly",
	}
	if !SourceMatchesPlayback(src, preroll) {
		t.Error("Pre-Roll eines TuneIn-Favoriten muss zur Szene passen")
	}
	other := preroll
	other.SourceURI = "x-sonosapi-stream:tunein%3a5216?sid=303&flags=8232&sn=9"
	other.SourceTitle = "ORF Hitradio OE3"
	if SourceMatchesPlayback(src, other) {
		t.Error("anderer TuneIn-Sender darf nicht passen")
	}
	// Ohne gespeicherte URI: Titel der Quelle (Sendername) zaehlt.
	byName := Source{Type: SourceFavorite, Name: "Radio Superfly"}
	if !SourceMatchesPlayback(byName, preroll) {
		t.Error("Favorit nur mit Name: SourceTitle muss passen")
	}
	if SourceMatchesPlayback(byName, NowPlayingInfo{Title: "Anderer Sender"}) {
		t.Error("fremder Titel darf nicht passen")
	}
}
