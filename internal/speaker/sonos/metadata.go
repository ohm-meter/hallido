package sonos

import (
	"encoding/xml"
	"html"
	"regexp"
	"strings"
)

var (
	reDCTitle   = regexp.MustCompile(`(?i)<dc:title[^>]*>([^<]*)</dc:title>`)
	reDCCreator = regexp.MustCompile(`(?i)<dc:creator[^>]*>([^<]*)</dc:creator>`)
	reUpnpAlbum = regexp.MustCompile(`(?i)<upnp:album[^>]*>([^<]*)</upnp:album>`)
)

// parseTrackMetadata extrahiert Titel/Kuenstler/Album aus Sonos-DIDL-Metadaten.
// Liefert den unveraenderten Input in raw, wenn nichts erkannt wurde.
func parseTrackMetadata(raw string) (title, artist, album string, leftover string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", ""
	}
	unescaped := html.UnescapeString(raw)

	type didlItem struct {
		Title   string `xml:"http://purl.org/dc/elements/1.1/ title"`
		Creator string `xml:"http://purl.org/dc/elements/1.1/ creator"`
		Album   string `xml:"http://schemas.upnp.org/metadata/1.0/upnp/ album"`
	}
	var doc struct {
		Item didlItem `xml:"item"`
	}
	if err := xml.Unmarshal([]byte(unescaped), &doc); err == nil {
		title = strings.TrimSpace(doc.Item.Title)
		artist = strings.TrimSpace(doc.Item.Creator)
		album = strings.TrimSpace(doc.Item.Album)
	}

	if title == "" {
		title = firstRegexGroup(reDCTitle, unescaped)
	}
	if artist == "" {
		artist = firstRegexGroup(reDCCreator, unescaped)
	}
	if album == "" {
		album = firstRegexGroup(reUpnpAlbum, unescaped)
	}

	if title == "" && artist == "" && album == "" {
		return "", "", "", raw
	}
	return title, artist, album, ""
}

func firstRegexGroup(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(m[1]))
}
