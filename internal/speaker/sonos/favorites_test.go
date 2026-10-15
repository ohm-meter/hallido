package sonos

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sonos-radio-agent/internal/speaker"
)

const sampleFavoritesDIDL = `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">
<item id="FV:2/21" parentID="FV:2" restricted="true">
  <dc:title>Spotify Track</dc:title>
  <upnp:class>object.itemobject.item.sonos-favorite</upnp:class>
  <res protocolInfo="sonos.com-spotify:*:audio/x-spotify:*">x-sonos-spotify:spotify:track:7nDoBWDvf02SyD8kEQuuPO?sid=9&amp;flags=8224</res>
  <upnp:albumArtURI>https://i.scdn.co/image/example</upnp:albumArtURI>
</item>
<item id="FV:2/9" parentID="FV:2" restricted="true">
  <dc:title>Q-Dance Radio</dc:title>
  <upnp:class>object.itemobject.item.sonos-favorite</upnp:class>
  <res protocolInfo="x-sonosapi-stream:*:*:*">x-sonosapi-stream:s106914?sid=254&amp;flags=32</res>
</item>
<item id="FV:2/3" parentID="FV:2" restricted="true">
  <dc:title>FM4 Stream</dc:title>
  <upnp:class>object.itemobject.item.sonos-favorite</upnp:class>
  <res protocolInfo="http-get:*:audio/mpeg:*">https://stream.example/fm4</res>
</item>
<container id="FV:2/99" parentID="FV:2" restricted="true">
  <dc:title>Ordner</dc:title>
  <upnp:class>object.container</upnp:class>
</container>
</DIDL-Lite>`

func TestParseFavoritesDIDL(t *testing.T) {
	favs, err := parseFavoritesDIDL(sampleFavoritesDIDL)
	if err != nil || len(favs) != 3 {
		t.Fatalf("favs=%v err=%v", favs, err)
	}
	if favs[0].ID != "FV:2/21" || favs[0].AlbumArt == "" {
		t.Fatalf("favorit 0: %+v", favs[0])
	}
	if favs[0].Integratable {
		t.Fatal("spotify darf nicht integrierbar sein")
	}
	if favs[2].Integratable != true || favs[2].URI == "" {
		t.Fatalf("fm4: %+v", favs[2])
	}
}

func TestParseFavoritesDIDL_SkipsContainerClass(t *testing.T) {
	didl := `<DIDL-Lite><item id="FV:2/1"><dc:title>X</dc:title><upnp:class>object.container.playlistContainer</upnp:class></item></DIDL-Lite>`
	favs, err := parseFavoritesDIDL(didl)
	if err != nil || len(favs) != 0 {
		t.Fatalf("container item skip: favs=%v err=%v", favs, err)
	}
}

func TestBrowseFavorites_SOAPMock(t *testing.T) {
	escaped := escapeXML(sampleFavoritesDIDL)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != contentDirectory.controlPath || !strings.Contains(r.Header.Get("SOAPACTION"), "Browse") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
		io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:BrowseResponse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1"><Result>`+escaped+`</Result><NumberReturned>3</NumberReturned><TotalMatches>3</TotalMatches><UpdateID>1</UpdateID></u:BrowseResponse></s:Body></s:Envelope>`)
	}))
	defer srv.Close()
	c := controllerFor(map[string]*device{"Kueche": {name: "Kueche", baseURL: srv.URL, uid: "RINCON_K"}})
	favs, err := c.BrowseFavorites(context.Background(), "Kueche")
	if err != nil || len(favs) != 3 {
		t.Fatalf("favs=%v err=%v", favs, err)
	}
}

func TestBrowseFavorites_FernseherHTTP500_WohnzimmerOK(t *testing.T) {
	tvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("SOAPACTION"), "Browse") {
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><s:Fault><faultstring>ContentDirectory unavailable</faultstring><detail><UPnPError xmlns="urn:schemas-upnp-org:control-1-0"><errorCode>701</errorCode></UPnPError></detail></s:Fault></s:Body></s:Envelope>`)
			return
		}
		http.NotFound(w, r)
	}))
	defer tvSrv.Close()

	escaped := escapeXML(`<DIDL-Lite><item id="FV:2/1"><dc:title>FM4</dc:title><upnp:class>object.item</upnp:class><res>https://stream.example/fm4</res></item></DIDL-Lite>`)
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != contentDirectory.controlPath || !strings.Contains(r.Header.Get("SOAPACTION"), "Browse") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
		io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:BrowseResponse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1"><Result>`+escaped+`</Result><NumberReturned>1</NumberReturned><TotalMatches>1</TotalMatches></u:BrowseResponse></s:Body></s:Envelope>`)
	}))
	defer okSrv.Close()

	c := controllerFor(map[string]*device{
		"Fernseher":  {name: "Fernseher", baseURL: tvSrv.URL, uid: "RINCON_TV"},
		"Wohnzimmer": {name: "Wohnzimmer", baseURL: okSrv.URL, uid: "RINCON_W"},
	})
	favs, err := c.browseFavoritesPrefer(context.Background(), "Fernseher")
	if err != nil || len(favs) != 1 {
		t.Fatalf("favs=%v err=%v", favs, err)
	}
	if favs[0].Title != "FM4" {
		t.Fatalf("favorit: %+v", favs[0])
	}
}

func TestBrowseFavorites_Pagination(t *testing.T) {
	page1 := `<DIDL-Lite><item id="FV:2/1"><dc:title>A</dc:title><upnp:class>object.item</upnp:class><res>https://a.example/1</res></item></DIDL-Lite>`
	page2 := `<DIDL-Lite><item id="FV:2/2"><dc:title>B</dc:title><upnp:class>object.item</upnp:class><res>https://b.example/2</res></item></DIDL-Lite>`
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		start := "0"
		if strings.Contains(string(body), "<StartingIndex>1</StartingIndex>") {
			start = "1"
		}
		var didl string
		var returned, total int
		if start == "0" {
			didl, returned, total = page1, 1, 2
		} else {
			didl, returned, total = page2, 1, 2
		}
		escaped := escapeXML(didl)
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
		io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:BrowseResponse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1"><Result>`+escaped+`</Result><NumberReturned>`+itoa(returned)+`</NumberReturned><TotalMatches>`+itoa(total)+`</TotalMatches></u:BrowseResponse></s:Body></s:Envelope>`)
	}))
	defer srv.Close()
	c := controllerFor(map[string]*device{"Z": {name: "Z", baseURL: srv.URL, uid: "U"}})
	favs, err := c.BrowseFavorites(context.Background(), "Z")
	if err != nil || len(favs) != 2 || calls != 2 {
		t.Fatalf("favs=%v calls=%d err=%v", favs, calls, err)
	}
}

func TestPlayFavorite_Integratable(t *testing.T) {
	didl := `<DIDL-Lite><item id="FV:2/1"><dc:title>FM4 Stream</dc:title><upnp:class>object.item</upnp:class><res>https://stream.example/fm4</res></item></DIDL-Lite>`
	srv := favoritesSOAPServer(didl, true)
	defer srv.Close()
	c := controllerFor(map[string]*device{"W": {name: "W", baseURL: srv.URL, uid: "RINCON_W"}})
	err := c.Play(context.Background(), "W", speaker.Source{Type: speaker.SourceFavorite, Name: "FM4 Stream"})
	if err != nil {
		t.Fatal(err)
	}
}

func favoritesSOAPServer(didl string, withPlay bool) *httptest.Server {
	escaped := escapeXML(didl)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.Header.Get("SOAPACTION")
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
		if strings.Contains(action, "Browse") {
			io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:BrowseResponse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1"><Result>`+escaped+`</Result><NumberReturned>1</NumberReturned><TotalMatches>1</TotalMatches></u:BrowseResponse></s:Body></s:Envelope>`)
			return
		}
		if withPlay && strings.Contains(action, "SetAVTransportURI") {
			io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body></s:Body></s:Envelope>`)
			return
		}
		if withPlay && strings.Contains(action, "Play") {
			io.WriteString(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body></s:Body></s:Envelope>`)
			return
		}
		http.NotFound(w, r)
	}))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func TestParseBrowseResponse_KeinDoppeltesUnescape(t *testing.T) {
	// SOAP-Ebene: Result ist escaptes DIDL; im DIDL steht "Tom &amp;amp; Jerry"
	// (Titel mit literalem "&amp;") und eine URI mit "&amp;" fuer "&".
	didl := `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"><item id="FV:2/1"><dc:title>Tom &amp;amp; Jerry</dc:title><res>http://a.example/s?x=1&amp;y=2</res></item></DIDL-Lite>`
	soap := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:BrowseResponse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1"><Result>` + escapeXML(didl) + `</Result><NumberReturned>1</NumberReturned><TotalMatches>1</TotalMatches></u:BrowseResponse></s:Body></s:Envelope>`
	br, err := parseBrowseResponse([]byte(soap))
	if err != nil {
		t.Fatal(err)
	}
	favs, _ := parseFavoritesDIDL(br.Result)
	if len(favs) != 1 {
		t.Fatalf("%+v", favs)
	}
	if favs[0].Title != "Tom &amp; Jerry" {
		t.Errorf("Titel doppelt dekodiert: %q", favs[0].Title)
	}
	if favs[0].URI != "http://a.example/s?x=1&y=2" {
		t.Errorf("URI: %q", favs[0].URI)
	}
}
