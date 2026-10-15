# How-to: Create a scene

Problem: configure a new radio preset (speakers, source, volume).

## Steps

1. Open `config.json` (template: `config.example.json`).
2. Add an entry under `scenes`:

```json
{
  "id": "fm4-living-room",
  "name": "FM4 – Living Room",
  "source": {
    "type": "stream",
    "uri": "https://orf-live.ors-shoutcast.at/fm4-q2a"
  },
  "members": [
    { "speaker": "Living Room", "volume": 28 },
    { "speaker": "Kitchen", "volume": 22 }
  ]
}
```

3. Restart the agent (the config is loaded at startup).
4. Check in the GUI: the scene appears; the names must show up in discovery (`GET /api/v1/speakers`).

## Rules

| Field | Note |
|-------|------|
| `id` | Unique, URL-safe (API path) |
| `members[0]` | **Coordinator** — leads the group, must be reachable |
| `members[].speaker` | Exact Sonos room name (case, umlauts) |
| `members[].volume` | 0–100 |
| `source.type` | `stream` (recommended) or `favorite` (Sonos: not active yet, ADR 0005) |

## Sources

- **Stream:** `type: stream`, `uri` with an HTTPS stream URL.
- **Favorite:** `type: favorite`, `name` as in the Sonos app — the Sonos driver currently returns a clear error; use a stream for now.

## Validation

Unknown speaker names → `POST …/play` responds with **400** and code `scene_misconfigured`. The GUI warns in advance if discovery knows different names.

## API test

```bash
curl -X POST -H "Authorization: Bearer $SRA_API_TOKEN" \
  http://localhost:8080/api/v1/scenes/fm4-living-room/play
```
