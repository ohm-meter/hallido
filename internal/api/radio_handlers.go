package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/radio"
	"sonos-radio-agent/internal/speaker"
)

const radioSearchTimeout = 12 * time.Second // bis zu 3 Mirror

type radioSearcher interface {
	Search(ctx context.Context, query string, limit int) ([]radio.Station, error)
}

func (s *Server) handleRadioSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Query parameter q missing").
			WithHint("Example: /api/v1/radio/search?q=fm4&limit=10"))
		return
	}
	limit := 10
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "limit must be a positive number"))
			return
		}
		limit = n
	}
	if limit > 50 {
		limit = 50
	}
	ctx, cancel := context.WithTimeout(r.Context(), radioSearchTimeout)
	defer cancel()
	raw, err := s.radio.Search(ctx, q, limit)
	if err != nil {
		writeError(w, s.log, apperr.As(err))
		return
	}
	stations := make([]radio.Station, 0, len(raw))
	for _, st := range raw {
		if !speaker.IsIntegratableStreamURI(st.URL) {
			continue
		}
		stations = append(stations, st)
	}
	if stations == nil {
		stations = []radio.Station{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"stations": stations})
}
