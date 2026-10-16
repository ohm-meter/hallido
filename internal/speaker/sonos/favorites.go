package sonos

import (
	"context"
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

// favoritesObjectID ist der Sonos-Container fuer in der App gespeicherte Favoriten.
const favoritesObjectID = "FV:2"

const favoritesPageSize = 100

var (
	reAlbumArtURI = regexp.MustCompile(`(?i)<upnp:albumArtURI[^>]*>([^<]*)</upnp:albumArtURI>`)
	reItemBlock   = regexp.MustCompile(`(?is)<item\b[^>]*\bid="(FV:[^"]+)"[^>]*>(.*?)</item>`)
	reResValue    = regexp.MustCompile(`(?is)<res[^>]*>([^<]*)</res>`)
	reUpnpClass   = regexp.MustCompile(`(?i)<upnp:class[^>]*>([^<]*)</upnp:class>`)
	reResMD       = regexp.MustCompile(`(?is)<r:resMD[^>]*>([^<]*)</r:resMD>`)
)

type browseResponse struct {
	Result         string `xml:"Body>BrowseResponse>Result"`
	NumberReturned int    `xml:"Body>BrowseResponse>NumberReturned"`
	TotalMatches   int    `xml:"Body>BrowseResponse>TotalMatches"`
}

// BrowseFavorites liest Sonos-Favoriten per ContentDirectory Browse (FV:2).
// proxySpeaker: beliebige erreichbare Zone. Paginiert bei mehr als favoritesPageSize Eintraegen.
func (c *Controller) BrowseFavorites(ctx context.Context, proxySpeaker string) ([]speaker.FavoriteEntry, error) {
	d, err := c.resolve(ctx, proxySpeaker)
	if err != nil {
		return nil, err
	}

	var all []speaker.FavoriteEntry
	start := 0
	for {
		body, err := c.soapCall(ctx, d, contentDirectory, "Browse", []arg{
			{"ObjectID", favoritesObjectID},
			{"BrowseFlag", "BrowseDirectChildren"},
			{"Filter", "*"},
			{"StartingIndex", strconv.Itoa(start)},
			{"RequestedCount", strconv.Itoa(favoritesPageSize)},
			{"SortCriteria", ""},
		})
		if err != nil {
			return nil, err
		}

		br, err := parseBrowseResponse(body)
		if err != nil {
			return nil, apperr.New(apperr.CodePlaybackFailed, "ContentDirectory response unreadable: "+err.Error()).
				WithSpeaker(proxySpeaker).
				WithHint("Favorites browse failed — speaker reachable?")
		}

		page, err := parseFavoritesDIDL(br.Result)
		if err != nil {
			return nil, apperr.New(apperr.CodePlaybackFailed, "Favorites DIDL unreadable: "+err.Error()).
				WithSpeaker(proxySpeaker)
		}
		all = append(all, page...)

		returned := br.NumberReturned
		if returned <= 0 {
			returned = len(page)
		}
		start += returned
		if returned == 0 || start >= br.TotalMatches || br.TotalMatches == 0 {
			break
		}
	}
	return all, nil
}

// browseFavoritesPrefer versucht Browse auf preferred und bei Fehler auf anderen Boxen.
func (c *Controller) browseFavoritesPrefer(ctx context.Context, preferred string) ([]speaker.FavoriteEntry, error) {
	var lastErr error
	for _, name := range c.favoriteProxyOrder(preferred) {
		favs, err := c.BrowseFavorites(ctx, name)
		if err == nil {
			return favs, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (c *Controller) favoriteProxyOrder(preferred string) []string {
	preferred = strings.TrimSpace(preferred)
	c.mu.RLock()
	defer c.mu.RUnlock()

	seen := map[string]bool{}
	var regular, tvLike []string
	for name := range c.devices {
		if seen[name] {
			continue
		}
		seen[name] = true
		if preferred != "" && name == preferred {
			continue
		}
		if speaker.IsTVLikeZone(name) {
			tvLike = append(tvLike, name)
		} else {
			regular = append(regular, name)
		}
	}
	sort.Strings(regular)
	sort.Strings(tvLike)

	out := make([]string, 0, len(regular)+len(tvLike)+1)
	if preferred != "" {
		if _, ok := c.devices[preferred]; ok {
			out = append(out, preferred)
		}
	}
	out = append(out, regular...)
	out = append(out, tvLike...)
	if len(out) == 0 && preferred != "" {
		return []string{preferred}
	}
	return out
}

func parseBrowseResponse(soapBody []byte) (browseResponse, error) {
	var br browseResponse
	if err := xml.Unmarshal(soapBody, &br); err != nil {
		return browseResponse{}, err
	}
	// xml.Unmarshal hat das SOAP-Escaping bereits aufgeloest: Result ist das
	// DIDL-XML selbst. Nicht erneut unescapen (Werte wie &amp; bleiben sonst nicht
	// wohlgeformt bzw. wuerden doppelt dekodiert); Text-Felder werden einzeln
	// in firstRegexGroup dekodiert.
	br.Result = strings.TrimSpace(br.Result)
	return br, nil
}

func parseFavoritesDIDL(didl string) ([]speaker.FavoriteEntry, error) {
	didl = strings.TrimSpace(didl)
	if didl == "" {
		return nil, nil
	}
	raw := parseFavoritesDIDLRegex(didl)
	out := make([]speaker.FavoriteEntry, 0, len(raw))
	for _, f := range raw {
		integratable := speaker.IsIntegratableStreamURI(f.URI)
		service := speaker.IsSonosServiceStreamURI(f.URI) && f.ResMD != ""
		out = append(out, speaker.FavoriteEntry{
			ID:           f.ID,
			Title:        f.Title,
			URI:          f.URI,
			AlbumArt:     f.AlbumArt,
			Integratable: integratable,
			Playable:     integratable || service,
			ResMD:        f.ResMD,
		})
	}
	return out, nil
}

type favoriteRaw struct {
	ID, Title, URI, AlbumArt, ResMD string
}

func parseFavoritesDIDLRegex(didl string) []favoriteRaw {
	seen := map[string]bool{}
	var out []favoriteRaw
	for _, m := range reItemBlock.FindAllStringSubmatch(didl, -1) {
		if len(m) < 3 || seen[m[1]] {
			continue
		}
		block := m[2]
		if isFavoriteContainer(block) {
			continue
		}
		title := firstRegexGroup(reDCTitle, block)
		if title == "" {
			continue
		}
		seen[m[1]] = true
		out = append(out, favoriteRaw{
			ID:       m[1],
			Title:    title,
			URI:      strings.TrimSpace(firstRegexGroup(reResValue, block)),
			AlbumArt: firstRegexGroup(reAlbumArtURI, block),
			// r:resMD ist im DIDL escaptes DIDL; firstRegexGroup dekodiert einmal.
			ResMD: firstRegexGroup(reResMD, block),
		})
	}
	return out
}

func isFavoriteContainer(block string) bool {
	cls := strings.ToLower(firstRegexGroup(reUpnpClass, block))
	if strings.Contains(cls, "object.container") {
		return true
	}
	return strings.Contains(strings.ToLower(block), "<container")
}

// playFavorite startet einen Favoriten per Titel: Streams (http(s)) wie
// eine Stream-Szene, Radio-Favoriten eines Sonos-Dienstes (TuneIn) mit den
// Metadaten des Favoriten (ADR 0018).
func (c *Controller) playFavorite(ctx context.Context, coord *device, favoriteName string) error {
	favoriteName = strings.TrimSpace(favoriteName)
	if favoriteName == "" {
		return apperr.New(apperr.CodeBadRequest, "Favorite name missing").WithSpeaker(coord.name)
	}

	favs, err := c.browseFavoritesPrefer(ctx, coord.name)
	if err != nil {
		return err
	}
	var match *speaker.FavoriteEntry
	for i := range favs {
		if strings.EqualFold(strings.TrimSpace(favs[i].Title), favoriteName) {
			match = &favs[i]
			break
		}
	}
	if match == nil {
		return apperr.New(apperr.CodePlaybackFailed, "Favorite not found: "+favoriteName).
			WithSpeaker(coord.name).
			WithHint("Title exactly as in the Sonos app? Favorite renamed or deleted? List: GET /api/v1/sonos/favorites.")
	}
	if match.URI == "" {
		return apperr.New(apperr.CodePlaybackFailed, "Favorite without playable URI").
			WithSpeaker(coord.name).
			WithHint("Folders and Sonos collections can only be played in the Sonos app.")
	}
	if match.Integratable {
		return c.setURIAndPlay(ctx, coord, match.URI)
	}
	if !match.Playable {
		return apperr.New(apperr.CodePlaybackFailed, "Favorite cannot be played by the agent (e.g. Spotify, Amazon, Sonos Radio)").
			WithSpeaker(coord.name).
			WithHint("Playable are internet radio streams and TuneIn radio favorites (x-sonosapi-stream).")
	}
	return c.playServiceFavorite(ctx, coord, *match)
}

// playServiceFavorite spielt einen Radio-Favoriten eines Sonos-Dienstes
// (TuneIn): Favoriten-URI plus r:resMD an SetAVTransportURI — ohne Stream-
// Probe, denn den Stream holt die Box ueber den Dienst. Startet die Box nicht,
// wird einmal neu gesetzt (Hardware-Test 2026-10-15: der erste Start eines
// TuneIn-Senders blieb einmal ohne Reaktion STOPPED, die Wiederholung spielte).
func (c *Controller) playServiceFavorite(ctx context.Context, coord *device, fav speaker.FavoriteEntry) error {
	var state string
	for attempt := 1; attempt <= 2; attempt++ {
		if err := c.soapRetry(ctx, coord, avTransport, "SetAVTransportURI", []arg{
			{"InstanceID", "0"},
			{"CurrentURI", fav.URI},
			{"CurrentURIMetaData", fav.ResMD},
		}); err != nil {
			return err
		}
		if err := c.startPlay(ctx, coord); err != nil {
			return err
		}
		var started bool
		state, started = c.waitPlaying(ctx, coord, c.favoriteVerifyWindow)
		if started {
			return nil
		}
		if ctx.Err() != nil {
			break
		}
		c.log.Warn("favorite does not start", "speaker", coord.name, "favorite", fav.Title,
			"uri", fav.URI, "state", state, "attempt", attempt)
	}
	return apperr.New(apperr.CodePlaybackFailed, "Sonos favorite does not start: "+fav.Title).
		WithSpeaker(coord.name).
		WithHint(fmt.Sprintf("Speaker state: %s. Does the favorite play in the Sonos app? "+
			"Is the service (e.g. TuneIn) still available there? Try again.", state))
}
