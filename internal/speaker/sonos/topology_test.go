package sonos

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"sonos-radio-agent/internal/apperr"
)

// rewriteTransport leitet Anfragen an feste Fake-IPs (Port 1400) auf lokale
// Testserver um, weil der Sonos-Port nicht konfigurierbar ist.
type rewriteTransport map[string]*httptest.Server

func (m rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	srv, ok := m[r.URL.Hostname()]
	if !ok {
		return http.DefaultTransport.RoundTrip(r)
	}
	u, _ := url.Parse(srv.URL)
	r2 := r.Clone(r.Context())
	r2.URL.Host = u.Host
	return http.DefaultTransport.RoundTrip(r2)
}

func fixedIPController(servers map[string]*httptest.Server, overrides map[string]string) *Controller {
	c := New(overrides, testLogger())
	c.client = &http.Client{Transport: rewriteTransport(servers), Timeout: time.Second}
	return c
}

func zoneGroupStateSOAP(state string) string {
	escaped := strings.ReplaceAll(state, "<", "&lt;")
	escaped = strings.ReplaceAll(escaped, ">", "&gt;")
	return `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1"><ZoneGroupState>` + escaped + `</ZoneGroupState></u:GetZoneGroupStateResponse></s:Body></s:Envelope>`
}

func deviceDescriptionXML(room, uid string) string {
	return `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <roomName>` + room + `</roomName>
    <UDN>uuid:` + uid + `</UDN>
  </device>
</root>`
}

func TestHiddenUUIDsFromZoneGroupState(t *testing.T) {
	state := `<ZoneGroupState><ZoneGroups>` +
		`<ZoneGroup Coordinator="RINCON_TV" ID="RINCON_TV:1">` +
		`<ZoneGroupMember UUID="RINCON_TV" ZoneName="KG: Fernseher" Invisible="0"/>` +
		`<ZoneGroupMember UUID="RINCON_SUB" ZoneName="Sub Mini" Invisible="1" HTSatChanMapSet="RINCON_TV"/>` +
		`<Satellite UUID="RINCON_REAR_L" ZoneName="KG: Fernseher"/>` +
		`</ZoneGroup></ZoneGroups></ZoneGroupState>`

	hidden := hiddenUUIDsFromZoneGroupState(state)
	for _, uid := range []string{"RINCON_SUB", "RINCON_REAR_L"} {
		if !hidden[uid] {
			t.Fatalf("erwartet unsichtbar: %s, hidden=%v", uid, hidden)
		}
	}
	if hidden["RINCON_TV"] {
		t.Fatalf("coordinator darf nicht gefiltert werden: %v", hidden)
	}
}

func TestParseZoneGroupStateSOAP(t *testing.T) {
	inner := `<ZoneGroupState><ZoneGroups></ZoneGroups></ZoneGroupState>`
	raw := []byte(zoneGroupStateSOAP(inner))
	got, err := parseZoneGroupStateSOAP(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != inner {
		t.Fatalf("erwartet entescaped XML, bekam %q", got)
	}
}

func TestRefreshDevices_FiltersBondedInvisible(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })

	zoneState := `<ZoneGroupState><ZoneGroups>` +
		`<ZoneGroup Coordinator="RINCON_TV" ID="RINCON_TV:1">` +
		`<ZoneGroupMember UUID="RINCON_TV" ZoneName="KG: Fernseher" Invisible="0"/>` +
		`<ZoneGroupMember UUID="RINCON_SUB" ZoneName="Sub Mini" Invisible="1"/>` +
		`</ZoneGroup></ZoneGroups></ZoneGroupState>`

	tvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xml/device_description.xml":
			io.WriteString(w, deviceDescriptionXML("KG: Fernseher", "RINCON_TV"))
		case "/ZoneGroupTopology/Control":
			w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
			io.WriteString(w, zoneGroupStateSOAP(zoneState))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer tvSrv.Close()

	subSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xml/device_description.xml" {
			io.WriteString(w, deviceDescriptionXML("Sub Mini", "RINCON_SUB"))
		}
	}))
	defer subSrv.Close()

	ssdpSearchHook = func(time.Duration) ([]string, error) {
		return []string{
			tvSrv.URL + "/xml/device_description.xml",
			subSrv.URL + "/xml/device_description.xml",
		}, nil
	}

	c := New(nil, testLogger())
	names, err := c.refreshDevices(context.Background())
	if err != nil {
		t.Fatalf("refreshDevices: %v", err)
	}
	if len(names) != 1 || names[0] != "KG: Fernseher" {
		t.Fatalf("erwartet nur KG: Fernseher, bekam %v", names)
	}
}

func TestRefreshDevices_TopologyFallbackGetInvisible(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })

	tvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xml/device_description.xml":
			io.WriteString(w, deviceDescriptionXML("TV", "RINCON_TV"))
		case "/ZoneGroupTopology/Control":
			w.WriteHeader(http.StatusInternalServerError)
		case "/DeviceProperties/Control":
			io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetInvisibleResponse xmlns:u="urn:schemas-upnp-org:service:DeviceProperties:1"><CurrentInvisible>0</CurrentInvisible></u:GetInvisibleResponse></s:Body></s:Envelope>`)
		}
	}))
	defer tvSrv.Close()

	subSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xml/device_description.xml":
			io.WriteString(w, deviceDescriptionXML("Sub Mini", "RINCON_SUB"))
		case "/ZoneGroupTopology/Control":
			w.WriteHeader(http.StatusInternalServerError)
		case "/DeviceProperties/Control":
			io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetInvisibleResponse xmlns:u="urn:schemas-upnp-org:service:DeviceProperties:1"><CurrentInvisible>1</CurrentInvisible></u:GetInvisibleResponse></s:Body></s:Envelope>`)
		}
	}))
	defer subSrv.Close()

	ssdpSearchHook = func(time.Duration) ([]string, error) {
		return []string{
			tvSrv.URL + "/xml/device_description.xml",
			subSrv.URL + "/xml/device_description.xml",
		}, nil
	}

	c := New(nil, testLogger())
	names, err := c.refreshDevices(context.Background())
	if err != nil {
		t.Fatalf("refreshDevices: %v", err)
	}
	if len(names) != 1 || names[0] != "TV" {
		t.Fatalf("erwartet nur TV nach GetInvisible-Fallback, bekam %v", names)
	}
}

// partnerTopology: Stereopaar mit sichtbarem Player RINCON_L und unsichtbarem
// Partner RINCON_R sowie ein weiterer sichtbarer Player RINCON_B.
func partnerTopology(t *testing.T, topoCalls *int32) (visible, partner, bad *httptest.Server) {
	t.Helper()
	zoneState := `<ZoneGroupState><ZoneGroups>` +
		`<ZoneGroup Coordinator="RINCON_L" ID="RINCON_L:1">` +
		`<ZoneGroupMember UUID="RINCON_L" ZoneName="EG: Wohnzimmer" Invisible="0"/>` +
		`<ZoneGroupMember UUID="RINCON_R" ZoneName="EG: Wohnzimmer" Invisible="1"/>` +
		`</ZoneGroup><ZoneGroup Coordinator="RINCON_B" ID="RINCON_B:1">` +
		`<ZoneGroupMember UUID="RINCON_B" ZoneName="OG: Bad" Invisible="0"/>` +
		`</ZoneGroup></ZoneGroups></ZoneGroupState>`
	mk := func(zone, uid string) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/xml/device_description.xml":
				io.WriteString(w, deviceDescriptionXML(zone, uid))
			case "/ZoneGroupTopology/Control":
				atomic.AddInt32(topoCalls, 1)
				w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
				io.WriteString(w, zoneGroupStateSOAP(zoneState))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(srv.Close)
		return srv
	}
	return mk("EG: Wohnzimmer", "RINCON_L"), mk("EG: Wohnzimmer", "RINCON_R"), mk("OG: Bad", "RINCON_B")
}

func TestEnsureSpeakers_LehntUnsichtbarenPartnerAb(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	ssdpSearchHook = func(time.Duration) ([]string, error) { return nil, nil }
	var calls int32
	visible, partner, bad := partnerTopology(t, &calls)
	_ = visible

	c := fixedIPController(map[string]*httptest.Server{"10.0.0.2": bad, "10.0.0.3": partner},
		map[string]string{"OG: Bad": "10.0.0.2", "HW Partner": "10.0.0.3"})

	err := c.EnsureSpeakers(context.Background(), []string{"OG: Bad", "HW Partner"})
	ae := apperr.As(err)
	if ae.Code != apperr.CodeSceneMisconfigured || !strings.Contains(ae.Hint, "HW Partner") || !strings.Contains(ae.Hint, "bonded") {
		t.Fatalf("scene_misconfigured mit Bonded-Hinweis erwartet, bekam %+v", ae)
	}
	c.mu.RLock()
	_, partnerKnown := c.devices["HW Partner"]
	_, badKnown := c.devices["OG: Bad"]
	failed := c.failed["HW Partner"]
	c.mu.RUnlock()
	if partnerKnown || !badKnown || !failed {
		t.Fatalf("Partner darf kein Device sein (known=%v), Bad schon (%v), Partner failed (%v)", partnerKnown, badKnown, failed)
	}
	if _, err := c.resolve(context.Background(), "HW Partner"); err == nil {
		t.Fatal("resolve soll den Partner ablehnen")
	}
	infos, _ := c.ListSpeakers(context.Background())
	for _, s := range infos {
		if s.Name == "HW Partner" && s.Reachable {
			t.Fatal("Partner darf nicht reachable sein")
		}
	}
}

func TestEnsureSpeakers_SichtbarerPlayerOkUndTopologieGecacht(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	ssdpSearchHook = func(time.Duration) ([]string, error) { return nil, nil }
	var calls int32
	visible, _, bad := partnerTopology(t, &calls)

	c := fixedIPController(map[string]*httptest.Server{"10.0.0.2": bad, "10.0.0.1": visible},
		map[string]string{"OG: Bad": "10.0.0.2", "EG: Wohnzimmer": "10.0.0.1"})
	names := []string{"OG: Bad", "EG: Wohnzimmer"}
	if err := c.EnsureSpeakers(context.Background(), names); err != nil {
		t.Fatalf("EnsureSpeakers: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("eine Topologie-Abfrage fuer beide Boxen erwartet, bekam %d", got)
	}
	if err := c.EnsureSpeakers(context.Background(), names); err != nil {
		t.Fatalf("EnsureSpeakers (2): %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("zweiter Aufruf soll den Cache nutzen, Abfragen: %d", got)
	}
}

func TestEnsureSpeakers_TopologieNichtLesbarBleibtSichtbar(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	ssdpSearchHook = func(time.Duration) ([]string, error) { return nil, nil }
	srv := httptest.NewServer(deviceDescriptionHandler("Box"))
	defer srv.Close()
	c := fixedIPController(map[string]*httptest.Server{"10.0.0.9": srv}, map[string]string{"Box": "10.0.0.9"})
	if err := c.EnsureSpeakers(context.Background(), []string{"Box"}); err != nil {
		t.Fatalf("Fail-open erwartet: %v", err)
	}
}
