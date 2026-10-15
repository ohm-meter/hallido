package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"sonos-radio-agent/internal/apperr"
)

// securityHeaders setzt konservative Header (OWASP Secure Headers).
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		// Restriktive CSP: nur eigene Assets, keine Inline-Skripte von Dritten.
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'")
		next.ServeHTTP(w, r)
	})
}

// requireToken prueft das Bearer-Token in konstanter Zeit (gegen Timing-Angriffe).
func (s *Server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	// Hash-Vergleich: konstante Zeit auch bezueglich der Token-Laenge.
	want := sha256.Sum256([]byte(s.token))
	tokenSet := s.token != ""
	return func(w http.ResponseWriter, r *http.Request) {
		got := sha256.Sum256([]byte(bearer(r)))
		// Leeres Server-Token akzeptiert nie (fail closed).
		if !tokenSet || subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
			writeError(w, s.log, apperr.New(apperr.CodeUnauthorized,
				"Invalid or missing API token").
				WithHint("Set the header 'Authorization: Bearer <token>'."))
			return
		}
		next(w, r)
	}
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const p = "Bearer "
	if strings.HasPrefix(h, p) {
		return strings.TrimSpace(h[len(p):])
	}
	return ""
}

// rateLimiter ist ein einfacher Token-Bucket je Client-IP. Bewusst simpel
// gehalten -- es geht um Missbrauchsschutz im LAN, nicht um harte SLAs.
type rateLimiter struct {
	mu        sync.Mutex
	hits      map[string][]time.Time
	limit     int
	window    time.Duration
	lastSweep time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{hits: make(map[string][]time.Time), limit: limit, window: window}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-rl.window)
	// Verwaiste Keys (wechselnde Quell-IPs) regelmaessig entfernen.
	if now.Sub(rl.lastSweep) > rl.window {
		for k, ts := range rl.hits {
			if len(ts) == 0 || !ts[len(ts)-1].After(cutoff) {
				delete(rl.hits, k)
			}
		}
		rl.lastSweep = now
	}
	kept := rl.hits[key][:0]
	for _, t := range rl.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.limit {
		rl.hits[key] = kept
		return false
	}
	rl.hits[key] = append(kept, now)
	return true
}

func (s *Server) limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Statische GUI-Assets nicht gegen API-Limit zaehlen (Playwright laedt pro Test HTML/CSS/JS).
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		ip := clientIP(r)
		if !s.rl.allow(ip) {
			w.Header().Set("Retry-After", strconv.Itoa(int(s.rl.window.Seconds())))
			writeError(w, s.log, apperr.New(apperr.CodeRateLimited,
				"Too many requests, please wait a moment").
				WithHint("Limit applies per client IP; GUI tabs and flows share the budget."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if i := strings.LastIndex(r.RemoteAddr, ":"); i >= 0 {
		return r.RemoteAddr[:i]
	}
	return r.RemoteAddr
}

// logRequests protokolliert jede Anfrage knapp.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.log.Debug("request", "method", r.Method, "path", r.URL.Path,
			"dur", time.Since(start).String())
	})
}
