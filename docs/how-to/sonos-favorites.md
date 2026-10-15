# Read, import, and play Sonos favorites

Prerequisite: The agent runs with `--driver=sonos` and the speakers are reachable on the LAN.

## API

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/v1/sonos/favorites` | Favorites from the Sonos app (`FV:2`) |
| `POST` | `/api/v1/config/scenes/from-favorite` | Scene from a favorite (stream or TuneIn) |

Optional: `?speaker=Living%20Room` — otherwise chosen automatically (non-TV speakers first, next speaker on failure).

**Recommended speaker:** Any regular zone (Play:1, One, Beam without TV mode) — **not** the TV speaker: Sonos TV devices often return HTTP 500 for ContentDirectory `Browse`.

`GET` response:

```json
{
  "favorites": [
    { "id": "FV:2/3", "title": "FM4", "uri": "https://…", "integratable": true, "playable": true },
    { "id": "FV:2/78", "title": "Radio Superfly", "uri": "x-sonosapi-stream:tunein%3a17315?sid=303&flags=8232&sn=9", "integratable": false, "playable": true }
  ],
  "speaker": "Living Room",
  "discovery_error": ""
}
```

- `integratable: true` — stream with an http(s) URI, saved as a **stream scene**.
- `playable: true`, `integratable: false` — **TuneIn station** (`x-sonosapi-stream`), saved as a **favorite scene** and played via TuneIn ([ADR 0018](../adr/0018-tunein-favorites-play.md)).
- `playable: false` — Spotify playlists, Amazon/Sonos Radio, folders: Sonos app only.

## GUI

1. Open **New scene**
2. Choose the source **From Sonos favorites**
3. Optionally set **Favorites from speaker** — the default "Automatic" skips TV speakers; on failure the API tries the next speaker
4. Pick a favorite from the list (streams and TuneIn stations; TuneIn entries carry the suffix "(Sonos favorite)")
5. Speakers + name/ID, **Save**

Streams are stored in `config.json` as `type: stream`, TuneIn stations as `type: favorite`:

```json
"source": { "type": "favorite", "name": "Radio Superfly", "uri": "x-sonosapi-stream:tunein%3a17315?sid=303&flags=8232&sn=9" }
```

When you **edit** a favorite scene, the source is kept as long as the URL field stays empty; entering a URL turns it into a stream scene.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `Device rejected Browse (HTTP 500)` on a TV speaker | TV zones often do not support ContentDirectory browse | In the GUI choose "Automatic" or another speaker (for example Living Room) |
| `unexpected device response` | SOAP fault without parseable details | Restart the speaker; use another zone as a proxy; the error text may contain a UPnP code or a response snippet |
| Empty list, `discovery_error` set | No speaker could read FV:2 | All speakers reachable? `--driver=sonos`? At least one non-TV zone online? |
| Favorite missing in the GUI | `playable: false` (Spotify, Amazon, Sonos Radio) | Only internet radio and TuneIn stations are playable |
| `Favorite cannot be played by the agent (e.g. Spotify, Amazon, Sonos Radio)` | The favorite is app-only | Play it in the Sonos app, or use a stream or TuneIn favorite |
| Scene reports "Favorite not found: …" | Favorite renamed or deleted in the Sonos app | Adjust the title in the scene (`source.name`) or create the favorite again |
| `Sonos favorite does not start: <title>` | The speaker stays stopped after two attempts | Title exactly as in the Sonos app? Favorite renamed or deleted? List: `GET /api/v1/sonos/favorites`. Check in the Sonos app that the station plays there (TuneIn signed in?) |
| Scene is playing, title shows the station name instead of a song | TuneIn plays an ad first (pre-roll, 40–50 s) | Normal; the song or stream title appears afterwards |

## Playing `type: favorite`

Scenes with `"type": "favorite"` and a `name` as in the Sonos app (case-insensitive): the driver reads the favorites and

- plays **streams** (http(s)) like a stream scene (probe, check after Play),
- plays **TuneIn stations** with the favorite's URI and metadata (`r:resMD`), waits up to 10 s for `PLAYING`, and retries once if the speaker stays stopped.

Useful for stations that Sonos blocks as a direct stream (for example Superfly, see [Stream does not start](troubleshoot-stream-not-playing.md)): save the station as a favorite in the Sonos app, then import it here.

## Hardware checklist

See [plan](../plans/sonos-favorites-import.md) §4: `TotalMatches` vs. the app, DIDL variants, one manual play test.
