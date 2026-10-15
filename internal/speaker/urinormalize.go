package speaker

import (
	"fmt"
	"strings"
)

// NormalizeStreamURIForConfig wandelt Now-Playing-/Sonos-URIs in eine https-Stream-URI
// fuer config.json um (Query wird entfernt).
func NormalizeStreamURIForConfig(raw string) (string, error) {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "", fmt.Errorf("empty uri")
	}
	if !IsIntegratableStreamURI(u) {
		return "", fmt.Errorf("uri cannot be configured as stream")
	}
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "file:") {
		return "", fmt.Errorf("file:// not allowed")
	}
	host, path := extractHostPath(u)
	if host == "" {
		return "", fmt.Errorf("uri not recognized")
	}
	if path == "" {
		path = "/"
	}
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "/"
	}
	return "https://" + host + path, nil
}
