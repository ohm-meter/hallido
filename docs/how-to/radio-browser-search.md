# Radio Browser search in the GUI

Find internet radio streams for new scenes via [Radio Browser](https://www.radio-browser.info/) — without googling URLs by hand.

## Prerequisites

- The agent is running with a valid API token (as for all GUI features).
- The agent host has **internet access** (DNS + HTTPS to `de1.api.radio-browser.info`).
- Scene editor enabled (`config.json` writable, ADR 0007).

## In the GUI

1. Open **New scene**.
2. In the **Source** section, scroll down to **Search radio**.
3. Enter a search term (for example `FM4`, `SWR3`, `Bayern 3`) — the search waits 300 ms (debounce), or click **Search**.
4. Click a result → **Custom stream URL** is set; name/ID are suggested if empty.
5. Choose speakers → **Save**.

Only streams that Sonos can play as a scene (`IsIntegratableStreamURI`) appear in the list.

## Examples

| Search | Expectation |
|--------|-------------|
| `fm4` | FM4 (ORF), https stream |
| `swr3` | SWR3 stations with an http(s) URL |

## API (optional)

```http
GET /api/v1/radio/search?q=fm4&limit=10
Authorization: Bearer <token>
```

Response:

```json
{
  "stations": [
    {
      "name": "FM4",
      "url": "https://streams.fm4.orf.at/fm4.mp3",
      "country": "Austria",
      "tags": "alternative"
    }
  ]
}
```

Error: `radio_search_failed` — for example, Radio Browser is offline or the request timed out (5 s).

## Troubleshooting

| Symptom | Hint |
|---------|------|
| "Radio search failed" | Check internet/DNS on the Pi; try again later |
| No results | Try a different search term; some stations only match their exact name |
| Token error | Set the token in the GUI |

## See also

- ADR: [`0011-radio-browser-api.md`](../adr/0011-radio-browser-api.md)
- Creating scenes: [`gui-scene-editor.md`](gui-scene-editor.md)
- OpenAPI: [`openapi.yaml`](../api/openapi.yaml)
