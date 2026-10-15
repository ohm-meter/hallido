package sonos

import (
	"context"
	"strings"
	"testing"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

func TestParseTransportState(t *testing.T) {
	tests := []struct {
		xml  string
		want speaker.PlaybackState
	}{
		{
			`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetTransportInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><CurrentTransportState>PLAYING</CurrentTransportState></u:GetTransportInfoResponse></s:Body></s:Envelope>`,
			speaker.PlaybackPlaying,
		},
		{
			`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><GetTransportInfoResponse><CurrentTransportState>STOPPED</CurrentTransportState></GetTransportInfoResponse></s:Body></s:Envelope>`,
			speaker.PlaybackStopped,
		},
		{
			`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><GetTransportInfoResponse><CurrentTransportState>PAUSED_PLAYBACK</CurrentTransportState></GetTransportInfoResponse></s:Body></s:Envelope>`,
			speaker.PlaybackPaused,
		},
		{
			`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><GetTransportInfoResponse><CurrentTransportState>NO_MEDIA_PRESENT</CurrentTransportState></GetTransportInfoResponse></s:Body></s:Envelope>`,
			speaker.PlaybackStopped,
		},
		{
			`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body></s:Body></s:Envelope>`,
			speaker.PlaybackUnknown,
		},
		{
			`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetTransportInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><u:CurrentTransportState>STOPPED</u:CurrentTransportState></u:GetTransportInfoResponse></s:Body></s:Envelope>`,
			speaker.PlaybackStopped,
		},
	}
	for _, tc := range tests {
		if got := parseTransportState([]byte(tc.xml)); got != tc.want {
			t.Errorf("parseTransportState() = %q, want %q", got, tc.want)
		}
	}
}

func TestGetTransportState_SendsSOAP(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	f.transportState = "PLAYING"
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_W"},
	})

	got, err := c.GetTransportState(context.Background(), "Wohnzimmer")
	if err != nil {
		t.Fatalf("GetTransportState: %v", err)
	}
	if got != speaker.PlaybackPlaying {
		t.Fatalf("state = %q, want playing", got)
	}
	if len(f.calls) != 1 {
		t.Fatalf("erwartet 1 aufruf, bekam %d", len(f.calls))
	}
	if !strings.Contains(f.calls[0].Action, "AVTransport:1#GetTransportInfo") {
		t.Errorf("falsche SOAPACTION: %s", f.calls[0].Action)
	}
}

func TestGetTransportState_SoftFailOnHTTP500(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	f.transportFault = true
	c := controllerFor(map[string]*device{
		"Fernseher": {name: "Fernseher", baseURL: f.srv.URL, uid: "RINCON_TV"},
	})

	got, err := c.GetTransportState(context.Background(), "Fernseher")
	if err != nil {
		t.Fatalf("GetTransportState: erwartet kein harter Fehler, bekam %v", err)
	}
	if got != speaker.PlaybackUnknown {
		t.Fatalf("state = %q, want unknown", got)
	}
}

func TestStopAndPause_SendAVTransportSOAP(t *testing.T) {
	for _, tc := range []struct {
		action string
		call   func(*Controller) error
	}{
		{"Stop", func(c *Controller) error { return c.Stop(context.Background(), "Wohnzimmer") }},
		{"Pause", func(c *Controller) error { return c.Pause(context.Background(), "Wohnzimmer") }},
	} {
		f := newFakeSonos()
		c := controllerFor(map[string]*device{
			"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_W"},
		})
		if err := tc.call(c); err != nil {
			t.Fatalf("%s: %v", tc.action, err)
		}
		if len(f.calls) != 1 {
			t.Fatalf("%s: erwartet 1 aufruf, bekam %d", tc.action, len(f.calls))
		}
		call := f.calls[0]
		if call.Path != "/MediaRenderer/AVTransport/Control" {
			t.Errorf("%s: falscher pfad %s", tc.action, call.Path)
		}
		if !strings.Contains(call.Action, "AVTransport:1#"+tc.action) {
			t.Errorf("%s: falsche SOAPACTION %s", tc.action, call.Action)
		}
		if !strings.Contains(call.Body, "<InstanceID>0</InstanceID>") {
			t.Errorf("%s: InstanceID fehlt: %s", tc.action, call.Body)
		}
		f.close()
	}
}

func TestStop_AlreadyStopped701IsNoError(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	f.fault = true // 701 Transition not available
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_W"},
	})
	if err := c.Stop(context.Background(), "Wohnzimmer"); err != nil {
		t.Fatalf("701 sollte als bereits gestoppt gelten: %v", err)
	}
}

func TestStop_OtherFaultIsPlaybackFailed(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	f.fault = true
	f.faultCode = "800"
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_W"},
	})
	err := c.Stop(context.Background(), "Wohnzimmer")
	if got := apperr.As(err).Code; got != apperr.CodePlaybackFailed {
		t.Fatalf("code = %q, want playback_failed (err=%v)", got, err)
	}
}

func TestStop_UnreachableBox(t *testing.T) {
	f := newFakeSonos()
	url := f.srv.URL
	f.close() // Box offline
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: url, uid: "RINCON_W"},
	})
	err := c.Stop(context.Background(), "Wohnzimmer")
	if got := apperr.As(err).Code; got != apperr.CodeSpeakerUnreachable {
		t.Fatalf("code = %q, want speaker_unreachable (err=%v)", got, err)
	}
}
