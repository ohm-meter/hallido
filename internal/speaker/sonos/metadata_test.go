package sonos

import "testing"

func TestParseTrackMetadata_DIDL(t *testing.T) {
	meta := `&lt;DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"&gt;&lt;item id="t1" parentID="p0" restricted="true"&gt;&lt;dc:title&gt;FM4&lt;/dc:title&gt;&lt;dc:creator&gt;ORF&lt;/dc:creator&gt;&lt;upnp:album&gt;Radio&lt;/upnp:album&gt;&lt;/item&gt;&lt;/DIDL-Lite&gt;`
	title, artist, album, raw := parseTrackMetadata(meta)
	if raw != "" {
		t.Fatalf("raw should be empty, got %q", raw)
	}
	if title != "FM4" || artist != "ORF" || album != "Radio" {
		t.Fatalf("metadata = %q / %q / %q", title, artist, album)
	}
}

func TestParseTrackMetadata_RegexFallback(t *testing.T) {
	meta := `<item><dc:title>Track A</dc:title><dc:creator>Artist B</dc:creator></item>`
	title, artist, album, raw := parseTrackMetadata(meta)
	if raw != "" {
		t.Fatalf("raw should be empty, got %q", raw)
	}
	if title != "Track A" || artist != "Artist B" || album != "" {
		t.Fatalf("metadata = %q / %q / %q", title, artist, album)
	}
}

func TestParseTrackMetadata_Unparseable(t *testing.T) {
	meta := "not-xml-metadata"
	title, artist, album, raw := parseTrackMetadata(meta)
	if title != "" || artist != "" || album != "" {
		t.Fatalf("expected empty fields, got %q/%q/%q", title, artist, album)
	}
	if raw != meta {
		t.Fatalf("raw = %q, want original", raw)
	}
}
