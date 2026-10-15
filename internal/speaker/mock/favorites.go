package mock

import (
	"context"

	"sonos-radio-agent/internal/speaker"
)

// mockFavorites sind Testdaten fuer BrowseFavorites (Mock-Treiber).
var mockFavorites = []speaker.FavoriteEntry{
	{
		ID:           "FV:2/1",
		Title:        "Test Radio",
		URI:          "https://stream.example/live",
		Integratable: true,
		Playable:     true,
	},
	{
		ID:           "FV:2/2",
		Title:        "Spotify Track",
		URI:          "x-sonos-spotify:spotify:track:abc",
		Integratable: false,
	},
	{
		ID:       "FV:2/3",
		Title:    "Radio Superfly",
		URI:      "x-sonosapi-stream:tunein%3a17315?sid=303&flags=8232&sn=9",
		Playable: true,
		ResMD:    `<DIDL-Lite><item id="-1"><dc:title>Radio Superfly</dc:title></item></DIDL-Lite>`,
	},
}

// mockPreRollURI ahmt den TuneIn-Pre-Roll nach, den Sonos vor dem Sender als
// Track meldet (Hardware 2026-10-15).
const mockPreRollURI = "x-rincon-mp3radio://https://tunein-ondemand.example/multi_bump_sonic_pre_pre.mp3"

// BrowseFavorites liefert statische Test-Favoriten (ohne SOAP).
func (c *Controller) BrowseFavorites(_ context.Context, proxySpeaker string) ([]speaker.FavoriteEntry, error) {
	if err := c.checkReachable(proxySpeaker); err != nil {
		return nil, err
	}
	out := make([]speaker.FavoriteEntry, len(mockFavorites))
	copy(out, mockFavorites)
	return out, nil
}

func (c *Controller) checkReachable(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.reachable[name] {
		return c.unreachable(name)
	}
	return nil
}
