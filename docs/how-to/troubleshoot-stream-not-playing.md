# How-to: Stream does not start ("Speaker does not start playback")

Problem: Starting a scene returns `playback_failed` with the message **"Speaker does not start playback"** or **"Stream unreachable"**. The GUI shows "Error" and the hint below it.

Sonos acknowledges `SetAVTransportURI` and `Play` even for streams it cannot play. A few seconds later the speaker goes back to `STOPPED`. After Play, the Sonos driver therefore checks the coordinator's transport state for up to 6 s and reports the failure ([#17](https://github.com/ohm-meter/hallido/issues/17)).

## What the driver does before playing

1. **Stream probe:** a short `GET` on the stream URL (follows redirects, max. 3 s, aborted after the headers). The result is cached for 10 min.
2. **URI scheme by format:**

   | Detected | Sent to Sonos | Note |
   |----------|---------------|------|
   | MP3 (`audio/mpeg`) | `x-rincon-mp3radio://host/path` | as before |
   | AAC/AAC+ (`audio/aac`, `audio/aacp`, ADTS) | `aac://https://host/path` | Original URL, redirect is not resolved (token URLs expire) |
   | HLS (`.m3u8`) | `x-rincon-mp3radio://…` | usually does not play → "HLS" hint |
   | Unknown / probe got no response | `x-rincon-mp3radio://…` | previous behavior |
   | HTTP 4xx/5xx | — | Immediately `Stream unreachable`; the speaker is not touched |

   URIs that are already wrapped (`x-rincon-mp3radio://…`, `aac://…`) stay unchanged.
3. **Check after Play:** `PLAYING` → ok. `STOPPED` (after `TRANSITIONING`, or after 1.5 s) or still `TRANSITIONING` after 6 s → `playback_failed`. If the state cannot be read, Play counts as successful, as before.

## Hint texts

| Hint contains | Meaning | Fix |
|---------------|---------|-----|
| `Stream responds with HTTP 404` | URL dead | Find a new URL (radio search in the GUI) |
| `detected: audio/aac, speaker state: STOPPED` | The speaker discarded the stream | Check the stream in a browser/VLC, choose a different endpoint |
| `Block on the Sonos user agent` | The station serves nothing to Sonos devices (example: Superfly) | Save the station as a **favorite** (TuneIn) in the Sonos app and import it as a favorite scene ([Sonos favorites](sonos-favorites.md)); a direct stream does not work |
| `HLS playlists (m3u8)` | Playlist instead of an audio stream | Use the station's MP3/AAC endpoint |
| `Speaker still buffering after 6 s` | Very slow stream | Try again later; if it does start, the GUI shows "playing" on the next status poll |

## Check it yourself

```bash
# Content type and redirect
curl -sIL -m 5 https://web.stream.superfly.fm/superfly-beach | grep -iE '^(HTTP|location|content-type)'
```

```bash
# Does the station also answer Sonos? (hangs if blocked, aborts after 5 s)
curl -sL -m 5 -o /dev/null -w '%{http_code}\n' -A 'Linux UPnP/1.0 Sonos/86.10-80260 (ZPS1)' https://web.stream.superfly.fm/superfly-beach
```

Log on the Pi: `journalctl -u sonos-agent | grep 'playback does not start'` shows the URI, speaker state, content type and detected format.

## See also

- [UPnP error 714](troubleshoot-upnp-714.md) — the speaker already rejects the URI when it is set
- [Radio Browser search](radio-browser-search.md)
