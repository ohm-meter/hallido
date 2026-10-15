package sonos

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"sonos-radio-agent/internal/apperr"
)

// zoneStateTwoGroups: Wohnzimmer fuehrt Kueche + Bad, Buero steht allein;
// Sub (unsichtbar) und ein Boost (Bridge) werden ausgeblendet.
const zoneStateTwoGroups = `<ZoneGroupState><ZoneGroups>` +
	`<ZoneGroup Coordinator="RINCON_W" ID="RINCON_W:1">` +
	`<ZoneGroupMember UUID="RINCON_K" ZoneName="Kueche" Location="http://192.168.1.12:1400/xml/device_description.xml" Invisible="0"/>` +
	`<ZoneGroupMember UUID="RINCON_W" ZoneName="Wohnzimmer" Location="http://192.168.1.11:1400/xml/device_description.xml">` +
	`<Satellite UUID="RINCON_REAR" ZoneName="Wohnzimmer" Invisible="1"/></ZoneGroupMember>` +
	`<ZoneGroupMember UUID="RINCON_SUB" ZoneName="Sub" Invisible="1"/>` +
	`<ZoneGroupMember UUID="RINCON_B" ZoneName="Bad" Location="http://192.168.1.13:1400/xml/device_description.xml"/>` +
	`</ZoneGroup>` +
	`<ZoneGroup Coordinator="RINCON_O" ID="RINCON_O:2">` +
	`<ZoneGroupMember UUID="RINCON_O" ZoneName="Buero" Location="http://192.168.1.14:1400/xml/device_description.xml"/>` +
	`</ZoneGroup>` +
	`<ZoneGroup Coordinator="RINCON_BOOST" ID="RINCON_BOOST:3">` +
	`<ZoneGroupMember UUID="RINCON_BOOST" ZoneName="BOOST" IsZoneBridge="1"/>` +
	`</ZoneGroup>` +
	`</ZoneGroups></ZoneGroupState>`

func TestParseZoneGroups(t *testing.T) {
	groups, err := parseZoneGroups(zoneStateTwoGroups)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("erwartet 2 gruppen (Boost ausgeblendet), bekam %d: %+v", len(groups), groups)
	}
	var names []string
	for _, m := range groups[0].members {
		names = append(names, m.name)
	}
	if groups[0].coordinator.name != "Wohnzimmer" || !reflect.DeepEqual(names, []string{"Wohnzimmer", "Bad", "Kueche"}) {
		t.Fatalf("gruppe 1 falsch: coord=%s members=%v", groups[0].coordinator.name, names)
	}
	if groups[0].coordinator.baseURL != "http://192.168.1.11:1400" {
		t.Errorf("baseURL aus Location falsch: %q", groups[0].coordinator.baseURL)
	}
	if groups[1].coordinator.name != "Buero" || len(groups[1].members) != 1 {
		t.Fatalf("gruppe 2 falsch: %+v", groups[1])
	}
}

func TestParseZoneGroups_OldFirmwareRoot(t *testing.T) {
	state := `<ZoneGroups><ZoneGroup Coordinator="RINCON_A" ID="x"><ZoneGroupMember UUID="RINCON_A" ZoneName="A"/></ZoneGroup></ZoneGroups>`
	groups, err := parseZoneGroups(state)
	if err != nil || len(groups) != 1 || groups[0].coordinator.name != "A" {
		t.Fatalf("groups=%+v err=%v", groups, err)
	}
}

func TestParseZoneGroups_Broken(t *testing.T) {
	if _, err := parseZoneGroups(`<ZoneGroups><ZoneGroup Coordinator="A"><ZoneGroupMember`); err == nil {
		t.Fatal("kaputtes XML muss einen Fehler liefern")
	}
}

func topologyServer(t *testing.T, state string, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ZoneGroupTopology/Control" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		io.WriteString(w, zoneGroupStateSOAP(state))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGetGroups_ReadsTopologyAndLearnsDevices(t *testing.T) {
	srv := topologyServer(t, zoneStateTwoGroups, http.StatusOK)
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: srv.URL, uid: "RINCON_W"},
	})
	groups, err := c.GetGroups(context.Background())
	if err != nil {
		t.Fatalf("GetGroups: %v", err)
	}
	if len(groups) != 2 || groups[0].Coordinator != "Buero" || groups[1].Coordinator != "Wohnzimmer" {
		t.Fatalf("groups (nach Coordinator sortiert) falsch: %+v", groups)
	}
	if !reflect.DeepEqual(groups[1].Members, []string{"Wohnzimmer", "Bad", "Kueche"}) {
		t.Fatalf("members: %v", groups[1].Members)
	}
	// Bekannte Box bleibt, neue Boxen kommen aus Location in den Cache.
	if c.devices["Wohnzimmer"].baseURL != srv.URL {
		t.Errorf("bekannte Box darf nicht ueberschrieben werden")
	}
	if d, ok := c.devices["Kueche"]; !ok || d.baseURL != "http://192.168.1.12:1400" || d.uid != "RINCON_K" {
		t.Errorf("Kueche nicht gelernt: %+v", d)
	}
}

func TestGetGroups_FallsBackToNextDevice(t *testing.T) {
	bad := topologyServer(t, "", http.StatusInternalServerError)
	good := topologyServer(t, zoneStateTwoGroups, http.StatusOK)
	c := controllerFor(map[string]*device{
		"A-kaputt": {name: "A-kaputt", baseURL: bad.URL, uid: "RINCON_X"},
		"B-ok":     {name: "B-ok", baseURL: good.URL, uid: "RINCON_Y"},
	})
	groups, err := c.GetGroups(context.Background())
	if err != nil || len(groups) != 2 {
		t.Fatalf("fallback auf zweite Box erwartet: groups=%v err=%v", groups, err)
	}
}

func TestGetGroups_AllUnreachable(t *testing.T) {
	srv := topologyServer(t, "", http.StatusOK)
	url := srv.URL
	srv.Close()
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: url, uid: "RINCON_W"},
	})
	_, err := c.GetGroups(context.Background())
	if got := apperr.As(err).Code; got != apperr.CodeSpeakerUnreachable {
		t.Fatalf("code = %q, want speaker_unreachable (err=%v)", got, err)
	}
}

func TestGetGroups_RejectedIsPlaybackFailed(t *testing.T) {
	srv := topologyServer(t, "", http.StatusInternalServerError)
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: srv.URL, uid: "RINCON_W"},
	})
	_, err := c.GetGroups(context.Background())
	if got := apperr.As(err).Code; got != apperr.CodePlaybackFailed {
		t.Fatalf("code = %q, want playback_failed (err=%v)", got, err)
	}
}

func TestResumeAndUngroup_SendAVTransportSOAP(t *testing.T) {
	for _, tc := range []struct {
		action string
		call   func(*Controller) error
		body   string
	}{
		{"Play", func(c *Controller) error { return c.Resume(context.Background(), "Wohnzimmer") }, "<Speed>1</Speed>"},
		{"BecomeCoordinatorOfStandaloneGroup", func(c *Controller) error { return c.Ungroup(context.Background(), "Wohnzimmer") }, "<InstanceID>0</InstanceID>"},
	} {
		f := newFakeSonos()
		c := controllerFor(map[string]*device{
			"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_W"},
		})
		if err := tc.call(c); err != nil {
			t.Fatalf("%s: %v", tc.action, err)
		}
		if len(f.calls) != 1 {
			t.Fatalf("%s: erwartet 1 aufruf (kein SetAVTransportURI), bekam %d", tc.action, len(f.calls))
		}
		call := f.calls[0]
		if call.Path != "/MediaRenderer/AVTransport/Control" || !strings.HasSuffix(call.Action, "AVTransport:1#"+tc.action+`"`) {
			t.Errorf("%s: falscher aufruf %s %s", tc.action, call.Path, call.Action)
		}
		if !strings.Contains(call.Body, tc.body) {
			t.Errorf("%s: %s fehlt: %s", tc.action, tc.body, call.Body)
		}
		f.close()
	}
}

func TestResume_NoMediaIsPlaybackFailed(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	f.fault = true // 701
	c := controllerFor(map[string]*device{
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_W"},
	})
	err := c.Resume(context.Background(), "Wohnzimmer")
	ae := apperr.As(err)
	if ae.Code != apperr.CodePlaybackFailed || ae.Hint == "" {
		t.Fatalf("erwartet playback_failed mit hinweis, bekam %+v", ae)
	}
}

func TestResumeAndUngroup_Errors(t *testing.T) {
	calls := map[string]func(*Controller) error{
		"Resume":  func(c *Controller) error { return c.Resume(context.Background(), "Wohnzimmer") },
		"Ungroup": func(c *Controller) error { return c.Ungroup(context.Background(), "Wohnzimmer") },
	}
	for name, call := range calls {
		// Box lehnt ab.
		f := newFakeSonos()
		f.fault = true
		f.faultCode = "800"
		c := controllerFor(map[string]*device{
			"Wohnzimmer": {name: "Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_W"},
		})
		if got := apperr.As(call(c)).Code; got != apperr.CodePlaybackFailed {
			t.Errorf("%s abgelehnt: code = %q, want playback_failed", name, got)
		}
		// Box offline.
		f.close()
		if got := apperr.As(call(c)).Code; got != apperr.CodeSpeakerUnreachable {
			t.Errorf("%s offline: code = %q, want speaker_unreachable", name, got)
		}
	}
}
