package sonos

import (
	"net/url"
	"path"
	"strings"
)

// tuneInServiceUDN kennzeichnet Internetradio in Sonos-DIDL (wie SoCo/TuneIn).
const tuneInServiceUDN = "SA_RINCON65031_"

// prepareStreamPlayback wandelt eine HTTP(S)-Stream-URL in das von Sonos
// erwartete Radio-Format um und liefert passende DIDL-Lite-Metadaten.
// Leere Metadaten oder rohe https-URIs ohne DIDL fuehren auf echter Hardware
// zu UPnP-Fehler 714 (Illegal MIME-Type).
//
// kind stammt aus der Stream-Probe: AAC wird als aac://<URL> uebergeben (so
// meldet Sonos AAC-Radio selbst als TrackURI; Hardware-Test 2026-10-15),
// alles andere (MP3, HLS, unbekannt) wie bisher als x-rincon-mp3radio://.
func prepareStreamPlayback(rawURI string, kind streamKind) (transportURI, metaData string) {
	transportURI = rawURI
	if isHTTPURL(rawURI) {
		if kind == kindAAC {
			transportURI = "aac://" + rawURI
		} else {
			transportURI = "x-rincon-mp3radio" + rawURI[strings.Index(rawURI, "://"):]
		}
	}
	metaData = radioStreamMetaData(streamTitle(rawURI))
	return transportURI, metaData
}

// isHTTPURL: rohe http(s)-URL (keine Sonos-Huelle wie x-rincon-mp3radio://).
func isHTTPURL(raw string) bool {
	i := strings.Index(raw, "://")
	if i <= 0 {
		return false
	}
	switch strings.ToLower(raw[:i]) {
	case "http", "https":
		return true
	}
	return false
}

// streamTitle leitet einen Anzeigenamen aus der Stream-URL ab.
func streamTitle(rawURI string) string {
	u, err := url.Parse(rawURI)
	if err != nil || u.Host == "" {
		return "Radio"
	}
	if base := path.Base(u.Path); base != "" && base != "." && base != "/" {
		return base
	}
	return u.Host
}

// radioStreamMetaData erzeugt minimale DIDL-Lite-Metadaten fuer
// object.item.audioItem.audioBroadcast (Internetradio).
func radioStreamMetaData(title string) string {
	return `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:r="urn:schemas-rinconnetworks-com:metadata-1-0/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"><item id="R:0/0/0" parentID="R:0/0" restricted="true"><dc:title>` +
		escapeXML(title) +
		`</dc:title><upnp:class>object.item.audioItem.audioBroadcast</upnp:class><desc id="cdudn" nameSpace="urn:schemas-rinconnetworks-com:metadata-1-0/">` +
		tuneInServiceUDN +
		`</desc></item></DIDL-Lite>`
}
