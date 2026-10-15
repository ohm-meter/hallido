# ADR 0015: Group control (pause/resume, controlling and ungrouping groups per speaker)

## Status

Accepted (2026-10-14)

## Context

- So far the agent only knew **scenes**: play and (since ADR 0013) stop on the scene's coordinator.
- In everyday use, music is often playing that was **not** started via a scene (Sonos app, Spotify Connect, TV). Homey or the GUI should still be able to stop/pause it — without knowing which speaker currently leads the group.
- Groups built by the Sonos app should also be possible to **ungroup** again.
- An AVTransport command sent to a group **member** (slave) is rejected by Sonos or has no effect on the group; the **coordinator** is always the one responsible.

## Decision

### Driver (`SpeakerController`)

| Method | Sonos (UPnP) | Note |
|--------|--------------|------|
| `Resume(coordinator)` | AVTransport `Play` (speed 1), **without** `SetAVTransportURI` | Without a loaded source, Sonos answers with 701 → `playback_failed` with a hint (unlike stop/pause, where 701 means "goal reached"). |
| `GetGroups()` | ZoneGroupTopology `GetZoneGroupState` | The topology is the same household-wide → first responding speaker in the cache, otherwise the next. Invisible members (sub, stereo slave), bridges/Boost, and home theater satellites are hidden. The coordinator comes first in `Members`. Speakers from the topology (`Location`) are added to the device cache. |
| `Ungroup(name)` | AVTransport `BecomeCoordinatorOfStandaloneGroup` | The speaker is on its own afterward (stopped). |

New helper function `speaker.GroupOf(groups, name)`.

### Service (`internal/scenes/groups.go`)

- **Scenes:** `Pause(id)` / `Resume(id)` on the scene's coordinator — symmetric to `Stop(id)`. Resume does **not** reset the source; to be sure of hearing the scene, use `play`.
- **Per speaker:** `ControlGroupOf(name, stop|pause|resume)` reads the topology, finds the speaker's group, and sends the command to its coordinator. It therefore works for any group, no matter who built it.
- **Ungrouping:** `Ungroup(name)`:
  - Member → only this speaker leaves the group; the rest keep playing.
  - Coordinator → the **whole group** is dissolved: each member leaves individually, and the coordinator keeps playing on its own. (A `BecomeCoordinatorOfStandaloneGroup` on the coordinator itself would only detach it and hand the group over to another member — not what "ungroup all" means.)
  - Standalone speaker → nothing to do (`standalone: true`).
  - **Partial failure** is not a hard error: result per speaker; an error only if no speaker could leave the group.
- **Overview:** `Groups()` = topology + now-playing of the coordinator + detected scene (`scene_id`, when the coordinator and source match a scene).

### API

| Endpoint | Purpose |
|----------|---------|
| `POST /api/v1/scenes/{id}/pause` · `/resume` | Pause / resume scene |
| `POST /api/v1/speakers/{name}/stop` · `/pause` · `/resume` | The speaker's entire current group |
| `POST /api/v1/speakers/{name}/ungroup` | Ungroup a speaker or dissolve the group |
| `GET /api/v1/groups` | Current groups with status |

All are token-protected. Handlers in `internal/api/group.go`, a single line in the router (`registerGroupRoutes`). Errors as before as `*apperr.Error`; if the group/partial result is known, it is attached to the error as `result` (as with scene play).

### Scheduler

- Stop time with a **paused** scene: the status is not `playing` → `stop_skipped` (existing rule from ADR 0013, now tested). The pause stays; a later resume continues.
- **Catch-up** after an agent restart now also skips `paused` — a restart should not override a deliberate pause.

## Alternatives

| Option | Con |
|--------|-----|
| Always send the command to the named speaker | Ineffective/rejected for members; the caller would have to know the coordinator |
| Group ID (`RINCON_…:n`) as API key | Unstable (changes with every regrouping), unwieldy for Homey flows; the room name is stable |
| Ungroup of the coordinator = only it leaves the group | Does not match "ungroup all"; ungrouping a member is enough for that |
| Resume = replay the scene if the source is foreign | Mixes two commands; `play` already exists |
| Cache the topology via UPnP event subscription | More state and lifecycle; one SOAP call per command is enough |

## Consequences

- Every driver/fake must implement `Resume`, `GetGroups`, `Ungroup` (mock: simulated topology, coordinator handover on ungroup, `RejectUngroup` for error paths).
- Group commands cost an additional SOAP call (topology).
- No new error code, no external module.
- GUI (2026-10-15): Pause/Resume per scene in the Now playing mini-player and ⋯ menu, Speakers tab by group with pause/stop, "Leave group" in the speaker detail — pattern in `docs/styleguide.md`.
- How-to: [`control-groups.md`](../how-to/control-groups.md); Homey reference and OpenAPI updated.
