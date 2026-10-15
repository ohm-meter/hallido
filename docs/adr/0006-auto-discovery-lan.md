# ADR 0006: Automatic Sonos discovery on the LAN (no manual IPs)

## Status

Accepted — increment A implemented (2026-05-28).

## Context

- The **Sonos driver** (`internal/speaker/sonos`) already performs **SSDP
  discovery** and fills `devices` with **room names** from the device
  description.
- Users still have to maintain names (and often IPs) under `speakers` in
  `config.json` and match scene member names to the Sonos app **exactly**.
- The **mock** reports only speakers named in the config/scenes as "online" — no
  realistic discovery behavior for the GUI.
- Goal: **no manual IP maintenance**; find all speakers on the network
  automatically and make them usable for scenes/API.

## Decision

Discovery becomes a **first-class feature** of the API and configuration, not
just an internal helper call in `Discover()` during the health check.

### Increment A — transparency (small, first)

1. New endpoint `GET /api/v1/speakers` (token), response e.g.:
   ```json
   {
     "speakers": [
       { "name": "Living Room", "ip": "192.168.1.10", "uid": "RINCON_...", "reachable": true }
     ],
     "discovery_error": ""
   }
   ```
2. Implementation: extend Sonos `Discover()` with structured `SpeakerInfo`
   (or a thin API layer that reads `devices`).
3. GUI: list of found speakers below the status (display only), with a hint if
   scene names do not appear in discovery.
4. OpenAPI + handler tests; the mock returns a fixed test list independent of
   the config.

### Increment B — relieve the config (medium)

1. `speakers` in `config.json` becomes **optional** — empty = SSDP only.
2. At startup and before `Play`: **validation** — every `members[].speaker`
   must appear in the last discovery; otherwise `scene_misconfigured` with a
   list "found: …, missing: …".
3. Optional: **aliases** `{ "Living Room": { "match": "Living Room left" } }`
   for minor differences between the config label and the Sonos room name (ADR
   extension only if a hardware test shows differences).

### Increment C — convenience (optional, later)

1. Endpoint `POST /api/v1/discovery/refresh` (token, rate limit).
2. Background refresh every N minutes (configurable, off by default).
3. No automatic overwriting of `config.json` on the Pi (explicitly out of
   scope — configuration stays file-based and reviewable).

## Non-goals

- No cloud login / no Sonos API key.
- No automatic scene generation from the Sonos app (discovery only).
- Not a replacement for increment 3 (favorites).

## Technology / risks

| Risk | Mitigation |
|------|------------|
| SSDP blocked (Docker/VLAN) | Keep the optional fixed IP per name; `discovery_error` in health/API |
| Names change in the app | Validation + alias map; documentation in the runbook |
| Information disclosure (IPs) | Endpoint only with token; health still returns only count + names |
| Rate / multicast load | Refresh is rate-limited; keep the SSDP timeout short as before |

## Consequences

- `SpeakerController` may be extended with `ListDevices(ctx) ([]DeviceInfo, error)`
  **or** a type in the `speaker` package, implemented by Sonos/mock.
- Mock: parity with Sonos (favorites error already implemented); discovery list
  configurable for tests.
- Documentation: README "First start without IPs", runbook SSDP troubleshooting.

## Acceptance criteria (definition of done)

- [x] `GET /api/v1/speakers` with token, OpenAPI, tests
- [x] Sonos: all SSDP speakers incl. name/IP without an entry under `speakers`
- [x] Scene with unknown member name → clear error before play (`scene_misconfigured`)
- [x] GUI shows found speakers; mock tests green; `make test` green

## References

- Existing: `internal/speaker/sonos/discovery.go`, `Discover()` in `sonos.go`
- Health: `GET /api/v1/health` (count + names only)
- ADR 0005: favorites (independent)
