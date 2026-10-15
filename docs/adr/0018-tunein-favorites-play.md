# ADR 0018: Play TuneIn stations as Sonos favorites (`type: favorite`)

## Status

Accepted (2026-10-15). Extends ADR 0005 (favorite playback was deferred until hardware validation).

## Context

- Some stations block Sonos when streamed directly (issue #17): Superfly (streaMonkey) does not answer requests with "sonos" in the user agent after the redirect. No URI scheme helps (finding 9 in `docs/how-to/hardware-validierung.md`).
- In the Sonos app the same stations play as a **TuneIn favorite**. The agent already reads favorites (`GET /api/v1/sonos/favorites`, ContentDirectory `FV:2`). Playing was only possible for favorites with an http(s) URI (`integratable`), and the config API only accepted `type: stream` (issue #6).
- A TuneIn favorite consists of `res` = `x-sonosapi-stream:tunein%3a17315?sid=303&flags=8232&sn=9` and `r:resMD` = DIDL-Lite with the service identifier (`desc` `SA_RINCON77575_X_#Svc77575-…-Token`). The URI alone is not enough; Sonos needs this metadata.

### Hardware test (2026-10-15, one speaker, volume 0)

| Favorite | URI + metadata | Result |
|----------|----------------|--------|
| ORF Hitradio OE3 (TuneIn, sid 303) | `res` + `r:resMD` | `PLAYING` after 3 s |
| Radio Superfly (TuneIn) | `res` + `r:resMD` | 4 of 5 starts `PLAYING` after 3–4 s, first the TuneIn pre-roll (`multi_bump_sonic_pre_pre.mp3`); in 2 runs, after 40–50 s the track `…superfly.fm/superfly-live?aggregator=tunein`, title "Superfly Live" (the others ended earlier). The first start stayed `STOPPED` without a response |
| Superfly Club (TuneIn) | `res` + `r:resMD` | `Play` responded only after > 10 s, then `PLAYING` |
| Entspannter Jazz (Amazon, `x-sonosapi-radio`, sid 201) | `res` + `r:resMD` | `Play` → UPnP **501**, `STOPPED` |

## Decision

- **No new source type.** `type: favorite` (already in the model, mock, and driver) is enabled for TuneIn stations. A dedicated source `type: tunein` with a station ID would have had to rebuild the service identifier in `desc` itself (account token, `sid` varies between 254/303/333); the favorite supplies it ready-made and stays maintained in the Sonos app.
- **The scene stores** `name` (title as in the Sonos app, required, ≤ 200 characters) and optionally `uri` (the favorite's `x-sonosapi-stream` URI, for status matching). The config API and `from-favorite` accept both; `from-favorite` stores TuneIn favorites this way and stream favorites still as `type: stream`.
- **Playing:** fetch favorites via Browse, find by title (case-insensitive). http(s) favorites go through the stream path as before (probe, check after Play). TuneIn: `SetAVTransportURI` with `res` and `r:resMD` unchanged, `Play`, then wait up to **10 s** for `PLAYING` (TuneIn needs 3–4 s); if the speaker stays stopped, set and start **once** more (the stalled start in the test), otherwise `playback_failed` "Sonos favorite does not start" with the speaker state. No stream probe: the speaker fetches the stream via TuneIn.
- **Playable** (`playable` in `GET /sonos/favorites`) are favorites with an http(s) URI and `x-sonosapi-stream` **with** `r:resMD`. Not playable: `x-sonosapi-radio` (Amazon/Sonos Radio, the speaker rejects with 501), `x-rincon-cpcontainer` (Spotify playlists, would need the queue), folders.
- **Status matching:** While a TuneIn favorite plays, the speaker first reports the pre-roll as the track, then the stream; the loaded source (`GetMediaInfo`) stays the favorite URI. `NowPlayingInfo` therefore gains `source_uri`/`source_title`. A favorite scene with `uri` counts as running when the station identifier (`tunein:17315`, without `sid`/`flags`) matches; without `uri` the title counts. If the track title is derived only from its URL, now-playing shows the station name.

## Consequences

- Superfly and other stations that Sonos blocks directly work through a TuneIn favorite, including the TuneIn ad before the station (pre-roll, 40–50 s in the test).
- Renaming or deleting a favorite in the Sonos app breaks the scene: Play returns `playback_failed` "Favorite not found" with a hint.
- Every Play costs one Browse of `FV:2` (≈ 0.1–0.3 s); a cache can be added if needed.
- Spotify, Amazon, and Sonos Radio favorites remain app-only (issue #6, remainder).
