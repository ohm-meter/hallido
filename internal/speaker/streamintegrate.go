package speaker

import (
	"net/url"
	"strings"
)

// IsIntegratableStreamURI meldet, ob eine Now-Playing-/Sonos-URI als Stream-Szene
// nutzbar ist (http(s), aac://http(s), x-rincon-mp3radio). Gruppen-URIs,
// Spotify, VLI und reine Gruppen-Coordinator-IDs werden abgelehnt.
func IsIntegratableStreamURI(raw string) bool {
	u := strings.TrimSpace(raw)
	if u == "" {
		return false
	}
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "spotify:") || strings.HasPrefix(lower, "x-sonos-spotify:") {
		return false
	}
	if strings.HasPrefix(lower, "x-sonos-vli:") {
		return false
	}
	if lower == "x-rincon" {
		return false
	}
	if strings.HasPrefix(lower, "x-rincon:") && !strings.HasPrefix(lower, "x-rincon-mp3radio:") {
		return false
	}
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		parsed, err := url.Parse(u)
		return err == nil && parsed.Host != ""
	}
	if strings.HasPrefix(lower, "x-rincon-mp3radio://") {
		host, _ := extractHostPath(u)
		return host != ""
	}
	if strings.HasPrefix(lower, "aac://") {
		inner := strings.ToLower(strings.TrimSpace(u[6:]))
		if strings.HasPrefix(inner, "x-rincon") || strings.HasPrefix(inner, "x-sonos-vli:") {
			return false
		}
		if strings.HasPrefix(inner, "http://") || strings.HasPrefix(inner, "https://") {
			return true
		}
		host, _ := extractHostPath(u)
		return host != ""
	}
	for _, p := range []string{"x-sonos-http://", "x-sonos-htastream://"} {
		if strings.HasPrefix(lower, p) {
			host, _ := extractHostPath(u)
			return host != ""
		}
	}
	return false
}

// serviceStreamPrefix kennzeichnet Radio-Favoriten eines Sonos-Musikdienstes
// (TuneIn: x-sonosapi-stream:tunein%3a17315?sid=303&...).
const serviceStreamPrefix = "x-sonosapi-stream:"

// IsSonosServiceStreamURI meldet Radio-Favoriten eines Sonos-Dienstes (z. B.
// TuneIn). Sie sind nur zusammen mit den DIDL-Metadaten des Favoriten
// (r:resMD) abspielbar, nicht als Stream-Szene (ADR 0018). Andere Dienst-URIs
// (x-sonosapi-radio fuer Amazon/Sonos Radio) lehnt die Box per SOAP ab (501).
func IsSonosServiceStreamURI(raw string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), serviceStreamPrefix)
}

// ServiceStreamKey normalisiert eine x-sonosapi-stream-URI fuer den Vergleich:
// Sender-Kennung dekodiert und klein, ohne Query (sid/flags/sn schwanken —
// die Box meldet z. B. flags=8232 fuer einen Favoriten mit flags=8292).
// Leer, wenn raw keine solche URI ist.
func ServiceStreamKey(raw string) string {
	u := strings.TrimSpace(raw)
	if !IsSonosServiceStreamURI(u) {
		return ""
	}
	rest := u[len(serviceStreamPrefix):]
	if q := strings.IndexByte(rest, '?'); q >= 0 {
		rest = rest[:q]
	}
	if dec, err := url.PathUnescape(rest); err == nil {
		rest = dec
	}
	rest = strings.ToLower(strings.TrimSpace(rest))
	if rest == "" {
		return ""
	}
	return serviceStreamPrefix + rest
}
