# Reference: API for Homey & flow builders

Quick reference for connecting Homey Advanced Flows (HTTP Request). Full contract: [`openapi.yaml`](../api/openapi.yaml).

**Planning:** ADR [`0009-homey-flow-interface.md`](../adr/0009-homey-flow-interface.md), plan [`homey-flow-integration.md`](../plans/homey-flow-integration.md).

**Base URL:** `http://<agent-host>:8080` (LAN only)

## Auth

```
Authorization: Bearer <api_token>
```

Exception: `GET /api/v1/health`

## Endpoints

### GET /api/v1/health

Public. Monitoring, precondition in flows.

```json
{
  "status": "ok",
  "speakers_online": 2,
  "speakers": ["Living Room", "Kitchen"],
  "discovery_error": ""
}
```

### GET /api/v1/scenes

List of configured scenes (IDs for play flows).

```json
{
  "scenes": [
    {
      "id": "fm4-living-room",
      "name": "FM4 – Living Room",
      "source": { "type": "stream", "uri": "https://…" },
      "members": [{ "speaker": "Living Room", "volume": 28 }]
    }
  ]
}
```

### POST /api/v1/scenes/{id}/play

Start a scene. No request body.

Success (200):

```json
{
  "scene_id": "fm4-living-room",
  "playing": true,
  "speakers": [
    { "speaker": "Living Room", "ok": true },
    { "speaker": "Kitchen", "ok": false, "error": "…" }
  ]
}
```

Errors (excerpt): 404 `scene_not_found`, 400 `scene_misconfigured`, 502 `speaker_unreachable` / `playback_failed` (possibly with a `result` object).

### POST /api/v1/scenes/{id}/stop

Stop a scene (AVTransport Stop on the first speaker; the group follows). No request body. Idempotent — an already stopped speaker is not an error. Does not check what is playing: to stop only "your own" scene, first query `GET /api/v1/scenes/status` (`state == playing`).

```json
{ "scene_id": "fm4-living-room", "stopped": true }
```

Errors: 404 `scene_not_found`, 502 `speaker_unreachable` / `playback_failed`.

### PUT /api/v1/speakers/{name}/volume

Set a speaker's volume live (0–100), e.g. "Living Room quieter". Speaker name URL-encoded as in the Sonos app. Does not change the scene's starting volume.

```json
{ "volume": 25 }
```

Response `{ "speaker": "Living Room", "volume": 25 }`. Errors: 400 `bad_request` (field missing, not an integer, outside 0–100), 502 `speaker_unreachable`.

### POST /api/v1/scenes/{id}/pause · /resume

Pause or resume a scene (AVTransport Pause / Play on the first speaker, without reloading the source). No request body. For live radio, Stop is usually the better choice (pause buffers). `resume` without a loaded source → 502 `playback_failed` (then use `…/play`).

```json
{ "scene_id": "fm4-living-room", "paused": true }
{ "scene_id": "fm4-living-room", "resumed": true }
```

### POST /api/v1/speakers/{name}/stop · /pause · /resume

Acts on the speaker's **whole current group** — even if it was built in the Sonos app. The command goes to the coordinator. Room name URL-encoded (`Basement%3A%20TV`).

```json
{ "speaker": "Kitchen", "coordinator": "Living Room", "members": ["Living Room", "Kitchen"], "action": "pause", "ok": true }
```

Errors: 502 `speaker_unreachable` (speaker not in the topology / offline), 502 `playback_failed` (speaker rejects the request); the affected group may be under `result`.

### POST /api/v1/speakers/{name}/ungroup

Member → leaves the group. Coordinator → whole group dissolved (`dissolved: true`), the coordinator keeps playing alone. Already standalone → `standalone: true`. Partial failure: 200 with the error per speaker in `speakers`; 502 only if no speaker could leave the group.

```json
{ "speaker": "Living Room", "coordinator": "Living Room", "dissolved": true, "standalone": false,
  "speakers": [{ "speaker": "Kitchen", "ok": true }] }
```

### GET /api/v1/groups

Current groups with the coordinator's state (`state`: `playing` | `paused` | `stopped` | `unknown` | `offline`), title/URI best effort, and `scene_id` when a scene is recognized.

```json
{ "groups": [ { "coordinator": "Living Room", "members": ["Living Room", "Kitchen"], "state": "playing", "title": "FM4", "scene_id": "fm4-living-room" } ] }
```

### GET /api/v1/schedules

The agent's schedules with `next_start`, `next_stop`, `active`, `last_run` — e.g. as a condition "alarm active?". Details: [Schedules](../how-to/schedules.md).

### GET /api/v1/speakers

Discovery list (name, IP, reachable).

```json
{
  "speakers": [
    { "name": "Living Room", "ip": "192.168.1.10", "uid": "RINCON_…", "reachable": true }
  ],
  "discovery_error": ""
}
```

### GET /api/v1/scenes/status

AVTransport state per scene (coordinator).

```json
{
  "scenes": [
    {
      "scene_id": "fm4-living-room",
      "coordinator": "Living Room",
      "state": "playing",
      "error": ""
    }
  ]
}
```

`state`: `playing` | `playing_other` | `stopped` | `paused` | `unknown` | `offline`

## Error JSON

```json
{
  "error": "unauthorized",
  "message": "Invalid or missing API token",
  "hint": "Set the header 'Authorization: Bearer <token>'."
}
```

Stable codes: `unauthorized`, `scene_not_found`, `scene_misconfigured`, `speaker_unreachable`, `playback_failed`, `grouping_failed`, `schedule_not_found`, `radio_search_failed`, `bad_request`, `not_found`, `rate_limited`, `internal`.

## Rate limit

300 requests / minute / client IP → HTTP 429, `rate_limited` (header `Retry-After`). Unknown `/api/` paths → HTTP 404, `not_found` (JSON).

## Flow actions → API (v1)

Header `Authorization: Bearer <token>` except for health.

| Flow role | Action | Method | Path | Body | Success |
|-----------|--------|--------|------|------|---------|
| Action | Start scene | POST | `/api/v1/scenes/{id}/play` | — | 200 + `playing` |
| Action | Stop scene | POST | `/api/v1/scenes/{id}/stop` | — | 200 + `stopped` |
| Action | Set volume | PUT | `/api/v1/speakers/{name}/volume` | `{"volume": 25}` | 200 + `volume` |
| Action | Pause / resume scene | POST | `/api/v1/scenes/{id}/pause` · `/resume` | — | 200 + `paused` / `resumed` |
| Action | Stop / pause / resume room (whole group) | POST | `/api/v1/speakers/{name}/stop` · `/pause` · `/resume` | — | 200 + `ok` |
| Action | Ungroup | POST | `/api/v1/speakers/{name}/ungroup` | — | 200 + `speakers` |
| Condition | Room is playing | GET | `/api/v1/groups` | — | Group with the room in `members`, `state == playing` |
| Action | Discovery | POST | `/api/v1/discovery/refresh` | — | 200 + `speakers` |
| Action | Speaker sync | POST | `/api/v1/config/speakers/sync` | JSON speakers | 200 `ok` |
| Condition | Agent OK | GET | `/api/v1/health` | — | no token |
| Condition | Scene is playing | GET | `/api/v1/scenes/status` | — | `playing` |
| Condition | Other content playing | GET | `/api/v1/scenes/status` | — | `playing_other` |
| Info | Scene IDs | GET | `/api/v1/scenes` | — | `scenes[].id` |
| Info | Speakers | GET | `/api/v1/speakers` | — | `reachable` |

**Phase B:** `POST /api/v1/scenes/{id}/stop` — implemented (ADR 0013).

## Homey HTTP Request — field mapping

| Flow field | Value |
|------------|-------|
| Method | `GET` or `POST` |
| URL | `http://<host>:8080/api/v1/…` |
| Headers | `Authorization: Bearer {{token}}` |
| Body | empty (play); JSON for sync |

How-to: [`homey-integration.md`](../how-to/homey-integration.md)
