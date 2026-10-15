package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/speaker"
	"sonos-radio-agent/internal/speaker/mock"
	"sonos-radio-agent/internal/web"
)

type selectiveFavoritesCtrl struct {
	*mock.Controller
	browseFail map[string]bool
	calls      []string
}

func (c *selectiveFavoritesCtrl) BrowseFavorites(ctx context.Context, proxySpeaker string) ([]speaker.FavoriteEntry, error) {
	c.calls = append(c.calls, proxySpeaker)
	if c.browseFail[proxySpeaker] {
		return nil, apperr.New(apperr.CodePlaybackFailed, "Device rejected Browse (HTTP 500): ContentDirectory unavailable").
			WithSpeaker(proxySpeaker)
	}
	return []speaker.FavoriteEntry{
		{ID: "FV:2/1", Title: "FM4", URI: "https://stream.example/fm4", Integratable: true},
	}, nil
}

func TestOrderedFavoriteProxySpeakers_DeprioritizeTV(t *testing.T) {
	speakers := []speaker.SpeakerInfo{
		{Name: "Fernseher", Reachable: true},
		{Name: "Kueche", Reachable: true},
		{Name: "EG: Wohnzimmer", Reachable: true},
	}
	got := orderedFavoriteProxySpeakers(speakers, "")
	want := []string{"EG: Wohnzimmer", "Kueche", "Fernseher"}
	if len(got) != len(want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got=%v want=%v", got, want)
		}
	}
}

func TestSonosFavorites_FallbackAfterTVBrowseFail(t *testing.T) {
	ctrl := &selectiveFavoritesCtrl{
		Controller: mock.New("Fernseher", "Wohnzimmer"),
		browseFail: map[string]bool{"Fernseher": true},
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := scenes.NewService(nil, ctrl, log)
	s := New(svc, ctrl, testToken, "", nil, web.Handler(), log)

	rec := do(t, s.Handler(), "GET", "/api/v1/sonos/favorites", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["speaker"] != "Wohnzimmer" {
		t.Fatalf("speaker=%v calls=%v", body["speaker"], ctrl.calls)
	}
	favs, _ := body["favorites"].([]any)
	if len(favs) != 1 {
		t.Fatalf("favorites=%v", body["favorites"])
	}
	if body["discovery_error"] != "" {
		t.Fatalf("discovery_error=%v", body["discovery_error"])
	}
	if len(ctrl.calls) != 1 || ctrl.calls[0] != "Wohnzimmer" {
		t.Fatalf("calls=%v", ctrl.calls)
	}
}
