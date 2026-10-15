package sonos

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

// tuneInResMD entspricht dem r:resMD des Favoriten „Radio Superfly“ auf
// echter Hardware (2026-10-15).
const tuneInResMD = `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:r="urn:schemas-rinconnetworks-com:metadata-1-0/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"><item id="10092028tunein%3a17315" parentID="00020000station:superfly" restricted="true"><dc:title>Radio Superfly</dc:title><upnp:class>object.item.audioItem.audioBroadcast</upnp:class><desc id="cdudn" nameSpace="urn:schemas-rinconnetworks-com:metadata-1-0/">SA_RINCON77575_X_#Svc77575-d6c9671f-Token</desc></item></DIDL-Lite>`

const tuneInURI = "x-sonosapi-stream:tunein%3a17315?sid=303&flags=8232&sn=9"

// serviceFavoritesDIDL: TuneIn-Favorit mit r:resMD, Amazon-Sender
// (x-sonosapi-radio) und TuneIn ohne resMD.
var serviceFavoritesDIDL = `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:r="urn:schemas-rinconnetworks-com:metadata-1-0/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">` +
	`<item id="FV:2/78" parentID="FV:2" restricted="false"><dc:title>Radio Superfly</dc:title><upnp:class>object.itemobject.item.sonos-favorite</upnp:class>` +
	`<res protocolInfo="x-sonosapi-stream:*:*:*">` + escapeXML(tuneInURI) + `</res><r:type>instantPlay</r:type>` +
	`<r:resMD>` + escapeXML(tuneInResMD) + `</r:resMD></item>` +
	`<item id="FV:2/53" parentID="FV:2" restricted="false"><dc:title>Entspannter Jazz</dc:title><upnp:class>object.itemobject.item.sonos-favorite</upnp:class>` +
	`<res protocolInfo="x-sonosapi-radio:*:*:*">x-sonosapi-radio:catalog%2fstations%2fA1P869RUAQNSRE?sid=201&amp;flags=8300&amp;sn=1</res>` +
	`<r:resMD>` + escapeXML(`<DIDL-Lite><item id="x"><dc:title>Entspannter Jazz</dc:title></item></DIDL-Lite>`) + `</r:resMD></item>` +
	`<item id="FV:2/9" parentID="FV:2" restricted="true"><dc:title>Ohne Metadaten</dc:title><upnp:class>object.itemobject.item.sonos-favorite</upnp:class>` +
	`<res protocolInfo="x-sonosapi-stream:*:*:*">x-sonosapi-stream:s106914?sid=254&amp;flags=32</res></item>` +
	`</DIDL-Lite>`

func TestParseFavoritesDIDL_DienstFavoritMitResMD(t *testing.T) {
	favs, err := parseFavoritesDIDL(serviceFavoritesDIDL)
	if err != nil || len(favs) != 3 {
		t.Fatalf("favs=%+v err=%v", favs, err)
	}
	tunein := favs[0]
	if tunein.URI != tuneInURI {
		t.Errorf("URI: %q", tunein.URI)
	}
	if tunein.ResMD != tuneInResMD {
		t.Errorf("resMD nicht einmal dekodiert:\n%s", tunein.ResMD)
	}
	if tunein.Integratable || !tunein.Playable {
		t.Errorf("TuneIn: integratable=%v playable=%v, erwartet false/true", tunein.Integratable, tunein.Playable)
	}
	if favs[1].Playable {
		t.Error("x-sonosapi-radio (Amazon) darf nicht playable sein")
	}
	if favs[2].Playable {
		t.Error("x-sonosapi-stream ohne resMD darf nicht playable sein")
	}
}

// serviceFavoriteSonos simuliert Browse + AVTransport; states liefert der
// Reihe nach den Transportstatus nach Play (letzter Wert bleibt stehen), ab
// dem startsAt-ten Play (1-basiert). Davor bleibt die Box STOPPED.
type serviceFavoriteSonos struct {
	mu       sync.Mutex
	srv      *httptest.Server
	sets     []string // CurrentURI|CurrentURIMetaData je SetAVTransportURI
	plays    int
	startsAt int
	states   []string
	polls    int
}

func newServiceFavoriteSonos(states ...string) *serviceFavoriteSonos {
	f := &serviceFavoriteSonos{states: states, startsAt: 1}
	escaped := escapeXML(serviceFavoritesDIDL)
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body := string(raw)
		action := r.Header.Get("SOAPACTION")
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
		switch {
		case strings.Contains(action, "#Browse"):
			io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:BrowseResponse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1"><Result>`+escaped+`</Result><NumberReturned>3</NumberReturned><TotalMatches>3</TotalMatches></u:BrowseResponse></s:Body></s:Envelope>`)
			return
		case strings.Contains(action, "#SetAVTransportURI"):
			f.sets = append(f.sets, between(body, "<CurrentURI>", "</CurrentURI>")+"|"+between(body, "<CurrentURIMetaData>", "</CurrentURIMetaData>"))
		case strings.HasSuffix(action, "#Play\""):
			f.plays++
			f.polls = 0
		case strings.Contains(action, "#GetTransportInfo"):
			state := "STOPPED"
			if f.plays >= f.startsAt && f.startsAt > 0 && len(f.states) > 0 {
				i := f.polls
				if i >= len(f.states) {
					i = len(f.states) - 1
				}
				state = f.states[i]
			}
			f.polls++
			io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetTransportInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><CurrentTransportState>`+state+`</CurrentTransportState></u:GetTransportInfoResponse></s:Body></s:Envelope>`)
			return
		}
		io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body></s:Body></s:Envelope>`)
	}))
	return f
}

func between(s, from, to string) string {
	i := strings.Index(s, from)
	if i < 0 {
		return ""
	}
	s = s[i+len(from):]
	if j := strings.Index(s, to); j >= 0 {
		return s[:j]
	}
	return ""
}

func TestPlayFavorite_TuneInMitResMD(t *testing.T) {
	f := newServiceFavoriteSonos("TRANSITIONING", "TRANSITIONING", "PLAYING")
	defer f.srv.Close()
	c := controllerFor(map[string]*device{"Bedroom": {name: "Bedroom", baseURL: f.srv.URL, uid: "RINCON_F"}})

	err := c.Play(context.Background(), "Bedroom", speaker.Source{Type: speaker.SourceFavorite, Name: "radio superfly"})
	if err != nil {
		t.Fatalf("Play: %v", err)
	}
	if len(f.sets) != 1 || f.plays != 1 {
		t.Fatalf("sets=%d plays=%d, erwartet je 1", len(f.sets), f.plays)
	}
	uri, meta, _ := strings.Cut(f.sets[0], "|")
	if uri != escapeXML(tuneInURI) {
		t.Errorf("CurrentURI: %q", uri)
	}
	// Metadaten sind das r:resMD des Favoriten, im SOAP-Body genau einmal escapt.
	if meta != escapeXML(tuneInResMD) {
		t.Errorf("CurrentURIMetaData:\n%s", meta)
	}
}

func TestPlayFavorite_TuneInZweiterVersuchSpielt(t *testing.T) {
	f := newServiceFavoriteSonos("PLAYING")
	f.startsAt = 2
	defer f.srv.Close()
	c := controllerFor(map[string]*device{"Bedroom": {name: "Bedroom", baseURL: f.srv.URL, uid: "RINCON_F"}})

	if err := c.Play(context.Background(), "Bedroom", speaker.Source{Type: speaker.SourceFavorite, Name: "Radio Superfly"}); err != nil {
		t.Fatalf("zweiter Versuch spielt, erwartet nil: %v", err)
	}
	if len(f.sets) != 2 || f.plays != 2 {
		t.Errorf("sets=%d plays=%d, erwartet je 2", len(f.sets), f.plays)
	}
}

func TestPlayFavorite_TuneInStartetNicht(t *testing.T) {
	f := newServiceFavoriteSonos("STOPPED")
	defer f.srv.Close()
	c := controllerFor(map[string]*device{"Bedroom": {name: "Bedroom", baseURL: f.srv.URL, uid: "RINCON_F"}})

	err := c.Play(context.Background(), "Bedroom", speaker.Source{Type: speaker.SourceFavorite, Name: "Radio Superfly"})
	ae := apperr.As(err)
	if err == nil || ae.Code != apperr.CodePlaybackFailed {
		t.Fatalf("erwartet playback_failed, bekam %v", err)
	}
	if !strings.Contains(ae.Message, "Radio Superfly") || !strings.Contains(ae.Hint, "STOPPED") || ae.Speaker != "Bedroom" {
		t.Errorf("Fehler ohne Favorit/Status/Box: %+v", ae)
	}
	if len(f.sets) != 2 || f.plays != 2 {
		t.Errorf("erwartet genau einen Wiederholungsversuch: sets=%d plays=%d", len(f.sets), f.plays)
	}
}

func TestPlayFavorite_AmazonNichtAbspielbar(t *testing.T) {
	f := newServiceFavoriteSonos("PLAYING")
	defer f.srv.Close()
	c := controllerFor(map[string]*device{"Bedroom": {name: "Bedroom", baseURL: f.srv.URL, uid: "RINCON_F"}})

	for _, name := range []string{"Entspannter Jazz", "Ohne Metadaten"} {
		err := c.Play(context.Background(), "Bedroom", speaker.Source{Type: speaker.SourceFavorite, Name: name})
		if ae := apperr.As(err); err == nil || ae.Code != apperr.CodePlaybackFailed || ae.Hint == "" {
			t.Errorf("%s: erwartet playback_failed mit Hinweis, bekam %v", name, err)
		}
	}
	if len(f.sets) != 0 {
		t.Errorf("nicht abspielbarer Favorit darf keine URI setzen: %v", f.sets)
	}
}

func TestPlayFavorite_UnbekannterTitel(t *testing.T) {
	f := newServiceFavoriteSonos("PLAYING")
	defer f.srv.Close()
	c := controllerFor(map[string]*device{"Bedroom": {name: "Bedroom", baseURL: f.srv.URL, uid: "RINCON_F"}})

	err := c.Play(context.Background(), "Bedroom", speaker.Source{Type: speaker.SourceFavorite, Name: "Superfly Beach"})
	if ae := apperr.As(err); err == nil || ae.Code != apperr.CodePlaybackFailed || !strings.Contains(ae.Message, "Superfly Beach") {
		t.Fatalf("erwartet playback_failed „nicht gefunden“, bekam %v", err)
	}
}

func TestGetNowPlaying_TuneInMeldetQuelleNebenTrack(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	f.transportState = "PLAYING"
	f.currentURI = tuneInURI
	f.currentMeta = strings.Replace(tuneInResMD, `id="10092028tunein%3a17315"`, `id="-1"`, 1)
	f.trackURI = "x-rincon-mp3radio://https://tunein-ondemand.cdnstream1.com/multi_bump_sonic_pre_pre.mp3?x=1"
	f.trackMeta = `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/"><item id="-1"><dc:title>multi_bump_sonic_pre_pre.mp3?x=1</dc:title></item></DIDL-Lite>`
	c := controllerFor(map[string]*device{"Bedroom": {name: "Bedroom", baseURL: f.srv.URL, uid: "RINCON_F"}})

	np, err := c.GetNowPlaying(context.Background(), "Bedroom")
	if err != nil {
		t.Fatal(err)
	}
	if np.SourceURI != tuneInURI || np.SourceTitle != "Radio Superfly" {
		t.Errorf("Quelle: uri=%q title=%q", np.SourceURI, np.SourceTitle)
	}
	// Titel aus der Track-URL ersetzt der Sendername.
	if np.URI != f.trackURI || np.Title != "Radio Superfly" {
		t.Errorf("Track: uri=%q title=%q", np.URI, np.Title)
	}
	src := speaker.Source{Type: speaker.SourceFavorite, Name: "Radio Superfly", URI: "x-sonosapi-stream:tunein%3A17315?sid=303&flags=8292&sn=9"}
	if !speaker.SourceMatchesPlayback(src, np) {
		t.Error("Szene mit TuneIn-Favorit muss als laufend erkannt werden")
	}
}

func TestGetNowPlaying_TuneInEchterTitelBleibt(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	f.transportState = "PLAYING"
	f.currentURI = tuneInURI
	f.currentMeta = tuneInResMD
	f.trackURI = "x-rincon-mp3radio://https://edge.example/superfly-live?aggregator=tunein"
	f.trackMeta = `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"><item id="-1"><dc:title>Superfly Live</dc:title><upnp:album>Radio Superfly</upnp:album></item></DIDL-Lite>`
	c := controllerFor(map[string]*device{"Bedroom": {name: "Bedroom", baseURL: f.srv.URL, uid: "RINCON_F"}})

	np, err := c.GetNowPlaying(context.Background(), "Bedroom")
	if err != nil || np.Title != "Superfly Live" || np.SourceTitle != "Radio Superfly" {
		t.Fatalf("np=%+v err=%v", np, err)
	}
}

// Hardware finding 10 (2026-10-15): during the TuneIn pre-roll the box
// reports title and URI in different encodings (":443" and "%2c" only in the
// URI), so a plain substring match missed it.
func TestGetNowPlaying_TuneInPreRollAndersKodiert(t *testing.T) {
	f := newFakeSonos()
	defer f.close()
	f.transportState = "PLAYING"
	f.currentURI = tuneInURI
	f.currentMeta = tuneInResMD
	f.trackURI = "x-rincon-mp3radio://https://tunein-ondemand.cdnstream1.com:443/multi_bump_sonic_pre_pre.mp3?aw_0_1st.playerid=TuneIn%2cSonos&aw_0_1st.skey=17315"
	f.trackMeta = `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/"><item id="-1"><dc:title>multi_bump_sonic_pre_pre.mp3?aw_0_1st.playerid=TuneIn,Sonos&amp;aw_0_1st.skey=17315</dc:title></item></DIDL-Lite>`
	c := controllerFor(map[string]*device{"EG: Wohnzimmer": {name: "EG: Wohnzimmer", baseURL: f.srv.URL, uid: "RINCON_F"}})

	np, err := c.GetNowPlaying(context.Background(), "EG: Wohnzimmer")
	if err != nil {
		t.Fatal(err)
	}
	if np.URI != f.trackURI || np.SourceTitle != "Radio Superfly" || np.Title != "Radio Superfly" {
		t.Errorf("uri=%q source_title=%q title=%q, want station name as title", np.URI, np.SourceTitle, np.Title)
	}
}

func TestTitleIsTrackURL(t *testing.T) {
	const preRoll = "x-rincon-mp3radio://https://tunein-ondemand.cdnstream1.com:443/multi_bump_sonic_pre_pre.mp3?aw_0_1st.playerid=TuneIn%2cSonos"
	tests := []struct {
		title, uri string
		want       bool
	}{
		{"", preRoll, true},
		{"multi_bump_sonic_pre_pre.mp3?aw_0_1st.playerid=TuneIn%2cSonos", preRoll, true},
		{"multi_bump_sonic_pre_pre.mp3?aw_0_1st.playerid=TuneIn,Sonos", preRoll, true},
		{"multi_bump_sonic_pre_pre.mp3?aw_0_1st.pl…", preRoll, true},
		{"https://tunein-ondemand.cdnstream1.com/multi_bump_sonic_pre_pre.mp3", preRoll, true},
		{"my%20station?x=1", "x-rincon-mp3radio://https://edge.example/my station?x=1", true},
		{"Superfly Live", "x-rincon-mp3radio://https://edge.example/superfly-live?aggregator=tunein", false},
		{"Get Lucky", preRoll, false},
		{"AC/DC – Thunderstruck", "x-rincon-mp3radio://https://edge.example/live?x=1", false},
		{"live?", "x-rincon-mp3radio://https://edge.example/?x=1", false},
	}
	for _, tt := range tests {
		if got := titleIsTrackURL(tt.title, tt.uri); got != tt.want {
			t.Errorf("titleIsTrackURL(%q, %q) = %v, want %v", tt.title, tt.uri, got, tt.want)
		}
	}
}
