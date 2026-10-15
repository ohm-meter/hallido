package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/config"
	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/speaker"
)

const (
	speakersHotReloadHint = "Speakers and fixed IPs are active immediately (hot reload)."
	configRestartNote     = "Restart only when changing listen or the driver (--driver), e.g. ./scripts/dev.sh restart"
)

func (s *Server) configEnabled() bool {
	return s.cfgPath != ""
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if !s.configEnabled() {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Config writing not configured"))
		return
	}
	cfg, err := config.ReadFile(s.cfgPath)
	if err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeInternal, "Config could not be read").
			WithHint(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, config.PublicView(cfg))
}

type speakersSyncRequest struct {
	Speakers []config.SpeakerSyncEntry `json:"speakers"`
}

// decodeBody liest und dekodiert den JSON-Body streng (unbekannte Felder
// verboten). Bei Fehler wird die Antwort geschrieben und false geliefert.
func (s *Server) decodeBody(w http.ResponseWriter, r *http.Request, max int64, dst any) bool {
	body, err := readJSONBody(r, max)
	if err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, err.Error()))
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Invalid JSON body"))
		return false
	}
	return true
}

// mutateConfig serialisiert Config-Aenderungen: lesen, mutieren, atomar
// speichern, danach after (Hot-Reload). Fehler werden als apperr beantwortet;
// ErrSceneNotFound wird zu scene_not_found. Liefert die gespeicherte Config
// oder nil, wenn bereits geantwortet wurde.
func (s *Server) mutateConfig(w http.ResponseWriter, mutate func(*config.Config) error, after func(*config.Config) error) *config.Config {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()

	cfg, err := config.ReadFile(s.cfgPath)
	if err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeInternal, "Config could not be read"))
		return nil
	}
	if err := mutate(cfg); err != nil {
		if errors.Is(err, config.ErrSceneNotFound) {
			writeError(w, s.log, apperr.New(apperr.CodeSceneNotFound, err.Error()).
				WithHint("Check available scenes via GET /api/v1/scenes."))
			return nil
		}
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, err.Error()))
		return nil
	}
	if err := config.SaveAtomic(s.cfgPath, cfg); err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeInternal, "Config could not be saved").
			WithHint(err.Error()))
		return nil
	}
	s.setKnownSpeakers(cfg.KnownSpeakers())
	s.invalidateStatus()
	if after != nil {
		if err := after(cfg); err != nil {
			writeError(w, s.log, apperr.New(apperr.CodeInternal, "Scenes could not be reloaded").
				WithHint(err.Error()))
			return nil
		}
	}
	return cfg
}

func (s *Server) reloadScenes(cfg *config.Config) error { return s.scenes.Reload(cfg.Scenes) }

func (s *Server) handleSpeakersSync(w http.ResponseWriter, r *http.Request) {
	if !s.configEnabled() {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Config writing not configured"))
		return
	}
	var req speakersSyncRequest
	if !s.decodeBody(w, r, 256*1024, &req) {
		return
	}
	var updated int
	cfg := s.mutateConfig(w, func(cfg *config.Config) (err error) {
		updated, err = config.MergeSpeakersFromDiscovery(cfg, req.Speakers)
		return err
	}, func(cfg *config.Config) error {
		if reloader, ok := s.ctrl.(speaker.FixedIPReloader); ok {
			reloader.ReloadFixedIPs(cfg.FixedIPOverrides())
		}
		return nil
	})
	if cfg == nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"speakers_updated": updated,
		"restart_hint":     speakersSavedHint(s.ctrl),
	})
}

func speakersSavedHint(ctrl speaker.SpeakerController) string {
	if _, ok := ctrl.(speaker.FixedIPReloader); ok {
		return speakersHotReloadHint + " " + configRestartNote
	}
	return "Speakers saved to config."
}

type sceneUpsertRequest struct {
	Scene scenes.Scene `json:"scene"`
}

type sceneUpdateRequest struct {
	Name    string          `json:"name"`
	Source  speaker.Source  `json:"source"`
	Members []scenes.Member `json:"members"`
}

func normalizeSceneFields(scene *scenes.Scene) error {
	scene.ID = strings.TrimSpace(scene.ID)
	scene.Name = strings.TrimSpace(scene.Name)
	if scene.Source.Type == "" {
		scene.Source.Type = speaker.SourceStream
	}
	if scene.Source.Type == speaker.SourceFavorite {
		scene.Source.Name = strings.TrimSpace(scene.Source.Name)
		scene.Source.URI = strings.TrimSpace(scene.Source.URI)
		return nil
	}
	if scene.Source.Type == speaker.SourceStream && scene.Source.URI != "" {
		norm, normErr := speaker.NormalizeStreamURIForConfig(scene.Source.URI)
		if normErr != nil {
			if err := config.ValidateStreamURI(scene.Source.URI); err != nil {
				return err
			}
		} else {
			scene.Source.URI = norm
		}
	}
	return nil
}

func (s *Server) handleSceneUpsert(w http.ResponseWriter, r *http.Request) {
	if !s.configEnabled() {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Config writing not configured"))
		return
	}
	var req sceneUpsertRequest
	if !s.decodeBody(w, r, 64*1024, &req) {
		return
	}
	scene := req.Scene
	if err := normalizeSceneFields(&scene); err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, err.Error()))
		return
	}
	cfg := s.mutateConfig(w, func(cfg *config.Config) error {
		return config.UpsertScene(cfg, scene)
	}, s.reloadScenes)
	if cfg == nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"scene_id":     scene.ID,
		"restart_hint": "Scene is playable immediately (hot reload).",
	})
}

// sceneIDFromPath liest und validiert die Szenen-ID aus dem Pfad.
func (s *Server) sceneIDFromPath(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Scene ID missing"))
		return "", false
	}
	if err := config.ValidateSceneID(id); err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, err.Error()))
		return "", false
	}
	return id, true
}

func (s *Server) handleSceneUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.configEnabled() {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Config writing not configured"))
		return
	}
	id, ok := s.sceneIDFromPath(w, r)
	if !ok {
		return
	}
	var req sceneUpdateRequest
	if !s.decodeBody(w, r, 64*1024, &req) {
		return
	}
	scene := scenes.Scene{
		ID:      id,
		Name:    strings.TrimSpace(req.Name),
		Source:  req.Source,
		Members: req.Members,
	}
	if err := normalizeSceneFields(&scene); err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, err.Error()))
		return
	}
	cfg := s.mutateConfig(w, func(cfg *config.Config) error {
		return config.UpdateScene(cfg, id, scene)
	}, s.reloadScenes)
	if cfg == nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"scene_id":     id,
		"restart_hint": "Scene is playable immediately (hot reload).",
	})
}

func (s *Server) handleSceneDelete(w http.ResponseWriter, r *http.Request) {
	if !s.configEnabled() {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Config writing not configured"))
		return
	}
	id, ok := s.sceneIDFromPath(w, r)
	if !ok {
		return
	}
	cfg := s.mutateConfig(w, func(cfg *config.Config) error {
		return config.DeleteScene(cfg, id)
	}, s.reloadScenes)
	if cfg == nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"scene_id": id,
		"deleted":  true,
	})
}

func readJSONBody(r *http.Request, max int64) ([]byte, error) {
	if r.Body == nil {
		return nil, errEmptyBody
	}
	defer r.Body.Close()
	lim := io.LimitReader(r.Body, max+1)
	raw, err := io.ReadAll(lim)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > max {
		return nil, errBodyTooLarge
	}
	if len(raw) == 0 {
		return nil, errEmptyBody
	}
	return raw, nil
}

var (
	errEmptyBody    = errConfigBody("empty request body")
	errBodyTooLarge = errConfigBody("request body too large")
)

type errConfigBody string

func (e errConfigBody) Error() string { return string(e) }
