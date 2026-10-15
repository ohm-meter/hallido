# ADR 0011: Radio Browser search (external HTTP API)

## Status

Accepted (2026-05-28)

## Context

- In the scene dialog, users often need a **stream URL** for internet radio — googling it manually is error-prone.
- [Radio Browser](https://www.radio-browser.info/) offers a free JSON API with station search.
- **Browser CORS** blocks direct calls from the GUI — a proxy through the agent is needed.
- Radio Browser requires a **descriptive User-Agent** (app name + contact/URL).
- Sonos does not accept every URL — only `IsIntegratableStreamURI` (http(s), aac://, x-rincon-mp3radio, …).

## Decision

### External dependency (the first in the project)

- HTTP client: **stdlib only** `net/http`, no new Go module.
- Base mirror v1: **`https://de1.api.radio-browser.info`** — a fallback mirror list may follow later.
- Timeout: **5 s** per request.
- No API key.

### Backend

| Endpoint | Description |
|----------|-------------|
| `GET /api/v1/radio/search?q=…&limit=10` | Token-protected; proxy to the Radio Browser API |

Upstream: `GET /json/stations/search?name={q}&limit={n}`

Response: `{ "stations": [{ "name", "url", "country", "tags?" }] }`

- Stations without a `url` are discarded.
- Only URIs with `IsIntegratableStreamURI` appear in the response.
- Error code: **`radio_search_failed`** (HTTP 502) with a hint.

Package: `internal/radio` — thin client, tests with `httptest`.

### GUI

- In the "New scene" dialog, source section: **"Search radio"** (search field + button).
- Result list → a click sets the **"Custom stream URL"** and optionally the name/ID.
- 300 ms debounce, loading/empty/error states in English.

## Alternatives

| Option | Pro | Con |
|--------|-----|-----|
| Directly from the browser | no backend | CORS, no UA, no URI validation |
| Own station DB | offline | maintenance, duplicates Radio Browser |
| Docs/manual URL only | zero effort | poor UX |

## Consequences

- First **external HTTP dependency** — noted in `AGENTS.md`.
- The agent host needs internet access for search (not for locally configured streams).
- Radio Browser may rate-limit — no aggressive polling in the GUI.

## References

- How-to: [`radio-browser-search.md`](../how-to/radio-browser-search.md)
- OpenAPI: `GET /api/v1/radio/search`
- Radio Browser API docs: https://api.radio-browser.info/
