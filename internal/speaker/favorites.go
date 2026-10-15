package speaker

import "context"

// FavoriteEntry beschreibt einen Sonos-Favoriten aus ContentDirectory FV:2.
//
// Integratable: als Stream-Szene importierbar (http(s)-URI).
// Playable: als Favoriten-Szene (type favorite) abspielbar — Streams und
// Radio-Favoriten eines Sonos-Dienstes wie TuneIn (x-sonosapi-stream mit
// r:resMD, ADR 0018). Spotify-Playlists, Amazon/Sonos-Radio und Ordner nicht.
type FavoriteEntry struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	URI          string `json:"uri,omitempty"`
	AlbumArt     string `json:"albumArt,omitempty"`
	Integratable bool   `json:"integratable"`
	Playable     bool   `json:"playable"`
	// ResMD sind die DIDL-Metadaten, die Sonos zum Abspielen der URI braucht
	// (r:resMD des Favoriten); nur intern, nicht in der API.
	ResMD string `json:"-"`
}

// FavoritesReader liest Sonos-Favoriten (nur Sonos-Treiber).
type FavoritesReader interface {
	BrowseFavorites(ctx context.Context, proxySpeaker string) ([]FavoriteEntry, error)
}
