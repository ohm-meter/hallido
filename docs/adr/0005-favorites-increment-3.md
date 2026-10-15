# ADR 0005: Favorites playback deferred to increment 3

Status: Accepted (partially) · Date: 2026-05-28 · Amended: 2026-05-28

**Plan for reading/import:** [`docs/plans/sonos-favorites-import.md`](../plans/sonos-favorites-import.md)  
**Spike:** `internal/speaker/sonos/favorites.go` — branch `feat/sonos-favorites-read`

## Context
A scene can have two source types: `stream` (direct URL) and `favorite`
(a favorite saved in the Sonos app). The data model, the mock driver, and the
GUI already support both. In the real Sonos driver (increment 2), only
`stream` is implemented.

Playing a favorite requires a ContentDirectory browse (container `FV:2`),
finding the favorite by title, and extracting the embedded source together
with its DIDL-Lite metadata, which is then passed to `SetAVTransportURI`. The
exact metadata format cannot be verified reliably without a real Sonos device;
a test that is green against a self-built mock would give false confidence
here.

## Decision
Increment 2 implements `stream` completely and tested. For source type
`favorite`, the Sonos driver returns a clear, structured error
(`playback_failed` with a hint) instead of silently doing nothing. Full
favorites playback becomes increment 3, once it can be tested against real
hardware.

## Rationale
A small, honest increment instead of untested DIDL logic with sham tests.
Nothing is lost: the architecture and the mock already support favorites; only
the hardware-dependent path waits for validation. A clear error is easier to
understand than a call that has no effect.

## Consequences
Until increment 3, radio stations must be configured as a stream URL
(`"type": "stream"`). The favorites path is marked in the code and covered by
a test as "not yet active".

### Addendum: *reading* favorites (2026-05-28)

ContentDirectory `Browse` / `FV:2` is productive for **reading** and **scene import** (`GET /api/v1/sonos/favorites`, GUI, `POST …/from-favorite`). Only URIs with `IsIntegratableStreamURI` are imported or played via `type: favorite`.

**Playing** Sonos Radio (`x-sonosapi-stream`), Spotify, and exact DIDL metadata remains restricted until hardware validation; error hints point to stream import.

### Addendum: favorite playback for TuneIn (2026-10-15)

Validated on real hardware and implemented: TuneIn favorites (`x-sonosapi-stream`) play with `res` + `r:resMD` — see [ADR 0018](0018-tunein-favorites-play.md). Spotify, Amazon, and Sonos Radio remain app-only.
