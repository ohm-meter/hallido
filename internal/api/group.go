package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/scenes"
)

// groupTimeout begrenzt Gruppenbefehle: Topologie lesen + Befehl an den
// Coordinator bzw. je Mitglied beim Aufloesen.
const groupTimeout = 20 * time.Second

// registerGroupRoutes haengt die Gruppensteuerung (ADR 0014) in den Router.
// Eigene Funktion, damit Handler() nur eine Zeile dafuer braucht.
func (s *Server) registerGroupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/scenes/{id}/pause", s.requireToken(s.handleSceneTransport(scenes.GroupPause)))
	mux.HandleFunc("POST /api/v1/scenes/{id}/resume", s.requireToken(s.handleSceneTransport(scenes.GroupResume)))
	mux.HandleFunc("POST /api/v1/speakers/{name}/stop", s.requireToken(s.handleSpeakerGroupAction(scenes.GroupStop)))
	mux.HandleFunc("POST /api/v1/speakers/{name}/pause", s.requireToken(s.handleSpeakerGroupAction(scenes.GroupPause)))
	mux.HandleFunc("POST /api/v1/speakers/{name}/resume", s.requireToken(s.handleSpeakerGroupAction(scenes.GroupResume)))
	mux.HandleFunc("POST /api/v1/speakers/{name}/ungroup", s.requireToken(s.handleSpeakerUngroup))
	mux.HandleFunc("GET /api/v1/groups", s.requireToken(s.handleListGroups))
}

func (s *Server) handleSceneTransport(action scenes.GroupAction) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Scene ID missing"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), stopTimeout)
		defer cancel()
		var err error
		if action == scenes.GroupPause {
			err = s.scenes.Pause(ctx, id)
		} else {
			err = s.scenes.Resume(ctx, id)
		}
		if err != nil {
			writeError(w, s.log, apperr.As(err))
			return
		}
		key := "paused"
		if action == scenes.GroupResume {
			key = "resumed"
		}
		writeJSON(w, http.StatusOK, map[string]any{"scene_id": id, key: true})
	}
}

func (s *Server) handleSpeakerGroupAction(action scenes.GroupAction) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Speaker name missing"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), groupTimeout)
		defer cancel()
		res, err := s.scenes.ControlGroupOf(ctx, name, action)
		if err != nil {
			writeErrorWithResult(w, s.log, apperr.As(err), res)
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func (s *Server) handleSpeakerUngroup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Speaker name missing"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), groupTimeout)
	defer cancel()
	res, err := s.scenes.Ungroup(ctx, name)
	if err != nil {
		writeErrorWithResult(w, s.log, apperr.As(err), res)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), statusTimeout)
	defer cancel()
	groups, err := s.scenes.Groups(ctx)
	if err != nil {
		writeError(w, s.log, apperr.As(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

// writeErrorWithResult schreibt den stabilen Fehler wie writeError und haengt
// ein Teilergebnis (pro Box) als "result" an -- analog zu Szenen-Play.
func writeErrorWithResult(w http.ResponseWriter, log *slog.Logger, ae *apperr.Error, result any) {
	if ae.HTTPStatus() >= 500 {
		log.Error("server error", "code", ae.Code, "msg", ae.Message)
	}
	payload := map[string]any{"error": ae.Code, "message": ae.Message}
	if ae.Speaker != "" {
		payload["speaker"] = ae.Speaker
	}
	if ae.Hint != "" {
		payload["hint"] = ae.Hint
	}
	if !isNilResult(result) {
		payload["result"] = result
	}
	writeJSON(w, ae.HTTPStatus(), payload)
}

func isNilResult(v any) bool {
	switch r := v.(type) {
	case nil:
		return true
	case *scenes.GroupActionResult:
		return r == nil
	case *scenes.UngroupResult:
		return r == nil
	}
	return false
}
