# How-to: Homey integration (REST API)

Problem: start scenes from Homey (or other home automation) via a flow and optionally query status.

**Canonical endpoint table:** [`reference/api-homey.md`](../reference/api-homey.md) — this how-to contains example flows and error handling.

There is **no dedicated Homey app** in this repo — integration works through the **HTTP Request** card (Advanced Flow) or comparable REST components.

## Base URL

Agent on the LAN, typically via mDNS:

```
http://<HOSTNAME>.local:8080
```

Example (Pi hostname `raspberrypi`): `http://raspberrypi.local:8080`

Alternative, fixed IP: `http://192.168.1.50:8080`

Port from `listen` in `config.json` (default `:8080`).

## Authentication

Protected endpoints require this header:

```
Authorization: Bearer <api_token>
```

Token in `config.json` or the `SRA_API_TOKEN` environment variable on the agent host.

Exception: `GET /api/v1/health` — **no** token (monitoring).

## Endpoints for flows

| Action | Method | Path | Auth |
|--------|--------|------|------|
| Check reachability | GET | `/api/v1/health` | no |
| List scenes | GET | `/api/v1/scenes` | yes |
| Start a scene | POST | `/api/v1/scenes/{id}/play` | yes |
| Speakers on the LAN | GET | `/api/v1/speakers` | yes |
| Playback status per scene | GET | `/api/v1/scenes/status` | yes |
| Pause / resume a scene | POST | `/api/v1/scenes/{id}/pause` · `/resume` | yes |
| Stop / pause / resume a speaker's group | POST | `/api/v1/speakers/{name}/stop` · `/pause` · `/resume` | yes |
| Ungroup | POST | `/api/v1/speakers/{name}/ungroup` | yes |
| Current groups | GET | `/api/v1/groups` | yes |

Full contract: [`docs/api/openapi.yaml`](../api/openapi.yaml)  
Quick reference for flow builders: [`api-homey.md`](../reference/api-homey.md)

## Example flow: start a scene

**Trigger:** e.g. "When scene X is activated" or a schedule

**Action — HTTP Request:**

- Method: `POST`
- URL: `http://<HOSTNAME>.local:8080/api/v1/scenes/fm4-living-room/play` (example: `http://raspberrypi.local:8080/...`)
- Headers: `Authorization: Bearer your-token`
- Body: empty

Success: HTTP 200, JSON e.g.:

```json
{
  "scene_id": "fm4-living-room",
  "playing": true,
  "speakers": [
    { "speaker": "Living Room", "ok": true }
  ]
}
```

Partial failure: still HTTP 200, with individual `speakers[].ok: false` — decide whether your flow should tolerate that.

## Example flow: check status before starting (optional)

1. **GET** `/api/v1/scenes/status` with the bearer token
2. Parse the JSON in Homey: an entry with `scene_id` and `state: "playing"` → the scene is already running
3. Otherwise **POST** `/api/v1/scenes/{id}/play`

Possible `state` values: `playing`, `stopped`, `paused`, `unknown`, `offline`.

## Example flow: health as a precondition

Before playing:

- **GET** `/api/v1/health`
- Condition: `speakers_online` ≥ 1 and no `discovery_error`

This avoids play attempts when SSDP is completely blocked.

## Error codes (JSON field `error`)

| Code | HTTP | Meaning |
|------|------|---------|
| `unauthorized` | 401 | Token missing or wrong |
| `scene_not_found` | 404 | Unknown scene ID |
| `scene_misconfigured` | 400 | Speaker name in the scene is unknown |
| `speaker_unreachable` | 502 | Coordinator offline |
| `playback_failed` | 502 | Speaker rejects playback |
| `bad_request` | 400 | Includes the rate limit (see below) |

The response often contains `message` and `hint` — use them in Homey for logs/notifications.

## Rate limit

Protection against abuse on the LAN: **60 requests per minute per client IP**.

When exceeded: HTTP 400, `error: bad_request`, message "Too many requests, please wait a moment".

Don't poll in tight loops in flows — query status every 30–60 s, for example, or event-driven.

## Security

- Use a long, random token; keep it only in a Homey secret/flow variable, not in screenshots
- Do **not** expose the agent to the internet (no port forwarding of 8080)
- HTTP on the LAN only — the agent has no TLS (trust boundary = home network)

## HTTP examples

Ready-made requests for testing (curl/REST client): [`docs/examples/homey-flow.http`](../examples/homey-flow.http)

## Planning

- [ADR 0009](../adr/0009-homey-flow-interface.md), [plan](../plans/homey-flow-integration.md), [API table](../reference/api-homey.md)

## Open items

- Homey app (P3), webhooks (phase 2)
- Stopping: `POST /api/v1/scenes/{id}/stop` is available (see the [API table](../reference/api-homey.md)); fixed times also work without Homey via [schedules](schedules.md)

See the [roadmap](../todos.md).
