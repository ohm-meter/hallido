# ADR 0009: Homey flow interface (HTTP request first)

## Status

Proposed (2026-05-28)

## Context

- Hallido exposes a **versioned REST API** (`/api/v1`, Bearer token, stable error codes, OpenAPI).
- Homey Pro can integrate third-party services through the **HTTP Request** card (Advanced Flow) — without an app store review.
- A **dedicated Homey app** (own triggers/cards) is **P3** in `docs/todos.md` — not a blocker for automation.
- Rate limit: **60 requests / minute / client IP**.
- There is **no** `POST /api/v1/scenes/{id}/stop` — phase B.

## Decision

### v1: HTTP request integration (without the Homey App SDK)

| Flow building block | v1 | API |
|---------------------|----|-----|
| Actions | HTTP POST/GET | `play`, `discovery/refresh`, `config/speakers/sync` |
| Conditions | GET + JSON parse | `scenes/status`, `health`, `speakers` |
| Triggers | Polling / Homey trigger | no push from the agent |

**Homey variables:** `agent_base_url`, `agent_token` → header `Authorization: Bearer …`.

**Why HTTP first:** usable immediately, API is stable; downside: manual URL/token, no scene dropdown (workaround: `GET /scenes`).

#### Actions

| Action | Method | Path | Body |
|--------|--------|------|------|
| Start scene | POST | `/api/v1/scenes/{id}/play` | empty |
| Discovery | POST | `/api/v1/discovery/refresh` | empty |
| Speaker sync | POST | `/api/v1/config/speakers/sync` | `{"speakers":[…]}` |
| Scene upsert | POST | `/api/v1/config/scenes` | `{"scene":…}` |

#### Conditions

| Condition | Path | Evaluation |
|-----------|------|------------|
| Agent OK | `/api/v1/health` | `status == ok` (no token) |
| Scene playing | `/api/v1/scenes/status` | `state == playing` |
| Other content | status | `playing_other` |
| Stopped | status | `stopped` / `paused` |
| Offline | status | `offline` |
| Speaker reachable | `/api/v1/speakers` | `reachable` |

#### Triggers

| Want | v1 | Recommendation |
|------|----|----------------|
| Scene started | No (push) | Poll status every 30–60 s |
| Scene stopped | No | Poll → `stopped` |
| Speaker offline | Polling | `GET /speakers` or status |
| Button/time | Yes | Homey trigger → play |

**Rate limit:** poll at intervals ≥ 30 s; trigger play on events; on a 400 `bad_request`, increase the interval.

### Phase B

- `POST /api/v1/scenes/{id}/stop` — implemented with ADR 0013
- Discovery → config in one call (ADR 0008)
- Webhooks phase 2: LAN agent → Homey without a tunnel is impractical

**Scene IDs:** `GET /api/v1/scenes` or `GET /api/v1/config` → flow variable.

## Alternatives

| Option | Pro | Con |
|--------|-----|-----|
| HTTP request | immediate | polling, manual |
| Homey App SDK | native UI | store, maintenance |

## Consequences

- [`homey-flow-integration.md`](../plans/homey-flow-integration.md), [`api-homey.md`](../reference/api-homey.md)
- The app stays P3

## References

ADR 0003, [`homey-integration.md`](../how-to/homey-integration.md), [`openapi.yaml`](../api/openapi.yaml)
