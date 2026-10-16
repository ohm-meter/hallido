package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"sonos-radio-agent/internal/applog"
	"sonos-radio-agent/internal/config"
	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/schedule"
	"sonos-radio-agent/internal/speaker/mock"
	"sonos-radio-agent/internal/web"
)

// newSetupServer: fresh installation (no config file, no token), setup open.
func newSetupServer(t *testing.T) (*Server, http.Handler, string) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctrl := mock.New("Kitchen")
	svc := scenes.NewService(nil, ctrl, log)
	s := New(svc, ctrl, "", cfgPath, nil, web.Handler(), log)
	s.SetScheduler(schedule.New(nil, nil, svc, schedule.RealClock{}, log))
	s.EnableSetup(time.Hour)
	return s, s.Handler(), cfgPath
}

// lanRequest is a request from a phone in the home network to the NAS.
func lanRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "192.168.1.50:51234"
	req.Host = "192.168.1.20:8790"
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(SetupHeader, "1")
		req.Header.Set("Origin", "http://192.168.1.20:8790")
	}
	return req
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func setupStatus(t *testing.T, h http.Handler, req *http.Request) setupStatusResponse {
	t.Helper()
	rec := serve(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d: %s", rec.Code, rec.Body)
	}
	var st setupStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	return st
}

var tokenFormat = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{4}(-[0-9A-HJKMNP-TV-Z]{4}){7}$`)

func TestSetup_HappyPath_TokenOnceThenNormalAuth(t *testing.T) {
	srv, h, cfgPath := newSetupServer(t)

	st := setupStatus(t, h, lanRequest("GET", "/api/v1/setup", ""))
	if !st.SetupRequired || !st.Open || st.ExpiresInSeconds <= 0 {
		t.Fatalf("setup should be open: %+v", st)
	}
	// Before setup every protected endpoint stays closed.
	if rec := serve(h, lanRequest("GET", "/api/v1/scenes", "")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("scenes without token: %d", rec.Code)
	}

	rec := serve(h, lanRequest("POST", "/api/v1/setup/token", `{"timezone":"Europe/Berlin"}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("token: code %d: %s", rec.Code, rec.Body)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	var out struct{ Token string }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if !tokenFormat.MatchString(out.Token) {
		t.Fatalf("token format: %q", out.Token)
	}

	cfg, err := config.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIToken != out.Token || cfg.Timezone != "Europe/Berlin" || cfg.Scenes == nil {
		t.Fatalf("config not written as expected: %+v", cfg)
	}
	info, _ := os.Stat(cfgPath)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode %v", info.Mode().Perm())
	}
	if got := srv.schedules.Location().String(); got != "Europe/Berlin" {
		t.Fatalf("scheduler time zone = %q", got)
	}
	if _, needs, err := config.LoadForSetup(cfgPath); err != nil || needs {
		t.Fatalf("restart after setup: needs=%v err=%v", needs, err)
	}

	// Second attempt: never a second token.
	if rec := serve(h, lanRequest("POST", "/api/v1/setup/token", `{}`)); rec.Code != http.StatusConflict ||
		strings.Contains(rec.Body.String(), out.Token) {
		t.Fatalf("second token request: %d %s", rec.Code, rec.Body)
	}
	if st := setupStatus(t, h, lanRequest("GET", "/api/v1/setup", "")); st.SetupRequired || st.Open {
		t.Fatalf("setup should be closed: %+v", st)
	}
	// Normal token requirement now applies.
	req := lanRequest("GET", "/api/v1/scenes", "")
	req.Header.Set("Authorization", "Bearer "+out.Token)
	if rec := serve(h, req); rec.Code != http.StatusOK {
		t.Fatalf("scenes with new token: %d", rec.Code)
	}
}

func TestSetup_RefusesNonLocalRequests(t *testing.T) {
	cases := map[string]func(r *http.Request){
		"public client": func(r *http.Request) { r.RemoteAddr = "203.0.113.7:4000" },
		"cgnat client":  func(r *http.Request) { r.RemoteAddr = "100.64.1.2:4000" },
		"proxy header":  func(r *http.Request) { r.Header.Set("X-Forwarded-For", "203.0.113.7") },
		"cloudflare": func(r *http.Request) {
			r.RemoteAddr = "127.0.0.1:4000"
			r.Header.Set("CF-Connecting-IP", "203.0.113.7")
		},
		"public host": func(r *http.Request) { r.Host = "hallido.example.com"; r.Header.Del("Origin") },
		"rebinding host": func(r *http.Request) {
			r.Host = "nas.attacker.example:8790"
			r.Header.Set("Origin", "http://nas.attacker.example:8790")
		},
		"public host ip":  func(r *http.Request) { r.Host = "203.0.113.9:8790"; r.Header.Del("Origin") },
		"foreign origin":  func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") },
		"null origin":     func(r *http.Request) { r.Header.Set("Origin", "null") },
		"quickconnect":    func(r *http.Request) { r.Host = "mynas.quickconnect.to"; r.Header.Del("Origin") },
		"bad remote addr": func(r *http.Request) { r.RemoteAddr = "garbage" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			_, h, cfgPath := newSetupServer(t)
			req := lanRequest("POST", "/api/v1/setup/token", `{}`)
			mutate(req)
			rec := serve(h, req)
			if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"forbidden"`) {
				t.Fatalf("code %d: %s", rec.Code, rec.Body)
			}
			if _, err := os.Stat(cfgPath); err == nil {
				t.Fatal("config must not be written")
			}
			get := lanRequest("GET", "/api/v1/setup", "")
			mutate(get)
			if st := setupStatus(t, h, get); st.Open || st.Reason != "not_local" {
				t.Fatalf("status: %+v", st)
			}
		})
	}
}

func TestSetup_LocalVariantsAccepted(t *testing.T) {
	for _, c := range []struct{ remote, host string }{
		{"10.0.0.5:1", "nas:8790"},
		{"172.20.0.1:1", "diskstation.local:8790"},
		{"[fd00::5]:1", "[fd00::1]:8790"},
		{"[fe80::1%eth0]:1", "nas.fritz.box:8790"},
		{"127.0.0.1:1", "localhost:8790"},
		{"[::ffff:192.168.0.9]:1", "nas.home.arpa"},
	} {
		_, h, _ := newSetupServer(t)
		req := lanRequest("POST", "/api/v1/setup/token", `{}`)
		req.RemoteAddr, req.Host = c.remote, c.host
		req.Header.Del("Origin")
		if rec := serve(h, req); rec.Code != http.StatusCreated {
			t.Errorf("%+v: code %d: %s", c, rec.Code, rec.Body)
		}
	}
}

func TestSetup_RequiresHeaderAndValidBody(t *testing.T) {
	_, h, _ := newSetupServer(t)
	req := lanRequest("POST", "/api/v1/setup/token", `{}`)
	req.Header.Del(SetupHeader)
	if rec := serve(h, req); rec.Code != http.StatusBadRequest {
		t.Fatalf("without header: %d", rec.Code)
	}
	for _, body := range []string{"", `{"x":1}`, `{"timezone":"` + strings.Repeat("a", 2000) + `"}`} {
		if rec := serve(h, lanRequest("POST", "/api/v1/setup/token", body)); rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q: %d", body[:min(len(body), 20)], rec.Code)
		}
	}
	// Invalid time zone is ignored, setup still works.
	if rec := serve(h, lanRequest("POST", "/api/v1/setup/token", `{"timezone":"Mars/Base"}`)); rec.Code != http.StatusCreated {
		t.Fatalf("invalid timezone should be ignored: %d", rec.Code)
	}
}

func TestSetup_WindowExpired(t *testing.T) {
	s, h, cfgPath := newSetupServer(t)
	s.setup.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if st := setupStatus(t, h, lanRequest("GET", "/api/v1/setup", "")); st.Open || st.Reason != "expired" || !st.SetupRequired {
		t.Fatalf("status: %+v", st)
	}
	rec := serve(h, lanRequest("POST", "/api/v1/setup/token", `{}`))
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "setup_closed") {
		t.Fatalf("code %d: %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(cfgPath); err == nil {
		t.Fatal("config must not be written")
	}
}

func TestSetup_NotEnabled(t *testing.T) {
	h := newTestServer(mock.New("Wohnzimmer"))
	if st := setupStatus(t, h, lanRequest("GET", "/api/v1/setup", "")); st.SetupRequired || st.Open {
		t.Fatalf("status: %+v", st)
	}
	if rec := serve(h, lanRequest("POST", "/api/v1/setup/token", `{}`)); rec.Code != http.StatusConflict {
		t.Fatalf("code %d", rec.Code)
	}
}

func TestSetup_KeepsExistingScenes(t *testing.T) {
	_, h, cfgPath := newSetupServer(t)
	existing := `{"listen":":8790","scenes":[{"id":"a","name":"A","source":{"type":"stream","uri":"https://x"},"members":[{"speaker":"Kitchen","volume":10}]}],"timezone":"Europe/Vienna"}`
	if err := os.WriteFile(cfgPath, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	if rec := serve(h, lanRequest("POST", "/api/v1/setup/token", `{"timezone":"Europe/Berlin"}`)); rec.Code != http.StatusCreated {
		t.Fatalf("code %d: %s", rec.Code, rec.Body)
	}
	cfg, err := config.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Scenes) != 1 || cfg.Listen != ":8790" || cfg.Timezone != "Europe/Vienna" || cfg.APIToken == "" {
		t.Fatalf("existing config changed: %+v", cfg)
	}
}

func TestSetup_ConfigAlreadyHasToken(t *testing.T) {
	_, h, cfgPath := newSetupServer(t)
	if err := os.WriteFile(cfgPath, []byte(`{"api_token":"from-elsewhere","scenes":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if rec := serve(h, lanRequest("POST", "/api/v1/setup/token", `{}`)); rec.Code != http.StatusConflict {
		t.Fatalf("code %d: %s", rec.Code, rec.Body)
	}
	cfg, _ := config.ReadFile(cfgPath)
	if cfg.APIToken != "from-elsewhere" {
		t.Fatal("existing token overwritten")
	}
}

func TestSetup_UnwritableConfig(t *testing.T) {
	s, _, _ := newSetupServer(t)
	blocker := filepath.Join(t.TempDir(), "file")
	_ = os.WriteFile(blocker, nil, 0o600)
	s.cfgPath = filepath.Join(blocker, "config.json")
	rec := serve(s.Handler(), lanRequest("POST", "/api/v1/setup/token", `{}`))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code %d", rec.Code)
	}
	if st := setupStatus(t, s.Handler(), lanRequest("GET", "/api/v1/setup", "")); !st.Open {
		t.Fatal("setup must stay open after a failed save")
	}
}

func TestNewSetupToken_FormatAndRandomness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		tok, err := newSetupToken()
		if err != nil {
			t.Fatal(err)
		}
		if !tokenFormat.MatchString(tok) || !config.HasUsableToken(tok) {
			t.Fatalf("format: %q", tok)
		}
		if seen[tok] {
			t.Fatal("duplicate token")
		}
		seen[tok] = true
	}
}

func TestLogs_Endpoint(t *testing.T) {
	var out bytes.Buffer
	log, st, err := applog.New(&out, applog.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctrl := mock.New("Wohnzimmer")
	s := New(scenes.NewService(nil, ctrl, log), ctrl, testToken, "", nil, web.Handler(), log)
	s.SetLogStore(st)
	log.Info("agent started", "api_token", "never")
	log.Info("scene started", "scene", "fm4")
	h := s.Handler()

	rec := do(t, h, "GET", "/api/v1/logs?limit=1", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d", rec.Code)
	}
	var resp struct {
		Entries []applog.Entry
		File    bool
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Entries) != 1 || resp.Entries[0].Msg != "scene started" || resp.File {
		t.Fatalf("resp: %+v", resp)
	}
	rec = do(t, h, "GET", "/api/v1/logs", testToken)
	if strings.Contains(rec.Body.String(), "never") {
		t.Fatal("secret in log response")
	}
	if rec := do(t, h, "GET", "/api/v1/logs?limit=x", testToken); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad limit: %d", rec.Code)
	}
	// Without a store: empty list.
	rec = do(t, newTestServer(ctrl), "GET", "/api/v1/logs", testToken)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"entries":[]`) {
		t.Fatalf("no store: %d %s", rec.Code, rec.Body)
	}
}
