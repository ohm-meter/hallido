package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/speaker"
	"sonos-radio-agent/internal/speaker/mock"
)

func TestGroupRoutes_RequireToken(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer", "Kueche"))
	for _, rt := range []struct{ method, path string }{
		{"POST", "/api/v1/scenes/fm4/pause"},
		{"POST", "/api/v1/scenes/fm4/resume"},
		{"POST", "/api/v1/speakers/Kueche/stop"},
		{"POST", "/api/v1/speakers/Kueche/pause"},
		{"POST", "/api/v1/speakers/Kueche/resume"},
		{"POST", "/api/v1/speakers/Kueche/ungroup"},
		{"GET", "/api/v1/groups"},
	} {
		if rec := do(t, h, rt.method, rt.path, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s ohne token: %d", rt.method, rt.path, rec.Code)
		}
	}
}

func TestScenePauseResume(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	h := newTestServer(ctrl)
	if rec := do(t, h, "POST", "/api/v1/scenes/fm4/play", testToken); rec.Code != http.StatusOK {
		t.Fatalf("play: %d %s", rec.Code, rec.Body)
	}
	rec := do(t, h, "POST", "/api/v1/scenes/fm4/pause", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("pause: %d %s", rec.Code, rec.Body)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["paused"] != true || body["scene_id"] != "fm4" {
		t.Fatalf("pause body: %v", body)
	}
	rec = do(t, h, "POST", "/api/v1/scenes/fm4/resume", testToken)
	json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusOK || body["resumed"] != true {
		t.Fatalf("resume: %d %s", rec.Code, rec.Body)
	}
	if len(ctrl.Pauses) != 1 || len(ctrl.Resumes) != 1 {
		t.Fatalf("pauses=%v resumes=%v", ctrl.Pauses, ctrl.Resumes)
	}
}

func TestScenePauseResume_Errors(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	h := newTestServer(ctrl)
	if rec := do(t, h, "POST", "/api/v1/scenes/nope/pause", testToken); rec.Code != http.StatusNotFound {
		t.Fatalf("unbekannte szene: %d", rec.Code)
	}
	rec := do(t, h, "POST", "/api/v1/scenes/fm4/resume", testToken)
	if rec.Code != http.StatusBadGateway || errorCode(t, rec.Body.Bytes()) != "playback_failed" {
		t.Fatalf("resume ohne quelle: %d %s", rec.Code, rec.Body)
	}
	ctrl.SetReachable("Wohnzimmer", false)
	rec = do(t, h, "POST", "/api/v1/scenes/fm4/pause", testToken)
	if rec.Code != http.StatusBadGateway || errorCode(t, rec.Body.Bytes()) != "speaker_unreachable" {
		t.Fatalf("offline: %d %s", rec.Code, rec.Body)
	}
}

func errorCode(t *testing.T, raw []byte) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("json: %v (%s)", err, raw)
	}
	return body.Error
}

// Eine in der Sonos-App gebaute Gruppe laesst sich ueber jedes Mitglied steuern.
func TestSpeakerGroupActions_AppGroup(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche", "KG: Fernseher")
	ctrl.SetGroup("KG: Fernseher", "Kueche")
	ctrl.SetNowPlaying("KG: Fernseher", speaker.NowPlayingInfo{State: speaker.PlaybackPlaying, Title: "Spotify"})
	h := newTestServer(ctrl)

	for _, action := range []string{"pause", "resume", "stop"} {
		rec := do(t, h, "POST", "/api/v1/speakers/Kueche/"+action, testToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", action, rec.Code, rec.Body)
		}
		var res scenes.GroupActionResult
		json.Unmarshal(rec.Body.Bytes(), &res)
		if !res.OK || res.Coordinator != "KG: Fernseher" || string(res.Action) != action ||
			!reflect.DeepEqual(res.Members, []string{"KG: Fernseher", "Kueche"}) {
			t.Fatalf("%s: %+v", action, res)
		}
	}
	// Name mit Doppelpunkt/Leerzeichen URL-kodiert.
	rec := do(t, h, "POST", "/api/v1/speakers/"+url.PathEscape("KG: Fernseher")+"/pause", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("kodierter name: %d %s", rec.Code, rec.Body)
	}
	if want := []string{"KG: Fernseher", "KG: Fernseher"}; !reflect.DeepEqual(ctrl.Pauses, want) {
		t.Fatalf("pauses=%v", ctrl.Pauses)
	}
}

func TestSpeakerGroupActions_Errors(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	h := newTestServer(ctrl)

	rec := do(t, h, "POST", "/api/v1/speakers/Garage/stop", testToken)
	if rec.Code != http.StatusBadGateway || errorCode(t, rec.Body.Bytes()) != "speaker_unreachable" {
		t.Fatalf("unbekannte box: %d %s", rec.Code, rec.Body)
	}
	// Resume ohne Quelle: Fehler + Ergebnis (welche Gruppe getroffen wurde).
	rec = do(t, h, "POST", "/api/v1/speakers/Kueche/resume", testToken)
	var body struct {
		Error  string                   `json:"error"`
		Result scenes.GroupActionResult `json:"result"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusBadGateway || body.Error != "playback_failed" || body.Result.Coordinator != "Kueche" {
		t.Fatalf("resume ohne quelle: %d %s", rec.Code, rec.Body)
	}
}

func TestSpeakerUngroup(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche", "Bad")
	ctrl.SetGroup("Wohnzimmer", "Kueche", "Bad")
	ctrl.RejectUngroup("Bad")
	h := newTestServer(ctrl)

	// Coordinator: Gruppe aufloesen, Bad lehnt ab -> Teilausfall, trotzdem 200.
	rec := do(t, h, "POST", "/api/v1/speakers/Wohnzimmer/ungroup", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("ungroup: %d %s", rec.Code, rec.Body)
	}
	var res scenes.UngroupResult
	json.Unmarshal(rec.Body.Bytes(), &res)
	if !res.Dissolved || len(res.Speakers) != 2 {
		t.Fatalf("res: %+v", res)
	}
	for _, st := range res.Speakers {
		if (st.Speaker == "Bad") == st.OK {
			t.Fatalf("pro-box-status falsch: %+v", res.Speakers)
		}
	}

	// Bad haengt noch an Wohnzimmer und lehnt weiter ab -> 502 mit result.
	rec = do(t, h, "POST", "/api/v1/speakers/Bad/ungroup", testToken)
	var errBody struct {
		Error  string               `json:"error"`
		Result scenes.UngroupResult `json:"result"`
	}
	json.Unmarshal(rec.Body.Bytes(), &errBody)
	if rec.Code != http.StatusBadGateway || errBody.Error != "playback_failed" || len(errBody.Result.Speakers) != 1 {
		t.Fatalf("alle gescheitert: %d %s", rec.Code, rec.Body)
	}

	// Allein stehende Box: 200, standalone.
	rec = do(t, h, "POST", "/api/v1/speakers/Kueche/ungroup", testToken)
	json.Unmarshal(rec.Body.Bytes(), &res)
	if rec.Code != http.StatusOK || !res.Standalone {
		t.Fatalf("standalone: %d %s", rec.Code, rec.Body)
	}
}

func TestListGroups(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche", "Bad")
	h := newTestServer(ctrl)
	if rec := do(t, h, "POST", "/api/v1/scenes/fm4/play", testToken); rec.Code != http.StatusOK {
		t.Fatalf("play: %d", rec.Code)
	}
	rec := do(t, h, "GET", "/api/v1/groups", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("groups: %d %s", rec.Code, rec.Body)
	}
	var body struct {
		Groups []scenes.GroupStatus `json:"groups"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Groups) != 2 || body.Groups[1].Coordinator != "Wohnzimmer" || body.Groups[1].State != "playing" ||
		body.Groups[1].SceneID != "fm4" || len(body.Groups[1].Members) != 2 || body.Groups[0].State != "stopped" {
		t.Fatalf("groups: %+v", body.Groups)
	}

	for _, n := range []string{"Wohnzimmer", "Kueche", "Bad"} {
		ctrl.SetReachable(n, false)
	}
	rec = do(t, h, "GET", "/api/v1/groups", testToken)
	if rec.Code != http.StatusBadGateway || errorCode(t, rec.Body.Bytes()) != "speaker_unreachable" {
		t.Fatalf("alle offline: %d %s", rec.Code, rec.Body)
	}
}
