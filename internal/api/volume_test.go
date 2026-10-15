package api

import (
	"net/http"
	"strings"
	"testing"

	"sonos-radio-agent/internal/speaker/mock"
)

func TestSetSpeakerVolume_RequiresToken(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer"))
	rec := doBody(t, h, "PUT", "/api/v1/speakers/Wohnzimmer/volume", "", strings.NewReader(`{"volume":20}`))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestSetSpeakerVolume_OK(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	h := newTestServer(ctrl)
	rec := doBody(t, h, "PUT", "/api/v1/speakers/Wohnzimmer/volume", testToken, strings.NewReader(`{"volume":35}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"volume":35`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if ctrl.Volumes["Wohnzimmer"] != 35 {
		t.Fatalf("volume = %d", ctrl.Volumes["Wohnzimmer"])
	}

	// Grenzwerte 0 und 100 sind erlaubt.
	for _, body := range []string{`{"volume":0}`, `{"volume":100}`} {
		if rec := doBody(t, h, "PUT", "/api/v1/speakers/Kueche/volume", testToken, strings.NewReader(body)); rec.Code != http.StatusOK {
			t.Fatalf("%s: code = %d", body, rec.Code)
		}
	}
	if ctrl.Volumes["Kueche"] != 100 {
		t.Fatalf("kueche = %d", ctrl.Volumes["Kueche"])
	}
}

func TestSetSpeakerVolume_BadRequest(t *testing.T) {
	ctrl := mock.New("Wohnzimmer")
	h := newTestServer(ctrl)
	cases := map[string]string{
		"leer":       ``,
		"kein json":  `laut`,
		"fehlt":      `{}`,
		"zu hoch":    `{"volume":101}`,
		"negativ":    `{"volume":-1}`,
		"kein int":   `{"volume":"20"}`,
		"extra feld": `{"volume":20,"mute":true}`,
		"kommazahl":  `{"volume":20.5}`,
	}
	for name, body := range cases {
		rec := doBody(t, h, "PUT", "/api/v1/speakers/Wohnzimmer/volume", testToken, strings.NewReader(body))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"bad_request"`) {
			t.Errorf("%s: code = %d body = %s", name, rec.Code, rec.Body.String())
		}
	}
	if _, set := ctrl.Volumes["Wohnzimmer"]; set {
		t.Fatalf("Lautstaerke trotz Fehler gesetzt: %v", ctrl.Volumes)
	}
}

func TestSetSpeakerVolume_Unreachable(t *testing.T) {
	ctrl := mock.New("Wohnzimmer")
	ctrl.SetReachable("Wohnzimmer", false)
	h := newTestServer(ctrl)
	rec := doBody(t, h, "PUT", "/api/v1/speakers/Wohnzimmer/volume", testToken, strings.NewReader(`{"volume":20}`))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "speaker_unreachable") || !strings.Contains(body, `"speaker":"Wohnzimmer"`) {
		t.Fatalf("body = %s", body)
	}
}

func TestSetSpeakerVolume_WrongMethod(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer"))
	rec := do(t, h, "POST", "/api/v1/speakers/Wohnzimmer/volume", testToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d", rec.Code)
	}
}
