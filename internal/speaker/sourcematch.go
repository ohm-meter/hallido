package speaker

import (
	"net/url"
	"strings"
)

// CanonicalStreamKey normalisiert Stream-URIs fuer den Vergleich (Host+Pfad,
// ohne Query, http/https und Sonos-Huellen wie aac:// oder x-rincon-mp3radio://).
func CanonicalStreamKey(raw string) string {
	host, path := extractHostPath(raw)
	if host == "" {
		return ""
	}
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "/"
	}
	return strings.ToLower(host) + strings.ToLower(path)
}

func extractHostPath(raw string) (host, path string) {
	u := strings.TrimSpace(raw)
	for i := 0; i < 8; i++ {
		if u == "" {
			return "", ""
		}
		if q := strings.Index(u, "?"); q >= 0 {
			u = u[:q]
		}
		if !strings.Contains(u, "://") {
			if idx := strings.Index(u, "/"); idx >= 0 {
				return u[:idx], u[idx:]
			}
			return u, "/"
		}
		idx := strings.Index(u, "://")
		scheme := strings.ToLower(u[:idx])
		rest := u[idx+3:]
		switch scheme {
		case "http", "https":
			parsed, err := url.Parse(scheme + "://" + rest)
			if err != nil || parsed.Host == "" {
				return "", ""
			}
			p := parsed.Path
			if p == "" {
				p = "/"
			}
			return parsed.Host, p
		case "aac", "x-rincon-mp3radio", "x-sonos-http", "x-sonos-htastream":
			u = rest
			continue
		default:
			u = rest
			continue
		}
	}
	return "", ""
}

// SourceMatchesPlayback prueft, ob die laufende Wiedergabe zur Szenen-Quelle passt.
func SourceMatchesPlayback(src Source, now NowPlayingInfo) bool {
	switch src.Type {
	case SourceStream:
		if src.URI == "" || now.URI == "" {
			return false
		}
		sceneKey := CanonicalStreamKey(src.URI)
		playKey := CanonicalStreamKey(now.URI)
		return sceneKey != "" && sceneKey == playKey
	case SourceFavorite:
		// Dienst-Favorit (TuneIn) mit gespeicherter URI: die Quelle der Box
		// (GetMediaInfo) vergleichen. Der Track wechselt (Pre-Roll, dann der
		// eigentliche Stream) und taugt dafuer nicht.
		if key := ServiceStreamKey(src.URI); key != "" {
			return key == ServiceStreamKey(now.SourceURI) || key == ServiceStreamKey(now.URI)
		}
		if src.Name == "" {
			return false
		}
		name := strings.ToLower(strings.TrimSpace(src.Name))
		for _, t := range []string{now.SourceTitle, now.Title} {
			title := strings.ToLower(strings.TrimSpace(t))
			if title != "" && (title == name || strings.Contains(title, name)) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
