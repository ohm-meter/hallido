package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/config"
	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/speaker"
)

const favoritesTimeout = 20 * time.Second

func (s *Server) handleSonosFavorites(w http.ResponseWriter, r *http.Request) {
	reader, ok := s.ctrl.(speaker.FavoritesReader)
	if !ok {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest,
			"Sonos favorites are only available with the sonos driver").
			WithHint("Start the server with --driver=sonos."))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), favoritesTimeout)
	defer cancel()

	requested := r.URL.Query().Get("speaker")
	favs, proxy, listErr, browseErr := s.browseFavoritesWithFallback(ctx, reader, requested)
	resp := map[string]any{
		"favorites":       favs,
		"speaker":         proxy,
		"discovery_error": discoveryErrMsg(listErr),
	}
	if favs == nil {
		resp["favorites"] = []speaker.FavoriteEntry{}
	}
	if browseErr != nil {
		resp["discovery_error"] = apperr.As(browseErr).Message
	}
	writeJSON(w, http.StatusOK, resp)
}

func discoveryErrMsg(err error) string {
	if err == nil {
		return ""
	}
	return apperr.As(err).Message
}

type sceneFromFavoriteRequest struct {
	SceneID    string          `json:"scene_id"`
	Name       string          `json:"name"`
	FavoriteID string          `json:"favorite_id"`
	Members    []scenes.Member `json:"members"`
	Speaker    string          `json:"speaker,omitempty"`
}

func (s *Server) handleSceneFromFavorite(w http.ResponseWriter, r *http.Request) {
	if !s.configEnabled() {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Config writing not configured"))
		return
	}
	reader, ok := s.ctrl.(speaker.FavoritesReader)
	if !ok {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest,
			"Scenes from favorites are only available with the sonos driver").
			WithHint("Start the server with --driver=sonos."))
		return
	}

	var req sceneFromFavoriteRequest
	if !s.decodeBody(w, r, 64*1024, &req) {
		return
	}

	req.SceneID = strings.TrimSpace(req.SceneID)
	req.Name = strings.TrimSpace(req.Name)
	req.FavoriteID = strings.TrimSpace(req.FavoriteID)
	if req.SceneID == "" || req.Name == "" || req.FavoriteID == "" {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "scene_id, name and favorite_id are required"))
		return
	}
	if len(req.Members) == 0 {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "at least one member required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), favoritesTimeout)
	defer cancel()

	favs, _, _, browseErr := s.browseFavoritesWithFallback(ctx, reader, req.Speaker)
	if browseErr != nil {
		writeError(w, s.log, apperr.As(browseErr))
		return
	}
	var entry *speaker.FavoriteEntry
	for i := range favs {
		if favs[i].ID == req.FavoriteID {
			entry = &favs[i]
			break
		}
	}
	if entry == nil {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Favorite not found: "+req.FavoriteID))
		return
	}
	if entry.URI == "" {
		writeError(w, s.log, apperr.New(apperr.CodeBadRequest, "Favorite without playable URI").
			WithHint("Check in the Sonos app or add manually as a stream URL."))
		return
	}
	source, err := sceneSourceFromFavorite(*entry)
	if err != nil {
		writeError(w, s.log, apperr.As(err))
		return
	}

	scene := scenes.Scene{
		ID:      req.SceneID,
		Name:    req.Name,
		Source:  source,
		Members: req.Members,
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
		"source_type":  scene.Source.Type,
		"source_uri":   scene.Source.URI,
		"restart_hint": "Scene is playable immediately (hot reload).",
	})
}

// sceneSourceFromFavorite waehlt die Szenen-Quelle fuer einen Favoriten:
// Streams (http(s)) als type stream mit normalisierter URL, Radio-Favoriten
// eines Sonos-Dienstes (TuneIn) als type favorite mit Titel und URI (ADR 0018).
func sceneSourceFromFavorite(entry speaker.FavoriteEntry) (speaker.Source, error) {
	if entry.Integratable {
		uri, normErr := speaker.NormalizeStreamURIForConfig(entry.URI)
		if normErr != nil {
			if err := config.ValidateStreamURI(entry.URI); err != nil {
				return speaker.Source{}, apperr.New(apperr.CodeBadRequest, err.Error())
			}
			uri = entry.URI
		}
		return speaker.Source{Type: speaker.SourceStream, URI: uri}, nil
	}
	if entry.Playable && speaker.IsSonosServiceStreamURI(entry.URI) {
		return speaker.Source{Type: speaker.SourceFavorite, Name: strings.TrimSpace(entry.Title), URI: entry.URI}, nil
	}
	return speaker.Source{}, apperr.New(apperr.CodeBadRequest, "Favorite cannot be imported as a scene").
		WithHint("Spotify, Amazon and Sonos Radio favorites need the Sonos app. " +
			"Internet radio (http(s)) and TuneIn stations work.")
}
