package sonos

import (
	"context"
	"encoding/xml"
	"regexp"
	"strings"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

var transportStateRE = regexp.MustCompile(`(?i)<(?:[\w-]+:)?CurrentTransportState>([^<]+)</`)

// GetTransportState fragt den AVTransport-Status eines Coordinators ab.
func (c *Controller) GetTransportState(ctx context.Context, coordinator string) (speaker.PlaybackState, error) {
	d, err := c.resolve(ctx, coordinator)
	if err != nil {
		return speaker.PlaybackUnknown, err
	}
	body, err := c.soapCall(ctx, d, avTransport, "GetTransportInfo", []arg{
		{"InstanceID", "0"},
	})
	if err != nil {
		if isSoftStatusSOAPError(err) {
			return speaker.PlaybackUnknown, nil
		}
		return speaker.PlaybackUnknown, err
	}
	return parseTransportState(body), nil
}

func parseTransportState(body []byte) speaker.PlaybackState {
	state := extractTransportState(body)
	if state == "" {
		return speaker.PlaybackUnknown
	}
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "PLAYING":
		return speaker.PlaybackPlaying
	case "PAUSED_PLAYBACK":
		return speaker.PlaybackPaused
	case "STOPPED", "NO_MEDIA_PRESENT":
		return speaker.PlaybackStopped
	default:
		return speaker.PlaybackUnknown
	}
}

func extractTransportState(body []byte) string {
	var resp struct {
		State string `xml:"Body>GetTransportInfoResponse>CurrentTransportState"`
	}
	if err := xml.Unmarshal(body, &resp); err == nil && resp.State != "" {
		return resp.State
	}
	if m := transportStateRE.FindSubmatch(body); len(m) > 1 {
		return string(m[1])
	}
	return ""
}

// Stop beendet die Wiedergabe auf dem Coordinator (AVTransport Stop).
func (c *Controller) Stop(ctx context.Context, coordinator string) error {
	return c.transportAction(ctx, coordinator, "Stop")
}

// Pause pausiert die Wiedergabe auf dem Coordinator (AVTransport Pause).
func (c *Controller) Pause(ctx context.Context, coordinator string) error {
	return c.transportAction(ctx, coordinator, "Pause")
}

func (c *Controller) transportAction(ctx context.Context, coordinator, action string) error {
	d, err := c.resolve(ctx, coordinator)
	if err != nil {
		return err
	}
	_, err = c.soapCall(ctx, d, avTransport, action, []arg{
		{"InstanceID", "0"},
	})
	if err != nil && isTransitionNotAvailable(err) {
		// UPnP 701: Box ist schon gestoppt/pausiert -- Ziel erreicht.
		return nil
	}
	return err
}

// isTransitionNotAvailable erkennt UPnP-Fehler 701 ("Transition not
// available"), den Sonos bei Stop/Pause auf einer ruhenden Box liefert.
func isTransitionNotAvailable(err error) bool {
	ae := apperr.As(err)
	return ae.Code == apperr.CodePlaybackFailed && strings.Contains(ae.Message, "UPnP error code 701")
}
