package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"sonos-radio-agent/internal/config"
	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/schedule"
	"sonos-radio-agent/internal/speaker/mock"
	"sonos-radio-agent/internal/web"
)

// newScheduleTestServer baut Server + Scheduler (Fake-Uhr, Mittwoch 07:00 Wien).
func newScheduleTestServer(t *testing.T, ctrl *mock.Controller) (http.Handler, *schedule.Scheduler, string) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfgPath := writeTestConfig(t)
	svc := scenes.NewService([]scenes.Scene{testScene()}, ctrl, log)
	loc, _ := schedule.LoadLocation("")
	clock := schedule.NewFakeClock(time.Date(2026, 10, 14, 7, 0, 0, 0, loc))
	sch := schedule.New(nil, loc, svc, clock, log)
	srv := New(svc, ctrl, testToken, cfgPath, map[string]string{"Wohnzimmer": "192.168.1.1"}, web.Handler(), log)
	srv.SetScheduler(sch)
	return srv.Handler(), sch, cfgPath
}

const oe3Body = `{"schedule":{"scene":"fm4","days":["mon","tue","wed","thu","fri"],"start":"06:10","stop":"08:30","enabled":true}}`

func TestSchedules_RequireToken(t *testing.T) {
	h, _, _ := newScheduleTestServer(t, mock.New("Wohnzimmer", "Kueche"))
	for _, rt := range [][2]string{
		{"GET", "/api/v1/schedules"},
		{"GET", "/api/v1/config/schedules"},
		{"POST", "/api/v1/config/schedules"},
		{"PUT", "/api/v1/config/schedules/x"},
		{"DELETE", "/api/v1/config/schedules/x"},
		{"POST", "/api/v1/scenes/fm4/stop"},
	} {
		if rec := do(t, h, rt[0], rt[1], ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: 401 erwartet, bekam %d", rt[0], rt[1], rec.Code)
		}
	}
}

func TestSchedules_CRUDAndHotReload(t *testing.T) {
	h, sch, cfgPath := newScheduleTestServer(t, mock.New("Wohnzimmer", "Kueche"))

	rec := doBody(t, h, "POST", "/api/v1/config/schedules", testToken, strings.NewReader(oe3Body))
	if rec.Code != http.StatusOK {
		t.Fatalf("post: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ScheduleID string `json:"schedule_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ScheduleID != "fm4-0610" {
		t.Fatalf("id = %q", created.ScheduleID)
	}
	if len(sch.Statuses()) != 1 {
		t.Fatal("hot-reload: scheduler kennt den zeitplan nicht")
	}

	rec = do(t, h, "GET", "/api/v1/config/schedules", testToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"timezone":"Europe/Vienna"`) ||
		!strings.Contains(rec.Body.String(), `"fm4-0610"`) {
		t.Fatalf("get config: %d %s", rec.Code, rec.Body.String())
	}

	// Laufzeit-Status: Mittwoch 07:00 -> aktiv, naechster Start Donnerstag 06:10.
	rec = do(t, h, "GET", "/api/v1/schedules", testToken)
	var status struct {
		Timezone  string `json:"timezone"`
		Schedules []struct {
			ID        string     `json:"id"`
			SceneName string     `json:"scene_name"`
			Active    bool       `json:"active"`
			NextStart *time.Time `json:"next_start"`
			NextStop  *time.Time `json:"next_stop"`
		} `json:"schedules"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	got := status.Schedules[0]
	if !got.Active || got.SceneName != "FM4" || got.NextStart == nil ||
		got.NextStart.Format(time.RFC3339) != "2026-10-15T06:10:00+02:00" ||
		got.NextStop.Format(time.RFC3339) != "2026-10-14T08:30:00+02:00" {
		t.Fatalf("status: %s", rec.Body.String())
	}

	// Ausschalten per PUT.
	rec = doBody(t, h, "PUT", "/api/v1/config/schedules/fm4-0610", testToken,
		strings.NewReader(`{"scene":"fm4","days":["mon"],"start":"06:10","enabled":false}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	if st := sch.Statuses()[0]; st.Enabled || st.NextStart != nil {
		t.Fatalf("nach put: %+v", st)
	}

	rec = do(t, h, "DELETE", "/api/v1/config/schedules/fm4-0610", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	cfg, _ := config.ReadFile(cfgPath)
	if len(cfg.Schedules) != 0 || len(sch.Statuses()) != 0 {
		t.Fatalf("delete nicht wirksam: cfg=%v", cfg.Schedules)
	}
}

func TestSchedules_Errors(t *testing.T) {
	h, _, _ := newScheduleTestServer(t, mock.New("Wohnzimmer", "Kueche"))
	cases := []struct {
		method, path, body string
		code               int
		errCode            string
	}{
		{"POST", "/api/v1/config/schedules", `{"schedule":{"scene":"gibtsnicht","days":["mon"],"start":"06:10"}}`, 400, "bad_request"},
		{"POST", "/api/v1/config/schedules", `{"schedule":{"scene":"fm4","days":["montag"],"start":"06:10"}}`, 400, "bad_request"},
		{"POST", "/api/v1/config/schedules", `{"schedule":{"scene":"fm4","days":["mon"],"start":"25:00"}}`, 400, "bad_request"},
		{"POST", "/api/v1/config/schedules", `{"schedule":{"scene":"fm4","days":["mon"],"begin":"06:10"}}`, 400, "bad_request"},
		{"PUT", "/api/v1/config/schedules/fehlt", `{"scene":"fm4","days":["mon"],"start":"06:10"}`, 404, "schedule_not_found"},
		{"PUT", "/api/v1/config/schedules/a", `{"id":"b","scene":"fm4","days":["mon"],"start":"06:10"}`, 400, "bad_request"},
		{"DELETE", "/api/v1/config/schedules/fehlt", "", 404, "schedule_not_found"},
	}
	for _, c := range cases {
		var body io.Reader
		if c.body != "" {
			body = strings.NewReader(c.body)
		}
		rec := doBody(t, h, c.method, c.path, testToken, body)
		if rec.Code != c.code || !strings.Contains(rec.Body.String(), `"error":"`+c.errCode+`"`) {
			t.Errorf("%s %s %s: %d %s", c.method, c.path, c.body, rec.Code, rec.Body.String())
		}
	}
}

func TestSceneDelete_BlockedBySchedule(t *testing.T) {
	h, _, _ := newScheduleTestServer(t, mock.New("Wohnzimmer", "Kueche"))
	if rec := doBody(t, h, "POST", "/api/v1/config/schedules", testToken, strings.NewReader(oe3Body)); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	rec := do(t, h, "DELETE", "/api/v1/config/scenes/fm4", testToken)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "schedule") {
		t.Fatalf("szene mit zeitplan: %d %s", rec.Code, rec.Body.String())
	}
}

func TestStopScene(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	h, _, _ := newScheduleTestServer(t, ctrl)
	if rec := do(t, h, "POST", "/api/v1/scenes/fm4/play", testToken); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	rec := do(t, h, "POST", "/api/v1/scenes/fm4/stop", testToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"stopped":true`) {
		t.Fatalf("stop: %d %s", rec.Code, rec.Body.String())
	}
	if len(ctrl.Stops) != 1 {
		t.Fatalf("stops = %v", ctrl.Stops)
	}

	if rec := do(t, h, "POST", "/api/v1/scenes/gibtsnicht/stop", testToken); rec.Code != http.StatusNotFound {
		t.Fatalf("unbekannte szene: %d", rec.Code)
	}
	ctrl.SetReachable("Wohnzimmer", false)
	rec = do(t, h, "POST", "/api/v1/scenes/fm4/stop", testToken)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "speaker_unreachable") {
		t.Fatalf("offline: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSchedulesStatus_WithoutScheduler(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer"))
	if rec := do(t, h, "GET", "/api/v1/schedules", testToken); rec.Code != http.StatusBadRequest {
		t.Fatalf("ohne scheduler 400 erwartet, bekam %d", rec.Code)
	}
}
