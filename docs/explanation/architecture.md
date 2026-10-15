# Explanation: Architecture

Quick overview — repo layout: [project-structure.md](project-structure.md). Conventions: [AGENTS.md](../../AGENTS.md). Decisions: [ADRs](../adr/).

## In one sentence

The GUI and external clients talk to the **REST API**; the API (and the **scheduler** for schedules) uses the **scene service**; the service controls speakers only through **SpeakerController** (mock or Sonos).

## Layers

```
Browser / Homey
       │  HTTP (Bearer token)
       ▼
 internal/api          ← routes, JSON, error codes
       │                 internal/schedule ← schedules (start/stop, catch-up)
       │                        │
 internal/scenes      ← load scenes, play/stop/pause/resume
       │
 internal/speaker     ← SpeakerController interface
       ├── mock/      ← tests, development without hardware
       └── sonos/     ← SSDP, SOAP, UPnP on the LAN
```

Driver selection: `cmd/server/main.go` (`buildController`, flag `--driver`).

## Scene model

- **Scene:** id, name, source, members (order matters).
- **Coordinator:** the first member starts the group and playback.
- **Source:** stream URI or Sonos favorite (only with an integrable URI).
- **Group control (ADR 0015):** stop/pause/resume/ungroup act on a speaker's *current* Sonos group, even if it was not started via a scene.

Config-only: a new scene is an entry in `config.json`, no code.

## Discovery (ADR 0006)

- Sonos: SSDP + device description → room names, IPs, UIDs.
- API: `GET /api/v1/speakers` makes discovery visible.
- Validation before play: unknown names → `scene_misconfigured`.
- Optional `speakers` in the config as an IP fallback.

## Error philosophy

- Outward: `*apperr.Error` with a stable `code`.
- Devices offline: timeout + `speaker_unreachable`; partial failure is not a hard abort — the remaining speakers play.
- SOAP/XML: escaped arguments (injection protection).

## Operations

- Typical: Raspberry Pi or Linux server on the same LAN as the speakers, systemd service `sonos-agent`: code owned by root under `/usr/local/sonos-agent`, config in `/var/lib/sonos-agent` (installer `scripts/install.sh`, [ADR 0017](../adr/0017-install-layout.md)), or Docker; updates via `sonos-agent update`.

## Frontend

Lean static site: `internal/web/static/` (HTML, CSS, JS, zero dependencies). Design: [styleguide](../styleguide.md).

## Tests

`go test ./...` — mock driver; Sonos against an HTTP SOAP mock, no real hardware in CI.
