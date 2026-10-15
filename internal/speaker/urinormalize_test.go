package speaker

import "testing"

func TestNormalizeStreamURIForConfig(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://radio.example/live?token=x", "https://radio.example/live"},
		{"aac://stream.example/radio", "https://stream.example/radio"},
		{"x-rincon-mp3radio://192.168.1.5/stream", "https://192.168.1.5/stream"},
	}
	for _, tc := range cases {
		got, err := NormalizeStreamURIForConfig(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeStreamURIForConfig_RejectsFile(t *testing.T) {
	if _, err := NormalizeStreamURIForConfig("file:///etc/passwd"); err == nil {
		t.Fatal("file:// sollte abgelehnt werden")
	}
}

func TestNormalizeStreamURIForConfig_RejectsSpotify(t *testing.T) {
	if _, err := NormalizeStreamURIForConfig("x-sonos-spotify:spotify%3atrack%3a1"); err == nil {
		t.Fatal("spotify sollte abgelehnt werden")
	}
}
