package speaker

import "testing"

func TestIsTVLikeZone(t *testing.T) {
	tests := map[string]bool{
		"Fernseher":      true,
		"EG: TV":         true,
		"Wohnzimmer-TV":  true,
		"TV":             true,
		"Kueche":         false,
		"EG: Wohnzimmer": false,
		"Stevie":         false,
	}
	for name, want := range tests {
		if got := IsTVLikeZone(name); got != want {
			t.Errorf("IsTVLikeZone(%q) = %v, want %v", name, got, want)
		}
	}
}
