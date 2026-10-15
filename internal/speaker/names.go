package speaker

import "strings"

// IsTVLikeZone ist true fuer Sonos-Zonen, die oft keinen ContentDirectory-Browse
// unterstuetzen (TV/HDMI, z. B. „Fernseher“, „EG: TV“).
func IsTVLikeZone(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if strings.Contains(lower, "fernseher") {
		return true
	}
	if strings.Contains(lower, " tv") || strings.HasSuffix(lower, " tv") ||
		strings.HasPrefix(lower, "tv ") || lower == "tv" {
		return true
	}
	for _, tok := range strings.FieldsFunc(lower, func(r rune) bool {
		return r == ' ' || r == ':' || r == '-' || r == '.' || r == '/'
	}) {
		if tok == "tv" {
			return true
		}
	}
	return false
}
