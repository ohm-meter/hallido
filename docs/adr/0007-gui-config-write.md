# ADR 0007: Writing the config via the REST API

## Status

Accepted (2026-05-28)

## Context

The GUI should be able to create scenes and speaker entries in `config.json` without editing the file manually on the Pi. Until now, the config was loaded only at startup.

## Decision

- Token-protected endpoints under `/api/v1/config/*` write **only** the config file the server was given at startup (`-config`).
- Writes are **atomic** (temp file + `rename`), with the same validation as at startup (`DisallowUnknownFields`, scene validation, stream URI rules).
- `GET /api/v1/config` returns a **token-free** overview (IDs, speaker names, `listen`) — no `api_token` in the response.
- **Scenes:** hot reload in the running process (`scenes.Service.Reload`) after `POST /api/v1/config/scenes`.
- **Speaker overrides:** hot reload in the Sonos driver (`ReloadFixedIPs`) after `POST /api/v1/config/speakers/sync`; a restart is now needed only for `listen` or a driver change (`--driver`).

## Alternatives

| Alternative | Why not |
|-------------|---------|
| GUI writes the file directly | The browser has no file access on the Pi |
| `PUT` for the whole config | High risk of accidentally overwriting the token/scenes |
| Restart for every config change | Unnecessary for scenes and speaker IPs; hot reload is enough |

## Consequences

- The API is a trusted write point — still only with a Bearer token and rate limit.
- Tests use temporary config files.
- Docs: `docs/how-to/gui-scene-editor.md`, OpenAPI updated.
