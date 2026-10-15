package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sonos-radio-agent/internal/config"
	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/speaker"
	"sonos-radio-agent/internal/speaker/mock"
)

const testConfigJSON = `{
  "listen": ":8080",
  "api_token": "test-token",
  "speakers": { "Wohnzimmer": { "ip": "192.168.1.10" } },
  "scenes": [
    {
      "id": "fm4",
      "name": "FM4",
      "source": { "type": "stream", "uri": "https://example/fm4" },
      "members": [ { "speaker": "Wohnzimmer", "volume": 30 } ]
    }
  ]
}`

func writeTestConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	if err := os.WriteFile(p, []byte(testConfigJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGetConfig_NoToken(t *testing.T) {
	h := newTestServerWithConfig(mock.New("Wohnzimmer"), writeTestConfig(t))
	rec := do(t, h, "GET", "/api/v1/config", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("401 erwartet, bekam %d", rec.Code)
	}
}

func TestGetConfig_OK(t *testing.T) {
	h := newTestServerWithConfig(mock.New("Wohnzimmer"), writeTestConfig(t))
	rec := do(t, h, "GET", "/api/v1/config", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("200 erwartet, bekam %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "test-token") {
		t.Fatal("token darf nicht in antwort stehen")
	}
	if !strings.Contains(rec.Body.String(), "fm4") {
		t.Fatalf("scene_ids fehlen: %s", rec.Body.String())
	}
}

func TestSpeakersSync_MergesIPs(t *testing.T) {
	cfgPath := writeTestConfig(t)
	ctrl := mock.New("Wohnzimmer", "Kueche")
	h := newTestServerWithConfig(ctrl, cfgPath)

	body := strings.NewReader(`{"speakers":[{"name":"Kueche","ip":"192.168.1.99"}]}`)
	rec := doBody(t, h, "POST", "/api/v1/config/speakers/sync", testToken, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("200 erwartet, bekam %d: %s", rec.Code, rec.Body.String())
	}
	cfg, err := config.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Speakers["Kueche"].IP != "192.168.1.99" {
		t.Fatalf("kueche ip: %+v", cfg.Speakers)
	}
	if cfg.Speakers["Wohnzimmer"].IP != "192.168.1.10" {
		t.Fatal("wohnzimmer soll unveraendert bleiben")
	}
	if cfg.APIToken != "test-token" {
		t.Fatal("token verloren")
	}
}

func TestSpeakersSync_HintNoSpeakerRestart(t *testing.T) {
	h := newTestServerWithConfig(mock.New("Wohnzimmer"), writeTestConfig(t))
	body := strings.NewReader(`{"speakers":[{"name":"Kueche","ip":"192.168.1.99"}]}`)
	rec := doBody(t, h, "POST", "/api/v1/config/speakers/sync", testToken, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("200 erwartet, bekam %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	hint, _ := resp["restart_hint"].(string)
	if strings.Contains(hint, "Speaker-IPs") || strings.Contains(hint, "erst nach Neustart") {
		t.Fatalf("veralteter neustart-hinweis: %q", hint)
	}
}

func TestSceneUpsert_AddAndHotReload(t *testing.T) {
	cfgPath := writeTestConfig(t)
	ctrl := mock.New("Wohnzimmer", "Kueche")
	h := newTestServerWithConfig(ctrl, cfgPath)

	payload := `{
	  "scene": {
	    "id": "neu-radio",
	    "name": "Neu Radio",
	    "source": { "type": "stream", "uri": "https://radio.example/live?x=1" },
	    "members": [
	      { "speaker": "Wohnzimmer", "volume": 22 },
	      { "speaker": "Kueche", "volume": 18 }
	    ]
	  }
	}`
	rec := doBody(t, h, "POST", "/api/v1/config/scenes", testToken, strings.NewReader(payload))
	if rec.Code != http.StatusOK {
		t.Fatalf("200 erwartet, bekam %d: %s", rec.Code, rec.Body.String())
	}

	cfg, _ := config.ReadFile(cfgPath)
	var neu scenes.Scene
	for _, sc := range cfg.Scenes {
		if sc.ID == "neu-radio" {
			neu = sc
		}
	}
	if neu.Source.URI != "https://radio.example/live" {
		t.Fatalf("uri normalisiert: %q", neu.Source.URI)
	}

	rec = do(t, h, "POST", "/api/v1/scenes/neu-radio/play", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("play hot-reload: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSceneUpsert_NormalizesNowPlayingURI(t *testing.T) {
	cfgPath := writeTestConfig(t)
	ctrl := mock.New("Wohnzimmer")
	ctrl.SetNowPlaying("Wohnzimmer", speaker.NowPlayingInfo{
		State: speaker.PlaybackPlaying,
		URI:   "x-rincon-mp3radio://stream.host/radio/live?sid=1",
	})
	h := newTestServerWithConfig(ctrl, cfgPath)

	payload := `{
	  "scene": {
	    "id": "von-box",
	    "name": "Von Box",
	    "source": { "type": "stream", "uri": "x-rincon-mp3radio://stream.host/radio/live?sid=1" },
	    "members": [ { "speaker": "Wohnzimmer", "volume": 20 } ]
	  }
	}`
	rec := doBody(t, h, "POST", "/api/v1/config/scenes", testToken, strings.NewReader(payload))
	if rec.Code != http.StatusOK {
		t.Fatalf("200 erwartet, bekam %d: %s", rec.Code, rec.Body.String())
	}
	cfg, _ := config.ReadFile(cfgPath)
	for _, sc := range cfg.Scenes {
		if sc.ID == "von-box" && sc.Source.URI != "https://stream.host/radio/live" {
			t.Fatalf("normalisiert: %q", sc.Source.URI)
		}
	}
}

func TestSceneUpsert_RejectsFileURI(t *testing.T) {
	h := newTestServerWithConfig(mock.New("Wohnzimmer"), writeTestConfig(t))
	payload := `{
	  "scene": {
	    "id": "evil",
	    "name": "Evil",
	    "source": { "type": "stream", "uri": "file:///etc/passwd" },
	    "members": [ { "speaker": "Wohnzimmer", "volume": 10 } ]
	  }
	}`
	rec := doBody(t, h, "POST", "/api/v1/config/scenes", testToken, strings.NewReader(payload))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("400 erwartet, bekam %d", rec.Code)
	}
}

func TestSceneUpsert_RejectsUnknownFields(t *testing.T) {
	h := newTestServerWithConfig(mock.New("Wohnzimmer"), writeTestConfig(t))
	payload := `{"scene":{"id":"x","name":"X","hack":true,"source":{"type":"stream","uri":"https://a/b"},"members":[{"speaker":"Wohnzimmer","volume":1}]}}`
	rec := doBody(t, h, "POST", "/api/v1/config/scenes", testToken, strings.NewReader(payload))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("400 erwartet, bekam %d", rec.Code)
	}
}

func TestGetConfig_DisabledWithoutPath(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer"))
	rec := do(t, h, "GET", "/api/v1/config", testToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("400 erwartet, bekam %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
}

func TestSceneDelete_RemovesAndHotReload(t *testing.T) {
	cfgPath := writeTestConfig(t)
	ctrl := mock.New("Wohnzimmer")
	h := newTestServerWithConfig(ctrl, cfgPath)

	rec := do(t, h, "DELETE", "/api/v1/config/scenes/fm4", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("200 erwartet, bekam %d: %s", rec.Code, rec.Body.String())
	}
	cfg, err := config.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, sc := range cfg.Scenes {
		if sc.ID == "fm4" {
			t.Fatal("fm4 soll aus config entfernt sein")
		}
	}

	rec = do(t, h, "POST", "/api/v1/scenes/fm4/play", testToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("404 nach delete, bekam %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSceneDelete_NotFound(t *testing.T) {
	h := newTestServerWithConfig(mock.New("Wohnzimmer"), writeTestConfig(t))
	rec := do(t, h, "DELETE", "/api/v1/config/scenes/unknown-scene", testToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("404 erwartet, bekam %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSceneDelete_RejectsInvalidID(t *testing.T) {
	h := newTestServerWithConfig(mock.New("Wohnzimmer"), writeTestConfig(t))
	rec := do(t, h, "DELETE", "/api/v1/config/scenes/INVALID", testToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("400 erwartet, bekam %d", rec.Code)
	}
}

func TestSceneDelete_DisabledWithoutPath(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer"))
	rec := do(t, h, "DELETE", "/api/v1/config/scenes/fm4", testToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("400 erwartet, bekam %d", rec.Code)
	}
}

func TestSceneUpdate_ReplacesAndHotReload(t *testing.T) {
	cfgPath := writeTestConfig(t)
	ctrl := mock.New("Wohnzimmer", "Kueche")
	h := newTestServerWithConfig(ctrl, cfgPath)

	payload := `{
	  "name": "FM4 Updated",
	  "source": { "type": "stream", "uri": "https://example/fm4-new" },
	  "members": [
	    { "speaker": "Wohnzimmer", "volume": 40 },
	    { "speaker": "Kueche", "volume": 35 }
	  ]
	}`
	rec := doBody(t, h, "PUT", "/api/v1/config/scenes/fm4", testToken, strings.NewReader(payload))
	if rec.Code != http.StatusOK {
		t.Fatalf("200 erwartet, bekam %d: %s", rec.Code, rec.Body.String())
	}

	cfg, err := config.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var updated scenes.Scene
	for _, sc := range cfg.Scenes {
		if sc.ID == "fm4" {
			updated = sc
		}
	}
	if updated.Name != "FM4 Updated" {
		t.Fatalf("name: %q", updated.Name)
	}
	if updated.Source.URI != "https://example/fm4-new" {
		t.Fatalf("uri: %q", updated.Source.URI)
	}
	if len(updated.Members) != 2 {
		t.Fatalf("members: %+v", updated.Members)
	}

	rec = do(t, h, "POST", "/api/v1/scenes/fm4/play", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("play hot-reload: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSceneUpdate_NotFound(t *testing.T) {
	h := newTestServerWithConfig(mock.New("Wohnzimmer"), writeTestConfig(t))
	payload := `{
	  "name": "X",
	  "source": { "type": "stream", "uri": "https://example/x" },
	  "members": [ { "speaker": "Wohnzimmer", "volume": 10 } ]
	}`
	rec := doBody(t, h, "PUT", "/api/v1/config/scenes/unknown-scene", testToken, strings.NewReader(payload))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("404 erwartet, bekam %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSceneUpdate_RejectsFileURI(t *testing.T) {
	h := newTestServerWithConfig(mock.New("Wohnzimmer"), writeTestConfig(t))
	payload := `{
	  "name": "Evil",
	  "source": { "type": "stream", "uri": "file:///etc/passwd" },
	  "members": [ { "speaker": "Wohnzimmer", "volume": 10 } ]
	}`
	rec := doBody(t, h, "PUT", "/api/v1/config/scenes/fm4", testToken, strings.NewReader(payload))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("400 erwartet, bekam %d", rec.Code)
	}
}

func TestSceneUpdate_RejectsUnknownFields(t *testing.T) {
	h := newTestServerWithConfig(mock.New("Wohnzimmer"), writeTestConfig(t))
	payload := `{"name":"X","hack":true,"source":{"type":"stream","uri":"https://a/b"},"members":[{"speaker":"Wohnzimmer","volume":1}]}`
	rec := doBody(t, h, "PUT", "/api/v1/config/scenes/fm4", testToken, strings.NewReader(payload))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("400 erwartet, bekam %d", rec.Code)
	}
}

func TestSpeakersSync_Fehlerpfade(t *testing.T) {
	cases := map[string]string{
		"leer":             `{"speakers":[]}`,
		"ohne Name":        `{"speakers":[{"name":"","ip":"192.168.1.5"}]}`,
		"doppelt":          `{"speakers":[{"name":"A","ip":"192.168.1.5"},{"name":"A","ip":"192.168.1.6"}]}`,
		"Hostname-IP":      `{"speakers":[{"name":"A","ip":"evil.example/x"}]}`,
		"kaputtes JSON":    `{"speakers":`,
		"unbekanntes Feld": `{"speakers":[],"x":1}`,
	}
	for name, body := range cases {
		cfgPath := writeTestConfig(t)
		h := newTestServerWithConfig(mock.New("Wohnzimmer"), cfgPath)
		rec := doBody(t, h, "POST", "/api/v1/config/speakers/sync", testToken, strings.NewReader(body))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: code = %d, want 400 (%s)", name, rec.Code, rec.Body.String())
		}
	}
}

func TestSpeakersSync_LeererBodyUndOhneConfig(t *testing.T) {
	h := newTestServerWithConfig(mock.New("Wohnzimmer"), writeTestConfig(t))
	if rec := doBody(t, h, "POST", "/api/v1/config/speakers/sync", testToken, strings.NewReader("")); rec.Code != http.StatusBadRequest {
		t.Errorf("leerer Body: %d", rec.Code)
	}
	h = newTestServerWithConfig(mock.New("Wohnzimmer"), "")
	if rec := doBody(t, h, "POST", "/api/v1/config/speakers/sync", testToken, strings.NewReader(`{"speakers":[]}`)); rec.Code != http.StatusBadRequest {
		t.Errorf("Config-Schreiben deaktiviert: %d", rec.Code)
	}
}

func TestSceneUpsert_AktualisiertBekannteBoxen(t *testing.T) {
	h := newTestServerWithConfig(mock.New("Wohnzimmer"), writeTestConfig(t))
	body := `{"scene":{"id":"neu","name":"Neu","source":{"type":"stream","uri":"https://x.example/a"},"members":[{"speaker":"Gaestezimmer","volume":10}]}}`
	if rec := doBody(t, h, "POST", "/api/v1/config/scenes", testToken, strings.NewReader(body)); rec.Code != http.StatusOK {
		t.Fatalf("upsert: %d %s", rec.Code, rec.Body.String())
	}
	rec := do(t, h, "GET", "/api/v1/speakers", testToken)
	if !strings.Contains(rec.Body.String(), "Gaestezimmer") {
		t.Fatalf("neue Box fehlt ohne Neustart: %s", rec.Body.String())
	}
}

func TestSceneUpdateDelete_NichtGefundenIstSceneNotFound(t *testing.T) {
	h := newTestServerWithConfig(mock.New("Wohnzimmer"), writeTestConfig(t))
	if rec := do(t, h, "DELETE", "/api/v1/config/scenes/gibtsnicht", testToken); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "scene_not_found") {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}
