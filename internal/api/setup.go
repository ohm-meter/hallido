package api

import (
	"crypto/rand"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/applog"
	"sonos-radio-agent/internal/config"
)

// First-run setup (ADR 0022). A fresh container has no API token. Until the
// setup wizard has created one, exactly one unauthenticated endpoint exists:
// POST /api/v1/setup/token. It works only
//   - while setup is active (no token yet, setup enabled by the operator),
//   - within the time window after start (DefaultSetupWindow),
//   - for requests from the home network (private/loopback/link-local client
//     address, no proxy headers, local Host name, same-origin),
//   - with the header X-Hallido-Setup: 1 (forces a CORS preflight, which is
//     never answered -> other websites cannot trigger it),
//   - once: the token is shown exactly once and never returned again.
// Afterwards the normal token requirement applies to everything.

// DefaultSetupWindow is how long setup stays open after the agent starts.
const DefaultSetupWindow = time.Hour

// SetupHeader must be sent with the token request (value "1").
const SetupHeader = "X-Hallido-Setup"

// setupTokenBytes: 20 random bytes = 160 bits, 32 characters in base32.
const setupTokenBytes = 20

// setupState is the first-run setup status.
type setupState struct {
	mu       sync.Mutex
	done     bool
	deadline time.Time
	now      func() time.Time
}

func (st *setupState) status() (done, expired bool, remaining time.Duration) {
	st.mu.Lock()
	defer st.mu.Unlock()
	now := st.now()
	return st.done, !now.Before(st.deadline), st.deadline.Sub(now)
}

// EnableSetup opens first-run setup for window (0 = DefaultSetupWindow). Call
// it only when the config has no usable token and the operator enabled setup.
func (s *Server) EnableSetup(window time.Duration) {
	if window <= 0 {
		window = DefaultSetupWindow
	}
	st := &setupState{now: time.Now}
	st.deadline = st.now().Add(window)
	s.setup = st
	s.log.Info("setup: waiting for first-time setup in the web app",
		"event", "setup", "open_until", st.deadline.Format(time.RFC3339),
		"note", "open the web app from a device in your home network")
	time.AfterFunc(window, func() {
		if done, _, _ := st.status(); !done {
			s.log.Warn("setup: time window closed without setup -- restart Hallido to try again",
				"event", "setup")
		}
	})
}

// setupStatusResponse is GET /api/v1/setup.
type setupStatusResponse struct {
	SetupRequired    bool   `json:"setup_required"`
	Open             bool   `json:"open"`
	Reason           string `json:"reason,omitempty"` // expired | not_local
	ExpiresInSeconds int    `json:"expires_in_seconds,omitempty"`
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	resp := setupStatusResponse{}
	if s.setup != nil {
		done, expired, remaining := s.setup.status()
		resp.SetupRequired = !done
		switch {
		case done:
		case expired:
			resp.Reason = "expired"
		case localRequestProblem(r) != "":
			resp.Reason = "not_local"
		default:
			resp.Open = true
			resp.ExpiresInSeconds = int(remaining.Seconds())
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

type setupTokenRequest struct {
	Timezone string `json:"timezone"`
}

func (s *Server) handleSetupToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	client := clientIP(r)
	if s.setup == nil {
		writeError(w, s.log, apperr.New(apperr.CodeAlreadySetUp, "Hallido is already set up").
			WithHint("Enter your access key in the web app (Speakers → Change API token)."))
		return
	}
	if problem := localRequestProblem(r); problem != "" {
		s.log.Warn("setup: request refused, not from the home network",
			"event", "setup", "client", client, "reason", problem)
		writeError(w, s.log, apperr.New(apperr.CodeForbidden,
			"Setup is only possible from your home network").
			WithHint("Open Hallido with the address of your NAS in the home network, e.g. http://192.168.1.20:8790, not through a remote-access service."))
		return
	}
	if r.Header.Get(SetupHeader) != "1" {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Setup header missing").
			WithHint("Send the header "+SetupHeader+": 1."))
		return
	}
	var req setupTokenRequest
	if !s.decodeBody(w, r, 1<<10, &req) {
		return
	}

	st := s.setup
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.done {
		writeError(w, s.log, apperr.New(apperr.CodeAlreadySetUp, "Hallido is already set up").
			WithHint("The access key was already created. Enter it in the web app (Speakers → Change API token)."))
		return
	}
	if !st.now().Before(st.deadline) {
		writeError(w, s.log, apperr.New(apperr.CodeSetupClosed, "The setup time window has closed").
			WithHint("Restart the Hallido container (Container Manager / Container Station) and open the web app again within one hour."))
		return
	}
	if !s.configEnabled() {
		writeError(w, s.log, apperr.New(apperr.CodeInternal, "No configuration file available"))
		return
	}

	token, err := newSetupToken()
	if err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeInternal, "Could not create a random access key"))
		return
	}
	s.cfgMu.Lock()
	err = config.SaveSetupToken(s.cfgPath, token, req.Timezone)
	s.cfgMu.Unlock()
	if errors.Is(err, config.ErrAlreadySetUp) {
		st.done = true
		writeError(w, s.log, apperr.New(apperr.CodeAlreadySetUp, "Hallido is already set up"))
		return
	}
	if err != nil {
		s.log.Error("setup: saving the access key failed", "event", "setup", "err", err)
		writeError(w, s.log, apperr.New(apperr.CodeInternal, "Could not save the configuration").
			WithHint("Check that the data volume (/data) is writable and has free space; details in the container log."))
		return
	}
	s.setToken(token)
	st.done = true
	// The wizard may have set the time zone: schedules created next use it.
	if cfg, err := config.ReadFile(s.cfgPath); err == nil && s.schedules != nil {
		s.schedules.SetLocation(cfg.Location())
	}
	s.log.Info("setup: access key created, setup closed", "event", "setup", "client", client)
	writeJSON(w, http.StatusCreated, map[string]string{"token": token})
}

// tokenAlphabet is Crockford base32 (no I, L, O, U): easy to read aloud and
// to type, 5 bits per character.
const tokenAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// newSetupToken returns 160 random bits as 8 groups of 4 characters
// (XXXX-XXXX-…), using crypto/rand.
func newSetupToken() (string, error) {
	buf := make([]byte, setupTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	var b strings.Builder
	var acc uint32
	bits, n := 0, 0
	for _, by := range buf {
		acc = acc<<8 | uint32(by)
		bits += 8
		for bits >= 5 {
			bits -= 5
			if n > 0 && n%4 == 0 {
				b.WriteByte('-')
			}
			b.WriteByte(tokenAlphabet[(acc>>bits)&31])
			n++
		}
	}
	return b.String(), nil
}

// proxyHeaders indicate a reverse proxy or tunnel: the real client is
// unknown, so setup is refused (a tunnel on the same host would otherwise
// look like loopback).
var proxyHeaders = []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host",
	"X-Real-IP", "CF-Connecting-IP", "True-Client-IP", "CF-Ray"}

// localHostSuffixes are names that only resolve in a home network (DNS
// rebinding: a public domain pointing to the NAS must not count as local).
var localHostSuffixes = []string{".local", ".lan", ".home", ".home.arpa",
	".internal", ".localdomain", ".fritz.box"}

// localRequestProblem returns "" if r comes from the home network, otherwise
// a short reason for the log.
func localRequestProblem(r *http.Request) string {
	for _, h := range proxyHeaders {
		if r.Header.Get(h) != "" {
			return "proxy header " + h
		}
	}
	addr, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil || !isLocalAddr(addr.Addr()) {
		return "client address not private"
	}
	host := hostOnly(r.Host)
	if !isLocalHostName(host) {
		return "host name not local"
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || !strings.EqualFold(u.Host, r.Host) {
			return "cross-origin request"
		}
	}
	return ""
}

func isLocalAddr(a netip.Addr) bool {
	a = a.Unmap()
	return a.IsValid() && (a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast())
}

func hostOnly(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return strings.Trim(hostport, "[]")
}

func isLocalHostName(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" {
		return false
	}
	if a, err := netip.ParseAddr(host); err == nil {
		return isLocalAddr(a)
	}
	if host == "localhost" || !strings.Contains(host, ".") {
		return true
	}
	for _, suf := range localHostSuffixes {
		if strings.HasSuffix(host, suf) {
			return true
		}
	}
	return false
}

// SetLogStore enables the log view (GET /api/v1/logs).
func (s *Server) SetLogStore(st *applog.Store) { s.logs = st }

// maxLogEntries caps one log response.
const maxLogEntries = 500

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "limit must be a positive number"))
			return
		}
		limit = min(n, maxLogEntries)
	}
	w.Header().Set("Cache-Control", "no-store")
	resp := map[string]any{"entries": []applog.Entry{}, "file": false}
	if s.logs != nil {
		resp["entries"] = s.logs.Entries(limit)
		resp["file"] = s.logs.FileEnabled()
	}
	writeJSON(w, http.StatusOK, resp)
}
