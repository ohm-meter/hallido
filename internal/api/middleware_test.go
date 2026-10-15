package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sonos-radio-agent/internal/speaker/mock"
)

func TestRateLimitSkipsStaticAssets(t *testing.T) {
	t.Parallel()
	s := &Server{rl: newRateLimiter(1, time.Minute)}
	h := s.limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("static request %d: status = %d", i, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first api request: status = %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second api request: status = %d, want 429", rec.Code)
	}
}

func TestRateLimitedResponseHasRetryAfterAndCode(t *testing.T) {
	t.Parallel()
	s := newTestServerStruct(mock.New(), "")
	s.rl = newRateLimiter(1, time.Minute)
	h := s.Handler()
	do(t, h, "GET", "/api/v1/health", "")
	rec := do(t, h, "GET", "/api/v1/health", "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("code = %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	if !strings.Contains(rec.Body.String(), `"error":"rate_limited"`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestRateLimiterSweepsStaleKeys(t *testing.T) {
	t.Parallel()
	rl := newRateLimiter(5, 10*time.Millisecond)
	rl.allow("a")
	rl.allow("b")
	time.Sleep(30 * time.Millisecond)
	rl.allow("c")
	rl.mu.Lock()
	n := len(rl.hits)
	rl.mu.Unlock()
	if n != 1 {
		t.Fatalf("verwaiste Keys muessen entfernt werden, hits=%d", n)
	}
}

func TestUnknownAPIPathReturnsJSON404(t *testing.T) {
	t.Parallel()
	h := newTestServer(mock.New())
	rec := do(t, h, "GET", "/api/v1/gibtsnicht", testToken)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"error":"not_found"`) {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

// Jede geschuetzte Route muss ohne Token 401 liefern -- faengt neue Routen
// ab, bei denen requireToken vergessen wurde.
func TestProtectedRoutesRequireToken(t *testing.T) {
	t.Parallel()
	h := newTestServerWithConfig(mock.New(), "")
	routes := []struct{ method, path string }{
		{"GET", "/api/v1/scenes"},
		{"GET", "/api/v1/scenes/status"},
		{"GET", "/api/v1/speakers"},
		{"GET", "/api/v1/speakers/now-playing"},
		{"GET", "/api/v1/speakers/stream-sources"},
		{"GET", "/api/v1/speakers/x/now-playing"},
		{"POST", "/api/v1/discovery/refresh"},
		{"POST", "/api/v1/scenes/fm4/play"},
		{"GET", "/api/v1/config"},
		{"POST", "/api/v1/config/speakers/sync"},
		{"POST", "/api/v1/config/scenes"},
		{"PUT", "/api/v1/config/scenes/fm4"},
		{"DELETE", "/api/v1/config/scenes/fm4"},
		{"GET", "/api/v1/sonos/favorites"},
		{"GET", "/api/v1/radio/search?q=fm4"},
		{"POST", "/api/v1/config/scenes/from-favorite"},
	}
	for _, r := range routes {
		for _, tok := range []string{"", "falsch"} {
			rec := do(t, h, r.method, r.path, tok)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s %s (token %q): code = %d, want 401", r.method, r.path, tok, rec.Code)
			}
		}
	}
}

func TestLeeresServerTokenAkzeptiertNie(t *testing.T) {
	t.Parallel()
	s := &Server{token: "", log: newTestServerStruct(mock.New(), "").log}
	h := s.requireToken(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d", rec.Code)
	}
}
