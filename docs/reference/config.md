# Reference: Configuration

File: `config.json` (template: `config.example.json`). Loaded at startup. Changes made through the GUI/API (scenes, schedules, fixed IPs) take effect immediately; manual edits to the file only after a restart (`sudo systemctl restart sonos-agent`).

## Top level

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `listen` | string | yes | Bind address (Go `http.Server.Addr`). Default when empty: `:8080`. Examples: `:9090` (all interfaces), `127.0.0.1:9090` (localhost only, e.g. with a Cloudflare Tunnel on the same host). **No** CLI flag or env var — this file only; restart required. |
| `api_token` | string | yes | Bearer token for protected API routes. The placeholder `BITTE-AENDERN` is rejected. Override: `SRA_API_TOKEN` |
| `speakers` | object | no | Map of room name → `{ "ip": "..." }`. A fixed IP is a fallback for SSDP problems; these names count as known at startup (no SSDP until refresh/play). See [Speaker config & GUI refresh](../how-to/speaker-config-and-refresh.md) |
| `scenes` | array | yes | List of scenes (an empty list is allowed) |
| `timezone` | string | no | IANA time zone for schedules, default `Europe/Vienna` (tzdata is embedded in the binary) |
| `schedules` | array | no | Schedules, see below and [Schedules](../how-to/schedules.md) |

## `scenes[]`

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique ID for the API (`/api/v1/scenes/{id}/play`) |
| `name` | string | Display name in the GUI |
| `source` | object | Playback source (see below) |
| `members` | array | Speakers and volumes; **index 0 = coordinator** |

## `source`

| `type` | Fields | Status |
|--------|--------|--------|
| `stream` | `uri` (HTTPS URL) | supported |
| `favorite` | `name` (Sonos favorite) | plays only if the favorite's URI can be integrated (otherwise a clear error); TuneIn/Spotify/Sonos Radio favorites usually can't — ADR 0005 |

## `schedules[]`

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique ID |
| `scene` | string | ID of an existing scene |
| `days` | array | `mon` … `sun` |
| `start` | string | `HH:MM` (local time per `timezone`) |
| `stop` | string | optional `HH:MM`; stops only if the scene is still `playing` |
| `enabled` | bool | on/off |

## `members[]`

| Field | Type | Description |
|-------|------|-------------|
| `speaker` | string | Room name (discovery / Sonos app). Don't list bonded satellites/subs — they never play on their own |
| `volume` | int | 0–100 |

## CLI / environment

| Variable / flag | Default | Description |
|-----------------|---------|-------------|
| `-config` | `config.json` | Path to the config |
| `-driver` / `SRA_DRIVER` | `mock` | `mock` or `sonos` |
| `-debug` | off | More logging |

## Security

- Don't commit the token to the repo.
- Keep `config.json` in `.gitignore`.

**Note:** `favorite` sources can only be added to `config.json` by hand; the config API (`POST/PUT /api/v1/config/scenes`) accepts `stream` scenes only. Fixed IPs under `speakers` must be IPv4 addresses.
