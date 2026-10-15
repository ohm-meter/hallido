package sonos

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// soapRequest protokolliert einen empfangenen SOAP-Aufruf.
type soapRequest struct {
	Path   string
	Action string
	Body   string
}

// fakeSonos ist ein httptest-Server, der die SOAP-Endpoints eines Sonos-Geraets
// simuliert. fault steuert, ob das Geraet die Aktion ablehnt (HTTP 500).
type fakeSonos struct {
	srv            *httptest.Server
	calls          []soapRequest
	fault          bool
	faultCode      string // UPnP-Fehlercode bei fault (Standard 701)
	transportFault bool
	transportState string
	currentURI     string
	currentMeta    string
	trackURI       string
	trackMeta      string
}

func newFakeSonos() *fakeSonos {
	f := &fakeSonos{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		f.calls = append(f.calls, soapRequest{
			Path:   r.URL.Path,
			Action: r.Header.Get("SOAPACTION"),
			Body:   string(raw),
		})
		if f.fault {
			code := f.faultCode
			if code == "" {
				code = "701"
			}
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><s:Fault><detail><UPnPError xmlns="urn:schemas-upnp-org:control-1-0"><errorCode>`+code+`</errorCode></UPnPError></detail></s:Fault></s:Body></s:Envelope>`)
			return
		}
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
		if strings.Contains(r.Header.Get("SOAPACTION"), "GetTransportInfo") {
			if f.transportFault {
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><s:Fault><faultstring>Transport unavailable</faultstring><detail><UPnPError xmlns="urn:schemas-upnp-org:control-1-0"><errorCode>701</errorCode></UPnPError></detail></s:Fault></s:Body></s:Envelope>`)
				return
			}
			state := f.transportState
			if state == "" {
				// Ohne Vorgabe verhaelt sich der Fake wie eine Box, die nach
				// Play spielt (Pruefung nach Play, verifyPlaying).
				state = "STOPPED"
				for _, c := range f.calls {
					if strings.HasSuffix(c.Action, "#Play\"") {
						state = "PLAYING"
					}
				}
			}
			io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetTransportInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><CurrentTransportState>`+state+`</CurrentTransportState><CurrentTransportStatus>OK</CurrentTransportStatus></u:GetTransportInfoResponse></s:Body></s:Envelope>`)
			return
		}
		if strings.Contains(r.Header.Get("SOAPACTION"), "GetMediaInfo") {
			io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetMediaInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><NrTracks>1</NrTracks><MediaDuration>0:00:00</MediaDuration><CurrentURI>`+escapeXML(f.currentURI)+`</CurrentURI><CurrentURIMetaData>`+escapeXML(f.currentMeta)+`</CurrentURIMetaData></u:GetMediaInfoResponse></s:Body></s:Envelope>`)
			return
		}
		if strings.Contains(r.Header.Get("SOAPACTION"), "GetPositionInfo") {
			io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetPositionInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><Track>1</Track><TrackDuration>0:03:00</TrackDuration><TrackMetaData>`+escapeXML(f.trackMeta)+`</TrackMetaData><TrackURI>`+escapeXML(f.trackURI)+`</TrackURI></u:GetPositionInfoResponse></s:Body></s:Envelope>`)
			return
		}
		io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body></s:Body></s:Envelope>`)
	}))
	return f
}

func (f *fakeSonos) close() { f.srv.Close() }

// controllerFor baut einen Controller mit vorab bekannten Geraeten, sodass
// resolve keine echte Discovery anstoesst.
func controllerFor(devices map[string]*device) *Controller {
	c := New(nil, testLogger())
	c.devices = devices
	c.prober = nil // keine echten Netzwerkzugriffe; Probe-Tests setzen eigene
	c.playVerifyWindow = 300 * time.Millisecond
	c.playVerifyInterval = 10 * time.Millisecond
	c.playVerifyGrace = 50 * time.Millisecond
	c.favoriteVerifyWindow = 300 * time.Millisecond
	return c
}

func TestSetVolume_SendsCorrectSOAP(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	c := controllerFor(map[string]*device{
		"Kueche": {name: "Kueche", baseURL: f.srv.URL, uid: "RINCON_K"},
	})

	if err := c.SetVolume(context.Background(), "Kueche", 30); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("erwartet 1 aufruf, bekam %d", len(f.calls))
	}
	call := f.calls[0]
	if call.Path != "/MediaRenderer/RenderingControl/Control" {
		t.Errorf("falscher pfad: %s", call.Path)
	}
	if !strings.Contains(call.Action, "RenderingControl:1#SetVolume") {
		t.Errorf("falsche SOAPACTION: %s", call.Action)
	}
	if !strings.Contains(call.Body, "<DesiredVolume>30</DesiredVolume>") {
		t.Errorf("DesiredVolume fehlt im body: %s", call.Body)
	}
	if !strings.Contains(call.Body, "<Channel>Master</Channel>") {
		t.Errorf("Channel fehlt im body: %s", call.Body)
	}
}

func TestPlayStream_SetsURIThenPlays(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_W"},
	})

	src := speaker.Source{Type: speaker.SourceStream, URI: "https://orf.at/stream?id=fm4&x=1"}
	if err := c.Play(context.Background(), "Wohnzimmer", src); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if len(f.calls) != 3 {
		t.Fatalf("erwartet 3 aufrufe (SetAVTransportURI, Play, GetTransportInfo), bekam %d", len(f.calls))
	}
	if !strings.Contains(f.calls[0].Action, "AVTransport:1#SetAVTransportURI") {
		t.Errorf("erster aufruf falsch: %s", f.calls[0].Action)
	}
	if !strings.Contains(f.calls[0].Body, "<CurrentURI>x-rincon-mp3radio://orf.at/stream?id=fm4&amp;x=1</CurrentURI>") {
		t.Errorf("radio-transport-uri fehlt oder falsch: %s", f.calls[0].Body)
	}
	if !strings.Contains(f.calls[0].Body, "object.item.audioItem.audioBroadcast") {
		t.Errorf("didl-metadata fehlt: %s", f.calls[0].Body)
	}
	if !strings.Contains(f.calls[0].Body, "<CurrentURIMetaData>") {
		t.Errorf("metadata darf nicht leer sein: %s", f.calls[0].Body)
	}
	if !strings.Contains(f.calls[1].Action, "AVTransport:1#Play") {
		t.Errorf("zweiter aufruf falsch: %s", f.calls[1].Action)
	}
	if !strings.Contains(f.calls[2].Action, "AVTransport:1#GetTransportInfo") {
		t.Errorf("dritter aufruf soll den Status pruefen: %s", f.calls[2].Action)
	}
}

func TestPlayFavorite_BrowseFehlerVorPlay(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_W"},
	})

	src := speaker.Source{Type: speaker.SourceFavorite, Name: "FM4"}
	err := c.Play(context.Background(), "Wohnzimmer", src)
	if err == nil {
		t.Fatal("erwartet fehler fuer favoriten-quelle, bekam nil")
	}
	if got := apperr.As(err).Code; got != apperr.CodePlaybackFailed {
		t.Errorf("erwartet code %q, bekam %q", apperr.CodePlaybackFailed, got)
	}
	// Browse wird versucht; ohne ContentDirectory-Mock scheitert die Wiedergabe vor Play.
	if len(f.calls) > 1 {
		t.Errorf("erwartet hoechstens Browse-Versuch, bekam %d aufrufe", len(f.calls))
	}
}

func TestGroup_SetsRinconURI(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_COORD"},
		"Kueche":     {name: "Kueche", baseURL: f.srv.URL, uid: "RINCON_K"},
	})

	if err := c.Group(context.Background(), "Wohnzimmer", []string{"Kueche"}); err != nil {
		t.Fatalf("Group: %v", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("erwartet 1 aufruf, bekam %d", len(f.calls))
	}
	if !strings.Contains(f.calls[0].Body, "<CurrentURI>x-rincon:RINCON_COORD</CurrentURI>") {
		t.Errorf("x-rincon-uri fehlt oder falsch: %s", f.calls[0].Body)
	}
}

func TestEnsureSpeakers_ReportsMissingMember(t *testing.T) {
	fWohn := newFakeSonos()
	defer fWohn.close()

	c := New(nil, testLogger())
	c.devices = map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: fWohn.srv.URL, uid: "RINCON_W"},
	}

	err := c.EnsureSpeakers(context.Background(), []string{"Wohnzimmer", "Kueche"})
	if err == nil {
		t.Fatal("erwarte scene_misconfigured fuer fehlende Kueche")
	}
	if got := apperr.As(err).Code; got != apperr.CodeSceneMisconfigured {
		t.Fatalf("code = %q, erwartet %q", got, apperr.CodeSceneMisconfigured)
	}
}

func TestSOAP_Unreachable(t *testing.T) {
	f := newFakeSonos()
	url := f.srv.URL
	f.close() // Server sofort schliessen -> Verbindung wird abgelehnt

	c := controllerFor(map[string]*device{
		"Kueche": {name: "Kueche", baseURL: url, uid: "RINCON_K"},
	})
	err := c.SetVolume(context.Background(), "Kueche", 20)
	if err == nil {
		t.Fatal("erwartet unreachable-fehler, bekam nil")
	}
	ae := apperr.As(err)
	if ae.Code != apperr.CodeSpeakerUnreachable {
		t.Errorf("erwartet code %q, bekam %q", apperr.CodeSpeakerUnreachable, ae.Code)
	}
	if ae.Speaker != "Kueche" {
		t.Errorf("erwartet speaker Kueche, bekam %q", ae.Speaker)
	}
	if !strings.Contains(ae.Hint, ":1400") {
		t.Errorf("hint soll ip:1400 enthalten: %q", ae.Hint)
	}
}

func TestSOAP_ContextDeadlineHint(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	c := controllerFor(map[string]*device{
		"Kueche": {name: "Kueche", baseURL: f.srv.URL, uid: "RINCON_K"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)

	err := c.SetVolume(ctx, "Kueche", 20)
	if err == nil {
		t.Fatal("erwartet fehler bei abgelaufenem context")
	}
	ae := apperr.As(err)
	if ae.Code != apperr.CodeSpeakerUnreachable {
		t.Errorf("erwartet code %q, bekam %q", apperr.CodeSpeakerUnreachable, ae.Code)
	}
	if !strings.Contains(ae.Hint, "Timeout") {
		t.Errorf("hint should mention timeout: %q", ae.Hint)
	}
}

func TestSOAP_DeviceRejectsAction(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	f.fault = true
	c := controllerFor(map[string]*device{
		"Kueche": {name: "Kueche", baseURL: f.srv.URL, uid: "RINCON_K"},
	})
	err := c.SetVolume(context.Background(), "Kueche", 20)
	if err == nil {
		t.Fatal("erwartet playback_failed, bekam nil")
	}
	ae := apperr.As(err)
	if ae.Code != apperr.CodePlaybackFailed {
		t.Errorf("erwartet code %q, bekam %q", apperr.CodePlaybackFailed, ae.Code)
	}
	if !strings.Contains(ae.Message, "701") {
		t.Errorf("UPnP-fehlercode 701 sollte in der meldung stehen: %s", ae.Message)
	}
}

func TestBuildEnvelope_EscapesValues(t *testing.T) {
	env := buildEnvelope("urn:test:1", "SetAVTransportURI", []arg{
		{"CurrentURI", "http://x/y?a=1&b=<2>"},
	})
	if strings.Contains(env, "a=1&b=<2>") {
		t.Errorf("envelope nicht escaped: %s", env)
	}
	if !strings.Contains(env, "a=1&amp;b=&lt;2&gt;") {
		t.Errorf("erwartete escaped-form fehlt: %s", env)
	}
}

func TestFetchDevice_ParsesDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xml/device_description.xml" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		io.WriteString(w, `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <roomName>Schlafzimmer</roomName>
    <UDN>uuid:RINCON_ABC123456701400</UDN>
  </device>
</root>`)
	}))
	defer srv.Close()

	d, err := fetchDevice(context.Background(), http.DefaultClient, srv.URL)
	if err != nil {
		t.Fatalf("fetchDevice: %v", err)
	}
	if d.name != "Schlafzimmer" {
		t.Errorf("erwartet Schlafzimmer, bekam %q", d.name)
	}
	if d.uid != "RINCON_ABC123456701400" {
		t.Errorf("uuid-praefix nicht entfernt: %q", d.uid)
	}
}

func TestBaseURLFromLocation(t *testing.T) {
	got := baseURLFromLocation("http://192.168.0.10:1400/xml/device_description.xml")
	if got != "http://192.168.0.10:1400" {
		t.Errorf("erwartet http://192.168.0.10:1400, bekam %q", got)
	}
}

func TestIPFromBaseURL(t *testing.T) {
	if got := ipFromBaseURL("http://192.168.0.10:1400"); got != "192.168.0.10" {
		t.Errorf("erwartet 192.168.0.10, bekam %q", got)
	}
}

func TestReloadFixedIPs_ReplacesOverridesAndClearsCache(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_1"},
	})
	c.overrides = map[string]string{"Wohnzimmer": "192.168.1.1"}

	c.ReloadFixedIPs(map[string]string{"Wohnzimmer": "192.168.1.2", "Kueche": "192.168.1.3"})

	if got := c.fixedOverrides()["Wohnzimmer"]; got != "192.168.1.2" {
		t.Fatalf("override wohnzimmer: %q", got)
	}
	if got := c.fixedOverrides()["Kueche"]; got != "192.168.1.3" {
		t.Fatalf("override kueche: %q", got)
	}
	c.mu.RLock()
	n := len(c.devices)
	c.mu.RUnlock()
	if n != 0 {
		t.Fatalf("devices-cache soll leer sein, hat %d eintraege", n)
	}
}

func TestGroup_FehlerpfadeMitCodes(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	devs := func() map[string]*device {
		return map[string]*device{
			"W": {name: "W", baseURL: f.srv.URL, uid: "RINCON_W"},
			"K": {name: "K", baseURL: f.srv.URL, uid: "RINCON_K"},
		}
	}

	f.fault = true
	err := controllerFor(devs()).Group(context.Background(), "W", []string{"K"})
	ae := apperr.As(err)
	if ae.Code != apperr.CodeGroupingFailed || ae.Speaker != "K" || ae.Hint == "" {
		t.Fatalf("Gerät lehnt ab -> grouping_failed mit Box+Hinweis, bekam %+v", ae)
	}

	// Unbekanntes Mitglied: Discovery liefert nichts -> speaker_unreachable.
	t.Cleanup(func() { ssdpSearchHook = nil })
	ssdpSearchHook = func(time.Duration) ([]string, error) { return nil, nil }
	f.fault = false
	err = controllerFor(devs()).Group(context.Background(), "W", []string{"Unbekannt"})
	if apperr.As(err).Code != apperr.CodeSpeakerUnreachable {
		t.Fatalf("unbekanntes Mitglied: %v", err)
	}
}

func TestResolve_UnbekannteBoxMitFesterIPHatHinweis(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	other := httptest.NewServer(deviceDescriptionHandler("Andere"))
	defer other.Close()
	ssdpSearchHook = func(time.Duration) ([]string, error) {
		return []string{other.URL + "/xml/device_description.xml"}, nil
	}
	c := New(map[string]string{"Box": "127.0.0.1"}, testLogger())
	_, err := c.resolve(context.Background(), "Box")
	ae := apperr.As(err)
	if ae.Code != apperr.CodeSpeakerUnreachable || ae.Speaker != "Box" || !strings.Contains(ae.Hint, "127.0.0.1") {
		t.Fatalf("%+v", ae)
	}
}

// groupServer simuliert Coordinator (liefert Topologie) und ein Mitglied, das
// SetAVTransportURI mit HTTP 500 ablehnt.
func groupServer(t *testing.T, zoneState string) *httptest.Server {
	t.Helper()
	return groupServerFault(t, zoneState, "800")
}

// groupServerFault liefert die Topologie und lehnt alle anderen Aktionen mit
// dem UPnP-Fehlercode code ab.
func groupServerFault(t *testing.T, zoneState, code string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ZoneGroupTopology/Control" {
			io.WriteString(w, zoneGroupStateSOAP(zoneState))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><s:Fault><detail><UPnPError xmlns="urn:schemas-upnp-org:control-1-0"><errorCode>`+code+`</errorCode></UPnPError></detail></s:Fault></s:Body></s:Envelope>`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGroup_UrsachenHinweise(t *testing.T) {
	state := `<ZoneGroupState><ZoneGroups><ZoneGroup Coordinator="RINCON_W" ID="RINCON_W:1">` +
		`<ZoneGroupMember UUID="RINCON_W" ZoneName="W" Invisible="0"/>` +
		`<ZoneGroupMember UUID="RINCON_SUB" ZoneName="Sub" Invisible="1"/>` +
		`<ZoneGroupMember UUID="RINCON_K" ZoneName="K" Invisible="0"/>` +
		`</ZoneGroup></ZoneGroups></ZoneGroupState>`
	srv := groupServer(t, state)

	cases := []struct {
		name     string
		member   *device
		wantPart string
	}{
		{"gebondet/unsichtbar", &device{name: "Sub", baseURL: srv.URL, uid: "RINCON_SUB"}, "bonded"},
		{"sichtbar, SOAP-Ablehnung", &device{name: "K", baseURL: srv.URL, uid: "RINCON_K"}, "rejected the grouping"},
		{"nicht erreichbar", &device{name: "Aus", baseURL: "http://127.0.0.1:1", uid: "RINCON_A"}, "unreachable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := controllerFor(map[string]*device{
				"W":            {name: "W", baseURL: srv.URL, uid: "RINCON_W"},
				tc.member.name: tc.member,
			})
			ae := apperr.As(c.Group(context.Background(), "W", []string{tc.member.name}))
			if ae.Code != apperr.CodeGroupingFailed || ae.Speaker != tc.member.name {
				t.Fatalf("grouping_failed fuer %s erwartet, bekam %+v", tc.member.name, ae)
			}
			if !strings.Contains(ae.Hint, tc.wantPart) {
				t.Fatalf("Hinweis soll %q enthalten, bekam %q", tc.wantPart, ae.Hint)
			}
		})
	}
}

func TestResolve_GebondeteBoxHatBondedHinweis(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	other := httptest.NewServer(deviceDescriptionHandler("Andere"))
	defer other.Close()
	ssdpSearchHook = func(time.Duration) ([]string, error) {
		return []string{other.URL + "/xml/device_description.xml"}, nil
	}
	c := New(nil, testLogger())
	c.bonded = map[string]bool{"Sub Mini": true}
	_, err := c.resolve(context.Background(), "Sub Mini")
	ae := apperr.As(err)
	if ae.Code != apperr.CodeSpeakerUnreachable || !strings.Contains(ae.Hint, "bonded") {
		t.Fatalf("%+v", ae)
	}
}

// UPnP 800 beim Gruppieren: der Ziel-Coordinator ist selbst nur Mitglied
// einer anderen Gruppe (Befund 2026-10-15, Zeitplan oe3-werktags).
func TestGroup_Upnp800NenntFremdeGruppe(t *testing.T) {
	state := `<ZoneGroupState><ZoneGroups><ZoneGroup Coordinator="RINCON_X" ID="RINCON_X:1">` +
		`<ZoneGroupMember UUID="RINCON_X" ZoneName="X" Invisible="0"/>` +
		`<ZoneGroupMember UUID="RINCON_W" ZoneName="W" Invisible="0"/>` +
		`</ZoneGroup><ZoneGroup Coordinator="RINCON_K" ID="RINCON_K:1">` +
		`<ZoneGroupMember UUID="RINCON_K" ZoneName="K" Invisible="0"/>` +
		`</ZoneGroup></ZoneGroups></ZoneGroupState>`
	srv := groupServer(t, state)
	c := controllerFor(map[string]*device{
		"W": {name: "W", baseURL: srv.URL, uid: "RINCON_W"},
		"K": {name: "K", baseURL: srv.URL, uid: "RINCON_K"},
	})
	ae := apperr.As(c.Group(context.Background(), "W", []string{"K"}))
	if ae.Code != apperr.CodeGroupingFailed {
		t.Fatalf("grouping_failed erwartet: %+v", ae)
	}
	for _, want := range []string{"UPnP error code 800", "W is currently only a member of another group"} {
		if !strings.Contains(ae.Hint, want) {
			t.Errorf("Hinweis soll %q enthalten, bekam %q", want, ae.Hint)
		}
	}
}

func TestGroup_AndererUpnpFehlerBleibtAllgemein(t *testing.T) {
	state := `<ZoneGroupState><ZoneGroups><ZoneGroup Coordinator="RINCON_W" ID="RINCON_W:1">` +
		`<ZoneGroupMember UUID="RINCON_W" ZoneName="W" Invisible="0"/>` +
		`</ZoneGroup></ZoneGroups></ZoneGroupState>`
	srv := groupServerFault(t, state, "714")
	c := controllerFor(map[string]*device{
		"W": {name: "W", baseURL: srv.URL, uid: "RINCON_W"},
		"K": {name: "K", baseURL: srv.URL, uid: "RINCON_K"},
	})
	ae := apperr.As(c.Group(context.Background(), "W", []string{"K"}))
	if !strings.Contains(ae.Hint, "UPnP error code 714") || strings.Contains(ae.Hint, "only a member") {
		t.Fatalf("allgemeiner Hinweis mit Fehlercode erwartet, bekam %q", ae.Hint)
	}
}
