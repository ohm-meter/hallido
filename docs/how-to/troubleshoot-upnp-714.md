# How-to: UPnP error 714 on play

Problem: Starting a scene fails with `playback_failed`, and the message contains **UPnP error code 714**.

## Meaning

**714 = Illegal MIME type** — Sonos rejects `SetAVTransportURI` because the URI and/or `CurrentURIMetaData` (DIDL-Lite) do not match the stream type.

Typical for internet radio via `type: stream` with a raw `https://` URL and empty metadata.

## Since the Sonos driver fix (Unreleased)

The driver converts HTTP(S) streams automatically:

- **URI:** `https://…` → `x-rincon-mp3radio://…` (MP3/unknown) or `aac://https://…` (AAC, detected by a stream probe, [#17](https://github.com/ohm-meter/hallido/issues/17))
- **Metadata:** minimal DIDL with `object.item.audioItem.audioBroadcast`

Without DIDL, Sonos rejects raw `http(s)://` URIs with 714 (hardware test 2026-10-15). If the speaker accepts the URI but does not play, see [Stream does not start](troubleshoot-stream-not-playing.md).

No config change is needed as long as the stream URL itself is reachable.

## Retest locally

```bash
./scripts/dev.sh
# GUI: play a scene (e.g. FM4)
# or:
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/scenes/fm4-living-room/play
```

The server must run with `--driver=sonos` (the dev script sets this via `.env`).

## If 714 persists

| Cause | Fix |
|-------|-----|
| Stream URL dead / redirect problems | Test the URL in a browser/VLC; try an alternative ORF URL |
| AAC instead of MP3 | Automatically `aac://` since #17; Sonos also detects AAC under `x-rincon-mp3radio` on its own (DLF test) |
| HTTPS/certificate on the speaker | Test the `http://` variant of the station (LAN only) |
| Station blocks the Sonos user agent | Not a 714; the speaker stays `STOPPED` — see [Stream does not start](troubleshoot-stream-not-playing.md) |

### Workaround: set the URI format manually

Set the Sonos radio prefix directly in `config.json` (without `https://`):

```json
"source": {
  "type": "stream",
  "uri": "x-rincon-mp3radio://orf-live.ors-shoutcast.at/fm4-q2a"
}
```

The driver still adds DIDL metadata; the URI is not converted twice.

## Related errors

- **701 / unreachable:** Speaker offline — see [Troubleshoot discovery](troubleshoot-discovery.md)
- **Favorites:** `type: favorite` is not active yet (ADR 0005) — use a stream URL

## See also

- [Stream does not start](troubleshoot-stream-not-playing.md)
- [Create a scene](create-scene.md)
- [Use the Sonos driver](use-sonos-driver.md)
