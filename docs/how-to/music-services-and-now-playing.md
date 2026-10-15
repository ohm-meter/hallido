# Music services (Spotify, Sonos Radio) and Now Playing

## Short answer

| Question | Answer |
|----------|--------|
| Use Spotify/Sonos Radio accounts from the Sonos app and **start them from here**? | **No** — no Sonos cloud API access and no Spotify OAuth in this project. |
| **Read** what is currently playing on a speaker (including Spotify/Sonos Radio)? | **Yes** — via local UPnP/SOAP (`GetMediaInfo`, `GetPositionInfo`, `GetTransportInfo`), as long as the Sonos app or the speaker is already playing. |
| Start streams/radio by URL? | **Yes** — scenes with `"type": "stream"` (for example ORF/FM4). |

## Why no Spotify/Sonos Radio start?

Hallido controls speakers **on the LAN only** via UPnP/SOAP. That needs:

- no Sonos cloud sign-in,
- no Spotify developer account,
- no OAuth flows in the GUI.

Spotify and Sonos Radio in the official app run through **account-bound services**. This project **cannot sign in to them or start them selectively** — it can only query what the speaker is **already** playing, or set your own stream URLs.

**Practical alternative:** Configure radio/streams as `type: stream` in `config.json` (see the how-to on creating a scene).

## Query now playing

### REST API (one speaker)

```http
GET /api/v1/speakers/{name}/now-playing
Authorization: Bearer <token>
```

### REST API (all speakers, bulk)

```http
GET /api/v1/speakers/now-playing
Authorization: Bearer <token>
```

Response: `{ "speakers": [ …NowPlayingInfo ] }` — for GUI polling without N individual requests.

### Sonos Radio URIs

Sonos Radio and TuneIn often deliver **wrapped URIs**, for example:

```
aac://http://inselradiomallorca.stream06.radiohost.de/mallorcasunshineradio-live_aac-128?…
```

For **stream scenes**, configure the plain HTTPS URL without a query:

```json
"uri": "https://inselradiomallorca.stream06.radiohost.de/mallorcasunshineradio-live_aac-128"
```

When comparing status, the agent normalizes host + path (without the query, regardless of `aac://` or `x-rincon-mp3radio://`).

Response (single-speaker example):

```json
{
  "speaker": "Living Room",
  "state": "playing",
  "title": "Title",
  "artist": "Artist",
  "album": "Album",
  "uri": "x-sonos-spotify:spotify:track:…"
}
```

- `state`: `playing` | `stopped` | `paused` | `unknown`
- `title`, `artist`, `album`, `uri`: best effort from UPnP metadata
- `raw`: only set if the metadata could not be parsed (raw string)

**URL-encode** the speaker name (spaces, colons): `Basement%3A%20TV`

Error: speaker offline → `502` with `speaker_unreachable`.

### Shell script

Prerequisites: a running agent, `curl`, `jq`.

```bash
export SRA_TOKEN="your-token-from-config"
./scripts/sonos-now-playing.sh "Living Room"
```

Optionally a different base URL:

```bash
SRA_BASE_URL=http://192.168.1.50:8080 SRA_TOKEN=… ./scripts/sonos-now-playing.sh Kitchen
```

### GUI — discover mode

1. At the bottom, in the **Found** speakers panel, click a speaker.
2. The now-playing panel appears above it (state, title, artist, album, URI).
3. **Close** ends the mode.

**Without discover:** Each speaker card shows one now-playing line (title or state), refreshed every 8 s.

### Scene badge

| state | Badge |
|-------|-------|
| `playing` | playing — the coordinator is playing the scene's source |
| `playing_other` | other content — playing, but the URI/title does not match the scene |
| `stopped` / `paused` / `offline` | as before |

Refreshed every 8 s (while the tab is visible).

## Limits

- Metadata depends on the source — some streams deliver only a URI without a title.
- No cover artwork, no playlist position, no scrubbing.
- The query is read-only — it starts/stops nothing.
- Hardware validation against real Spotify playback is recommended (the mock tests SOAP parsing).

## See also

- [`create-scene.md`](create-scene.md) — stream scenes
- [`use-sonos-driver.md`](use-sonos-driver.md) — driver `--driver=sonos`
- OpenAPI: `GET /api/v1/speakers/{name}/now-playing`
