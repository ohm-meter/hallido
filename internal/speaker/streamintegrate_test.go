package speaker

import "testing"

func TestIsIntegratableStreamURI(t *testing.T) {
	ok := []string{
		"https://radio.example/live",
		"http://192.168.1.10/stream",
		"aac://http://stream.example/radio",
		"aac://https://stream.example/radio",
		"x-rincon-mp3radio://192.168.1.5/stream",
		"x-sonos-http://stream.example/path",
	}
	for _, u := range ok {
		if !IsIntegratableStreamURI(u) {
			t.Errorf("sollte integrierbar sein: %q", u)
		}
	}
	reject := []string{
		"",
		"x-rincon",
		"x-rincon:348728473",
		"x-sonos-vli:123",
		"spotify:track:abc",
		"x-sonos-spotify:spotify%3atrack%3aabc",
		"aac://x-rincon:123",
		"file:///etc/passwd",
	}
	for _, u := range reject {
		if IsIntegratableStreamURI(u) {
			t.Errorf("sollte abgelehnt werden: %q", u)
		}
	}
}

func TestIsSonosServiceStreamURI(t *testing.T) {
	if !IsSonosServiceStreamURI("x-sonosapi-stream:tunein%3a17315?sid=303&flags=8232&sn=9") {
		t.Error("TuneIn-Favorit erwartet")
	}
	for _, raw := range []string{"", "https://a.example/x", "x-sonosapi-radio:catalog?sid=201", "x-rincon-cpcontainer:1006206cspotify"} {
		if IsSonosServiceStreamURI(raw) {
			t.Errorf("%q darf kein Dienst-Stream sein", raw)
		}
	}
	if IsIntegratableStreamURI("x-sonosapi-stream:tunein%3a17315?sid=303") {
		t.Error("Dienst-Stream ist nicht als Stream-Szene importierbar")
	}
}
