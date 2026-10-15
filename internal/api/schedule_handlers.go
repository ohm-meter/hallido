package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/config"
	"sonos-radio-agent/internal/schedule"
)

// stopTimeout begrenzt das Stoppen einer Szene (ein SOAP-Aufruf + Discovery).
const stopTimeout = 15 * time.Second

const scheduleHotReloadHint = "Schedule is active immediately (hot reload)."

func (s *Server) handleStopScene(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Scene ID missing"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), stopTimeout)
	defer cancel()
	if err := s.scenes.Stop(ctx, id); err != nil {
		writeError(w, s.log, apperr.As(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scene_id": id, "stopped": true})
}

// scheduleView ergaenzt den Laufzeitstatus um den Szenen-Namen fuer die GUI.
type scheduleView struct {
	schedule.Status
	SceneName string `json:"scene_name,omitempty"`
}

func (s *Server) handleSchedulesStatus(w http.ResponseWriter, r *http.Request) {
	if s.schedules == nil {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Scheduler not active"))
		return
	}
	statuses := s.schedules.Statuses()
	out := make([]scheduleView, 0, len(statuses))
	for _, st := range statuses {
		v := scheduleView{Status: st}
		if sc, ok := s.scenes.Get(st.Scene); ok {
			v.SceneName = sc.Name
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"timezone":  s.schedules.Location().String(),
		"now":       s.schedules.Now(),
		"schedules": out,
	})
}

func (s *Server) handleScheduleConfigList(w http.ResponseWriter, r *http.Request) {
	if !s.configEnabled() {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Config writing not configured"))
		return
	}
	cfg, err := config.ReadFile(s.cfgPath)
	if err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeInternal, "Config could not be read"))
		return
	}
	list := cfg.Schedules
	if list == nil {
		list = []schedule.Schedule{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"timezone":  cfg.Location().String(),
		"schedules": list,
	})
}

type scheduleUpsertRequest struct {
	Schedule schedule.Schedule `json:"schedule"`
}

func (s *Server) handleScheduleUpsert(w http.ResponseWriter, r *http.Request) {
	var req scheduleUpsertRequest
	if !s.decodeConfigBody(w, r, &req) {
		return
	}
	var id string
	ok := s.mutateSchedules(w, func(cfg *config.Config) error {
		var err error
		id, err = config.UpsertSchedule(cfg, req.Schedule)
		return err
	})
	if ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "schedule_id": id, "restart_hint": scheduleHotReloadHint,
		})
	}
}

func (s *Server) handleScheduleUpdate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Schedule ID missing"))
		return
	}
	var sch schedule.Schedule
	if !s.decodeConfigBody(w, r, &sch) {
		return
	}
	if sch.ID != "" && strings.TrimSpace(sch.ID) != id {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "id in body does not match path"))
		return
	}
	if s.mutateSchedules(w, func(cfg *config.Config) error { return config.UpdateSchedule(cfg, id, sch) }) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "schedule_id": id, "restart_hint": scheduleHotReloadHint,
		})
	}
}

func (s *Server) handleScheduleDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Schedule ID missing"))
		return
	}
	if s.mutateSchedules(w, func(cfg *config.Config) error { return config.DeleteSchedule(cfg, id) }) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "schedule_id": id, "deleted": true})
	}
}

// decodeConfigBody liest einen strikten JSON-Body (max. 16 KiB).
func (s *Server) decodeConfigBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if !s.configEnabled() {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Config writing not configured"))
		return false
	}
	body, err := readJSONBody(r, 16*1024)
	if err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, err.Error()))
		return false
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Invalid JSON body").WithHint(err.Error()))
		return false
	}
	return true
}

// mutateSchedules liest die Config, wendet fn an, speichert atomar und laedt
// den Scheduler neu. Fehler werden direkt geschrieben; true = Erfolg.
func (s *Server) mutateSchedules(w http.ResponseWriter, fn func(*config.Config) error) bool {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()

	cfg, err := config.ReadFile(s.cfgPath)
	if err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeInternal, "Config could not be read"))
		return false
	}
	if err := fn(cfg); err != nil {
		if errors.Is(err, config.ErrScheduleNotFound) {
			writeError(w, s.log, apperr.New(apperr.CodeScheduleNotFound, err.Error()).
				WithHint("Check existing schedules via GET /api/v1/config/schedules."))
			return false
		}
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, err.Error()))
		return false
	}
	if err := config.SaveAtomic(s.cfgPath, cfg); err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeInternal, "Config could not be saved").
			WithHint(err.Error()))
		return false
	}
	if s.schedules != nil {
		s.schedules.Reload(cfg.Schedules)
	}
	return true
}
