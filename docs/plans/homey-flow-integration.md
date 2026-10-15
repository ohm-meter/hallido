# Plan: Homey Advanced Flows via HTTP Request

**ADR:** [`0009-homey-flow-interface.md`](../adr/0009-homey-flow-interface.md)  
**API:** [`api-homey.md`](../reference/api-homey.md)  
**Status:** Docs implemented — for flows see [homey-integration.md](../how-to/homey-integration.md); stop/pause/resume available (ADR 0013/0015). Only the optional Homey app remains open (P3).

## Prerequisites

- Homey Pro + agent on the LAN; token in the config (not `change-me`).
- Test: [`homey-flow.http`](../examples/homey-flow.http) or curl.

## Step 1 — URL & token

`curl http://HOST:8080/api/v1/health` → `status: ok`. Get the token from the Pi.

## Step 2 — Homey variables

`SonosAgentURL`, `SonosAgentToken` (secret).

## Step 3 — HTTP Request

| Field | Value |
|-------|-------|
| URL | `{SonosAgentURL}/api/v1/…` |
| Header | `Authorization: Bearer {SonosAgentToken}` |

Screenshot placeholder: Advanced Flow → HTTP Request → Authorization.

## Flow 1 — Button → FM4 scene

POST `/api/v1/scenes/fm4-living-room/play`

```bash
curl -sS -X POST "http://HOST:8080/api/v1/scenes/fm4-living-room/play" \
  -H "Authorization: Bearer TOKEN"
```

Errors: 401 token, 404 `scene_not_found`, 400 `scene_misconfigured`.

## Flow 2 — Cron → discovery

POST `/api/v1/discovery/refresh`. Optionally GET `/speakers` → POST `/config/speakers/sync` (phase B: a single call).

```bash
curl -sS -X POST "http://HOST:8080/api/v1/discovery/refresh" -H "Authorization: Bearer TOKEN"
```

## Flow 3 — IF scene is playing

Interval **45 s**, GET `/api/v1/scenes/status`, `state == playing`.

```bash
curl -sS "http://HOST:8080/api/v1/scenes/status" -H "Authorization: Bearer TOKEN"
```

## Scene IDs (dropdown substitute)

```bash
curl -sS "http://HOST:8080/api/v1/scenes" -H "Authorization: Bearer TOKEN"
```

## Stop

No `POST …/stop` in v1 — ADR 0009 phase B.

## Rate limit

300 requests/min per client IP (see [API reference](../reference/api.md)) — still, don't poll more often than every 30 s.

## Checklist

- [ ] curl health, play, status
- [ ] Homey play audible
- [ ] 401 visible with a wrong token

## Without a Homey app

No push trigger, no dropdown, no stop, no pairing → P3 SDK.
