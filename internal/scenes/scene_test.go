package scenes

import (
	"testing"

	"sonos-radio-agent/internal/speaker"
)

func validStreamScene() Scene {
	return Scene{
		ID:     "fm4",
		Name:   "FM4",
		Source: speaker.Source{Type: speaker.SourceStream, URI: "https://example/stream"},
		Members: []Member{
			{Speaker: "Wohnzimmer", Volume: 30},
			{Speaker: "Kueche", Volume: 20},
		},
	}
}

func TestScene_Validate_OK(t *testing.T) {
	if err := validStreamScene().Validate(); err != nil {
		t.Fatalf("erwarte gueltig, bekam: %v", err)
	}
	fav := validStreamScene()
	fav.Source = speaker.Source{Type: speaker.SourceFavorite, Name: "Ö1"}
	if err := fav.Validate(); err != nil {
		t.Fatalf("favorit sollte gueltig sein: %v", err)
	}
}

func TestScene_Validate_Errors(t *testing.T) {
	cases := map[string]func(*Scene){
		"kein name":         func(s *Scene) { s.Name = "" },
		"keine member":      func(s *Scene) { s.Members = nil },
		"volume zu hoch":    func(s *Scene) { s.Members[0].Volume = 101 },
		"volume negativ":    func(s *Scene) { s.Members[0].Volume = -1 },
		"doppelter speaker": func(s *Scene) { s.Members[1].Speaker = "Wohnzimmer" },
		"stream ohne uri":   func(s *Scene) { s.Source = speaker.Source{Type: speaker.SourceStream} },
		"favorit ohne name": func(s *Scene) { s.Source = speaker.Source{Type: speaker.SourceFavorite} },
		"unbekannter typ":   func(s *Scene) { s.Source = speaker.Source{Type: "spotify"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := validStreamScene()
			mutate(&s)
			if err := s.Validate(); err == nil {
				t.Fatalf("erwarte fehler fuer %q, bekam keinen", name)
			}
		})
	}
}

func TestScene_Coordinator(t *testing.T) {
	s := validStreamScene()
	if got := s.Coordinator(); got != "Wohnzimmer" {
		t.Fatalf("coordinator = %q, erwartet Wohnzimmer", got)
	}
	if got := (Scene{}).Coordinator(); got != "" {
		t.Fatalf("leere szene sollte leeren coordinator liefern, bekam %q", got)
	}
}
