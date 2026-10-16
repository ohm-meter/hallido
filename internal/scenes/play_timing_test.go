package scenes

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"sonos-radio-agent/internal/speaker"
	"sonos-radio-agent/internal/speaker/mock"
)

// countingCtrl zaehlt Group-Aufrufe je Box und laesst SetVolume fuer
// failVolume scheitern.
type countingCtrl struct {
	*mock.Controller
	mu         sync.Mutex
	grouped    map[string]int
	failVolume string
}

func (c *countingCtrl) Group(ctx context.Context, coordinator string, members []string) error {
	c.mu.Lock()
	for _, m := range members {
		c.grouped[m]++
	}
	c.mu.Unlock()
	return c.Controller.Group(ctx, coordinator, members)
}

func (c *countingCtrl) SetVolume(ctx context.Context, name string, volume int) error {
	if name == c.failVolume {
		c.Controller.SetReachable(name, false)
		defer c.Controller.SetReachable(name, true)
	}
	return c.Controller.SetVolume(ctx, name, volume)
}

func threeBoxScene() Scene {
	return Scene{
		ID: "drei", Name: "Drei",
		Source: speaker.Source{Type: speaker.SourceStream, URI: "https://example/stream"},
		Members: []Member{
			{Speaker: "Wohnzimmer", Volume: 30},
			{Speaker: "Kueche", Volume: 20},
			{Speaker: "Bad", Volume: 10},
		},
	}
}

func TestService_Play_UeberspringtSchonGruppierteBoxen(t *testing.T) {
	m := mock.New("Wohnzimmer", "Kueche", "Bad")
	m.SetGroup("Wohnzimmer", "Kueche") // Kueche folgt Wohnzimmer bereits
	ctrl := &countingCtrl{Controller: m, grouped: map[string]int{}}
	var logBuf bytes.Buffer
	svc := NewService([]Scene{threeBoxScene()}, ctrl, slog.New(slog.NewTextHandler(&logBuf, nil)))

	res, err := svc.Play(context.Background(), "drei")
	if err != nil || !res.Playing {
		t.Fatalf("Play: res=%+v err=%v", res, err)
	}
	if ctrl.grouped["Kueche"] != 0 {
		t.Errorf("Kueche ist schon in der Gruppe, Group-Aufrufe = %d, want 0", ctrl.grouped["Kueche"])
	}
	if ctrl.grouped["Bad"] != 1 {
		t.Errorf("Bad muss gruppiert werden, Group-Aufrufe = %d", ctrl.grouped["Bad"])
	}
	if m.Volumes["Wohnzimmer"] != 30 || m.Volumes["Kueche"] != 20 || m.Volumes["Bad"] != 10 {
		t.Errorf("Lautstaerken: %+v", m.Volumes)
	}
	line := logBuf.String()
	for _, want := range []string{`msg="scene started"`, "already_grouped=1", "took=", "group=", "volume=", "play="} {
		if !strings.Contains(line, want) {
			t.Errorf("Log ohne %q: %s", want, line)
		}
	}
}

func TestService_Play_FremdeGruppeWirdNeuGruppiert(t *testing.T) {
	// Coordinator ist Mitglied einer fremden Gruppe: er wird geloest und alle
	// Mitglieder werden normal gruppiert (nichts uebersprungen).
	m := mock.New("Wohnzimmer", "Kueche", "Bad")
	m.SetGroup("Bad", "Wohnzimmer", "Kueche")
	ctrl := &countingCtrl{Controller: m, grouped: map[string]int{}}
	svc := NewService([]Scene{threeBoxScene()}, ctrl, quietLogger())

	if _, err := svc.Play(context.Background(), "drei"); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if ctrl.grouped["Kueche"] != 1 || ctrl.grouped["Bad"] != 1 {
		t.Errorf("Group-Aufrufe: %+v, want je 1", ctrl.grouped)
	}
}

func TestService_Play_VolumeFehlerParallelIstTeilausfall(t *testing.T) {
	m := mock.New("Wohnzimmer", "Kueche", "Bad")
	ctrl := &countingCtrl{Controller: m, grouped: map[string]int{}, failVolume: "Bad"}
	svc := NewService([]Scene{threeBoxScene()}, ctrl, quietLogger())

	res, err := svc.Play(context.Background(), "drei")
	if err != nil || !res.Playing {
		t.Fatalf("Teilausfall ist kein harter Fehler: res=%+v err=%v", res, err)
	}
	for _, s := range res.Speakers {
		if want := s.Speaker != "Bad"; s.OK != want {
			t.Errorf("%s: ok=%v, want %v (%s)", s.Speaker, s.OK, want, s.Error)
		}
	}
}
