# Reference: REST API

**Contract:** [`docs/api/openapi.yaml`](../api/openapi.yaml) (OpenAPI 3.0)

Local base URL: `http://localhost:8080`

## Authentication

Protected routes: header `Authorization: Bearer <api_token>`.

Exception: `GET /api/v1/health` (public).

## Endpoints (overview)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/health` | no | Status, `speakers_online`, `speakers` (names) |
| GET | `/api/v1/scenes` | yes | Configured scenes |
| GET | `/api/v1/scenes/status` | yes | Playback per scene (`playing`, `playing_other`, …); cached for 3 s |
| POST | `/api/v1/scenes/{id}/play` | yes | Start a scene |
| POST | `/api/v1/scenes/{id}/stop` | yes | Stop a scene (coordinator) — ADR 0013 |
| POST | `/api/v1/scenes/{id}/pause` · `/resume` | yes | Pause / resume a scene — ADR 0015 |
| GET | `/api/v1/groups` | yes | Current Sonos groups (coordinator, members, state) — ADR 0015 |
| POST | `/api/v1/speakers/{name}/stop` · `/pause` · `/resume` | yes | Acts on the speaker's whole current group (even without a scene, e.g. a stream started from the Sonos app) |
| PUT | `/api/v1/speakers/{name}/volume` | yes | Set a speaker's volume (0–100) |
| POST | `/api/v1/speakers/{name}/ungroup` | yes | Speaker leaves its group; for the coordinator, the group is dissolved |
| GET | `/api/v1/schedules` | yes | Schedules with `next_start`/`last_run` — ADR 0013 |
| GET · POST | `/api/v1/config/schedules` | yes | Read / create schedules |
| PUT · DELETE | `/api/v1/config/schedules/{id}` | yes | Update / delete a schedule |
| GET | `/api/v1/speakers` | yes | Known speakers: `{ name, ip, uid, reachable }[]` (no SSDP) |
| GET | `/api/v1/speakers/now-playing` | yes | Now playing on all speakers (bulk, cached for 3 s) |
| GET | `/api/v1/speakers/{name}/now-playing` | yes | Now playing on one speaker |
| GET | `/api/v1/speakers/stream-sources` | yes | Speakers with a directly importable stream |
| POST | `/api/v1/discovery/refresh` | yes | Re-run SSDP/fixed-IP discovery |
| GET | `/api/v1/config` | yes | Config overview without the token |
| POST | `/api/v1/config/speakers/sync` | yes | Save the discovery result (`name`, `ip`) to the config |
| POST | `/api/v1/config/scenes` | yes | Create/replace a scene |
| PUT | `/api/v1/config/scenes/{id}` | yes | Update a scene |
| DELETE | `/api/v1/config/scenes/{id}` | yes | Delete a scene |
| POST | `/api/v1/config/scenes/from-favorite` | yes | Create a scene from a Sonos favorite |
| GET | `/api/v1/sonos/favorites` | yes | Sonos favorites (`sonos` driver only) |
| GET | `/api/v1/radio/search?q=` | yes | Radio Browser search (ADR 0011) |

Details, request bodies and examples: OpenAPI, [Schedules](../how-to/schedules.md), [Control groups](../how-to/control-groups.md).

Config writes back up the previous state as `config.json.bak` and reload scenes/fixed IPs without a restart.

## Error responses

JSON with `error` (stable code), `message`, and optionally `speaker` and `hint`. Stable codes:

- `unauthorized` — token missing/invalid (401)
- `scene_not_found` — unknown scene (404)
- `schedule_not_found` — unknown schedule (404)
- `scene_misconfigured` — unknown speaker name (400)
- `speaker_unreachable` — speaker offline (502 for the coordinator)
- `playback_failed` — speaker rejects the request
- `grouping_failed` — grouping failed (502)
- `radio_search_failed` — Radio Browser unreachable (502)
- `bad_request` — invalid request (400)
- `not_found` — unknown API path (404)
- `rate_limited` — limit of 300 requests/min per client IP (429, `Retry-After`)
- `internal` — internal error (500)

Partial failure on play: HTTP 200 with `speakers[]`, one `{ speaker, ok, … }` entry per speaker.

## Security

The API speaks **unencrypted HTTP** on the LAN; the bearer token can be read on the network. Run it only on a trusted LAN or behind TLS termination (e.g. [Cloudflare Tunnel](../how-to/remote-access.md)). The rate limit and token check are no substitute for network segmentation.

## GUI

Static files under `/` — they use the same endpoints (`internal/web/static/app.js`).

## External integration

OpenAPI for Homey and similar; versioned prefix `/api/v1`.

- How-to: [`homey-integration.md`](../how-to/homey-integration.md)
- Flow builder quick reference: [`api-homey.md`](api-homey.md)
