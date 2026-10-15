// Package scenes enthaelt das Szenen-Modell und die Logik, eine Szene
// abzuspielen. Eine Szene ist eine benannte Kombination aus Lautsprechern,
// einer Audioquelle und einer Lautstaerke je Lautsprecher.
package scenes

import (
	"fmt"

	"sonos-radio-agent/internal/speaker"
)

// Member ist ein Lautsprecher mit seiner Ziel-Lautstaerke in der Szene.
type Member struct {
	Speaker string `json:"speaker"`
	Volume  int    `json:"volume"`
}

// Scene ist eine abspielbare Voreinstellung.
type Scene struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Source  speaker.Source `json:"source"`
	Members []Member       `json:"members"`
}

// Coordinator ist der erste Lautsprecher der Szene; er fuehrt die Gruppe an.
func (s Scene) Coordinator() string {
	if len(s.Members) == 0 {
		return ""
	}
	return s.Members[0].Speaker
}

// Validate prueft eine Szene auf Vollstaendigkeit und Plausibilitaet.
func (s Scene) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("scene without id")
	}
	if s.Name == "" {
		return fmt.Errorf("scene %q without name", s.ID)
	}
	if len(s.Members) == 0 {
		return fmt.Errorf("scene %q has no speakers", s.ID)
	}
	switch s.Source.Type {
	case speaker.SourceStream:
		if s.Source.URI == "" {
			return fmt.Errorf("scene %q: stream source without uri", s.ID)
		}
	case speaker.SourceFavorite:
		if s.Source.Name == "" {
			return fmt.Errorf("scene %q: favorite source without name", s.ID)
		}
	default:
		return fmt.Errorf("scene %q: unknown source type %q", s.ID, s.Source.Type)
	}
	seen := make(map[string]bool, len(s.Members))
	for _, m := range s.Members {
		if m.Speaker == "" {
			return fmt.Errorf("scene %q: speaker without name", s.ID)
		}
		if seen[m.Speaker] {
			return fmt.Errorf("scene %q: speaker %q duplicated", s.ID, m.Speaker)
		}
		seen[m.Speaker] = true
		if m.Volume < 0 || m.Volume > 100 {
			return fmt.Errorf("scene %q: volume %d for %q outside 0-100",
				s.ID, m.Volume, m.Speaker)
		}
	}
	return nil
}
