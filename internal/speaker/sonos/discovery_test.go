package sonos

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/speaker"
)

func deviceDescriptionHandler(room string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xml/device_description.xml" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		io.WriteString(w, `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <roomName>`+room+`</roomName>
    <UDN>uuid:RINCON_`+room+`1400</UDN>
  </device>
</root>`)
	}
}

func TestListSpeakers_NoSSDP(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	ssdpSearchHook = func(time.Duration) ([]string, error) {
		t.Fatal("ListSpeakers darf kein SSDP ausloesen")
		return nil, nil
	}

	c := New(map[string]string{"EG: Wohnzimmer": "192.168.1.50"}, testLogger())
	infos, err := c.ListSpeakers(context.Background())
	if err != nil {
		t.Fatalf("ListSpeakers: %v", err)
	}
	if len(infos) != 1 || infos[0].Name != "EG: Wohnzimmer" {
		t.Fatalf("erwartet Config-Box, bekam %+v", infos)
	}
}

func TestRefreshSpeakers_RunsSSDP(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })

	good := httptest.NewServer(deviceDescriptionHandler("Wohnzimmer"))
	defer good.Close()

	called := false
	ssdpSearchHook = func(time.Duration) ([]string, error) {
		called = true
		return []string{good.URL + "/xml/device_description.xml"}, nil
	}

	c := New(nil, testLogger())
	if _, err := c.RefreshSpeakers(context.Background()); err != nil {
		t.Fatalf("RefreshSpeakers: %v", err)
	}
	if !called {
		t.Fatal("RefreshSpeakers soll SSDP ausloesen")
	}
}

func TestProbeFixedIPs_UnreachableCompletesQuickly(t *testing.T) {
	c := New(map[string]string{
		"A": "127.0.0.1",
		"B": "127.0.0.1",
		"C": "127.0.0.1",
	}, testLogger())

	start := time.Now()
	found := c.probeFixedIPs(context.Background())
	elapsed := time.Since(start)

	if len(found) != 0 {
		t.Fatalf("erwartet 0 boxen, bekam %d", len(found))
	}
	if elapsed > 2*time.Second {
		t.Fatalf("fixed-ip-probes zu langsam: %v", elapsed)
	}
}

func TestRefreshDevices_WrongFixedIPsStillUseSSDP(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })

	good := httptest.NewServer(deviceDescriptionHandler("Wohnzimmer"))
	defer good.Close()

	ssdpSearchHook = func(time.Duration) ([]string, error) {
		return []string{good.URL + "/xml/device_description.xml"}, nil
	}

	c := New(map[string]string{"Alt": "127.0.0.1"}, testLogger())
	names, err := c.refreshDevices(context.Background())
	if err != nil {
		t.Fatalf("refreshDevices: %v", err)
	}
	if len(names) != 1 || names[0] != "Wohnzimmer" {
		t.Fatalf("erwartet [Wohnzimmer], bekam %v", names)
	}
}

func TestFetchDevicesFromLocations_Parallel(t *testing.T) {
	srvA := httptest.NewServer(deviceDescriptionHandler("A"))
	defer srvA.Close()
	srvB := httptest.NewServer(deviceDescriptionHandler("B"))
	defer srvB.Close()

	locations := []string{
		srvA.URL + "/xml/device_description.xml",
		srvB.URL + "/xml/device_description.xml",
	}
	found := fetchDevicesFromLocations(context.Background(), http.DefaultClient, locations)
	if len(found) != 2 {
		t.Fatalf("erwartet 2 boxen, bekam %d", len(found))
	}
	if found["A"] == nil || found["B"] == nil {
		t.Fatalf("fehlende boxen: %#v", found)
	}
}

func TestFetchDevice_RespectsShortContext(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		time.Sleep(2 * time.Second)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), fixedIPProbeTimeout)
	defer cancel()
	_, err = fetchDevice(ctx, http.DefaultClient, "http://"+ln.Addr().String())
	if err == nil {
		t.Fatal("erwartet fehler bei kurzem context")
	}
}

func TestLocationMatchesSender(t *testing.T) {
	from := &net.UDPAddr{IP: net.ParseIP("192.168.1.20")}
	if !locationMatchesSender("http://192.168.1.20:1400/xml/device_description.xml", from) {
		t.Error("passende LOCATION muss akzeptiert werden")
	}
	for _, bad := range []string{"http://10.9.9.9:1400/x", "ftp://192.168.1.20/x", "::bad::"} {
		if locationMatchesSender(bad, from) {
			t.Errorf("%q muss abgelehnt werden", bad)
		}
	}
	if locationMatchesSender("http://192.168.1.20/x", nil) {
		t.Error("ohne Absender ablehnen")
	}
}

func TestFetchDeviceRejectsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`<root><device><roomName>X</roomName></device></root>`))
	}))
	defer srv.Close()
	if _, err := fetchDevice(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Fatal("HTTP 500 muss als Fehler gelten")
	}
}

func TestRefresh_EmptySSDPKeepsCache(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	good := httptest.NewServer(deviceDescriptionHandler("Wohnzimmer"))
	defer good.Close()

	results := [][]string{{good.URL + "/xml/device_description.xml"}, nil}
	call := 0
	ssdpSearchHook = func(time.Duration) ([]string, error) {
		r := results[call]
		call++
		return r, nil
	}

	c := New(nil, testLogger())
	if _, err := c.RefreshSpeakers(context.Background()); err != nil {
		t.Fatalf("erster Refresh: %v", err)
	}
	if _, err := c.RefreshSpeakers(context.Background()); err == nil {
		t.Fatal("leerer Refresh soll den Discovery-Fehler melden")
	}
	infos, _ := c.ListSpeakers(context.Background())
	if len(infos) != 1 || infos[0].Name != "Wohnzimmer" {
		t.Fatalf("Cache muss erhalten bleiben, bekam %+v", infos)
	}
}

func TestResolve_ParallelUnknownRunsOneDiscovery(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	good := httptest.NewServer(deviceDescriptionHandler("Wohnzimmer"))
	defer good.Close()

	var mu sync.Mutex
	calls := 0
	ssdpSearchHook = func(time.Duration) ([]string, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		return []string{good.URL + "/xml/device_description.xml"}, nil
	}
	c := New(nil, testLogger())
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.resolve(context.Background(), "Wohnzimmer"); err != nil {
				t.Errorf("resolve: %v", err)
			}
		}()
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("erwartet 1 SSDP-Lauf, bekam %d", calls)
	}
}

func reachableOf(t *testing.T, c *Controller, name string) bool {
	t.Helper()
	infos, _ := c.ListSpeakers(context.Background())
	for _, in := range infos {
		if in.Name == name {
			return in.Reachable
		}
	}
	t.Fatalf("Box %q nicht in ListSpeakers: %+v", name, infos)
	return false
}

// Nie geprobte Config-Boxen bleiben "erreichbar" (lazy Discovery vertraut der
// Config); nach fehlgeschlagener Probe nicht mehr (Issue #4).
func TestListSpeakers_ReachableHonestAfterFailedProbe(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	ssdpSearchHook = func(time.Duration) ([]string, error) { return nil, nil }

	// 127.0.0.1:1400 antwortet nicht -> Probe scheitert.
	c := New(map[string]string{"EG: Offline": "127.0.0.1"}, testLogger())
	if !reachableOf(t, c, "EG: Offline") {
		t.Fatal("vor jeder Probe soll die Config-Box als erreichbar gelten")
	}
	if _, err := c.RefreshSpeakers(context.Background()); err == nil {
		t.Fatal("Refresh ohne Treffer soll Discovery-Fehler melden")
	}
	if reachableOf(t, c, "EG: Offline") {
		t.Fatal("nach fehlgeschlagener Probe darf reachable nicht true sein")
	}
}

func TestEnsureSpeakers_FailedProbeMarksUnreachable(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	ssdpSearchHook = func(time.Duration) ([]string, error) { return nil, nil }

	c := New(map[string]string{"EG: Offline": "127.0.0.1"}, testLogger())
	// Config-IP, aber offline: bekannt, kein scene_misconfigured (Teilausfall).
	if err := c.EnsureSpeakers(context.Background(), []string{"EG: Offline"}); err != nil {
		t.Fatalf("konfigurierte offline Box darf nicht abgelehnt werden: %v", err)
	}
	if reachableOf(t, c, "EG: Offline") {
		t.Fatal("reachable muss false sein")
	}
}

// Config-IP zeigt auf einen unsichtbaren Partner: Probe liefert ihn nicht
// als sichtbares Geraet, also nicht erreichbar.
func TestRefresh_HiddenFixedIPNotReachable(t *testing.T) {
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
			io.WriteString(w, zoneGroupStateSOAP(zoneState))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer tvSrv.Close()
	subSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, deviceDescriptionXML("Sub Mini", "RINCON_SUB"))
	}))
	defer subSrv.Close()
	ssdpSearchHook = func(time.Duration) ([]string, error) {
		return []string{
			tvSrv.URL + "/xml/device_description.xml",
			subSrv.URL + "/xml/device_description.xml",
		}, nil
	}

	c := New(map[string]string{"Sub Mini": "127.0.0.1"}, testLogger())
	if _, err := c.RefreshSpeakers(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if reachableOf(t, c, "Sub Mini") {
		t.Fatal("unsichtbarer Partner darf nicht als erreichbar gelten")
	}
	if !reachableOf(t, c, "KG: Fernseher") {
		t.Fatal("sichtbare Box muss erreichbar bleiben")
	}
}

// Erfolgreiche Aufloesung per SSDP stellt reachable wieder her; ReloadFixedIPs
// setzt den Status zurueck.
func TestReachable_RecoversAndResets(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	good := httptest.NewServer(deviceDescriptionHandler("Wohnzimmer"))
	defer good.Close()

	ssdpSearchHook = func(time.Duration) ([]string, error) { return nil, nil }
	c := New(map[string]string{"Wohnzimmer": "127.0.0.1"}, testLogger())
	_, _ = c.RefreshSpeakers(context.Background())
	if reachableOf(t, c, "Wohnzimmer") {
		t.Fatal("erste Probe scheitert, erwartet false")
	}

	ssdpSearchHook = func(time.Duration) ([]string, error) {
		return []string{good.URL + "/xml/device_description.xml"}, nil
	}
	if _, err := c.RefreshSpeakers(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if !reachableOf(t, c, "Wohnzimmer") {
		t.Fatal("nach erfolgreicher Aufloesung erwartet true")
	}

	ssdpSearchHook = func(time.Duration) ([]string, error) { return nil, nil }
	_, _ = c.RefreshSpeakers(context.Background())
	// Cache haelt das Geraet, aber die letzte Probe der Config-IP schlug fehl.
	c.ReloadFixedIPs(map[string]string{"Wohnzimmer": "127.0.0.1"})
	if !reachableOf(t, c, "Wohnzimmer") {
		t.Fatal("nach ReloadFixedIPs gilt die Box wieder als ungeprobt")
	}
}

// Namen ohne Config-IP und ohne Discovery bleiben scene_misconfigured, auch
// wenn daneben eine konfigurierte Offline-Box in der Szene steht.
func TestEnsureSpeakers_UnknownNameStillRejectedNextToOffline(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	ssdpSearchHook = func(time.Duration) ([]string, error) { return nil, nil }

	c := New(map[string]string{"EG: Offline": "127.0.0.1"}, testLogger())
	err := c.EnsureSpeakers(context.Background(), []string{"EG: Offline", "Gibt es nicht"})
	if got := apperr.As(err).Code; got != apperr.CodeSceneMisconfigured {
		t.Fatalf("code = %q, erwartet scene_misconfigured", got)
	}
	if h := apperr.As(err).Hint; !strings.Contains(h, "Gibt es nicht") || strings.Contains(h, "Missing: EG: Offline") {
		t.Fatalf("hinweis nennt falsche Boxen: %q", h)
	}
}

// Issue #12: Szene mit erreichbarem Coordinator und konfigurierter, aber
// toter Box spielt; die tote Box erscheint mit ok:false und Hinweis.
func TestScenePlay_OfflineConfiguredMemberIsPartialFailure(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	ssdpSearchHook = func(time.Duration) ([]string, error) { return nil, nil }

	f := newFakeSonos()
	defer f.close()
	// 127.0.0.1:1400 ist nicht belegt -> Verbindung abgelehnt.
	c := New(map[string]string{"HW Tot": "127.0.0.1"}, testLogger())
	c.devices = map[string]*device{
		"OG: Bad": {name: "OG: Bad", baseURL: f.srv.URL, uid: "RINCON_B"},
	}
	svc := scenes.NewService([]scenes.Scene{{
		ID:     "s",
		Name:   "S",
		Source: speaker.Source{Type: speaker.SourceStream, URI: "https://example/stream"},
		Members: []scenes.Member{
			{Speaker: "OG: Bad", Volume: 20},
			{Speaker: "HW Tot", Volume: 20},
		},
	}}, c, testLogger())

	res, err := svc.Play(context.Background(), "s")
	if err != nil {
		t.Fatalf("teilausfall darf kein harter fehler sein: %v", err)
	}
	if !res.Playing {
		t.Fatal("coordinator erreichbar -> playing erwartet")
	}
	for _, st := range res.Speakers {
		switch st.Speaker {
		case "OG: Bad":
			if !st.OK {
				t.Errorf("Bad sollte ok sein: %s", st.Error)
			}
		case "HW Tot":
			if st.OK || st.Error == "" || st.Hint == "" {
				t.Errorf("HW Tot sollte ok:false mit Fehler und Hinweis sein: %+v", st)
			}
		}
	}
}

// Ist der Coordinator selbst offline, bleibt es beim harten Fehler
// speaker_unreachable (nicht scene_misconfigured).
func TestScenePlay_OfflineCoordinatorStillFails(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	ssdpSearchHook = func(time.Duration) ([]string, error) { return nil, nil }

	c := New(map[string]string{"HW Tot": "127.0.0.1"}, testLogger())
	svc := scenes.NewService([]scenes.Scene{{
		ID:      "s",
		Name:    "S",
		Source:  speaker.Source{Type: speaker.SourceStream, URI: "https://example/stream"},
		Members: []scenes.Member{{Speaker: "HW Tot", Volume: 20}},
	}}, c, testLogger())

	_, err := svc.Play(context.Background(), "s")
	if got := apperr.As(err).Code; got != apperr.CodeSpeakerUnreachable {
		t.Fatalf("code = %q, erwartet speaker_unreachable (err=%v)", got, err)
	}
}
