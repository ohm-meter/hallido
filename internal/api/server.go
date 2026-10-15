// Package api stellt die HTTP-Schnittstelle bereit: REST-API (von GUI und
// spaeter Homey genutzt) plus die ausgelieferte GUI. Alle state-aendernden
// Endpoints sind tokengeschuetzt.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/radio"
	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/schedule"
	"sonos-radio-agent/internal/speaker"
)

// playTimeout begrenzt, wie lange ein Szenen-Start auf Geraete wartet.
// Gruppierung + Wiedergabe auf mehreren Boxen braucht bei aufwachenden Geraeten
// deutlich mehr Zeit als ein einzelner SOAP-Aufruf.
const playTimeout = 45 * time.Second

// MaxRequestDuration ist die laengste Bearbeitungszeit eines Requests (Szenen-Start);
// Server-Write-Timeout und Graceful Shutdown richten sich danach.
const MaxRequestDuration = playTimeout

// statusTimeout begrenzt die Abfrage aller Szenen-Transportzustaende.
const statusTimeout = 15 * time.Second

// statusCacheTTL: Gueltigkeit der gebuendelten Status-/Now-Playing-Antworten.
const statusCacheTTL = 3 * time.Second

// nowPlayingTimeout begrenzt die Now-Playing-Abfrage einer Box.
const nowPlayingTimeout = 10 * time.Second

// discoveryTimeout begrenzt SSDP + Geraetebeschreibungen (viele Boxen parallel).
const discoveryTimeout = 10 * time.Second

// rateLimitPerMinute: Anfragen je Client-IP und Minute. Ein GUI-Tab pollt
// ca. 30 Requests/min (Status, Now-Playing, Health); Reserve fuer mehrere Tabs und Homey.
const rateLimitPerMinute = 300

// Server buendelt Abhaengigkeiten der HTTP-Schicht.
type Server struct {
	scenes        *scenes.Service
	ctrl          speaker.SpeakerController
	token         string
	cfgPath       string
	knownMu       sync.RWMutex
	knownSpeakers map[string]string // Raumname -> IP (leer = nur Name aus Config/Szenen)
	cfgMu         sync.Mutex
	log           *slog.Logger
	rl            *rateLimiter
	gui           http.Handler
	radio         radioSearcher
	statusCache   ttlCache
	npCache       ttlCache
	schedules     *schedule.Scheduler // nil = Zeitplaene nur in Config, ohne Laufzeit
}

// New erstellt den Server. gui ist der Handler fuer die statische Oberflaeche.
// cfgPath ist der Pfad zur config.json fuer Lese-/Schreib-Endpoints (leer = deaktiviert).
// knownSpeakers enthaelt alle aus Config/Szenen referenzierten Raumnamen (IP optional).
func New(svc *scenes.Service, ctrl speaker.SpeakerController, token, cfgPath string, knownSpeakers map[string]string, gui http.Handler, log *slog.Logger) *Server {
	return &Server{
		scenes:        svc,
		ctrl:          ctrl,
		token:         token,
		cfgPath:       cfgPath,
		knownSpeakers: cloneSpeakerMap(knownSpeakers),
		log:           log,
		rl:            newRateLimiter(rateLimitPerMinute, time.Minute),
		gui:           gui,
		radio:         radio.NewClient(),
	}
}

// SetScheduler haengt den laufenden Scheduler ein (Status + Hot-Reload).
func (s *Server) SetScheduler(sch *schedule.Scheduler) {
	s.schedules = sch
}

func cloneSpeakerMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Handler baut den Router mit der Middleware-Kette.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Offen (kein Token): Health und die GUI-Seite selbst.
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	// Catch-all: GUI-Dateien; unbekannte /api/-Pfade bekommen den stabilen
	// JSON-Fehler statt der Datei-404 der GUI.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.Method != http.MethodGet {
			s.handleAPINotFound(w, r)
			return
		}
		s.gui.ServeHTTP(w, r)
	})

	// Geschuetzt: Szenen lesen und ausloesen.
	mux.HandleFunc("GET /api/v1/scenes", s.requireToken(s.handleListScenes))
	mux.HandleFunc("GET /api/v1/scenes/status", s.requireToken(s.handleScenesStatus))
	mux.HandleFunc("GET /api/v1/speakers", s.requireToken(s.handleListSpeakers))
	mux.HandleFunc("GET /api/v1/speakers/now-playing", s.requireToken(s.handleSpeakersNowPlaying))
	mux.HandleFunc("GET /api/v1/speakers/stream-sources", s.requireToken(s.handleSpeakersStreamSources))
	mux.HandleFunc("GET /api/v1/speakers/{name}/now-playing", s.requireToken(s.handleSpeakerNowPlaying))
	mux.HandleFunc("PUT /api/v1/speakers/{name}/volume", s.requireToken(s.handleSetSpeakerVolume))
	mux.HandleFunc("POST /api/v1/discovery/refresh", s.requireToken(s.handleDiscoveryRefresh))
	mux.HandleFunc("POST /api/v1/scenes/{id}/play", s.requireToken(s.handlePlayScene))
	mux.HandleFunc("POST /api/v1/scenes/{id}/stop", s.requireToken(s.handleStopScene))
	s.registerGroupRoutes(mux) // Pause/Resume, Boxen-Gruppen (group.go)
	mux.HandleFunc("GET /api/v1/schedules", s.requireToken(s.handleSchedulesStatus))
	mux.HandleFunc("GET /api/v1/config/schedules", s.requireToken(s.handleScheduleConfigList))
	mux.HandleFunc("POST /api/v1/config/schedules", s.requireToken(s.handleScheduleUpsert))
	mux.HandleFunc("PUT /api/v1/config/schedules/{id}", s.requireToken(s.handleScheduleUpdate))
	mux.HandleFunc("DELETE /api/v1/config/schedules/{id}", s.requireToken(s.handleScheduleDelete))
	mux.HandleFunc("GET /api/v1/config", s.requireToken(s.handleGetConfig))
	mux.HandleFunc("POST /api/v1/config/speakers/sync", s.requireToken(s.handleSpeakersSync))
	mux.HandleFunc("POST /api/v1/config/scenes", s.requireToken(s.handleSceneUpsert))
	mux.HandleFunc("PUT /api/v1/config/scenes/{id}", s.requireToken(s.handleSceneUpdate))
	mux.HandleFunc("DELETE /api/v1/config/scenes/{id}", s.requireToken(s.handleSceneDelete))
	mux.HandleFunc("GET /api/v1/sonos/favorites", s.requireToken(s.handleSonosFavorites))
	mux.HandleFunc("GET /api/v1/radio/search", s.requireToken(s.handleRadioSearch))
	mux.HandleFunc("POST /api/v1/config/scenes/from-favorite", s.requireToken(s.handleSceneFromFavorite))

	// Middleware-Kette (aeusserste zuerst).
	return s.logRequests(securityHeaders(s.limit(mux)))
}

func (s *Server) handleAPINotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, s.log, apperr.New(apperr.CodeNotFound, "Unknown API path or wrong method").
		WithHint("See docs/api/openapi.yaml."))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	speakers, _ := s.enrichSpeakers(s.ctrl.ListSpeakers(ctx))
	names := make([]string, 0, len(speakers))
	reachable := 0
	for _, sp := range speakers {
		names = append(names, sp.Name)
		if sp.Reachable {
			reachable++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"speakers_online": reachable,
		"speakers":        names,
	})
}

func (s *Server) handleListSpeakers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), discoveryTimeout)
	defer cancel()
	speakers, err := s.enrichSpeakers(s.ctrl.ListSpeakers(ctx))
	resp := map[string]any{
		"speakers":        speakers,
		"discovery_error": "",
	}
	if speakers == nil {
		resp["speakers"] = []speaker.SpeakerInfo{}
	}
	if err != nil {
		resp["discovery_error"] = apperr.As(err).Message
	}
	writeJSON(w, http.StatusOK, resp)
}

// enrichSpeakers ergaenzt die Treiber-Liste um Config-/Szenen-Namen ohne Discovery.
func (s *Server) setKnownSpeakers(m map[string]string) {
	s.knownMu.Lock()
	s.knownSpeakers = cloneSpeakerMap(m)
	s.knownMu.Unlock()
}

func (s *Server) enrichSpeakers(list []speaker.SpeakerInfo, err error) ([]speaker.SpeakerInfo, error) {
	s.knownMu.RLock()
	defer s.knownMu.RUnlock()
	byName := make(map[string]speaker.SpeakerInfo, len(list)+len(s.knownSpeakers))
	for _, sp := range list {
		byName[sp.Name] = sp
	}
	for name, ip := range s.knownSpeakers {
		if existing, ok := byName[name]; ok {
			if existing.IP == "" && ip != "" {
				existing.IP = ip
				if !existing.Reachable {
					existing.Reachable = true
				}
				byName[name] = existing
			}
			continue
		}
		byName[name] = speaker.SpeakerInfo{
			Name:      name,
			IP:        ip,
			Reachable: ip != "",
		}
	}
	out := make([]speaker.SpeakerInfo, 0, len(byName))
	for _, sp := range byName {
		out = append(out, sp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, err
}

func (s *Server) handleListScenes(w http.ResponseWriter, r *http.Request) {
	list := s.scenes.List()
	out := make([]map[string]any, 0, len(list))
	for _, sc := range list {
		out = append(out, map[string]any{
			"id":      sc.ID,
			"name":    sc.Name,
			"source":  sc.Source,
			"members": sc.Members,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"scenes": out})
}

func (s *Server) handleScenesStatus(w http.ResponseWriter, r *http.Request) {
	// Kurz-Cache: mehrere Tabs/Flows loesen sonst jeweils dutzende SOAP-Calls aus.
	v := s.statusCache.get(statusCacheTTL, func() any {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), statusTimeout)
		defer cancel()
		return s.scenes.PlaybackStatus(ctx)
	})
	writeJSON(w, http.StatusOK, map[string]any{"scenes": v})
}

// collectSpeakersNowPlaying liefert Now-Playing fuer alle bekannten Boxen (Bulk).
func (s *Server) collectSpeakersNowPlaying(ctx context.Context) ([]speaker.NowPlayingInfo, error) {
	speakers, err := s.enrichSpeakers(s.ctrl.ListSpeakers(ctx))
	if speakers == nil {
		speakers = []speaker.SpeakerInfo{}
	}
	out := make([]speaker.NowPlayingInfo, 0, len(speakers))
	for _, sp := range speakers {
		if !sp.Reachable {
			out = append(out, speaker.NowPlayingInfo{
				Speaker: sp.Name,
				State:   speaker.PlaybackUnknown,
			})
			continue
		}
		np, npErr := s.ctrl.GetNowPlaying(ctx, sp.Name)
		if npErr != nil {
			out = append(out, speaker.NowPlayingInfo{
				Speaker: sp.Name,
				State:   speaker.PlaybackUnknown,
			})
			continue
		}
		np.Speaker = sp.Name
		np.Integratable = speaker.IsIntegratableStreamURI(np.URI)
		out = append(out, np)
	}
	return out, err
}

func (s *Server) handleSpeakersNowPlaying(w http.ResponseWriter, r *http.Request) {
	type result struct {
		out []speaker.NowPlayingInfo
		err error
	}
	res := s.npCache.get(statusCacheTTL, func() any {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), statusTimeout)
		defer cancel()
		out, err := s.collectSpeakersNowPlaying(ctx)
		return result{out, err}
	}).(result)
	resp := map[string]any{"speakers": res.out}
	if res.err != nil {
		resp["discovery_error"] = apperr.As(res.err).Message
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSpeakersStreamSources(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), statusTimeout)
	defer cancel()
	all, err := s.collectSpeakersNowPlaying(ctx)
	out := make([]speaker.NowPlayingInfo, 0)
	for _, np := range all {
		if np.Integratable {
			out = append(out, np)
		}
	}
	resp := map[string]any{"speakers": out}
	if err != nil {
		resp["discovery_error"] = apperr.As(err).Message
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDiscoveryRefresh(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), discoveryTimeout)
	defer cancel()
	speakers, err := s.enrichSpeakers(s.ctrl.RefreshSpeakers(ctx))
	resp := map[string]any{
		"speakers":        speakers,
		"discovery_error": "",
	}
	if speakers == nil {
		resp["speakers"] = []speaker.SpeakerInfo{}
	}
	if err != nil {
		resp["discovery_error"] = apperr.As(err).Message
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSpeakerNowPlaying(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Speaker name missing"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), nowPlayingTimeout)
	defer cancel()
	info, err := s.ctrl.GetNowPlaying(ctx, name)
	if err != nil {
		ae := apperr.As(err)
		if ae.Code == apperr.CodeSpeakerUnreachable {
			writeError(w, s.log, ae)
			return
		}
		writeJSON(w, http.StatusOK, speaker.NowPlayingInfo{
			Speaker: name,
			State:   speaker.PlaybackUnknown,
		})
		return
	}
	info.Integratable = speaker.IsIntegratableStreamURI(info.URI)
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handlePlayScene(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Scene ID missing"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), playTimeout)
	defer cancel()

	result, err := s.scenes.Play(ctx, id)
	s.invalidateStatus()
	if err != nil {
		ae := apperr.As(err)
		// Bei teilweisem Erfolg liefern wir trotzdem das Result mit.
		status := ae.HTTPStatus()
		payload := map[string]any{"error": ae.Code, "message": ae.Message}
		if ae.Speaker != "" {
			payload["speaker"] = ae.Speaker
		}
		if ae.Hint != "" {
			payload["hint"] = ae.Hint
		}
		if result != nil {
			payload["result"] = result
		}
		writeJSON(w, status, payload)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, log *slog.Logger, e *apperr.Error) {
	if e.HTTPStatus() >= 500 {
		log.Error("server error", "code", e.Code, "msg", e.Message)
	}
	writeJSON(w, e.HTTPStatus(), e)
}

// ttlCache haelt das letzte Ergebnis kurz vor; parallele Anfragen warten auf
// dieselbe Berechnung (Mutex), statt Geraete mehrfach abzufragen.
type ttlCache struct {
	mu   sync.Mutex
	at   time.Time
	val  any
	have bool
}

func (c *ttlCache) get(ttl time.Duration, compute func() any) any {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.have && time.Since(c.at) < ttl {
		return c.val
	}
	c.val = compute()
	c.at = time.Now()
	c.have = true
	return c.val
}

func (c *ttlCache) invalidate() {
	c.mu.Lock()
	c.have = false
	c.mu.Unlock()
}

// invalidateStatus verwirft gecachte Status-Antworten (nach Play/Config-Aenderung).
func (s *Server) invalidateStatus() {
	s.statusCache.invalidate()
	s.npCache.invalidate()
}
