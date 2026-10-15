# Plan: Read Sonos favorites & import scenes

As of 2026-10-14 · in `master` · **Phases 2–3 implemented**; browse on real hardware ok (39 favorites, none integrable). Open: favorite playback via DIDL (Forgejo issue #6)

## Status

| Phase | Content | Status |
|-------|---------|--------|
| 0 | Spike `BrowseFavorites` | done |
| 1 | Hardware validation DIDL/play | **open** |
| 2 | `GET /api/v1/sonos/favorites` | done |
| 3 | GUI import + `POST …/from-favorite` | done |
| 4 | Play `type: favorite` (integrable URI) | partial (mock + SOAP mock; hardware open) |

## Research

**Yes** — ContentDirectory `Browse`, `ObjectID=FV:2`, port 1400.

| Field | Source |
|-------|--------|
| `id` | e.g. `FV:2/21` |
| `dc:title` | Display name |
| `<res>` | Play URI |
| `integratable` | `IsIntegratableStreamURI(uri)` |

## API

- `GET /api/v1/sonos/favorites` → `{ favorites, speaker, discovery_error? }`
- `POST /api/v1/config/scenes/from-favorite` → stream scene

## Phase 1 hardware (checklist)

1. Browse `FV:2` — `TotalMatches` vs. the Sonos app
2. DIDL: Spotify, Sonos Radio, empty `res`, containers
3. `type: favorite` play on a real speaker vs. stream import
4. Record the result in `.agent/HANDOVER.md`

## References

- [ADR 0005](../adr/0005-favorites-increment-3.md)
- [How-to](../how-to/sonos-favorites.md)
- Code: `internal/speaker/sonos/favorites.go`, `internal/api/favorites_handlers.go`
