// Package apperr definiert strukturierte, maschinenlesbare Fehler.
//
// Jeder Fehler hat einen stabilen Code (fuer Homey & GUI), eine
// menschenlesbare Meldung und optional den betroffenen Lautsprecher sowie
// einen Hinweis zur Behebung. Die Codes sind Teil des API-Vertrags und
// duerfen nicht leichtfertig geaendert werden.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Stabile Fehlercodes. Aenderungen hier sind API-Vertragsaenderungen.
const (
	CodeSceneNotFound      = "scene_not_found"
	CodeScheduleNotFound   = "schedule_not_found"
	CodeSceneMisconfigured = "scene_misconfigured"
	CodeSpeakerUnreachable = "speaker_unreachable"
	CodeGroupingFailed     = "grouping_failed"
	CodePlaybackFailed     = "playback_failed"
	CodeRadioSearchFailed  = "radio_search_failed"
	CodeUnauthorized       = "unauthorized"
	CodeBadRequest         = "bad_request"
	CodeNotFound           = "not_found"
	CodeRateLimited        = "rate_limited"
	CodeInternal           = "internal"
)

// Error ist ein strukturierter Anwendungsfehler.
type Error struct {
	Code    string `json:"error"`             // maschinenlesbarer Code
	Message string `json:"message"`           // menschenlesbar
	Speaker string `json:"speaker,omitempty"` // betroffener Lautsprecher, falls relevant
	Hint    string `json:"hint,omitempty"`    // konkreter Hinweis zur Behebung
}

func (e *Error) Error() string {
	if e.Speaker != "" {
		return fmt.Sprintf("%s [%s]: %s", e.Code, e.Speaker, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// New erzeugt einen Fehler mit Code und Meldung.
func New(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// WithSpeaker setzt den betroffenen Lautsprecher (Builder-Stil).
func (e *Error) WithSpeaker(name string) *Error {
	e.Speaker = name
	return e
}

// WithHint setzt einen Behebungshinweis (Builder-Stil).
func (e *Error) WithHint(hint string) *Error {
	e.Hint = hint
	return e
}

// HTTPStatus mappt den Code auf einen passenden HTTP-Statuscode.
func (e *Error) HTTPStatus() int {
	switch e.Code {
	case CodeSceneNotFound, CodeScheduleNotFound:
		return http.StatusNotFound
	case CodeSceneMisconfigured:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeBadRequest:
		return http.StatusBadRequest
	case CodeNotFound:
		return http.StatusNotFound
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeSpeakerUnreachable, CodeGroupingFailed, CodePlaybackFailed, CodeRadioSearchFailed:
		// Geraet/Upstream nicht erreichbar -> 502, der Agent selbst ist ok.
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// As extrahiert einen *Error aus einer Fehlerkette, oder verpackt einen
// beliebigen Fehler als Internal-Error. So bekommt der HTTP-Layer immer
// einen strukturierten Fehler.
func As(err error) *Error {
	var ae *Error
	if errors.As(err, &ae) {
		return ae
	}
	return &Error{Code: CodeInternal, Message: err.Error()}
}
