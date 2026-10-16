package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sonos-radio-agent/internal/radio"
	"sonos-radio-agent/internal/speaker/mock"
)

func TestRadioSearch_MockAPI(t *testing.T) {
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{"name":"FM4","url":"https://streams.fm4.orf.at/fm4.mp3","country":"Austria","tags":"alternative"},
			{"name":"Spotify","url":"spotify:track:abc","country":""}
		]`))
	}))
	defer mockAPI.Close()
	client := radio.NewClient().WithBaseURL(mockAPI.URL).WithHTTPClient(mockAPI.Client())
	h := newTestServerRadio(client)
	rec := do(t, h, "GET", "/api/v1/radio/search?q=fm4&limit=5", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	stations, _ := body["stations"].([]any)
	if len(stations) != 1 {
		t.Fatalf("stations=%v", body["stations"])
	}
}

func TestRadioSearch_MissingQuery(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer"))
	rec := do(t, h, "GET", "/api/v1/radio/search", testToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestRadioSearch_UpstreamError(t *testing.T) {
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer mockAPI.Close()
	client := radio.NewClient().WithBaseURL(mockAPI.URL).WithHTTPClient(mockAPI.Client())
	h := newTestServerRadio(client)
	rec := do(t, h, "GET", "/api/v1/radio/search?q=test", testToken)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func newTestServerRadio(client *radio.Client) http.Handler {
	s := newTestServerStruct(mock.New("Wohnzimmer"), "")
	s.radio = client
	return s.Handler()
}

func TestRadioSearch_Disabled(t *testing.T) {
	srv := newTestServerStruct(mock.New("Wohnzimmer"), "")
	srv.DisableRadioSearch()
	rec := do(t, srv.Handler(), "GET", "/api/v1/radio/search?q=fm4", testToken)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "turned off") {
		t.Fatalf("code %d: %s", rec.Code, rec.Body)
	}
}
