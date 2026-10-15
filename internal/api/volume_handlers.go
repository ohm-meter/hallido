package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"sonos-radio-agent/internal/apperr"
)

// volumeTimeout begrenzt das Setzen der Lautstaerke einer Box (ein SOAP-Aufruf).
const volumeTimeout = 10 * time.Second

// volumeRequest ist der Body von PUT /api/v1/speakers/{name}/volume.
// Pointer, damit ein fehlendes Feld von 0 unterscheidbar ist.
type volumeRequest struct {
	Volume *int `json:"volume"`
}

// handleSetSpeakerVolume setzt die Lautstaerke einer Box live (GUI-Lautstaerke-Sheet).
// Aendert nicht die in der Szene gespeicherte Startlautstaerke.
func (s *Server) handleSetSpeakerVolume(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Speaker name missing"))
		return
	}
	body, err := readJSONBody(r, 1024)
	if err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, err.Error()))
		return
	}
	var req volumeRequest
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Invalid JSON body").
			WithHint(`Expected: {"volume": 0-100}`))
		return
	}
	if req.Volume == nil {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "volume missing").
			WithHint(`Expected: {"volume": 0-100}`))
		return
	}
	if *req.Volume < 0 || *req.Volume > 100 {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "volume outside 0-100").WithSpeaker(name))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), volumeTimeout)
	defer cancel()
	if err := s.ctrl.SetVolume(ctx, name, *req.Volume); err != nil {
		ae := apperr.As(err)
		if ae.Speaker == "" {
			ae.Speaker = name
		}
		writeError(w, s.log, ae)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"speaker": name, "volume": *req.Volume})
}
