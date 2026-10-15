package apperr

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestHTTPStatusMapping(t *testing.T) {
	cases := map[string]int{
		CodeSceneNotFound:      http.StatusNotFound,
		CodeSceneMisconfigured: http.StatusBadRequest,
		CodeSpeakerUnreachable: http.StatusBadGateway,
		CodeGroupingFailed:     http.StatusBadGateway,
		CodePlaybackFailed:     http.StatusBadGateway,
		CodeRadioSearchFailed:  http.StatusBadGateway,
		CodeUnauthorized:       http.StatusUnauthorized,
		CodeBadRequest:         http.StatusBadRequest,
		CodeNotFound:           http.StatusNotFound,
		CodeRateLimited:        http.StatusTooManyRequests,
		CodeInternal:           http.StatusInternalServerError,
		"unbekannt":            http.StatusInternalServerError,
	}
	for code, want := range cases {
		if got := New(code, "x").HTTPStatus(); got != want {
			t.Errorf("%s: %d, want %d", code, got, want)
		}
	}
}

func TestBuilderAndError(t *testing.T) {
	e := New(CodePlaybackFailed, "kaputt").WithSpeaker("Kueche").WithHint("neu starten")
	if e.Speaker != "Kueche" || e.Hint != "neu starten" {
		t.Fatalf("%+v", e)
	}
	if e.Error() != "playback_failed [Kueche]: kaputt" {
		t.Errorf("Error() = %q", e.Error())
	}
	if New(CodeInternal, "m").Error() != "internal: m" {
		t.Error("Error() ohne Speaker")
	}
}

func TestAs(t *testing.T) {
	orig := New(CodeSceneNotFound, "weg")
	if As(fmt.Errorf("wrap: %w", orig)) != orig {
		t.Error("As muss verpackten apperr finden")
	}
	got := As(errors.New("boom"))
	if got.Code != CodeInternal || got.Message != "boom" {
		t.Errorf("fremder Fehler -> %+v", got)
	}
}
