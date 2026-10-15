package sonos

import (
	"context"
	"encoding/xml"
	"html"
	"net/url"
	"strings"

	"sonos-radio-agent/internal/speaker"
)

// GetNowPlaying fragt Transportzustand und Metadaten einer Box ab.
func (c *Controller) GetNowPlaying(ctx context.Context, name string) (speaker.NowPlayingInfo, error) {
	d, err := c.resolve(ctx, name)
	if err != nil {
		return speaker.NowPlayingInfo{}, err
	}

	info, err := c.nowPlayingOf(ctx, d, name)
	if err != nil {
		return info, err
	}
	// Gruppenmitglied: Sonos meldet am Slave PLAYING mit x-rincon:<UID>, auch
	// wenn der Coordinator STOPPED ist. Dann gilt der Zustand des Coordinators.
	if coordUID, ok := strings.CutPrefix(info.URI, rinconURIPrefix); ok && coordUID != "" && coordUID != d.uid {
		if coord := c.deviceByUID(ctx, coordUID); coord != nil && coord != d {
			ci, cerr := c.nowPlayingOf(ctx, coord, name)
			if cerr == nil {
				return ci, nil
			}
			c.log.Debug("coordinator status not readable, using slave status",
				"speaker", name, "coordinator", coord.name, "err", cerr)
		}
	}
	return info, nil
}

// rinconURIPrefix kennzeichnet ein Gruppenmitglied (x-rincon:RINCON_...);
// Streams wie x-rincon-mp3radio: haben ein anderes Praefix.
const rinconURIPrefix = "x-rincon:"

// deviceByUID sucht im Cache; bei Fehlen einmal Discovery, dann Fallback nil.
func (c *Controller) deviceByUID(ctx context.Context, uid string) *device {
	find := func() *device {
		c.mu.RLock()
		defer c.mu.RUnlock()
		for _, d := range c.devices {
			if d.uid == uid {
				return d
			}
		}
		return nil
	}
	if d := find(); d != nil {
		return d
	}
	c.discMu.Lock()
	defer c.discMu.Unlock()
	if d := find(); d != nil {
		return d
	}
	_, _ = c.Discover(ctx)
	return find()
}

// nowPlayingOf liest Transportzustand und Metadaten genau dieses Geraets.
// speakerName landet im Ergebnis (bei Coordinator-Aufloesung der Slave-Name).
func (c *Controller) nowPlayingOf(ctx context.Context, d *device, speakerName string) (speaker.NowPlayingInfo, error) {
	name := speakerName
	info := speaker.NowPlayingInfo{
		Speaker: name,
		State:   speaker.PlaybackUnknown,
	}

	body, err := c.soapCall(ctx, d, avTransport, "GetTransportInfo", []arg{
		{"InstanceID", "0"},
	})
	if err != nil {
		if isSoftStatusSOAPError(err) {
			return info, nil
		}
		return info, err
	}
	info.State = parseTransportState(body)

	mediaBody, err := c.soapCall(ctx, d, avTransport, "GetMediaInfo", []arg{
		{"InstanceID", "0"},
	})
	if err != nil && !isSoftStatusSOAPError(err) {
		return info, err
	}
	currentURI, currentMeta := parseMediaInfo(mediaBody)

	posBody, err := c.soapCall(ctx, d, avTransport, "GetPositionInfo", []arg{
		{"InstanceID", "0"},
	})
	if err != nil && !isSoftStatusSOAPError(err) {
		return info, err
	}
	trackURI, trackMeta := parsePositionInfo(posBody)

	uri := strings.TrimSpace(trackURI)
	if uri == "" {
		uri = strings.TrimSpace(currentURI)
	}
	info.URI = uri
	if src := strings.TrimSpace(currentURI); src != "" && src != uri {
		info.SourceURI = src
		if t, _, _, _ := parseTrackMetadata(currentMeta); t != "" {
			info.SourceTitle = t
		}
	}

	meta := strings.TrimSpace(trackMeta)
	if meta == "" {
		meta = strings.TrimSpace(currentMeta)
	}
	if meta == "" {
		info.Title = info.SourceTitle
		return info, nil
	}

	title, artist, album, raw := parseTrackMetadata(meta)
	// Service favorites (TuneIn): Sonos names the track after its URL
	// (pre-roll "multi_bump_…mp3?…", then "superfly-live?…") — show the
	// station name of the source instead.
	if info.SourceTitle != "" && titleIsTrackURL(title, uri) {
		title = info.SourceTitle
	}
	info.Title = title
	info.Artist = artist
	info.Album = album
	info.Raw = raw
	return info, nil
}

// titleIsTrackURL reports whether title is only the file name of the track
// URI and not a real song or station title. Title and URI are often encoded
// differently (":443" only in the URI, "%2c" vs ",", "&amp;"), so besides a
// plain substring match the decoded last path segments before "?" are compared.
func titleIsTrackURL(title, uri string) bool {
	if title == "" || strings.Contains(uri, title) {
		return true
	}
	t := lastPathSegment(title)
	return t != "" && t == lastPathSegment(uri)
}

// lastPathSegment returns the HTML- and URL-decoded part between the last "/"
// and the first "?" of s.
func lastPathSegment(s string) string {
	s = html.UnescapeString(strings.TrimSpace(s))
	s, _, _ = strings.Cut(s, "?")
	s = s[strings.LastIndex(s, "/")+1:]
	if u, err := url.PathUnescape(s); err == nil {
		s = u
	}
	return strings.TrimSpace(s)
}

func parseMediaInfo(body []byte) (currentURI, currentMeta string) {
	var resp struct {
		URI  string `xml:"Body>GetMediaInfoResponse>CurrentURI"`
		Meta string `xml:"Body>GetMediaInfoResponse>CurrentURIMetaData"`
	}
	_ = xml.Unmarshal(body, &resp)
	return resp.URI, resp.Meta
}

func parsePositionInfo(body []byte) (trackURI, trackMeta string) {
	var resp struct {
		URI  string `xml:"Body>GetPositionInfoResponse>TrackURI"`
		Meta string `xml:"Body>GetPositionInfoResponse>TrackMetaData"`
	}
	_ = xml.Unmarshal(body, &resp)
	return resp.URI, resp.Meta
}
