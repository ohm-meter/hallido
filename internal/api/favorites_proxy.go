package api

import (
	"context"
	"sort"
	"strings"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

// orderedFavoriteProxySpeakers liefert erreichbare Boxen fuer Favoriten-Browse.
// Bevorzugt explizite Auswahl, sonst Nicht-TV-Zonen vor TV/Fernseher.
func orderedFavoriteProxySpeakers(speakers []speaker.SpeakerInfo, preferred string) []string {
	preferred = strings.TrimSpace(preferred)
	seen := map[string]bool{}
	var preferredReachable string
	var regular, tvLike []string

	for _, sp := range speakers {
		if !sp.Reachable || seen[sp.Name] {
			continue
		}
		seen[sp.Name] = true
		if preferred != "" && sp.Name == preferred {
			preferredReachable = sp.Name
			continue
		}
		if speaker.IsTVLikeZone(sp.Name) {
			tvLike = append(tvLike, sp.Name)
		} else {
			regular = append(regular, sp.Name)
		}
	}
	sort.Strings(regular)
	sort.Strings(tvLike)

	out := make([]string, 0, len(regular)+len(tvLike)+1)
	if preferredReachable != "" {
		out = append(out, preferredReachable)
	}
	out = append(out, regular...)
	out = append(out, tvLike...)
	return out
}

// browseFavoritesWithFallback versucht Browse auf mehreren Boxen (TV zuletzt).
func (s *Server) browseFavoritesWithFallback(
	ctx context.Context,
	reader speaker.FavoritesReader,
	requested string,
) (favs []speaker.FavoriteEntry, proxy string, listErr error, browseErr error) {
	speakers, listErr := s.ctrl.ListSpeakers(ctx)
	requested = strings.TrimSpace(requested)
	if requested != "" {
		found := false
		for _, sp := range speakers {
			if sp.Name == requested && sp.Reachable {
				found = true
				break
			}
		}
		if !found {
			return nil, "", listErr, apperr.New(apperr.CodeBadRequest, "Speaker unreachable: "+requested)
		}
	}

	candidates := orderedFavoriteProxySpeakers(speakers, requested)
	if len(candidates) == 0 {
		if listErr != nil {
			return nil, "", listErr, listErr
		}
		return nil, "", nil, apperr.New(apperr.CodeBadRequest, "no reachable speaker for favorites browse")
	}

	var lastErr error
	for _, name := range candidates {
		favs, err := reader.BrowseFavorites(ctx, name)
		if err == nil {
			return favs, name, listErr, nil
		}
		lastErr = err
	}
	return nil, candidates[0], listErr, lastErr
}
