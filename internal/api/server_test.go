package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/speaker"
	"sonos-radio-agent/internal/speaker/mock"
	"sonos-radio-agent/internal/web"
)

const testToken = "test-token"

func testScene() scenes.Scene {
	return scenes.Scene{
		ID:     "fm4",
		Name:   "FM4",
		Source: speaker.Source{Type: speaker.SourceStream, URI: "https://example/fm4"},
		Members: []scenes.Member{
			{Speaker: "Wohnzimmer", Volume: 30},
			{Speaker: "Kueche", Volume: 20},
		},
	}
}

func newTestServer(ctrl *mock.Controller) http.Handler {
	return newTestServerWithConfig(ctrl, "")
}

func newTestServerWithConfig(ctrl *mock.Controller, cfgPath string) http.Handler {
	return newTestServerStruct(ctrl, cfgPath).Handler()
}

func newTestServerStruct(ctrl *mock.Controller, cfgPath string) *Server {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	sc := testScene()
	svc := scenes.NewService([]scenes.Scene{sc}, ctrl, log)
	known := map[string]string{
		"Wohnzimmer": "192.168.1.1",
		"Kueche":     "192.168.1.2",
	}
	return New(svc, ctrl, testToken, cfgPath, known, web.Handler(), log)
}

func do(t *testing.T, h http.Handler, method, path, token string) *httptest.ResponseRecorder {
	return doBody(t, h, method, path, token, nil)
}

func doBody(t *testing.T, h http.Handler, method, path, token string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestListSpeakers_IncludesConfiguredNames(t *testing.T) {
	ctrl := mock.New()
	ctrl.SetDiscovery([]speaker.SpeakerInfo{})
	h := newTestServerStruct(ctrl, "").Handler()
	rec := do(t, h, "GET", "/api/v1/speakers", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var body struct {
		Speakers []speaker.SpeakerInfo `json:"speakers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(body.Speakers) != 2 {
		t.Fatalf("erwarte 2 konfigurierte speaker, bekam %d: %+v", len(body.Speakers), body.Speakers)
	}
	names := map[string]bool{}
	for _, sp := range body.Speakers {
		names[sp.Name] = true
	}
	if !names["Wohnzimmer"] || !names["Kueche"] {
		t.Fatalf("fehlende konfigurierte namen: %+v", body.Speakers)
	}
}

func TestHealth_NoTokenNeeded(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer", "Kueche"))
	rec := do(t, h, "GET", "/api/v1/health", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("health code = %d", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["speakers_online"].(float64) != 2 {
		t.Fatalf("erwarte 2 boxen online, bekam %v", body["speakers_online"])
	}
}

func TestListScenes_RequiresToken(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer", "Kueche"))

	if rec := do(t, h, "GET", "/api/v1/scenes", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne token erwarte 401, bekam %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/api/v1/scenes", "falsch"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("falsches token erwarte 401, bekam %d", rec.Code)
	}
	rec := do(t, h, "GET", "/api/v1/scenes", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("mit token erwarte 200, bekam %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "\"fm4\"") {
		t.Fatalf("szenenliste sollte fm4 enthalten: %s", rec.Body.String())
	}
}

func TestListSpeakers_RequiresToken(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer", "Kueche"))

	if rec := do(t, h, "GET", "/api/v1/speakers", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne token erwarte 401, bekam %d", rec.Code)
	}
	rec := do(t, h, "GET", "/api/v1/speakers", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("mit token erwarte 200, bekam %d", rec.Code)
	}
	var body struct {
		Speakers       []speaker.SpeakerInfo `json:"speakers"`
		DiscoveryError string                `json:"discovery_error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(body.Speakers) != 2 {
		t.Fatalf("erwarte 2 speaker, bekam %d", len(body.Speakers))
	}
	if body.Speakers[0].Name == "" || body.Speakers[0].IP == "" || body.Speakers[0].UID == "" {
		t.Fatalf("speaker-info unvollstaendig: %+v", body.Speakers[0])
	}
	if body.DiscoveryError != "" {
		t.Fatalf("discovery_error soll leer sein, bekam %q", body.DiscoveryError)
	}
}

func TestPlayScene_UnknownMember(t *testing.T) {
	ctrl := mock.New("Wohnzimmer")
	ctrl.SetDiscovery([]speaker.SpeakerInfo{
		{Name: "Wohnzimmer", IP: "192.168.1.10", UID: "RINCON_W", Reachable: true},
	})
	h := newTestServer(ctrl)
	rec := do(t, h, "POST", "/api/v1/scenes/fm4/play", testToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("erwarte 400, bekam %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "scene_misconfigured") {
		t.Fatalf("erwarte scene_misconfigured: %s", rec.Body.String())
	}
}

func TestNowPlaying_RequiresToken(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer", "Kueche"))
	if rec := do(t, h, "GET", "/api/v1/speakers/Wohnzimmer/now-playing", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne token erwarte 401, bekam %d", rec.Code)
	}
}

func TestNowPlaying_OK(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	ctrl.SetNowPlaying("Wohnzimmer", speaker.NowPlayingInfo{
		State:  speaker.PlaybackPlaying,
		Title:  "FM4",
		Artist: "ORF",
		Album:  "Radio",
		URI:    "x-rincon-mp3radio://example",
	})
	h := newTestServer(ctrl)
	rec := do(t, h, "GET", "/api/v1/speakers/Wohnzimmer/now-playing", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("mit token erwarte 200, bekam %d, body=%s", rec.Code, rec.Body.String())
	}
	var body speaker.NowPlayingInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body.Speaker != "Wohnzimmer" || body.State != speaker.PlaybackPlaying {
		t.Fatalf("unexpected body: %+v", body)
	}
	if body.Title != "FM4" || body.Artist != "ORF" {
		t.Fatalf("metadata missing: %+v", body)
	}
}

func TestScenesStatus_RequiresToken(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer", "Kueche"))

	if rec := do(t, h, "GET", "/api/v1/scenes/status", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne token erwarte 401, bekam %d", rec.Code)
	}
	rec := do(t, h, "GET", "/api/v1/scenes/status", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("mit token erwarte 200, bekam %d", rec.Code)
	}
	var body struct {
		Scenes []scenes.ScenePlaybackStatus `json:"scenes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(body.Scenes) != 1 || body.Scenes[0].SceneID != "fm4" {
		t.Fatalf("erwartete fm4-status, bekam %+v", body.Scenes)
	}
}

func TestSpeakersNowPlayingBulk_OK(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	ctrl.SetNowPlaying("Wohnzimmer", speaker.NowPlayingInfo{
		State: speaker.PlaybackPlaying,
		Title: "FM4",
		URI:   "x-rincon-mp3radio://example/fm4",
	})
	h := newTestServer(ctrl)
	rec := do(t, h, "GET", "/api/v1/speakers/now-playing", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("mit token erwarte 200, bekam %d, body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Speakers []speaker.NowPlayingInfo `json:"speakers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(body.Speakers) != 2 {
		t.Fatalf("erwarte 2 speaker, bekam %d", len(body.Speakers))
	}
	if body.Speakers[0].Title != "FM4" && body.Speakers[1].Title != "FM4" {
		t.Fatalf("fm4 titel fehlt: %+v", body.Speakers)
	}
}

func TestSpeakersStreamSources_FiltersIntegratable(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	ctrl.SetNowPlaying("Wohnzimmer", speaker.NowPlayingInfo{
		State: speaker.PlaybackPlaying,
		Title: "FM4",
		URI:   "x-rincon-mp3radio://example/fm4",
	})
	ctrl.SetNowPlaying("Kueche", speaker.NowPlayingInfo{
		State: speaker.PlaybackPlaying,
		Title: "Spotify",
		URI:   "spotify:track:abc",
	})
	h := newTestServer(ctrl)
	rec := do(t, h, "GET", "/api/v1/speakers/stream-sources", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("erwarte 200, bekam %d, body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Speakers []speaker.NowPlayingInfo `json:"speakers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(body.Speakers) != 1 {
		t.Fatalf("erwarte 1 integratable, bekam %d: %+v", len(body.Speakers), body.Speakers)
	}
	if body.Speakers[0].Speaker != "Wohnzimmer" {
		t.Fatalf("erwarte Wohnzimmer, bekam %+v", body.Speakers[0])
	}
	if !body.Speakers[0].Integratable {
		t.Fatal("integratable soll true sein")
	}
}

func TestDiscoveryRefresh_OK(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer", "Kueche"))
	rec := do(t, h, "POST", "/api/v1/discovery/refresh", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("erwarte 200, bekam %d", rec.Code)
	}
	var body struct {
		Speakers []speaker.SpeakerInfo `json:"speakers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(body.Speakers) != 2 {
		t.Fatalf("erwarte 2 speaker, bekam %d", len(body.Speakers))
	}
}

func TestPlayScene_OK(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer", "Kueche"))
	rec := do(t, h, "POST", "/api/v1/scenes/fm4/play", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("play code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var res scenes.PlayResult
	json.Unmarshal(rec.Body.Bytes(), &res)
	if !res.Playing {
		t.Fatal("erwarte playing=true")
	}
}

func TestPlayScene_NotFound(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer", "Kueche"))
	rec := do(t, h, "POST", "/api/v1/scenes/nope/play", testToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("erwarte 404, bekam %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "scene_not_found") {
		t.Fatalf("erwarte scene_not_found code: %s", rec.Body.String())
	}
}

func TestPlayScene_CoordinatorDown(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	ctrl.SetReachable("Wohnzimmer", false)
	h := newTestServer(ctrl)
	rec := do(t, h, "POST", "/api/v1/scenes/fm4/play", testToken)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("erwarte 502, bekam %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "speaker_unreachable") {
		t.Fatalf("erwarte speaker_unreachable: %s", rec.Body.String())
	}
}

func TestGUI_Served(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer"))
	rec := do(t, h, "GET", "/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("gui code = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `class="brand-text"`) {
		t.Fatal("gui sollte das markup enthalten")
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer"))
	rec := do(t, h, "GET", "/api/v1/health", "")
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("nosniff-header fehlt")
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("csp-header fehlt")
	}
}

func TestScenesStatus_KurzCacheUndInvalidierungNachPlay(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	h := newTestServer(ctrl)
	first := do(t, h, "GET", "/api/v1/scenes/status", testToken).Body.String()
	if second := do(t, h, "GET", "/api/v1/scenes/status", testToken).Body.String(); second != first {
		t.Fatal("zweite Antwort innerhalb der TTL muss aus dem Cache kommen")
	}
	if rec := do(t, h, "POST", "/api/v1/scenes/fm4/play", testToken); rec.Code != http.StatusOK {
		t.Fatalf("play: %d %s", rec.Code, rec.Body.String())
	}
	after := do(t, h, "GET", "/api/v1/scenes/status", testToken).Body.String()
	if after == first {
		t.Fatalf("nach Play muss der Status frisch berechnet werden: %s", after)
	}
}
