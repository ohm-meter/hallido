package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sonos-radio-agent/internal/config"
	"sonos-radio-agent/internal/speaker"
	"sonos-radio-agent/internal/speaker/mock"
)

func TestSonosFavorites_Mock(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	h := newTestServer(ctrl)
	rec := do(t, h, "GET", "/api/v1/sonos/favorites", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	favs, _ := body["favorites"].([]any)
	if len(favs) != 3 {
		t.Fatalf("favorites=%v", body["favorites"])
	}
	if raw := rec.Body.String(); strings.Contains(raw, "resMD") || strings.Contains(raw, "DIDL") {
		t.Fatalf("interne Metadaten duerfen nicht in der API landen: %s", raw)
	}
	if body["speaker"] == "" {
		t.Fatal("speaker fehlt")
	}
}

func TestSonosFavorites_QuerySpeaker(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	h := newTestServer(ctrl)
	rec := do(t, h, "GET", "/api/v1/sonos/favorites?speaker=Kueche", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["speaker"] != "Kueche" {
		t.Fatalf("speaker=%v", body["speaker"])
	}
}

func TestSceneFromFavorite_Mock(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg := `{"listen":":8080","api_token":"x","scenes":[]}`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	ctrl := mock.New("Wohnzimmer")
	h := newTestServerWithConfig(ctrl, cfgPath)
	payload := `{"scene_id":"fm4-test","name":"FM4 Test","favorite_id":"FV:2/1","members":[{"speaker":"Wohnzimmer","volume":20}]}`
	rec := doBody(t, h, "POST", "/api/v1/config/scenes/from-favorite", testToken, strings.NewReader(payload))
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["scene_id"] != "fm4-test" {
		t.Fatalf("resp=%v", resp)
	}
}

func TestSceneFromFavorite_NotIntegratable(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{"listen":":8080","api_token":"x","scenes":[]}`), 0o600)
	ctrl := mock.New("Wohnzimmer")
	h := newTestServerWithConfig(ctrl, cfgPath)
	payload := `{"scene_id":"spot","name":"Spotify","favorite_id":"FV:2/2","members":[{"speaker":"Wohnzimmer","volume":10}]}`
	rec := doBody(t, h, "POST", "/api/v1/config/scenes/from-favorite", testToken, strings.NewReader(payload))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSceneFromFavorite_TuneInWirdFavoritenSzene(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{"listen":":8080","api_token":"x","scenes":[]}`), 0o600)
	ctrl := mock.New("Wohnzimmer")
	h := newTestServerWithConfig(ctrl, cfgPath)
	payload := `{"scene_id":"superfly-fm","name":"Superfly Fm","favorite_id":"FV:2/3","members":[{"speaker":"Wohnzimmer","volume":10}]}`
	rec := doBody(t, h, "POST", "/api/v1/config/scenes/from-favorite", testToken, strings.NewReader(payload))
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["source_type"] != "favorite" {
		t.Fatalf("resp=%v", resp)
	}
	cfg, err := config.ReadFile(cfgPath)
	if err != nil || len(cfg.Scenes) != 1 {
		t.Fatalf("config: %v %+v", err, cfg)
	}
	src := cfg.Scenes[0].Source
	if src.Type != speaker.SourceFavorite || src.Name != "Radio Superfly" || !strings.HasPrefix(src.URI, "x-sonosapi-stream:tunein") {
		t.Fatalf("Quelle: %+v", src)
	}

	if rec := do(t, h, "POST", "/api/v1/scenes/superfly-fm/play", testToken); rec.Code != http.StatusOK {
		t.Fatalf("play: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, h, "GET", "/api/v1/scenes/status", testToken)
	var status struct {
		Scenes []struct {
			ID    string `json:"scene_id"`
			State string `json:"state"`
		} `json:"scenes"`
	}
	json.Unmarshal(rec.Body.Bytes(), &status)
	var state string
	for _, sc := range status.Scenes {
		if sc.ID == "superfly-fm" {
			state = sc.State
		}
	}
	if state != "playing" {
		t.Fatalf("Status erkennt laufenden TuneIn-Favoriten nicht (state=%q): %s", state, rec.Body.String())
	}
}

func TestSceneUpsert_FavoritenQuelle(t *testing.T) {
	cfgPath := writeTestConfig(t)
	ctrl := mock.New("Wohnzimmer")
	h := newTestServerWithConfig(ctrl, cfgPath)
	ok := `{"scene":{"id":"fav","name":"Fav","source":{"type":"favorite","name":" Radio Superfly "},"members":[{"speaker":"Wohnzimmer","volume":5}]}}`
	if rec := doBody(t, h, "POST", "/api/v1/config/scenes", testToken, strings.NewReader(ok)); rec.Code != http.StatusOK {
		t.Fatalf("favorite: %d %s", rec.Code, rec.Body.String())
	}
	cfg, _ := config.ReadFile(cfgPath)
	var found bool
	for _, sc := range cfg.Scenes {
		if sc.ID == "fav" {
			found = sc.Source.Name == "Radio Superfly"
		}
	}
	if !found {
		t.Fatal("Favoritenname nicht getrimmt gespeichert")
	}
	bad := `{"scene":{"id":"fav2","name":"Fav","source":{"type":"favorite","name":"Jazz","uri":"x-sonosapi-radio:x?sid=201"},"members":[{"speaker":"Wohnzimmer","volume":5}]}}`
	if rec := doBody(t, h, "POST", "/api/v1/config/scenes", testToken, strings.NewReader(bad)); rec.Code != http.StatusBadRequest {
		t.Fatalf("x-sonosapi-radio: erwartet 400, bekam %d", rec.Code)
	}
}
