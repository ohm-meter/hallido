# Tutorial: First run locally

Goal: start the agent locally, set a token, play scenes and check discovery.

## Prerequisites

- Go 1.22+ (as in the project)
- No Sonos hardware needed for the mock driver

## 1. Create the configuration

```bash
cp config.example.json config.json
```

Set your own `api_token` in `config.json` — the placeholder `BITTE-AENDERN` is rejected.

Alternatively (takes precedence over the file):

```bash
export SRA_API_TOKEN=my-secret-token
```

## 2. Start

**With real Sonos speakers (recommended on a Mac):**

```bash
make dev
# or: ./scripts/dev.sh
```

Sonos driver, debug logging. Optionally add a `.env` from `.env.example`.

**Without hardware (mock):**

```bash
make run
```

Default: `--driver=mock`. All speakers named in the scenes count as online.

Details: [Run locally](../how-to/local-start.md).

Browser: http://localhost:8080

## 3. Token in the GUI

On first load the token dialog appears. Paste the token from `config.json` (Latin-1 characters only, no emojis). It is stored locally in `localStorage`.

## 4. Play a scene

The cards show the configured scenes. A click starts `POST /api/v1/scenes/{id}/play`. Status: border color and badge (`starting…`, `playing`, partial failure, error).

## 5. Discovery and the speakers endpoint

With a valid token the GUI also loads `GET /api/v1/speakers`:

```bash
curl -H "Authorization: Bearer $SRA_API_TOKEN" http://localhost:8080/api/v1/speakers
```

Response (example):

```json
{
  "speakers": [
    { "name": "Living Room", "ip": "192.168.1.10", "uid": "RINCON_...", "reachable": true }
  ],
  "discovery_error": ""
}
```

With the mock driver the speakers come from a fixed test list. Found speakers appear below the status; unknown scene names are reported as a hint.

Health (no token):

```bash
curl http://localhost:8080/api/v1/health
```

## 6. Sonos driver (details)

`make dev` already starts with `-driver=sonos`. Speaker names in scenes must match
the room names in the Sonos app exactly.

Details: [Use the Sonos driver](../how-to/use-sonos-driver.md).

## Next steps

- [Create a scene](../how-to/create-scene.md)
- [API reference](../reference/api.md)
- [Style guide](../styleguide.md) when customizing the GUI
