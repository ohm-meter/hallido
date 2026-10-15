# Running tests

Quick overview of the test pyramid for Sonos Radio Agent: Go unit tests,
shell integration against the running API, and static GUI checks without a browser.

## Test pyramid

| Level | Tool | When | Hardware |
|-------|------|------|----------|
| **Unit** | `make test` (`go vet` + `go test`) | Every commit, CI | None (mock in code) |
| **API integration** | `scripts/test/api-*.sh` | Before a release, after API changes | None (mock driver) |
| **GUI static** | `scripts/test/gui-static.sh` | After GUI changes | None |
| **GUI layout** | `scripts/test/gui-layout.sh` (Playwright) | **Required** for `internal/web/static/*` | Node + Chromium (CI) |
| **Install layout** | `scripts/test/install-layout.sh` (in `make test`), `scripts/test/install-migration-docker.sh` (`make test-install-docker`) | When changing `scripts/install.sh`, `scripts/sonos-agent-cli.sh`, `deploy/systemd/` | Docker only for the migration test |
| **GUI manual** | Checklist below | Before go-live, after UI features | Optional mock or Sonos |
| **Sonos hardware** | `make dev` + manual checks | Driver validation | Real speakers on the LAN |

```
        ┌─────────────┐
        │ GUI manual  │  rarely, with a real browser
        ├─────────────┤
        │ GUI static  │  scripts/test/gui-static.sh
        ├─────────────┤
        │ API shell   │  scripts/test/api-mock.sh
        ├─────────────┤
        │ go test     │  make test (base, CI)
        └─────────────┘
```

## Quick start

```bash
make test              # required before commit: vet + all Go tests
make test-api          # API integration (starts a mock agent if needed)
make test-api-smoke    # 3-check quick gate (CI-friendly)
make test-gui          # static + Playwright layout
make test-scripts      # API + GUI (run-all)
```

Individually:

```bash
./scripts/test/api-smoke.sh
./scripts/test/api-mock.sh
./scripts/test/gui-static.sh
./scripts/test/gui-layout.sh
./scripts/test/run-all.sh
./scripts/test/install-layout.sh            # .install-meta parser, unit rewrite (no root)
./scripts/test/install-migration-docker.sh  # migration with real paths in a container
./scripts/sonos-now-playing.sh "Living Room"   # needs a running agent + SRA_TOKEN
```

## Mock vs. Sonos

| Scenario | Recommendation |
|----------|----------------|
| CI, pre-commit (`make test`), API contract | **Mock** (`-driver=mock`, fixture `scripts/test/fixtures/config.test.json`) |
| Pre-commit with staged GUI files | **Mock** + `make test-gui` (see below) |
| Local GUI/API development | Mock (`make run`) or a running dev agent |
| Stream, grouping, discovery | **Sonos** (`make dev`, real speakers) — can't be automated in shell scripts |

By default the shell scripts start a **mock agent** on port **18080**
and shut it down afterwards. To use an agent already running on another port:

```bash
SRA_TEST_BASE_URL=http://127.0.0.1:8080 SRA_TEST_AUTO_START=0 make test-api
```

## Environment variables

| Variable | Default | Meaning |
|----------|---------|---------|
| `SRA_TEST_BASE_URL` | `http://127.0.0.1:18080` | Base URL of the agent |
| `SRA_TEST_TOKEN` | from fixture / `config.json` | Bearer token for protected endpoints |
| `SRA_TEST_AUTO_START` | `1` | `0` = no auto-start, the agent must already be running |

Token order: `SRA_TEST_TOKEN` → fixture `integration-test-token` → `config.json`
(not `BITTE-AENDERN`).

## API scripts in detail

### `api-smoke.sh` (minimal)

1. `GET /api/v1/health` → 200
2. `GET /api/v1/scenes` without a token → 401
3. `GET /api/v1/scenes` with a token → 200

### `api-mock.sh` (full)

In addition to smoke:

- `GET /api/v1/speakers` with a token → 200
- `GET /api/v1/scenes/status` with a token → 200
- `GET /api/v1/speakers/{name}/now-playing` with a token → 200
- `POST /api/v1/scenes/{id}/play` → 200 (first scene in the list)

Output: `PASS`/`FAIL` per check, exit code 1 on failure.
`jq` is optional for JSON parsing; without it, grep fallbacks apply.

## GUI static

Checks without Node or a browser:

- `index.html` references `app.js` and `style.css`
- Required element IDs (`statusDot`, `grid`, `tokenDialog`, `refreshSpeakersBtn`, …)
- `app.js`: `STATUS_POLL_MS`, `tokenHeaderSafe`, status polling, refresh button

## GUI layout (Playwright)

Catches **visible overlaps** in the browser (e.g. the member list covering the source area in the "New group" dialog).

```bash
npm ci
npx playwright install chromium   # once locally; CI uses --with-deps
make build-gui                    # binary with embedded GUI (go:embed)
make test-gui                     # build-gui + static + layout
# layout only:
./scripts/test/gui-layout.sh
```

Playwright starts and stops the mock agent (`webServer` in `playwright.config.js`, script `scripts/test/start-gui-layout-agent.sh`) — no SIGHUP from npm background jobs.

**Viewports** (Playwright projects in `playwright.config.js`):

| Project | Size | Orientation |
|---------|------|-------------|
| `mobile` | 390×844 | iPhone-like |
| `tablet` | 820×1180 | iPad-like |
| `desktop` | 1280×800 | Laptop/desktop |

**Specs** (`tests/gui/`):

| File | Contents |
|------|----------|
| `touch-targets.spec.js` | **Mobile redesign acceptance:** in all tabs and sheets every visible control ≥ 44×44 px, no overlapping targets (visible area, per layer), no horizontal overflow |
| `navigation.spec.js` | Tab bar + hash routing (`#szenen`/`#zeitplaene`/`#boxen`), back button, deep link, tab bar at the bottom of the phone column (all widths) |
| `header-layout.spec.js` | Header 48 px, status/plus aligned, theme toggle in the Speakers tab |
| `scene-cards-layout.spec.js` | Scene rows 72 px, round 56 px play button, gaps ≥ 8 px, **≥ 5 scenes without scrolling**, play/stop, neutral `playing_other`, delete only with confirmation |
| `now-playing.spec.js` | "Now playing" bar above the tab bar, covers nothing, stop |
| `scene-dialog-layout.spec.js` | Scene editor as a sheet (full screen on phones), segmented control, no scroll-in-scroll, radio search → Save & play, edit prefill |
| `schedules-layout.spec.js` | Schedule rows, 48 px toggle hit area, round 44 px day chips, quick select, PUT/DELETE |
| `volume-sheet.spec.js` | ⋯ menu: edit + volume per room, −/+ by 1, save in the scene, live only while the scene is playing, per-room errors |
| `icons-toolbar-layout.spec.js` | Icon sizes vs. tokens (header/tab 22 px, theme 20 px), no overflow on mobile |
| `pwa.spec.js` | Web app (ADR 0016): manifest valid + icons load, service worker registered and serves the page, `/api/` never served from the SW/cache, phone column (max 460 px, centered, sheets inside the column), `theme-color` follows the appearance |

Shared helpers: `_layout-helpers.js` (API stubs with 6 scenes/15 speakers, token seed, `gotoTab`, `settleAnimations` before geometry measurements, `readTouchTargets`).

Service workers are blocked in all specs (`serviceWorkers: "block"` in `playwright.config.js`); only `pwa.spec.js` allows them — otherwise requests would bypass the `page.route` stub.

**Approach:** The specs stub the GUI API (`stubGuiApiRoutes`); the mock agent (`config.gui-layout.json`) serves only the static GUI and any calls that aren't stubbed.

**New GUI features:** add or extend the matching spec under `tests/gui/`; without a green `make test-gui`, no merge/commit for GUI diffs.

**Pre-commit** (`.githooks/pre-commit`, enable with `git config core.hooksPath .githooks`):

1. Always `make test`
2. If staged files are under `internal/web/static/` → also `make test-gui`

**Limits:** No pixel screenshot comparison; instead, geometric layout checks (bounding boxes, touch target sizes, icon widths vs. tokens). Doesn't cover semi-transparent overlays or z-index without area overlap. Not a replacement for manual checks on real Sonos.

**Preventing regressions in GUI changes:** Add new sheets/tabs to `touch-targets.spec.js`; set sizes only via the tokens (`--tap-min`, `--tap-primary`, `--tap-gap`, `--icon-*`) in `style.css` — `gui-static.sh` checks that they exist.

## GUI manual (E2E checklist)

Check in the browser before go-live or after larger UI changes:

1. **Start:** `make run` or `make dev`, open http://localhost:8080; after a code change: `./scripts/dev.sh restart`
2. **Health:** the status dot turns green, the text shows reachable speakers
3. **Token dialog:** Speakers tab → "Change API token" → save → scenes load
4. **Scenes:** tapping a row starts it (button turns amber/stop), tapping again stops it
5. **Status:** after ~8 s the playback status updates (polling); the "Now playing" bar appears
6. **Volume:** tap the bar → sheet, slider/−/+ change the speaker immediately
7. **Navigation:** switch tabs, the phone's back button returns to the previous tab
8. **Speakers:** "Refresh" fills the list; tap a speaker → now-playing panel
9. **Error case:** wrong token → hint text, no silent empty state

Browser E2E with Playwright is in the repo for **layout regressions** (`tests/gui/`, `make test-gui`).
A full manual E2E checklist is still needed for go-live (see above).

## Sonos hardware (optional)

There is no CI script for the real driver. Manually:

```bash
cp config.example.json config.json   # set token + speaker IPs
make dev
curl -s http://localhost:8080/api/v1/health | jq .
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/scenes
```

Discovery, stream start and AVTransport status against real speakers: Record deviations in an issue.

## Dependencies

- **Go tests:** Go toolchain only
- **Shell scripts:** `bash`, `curl`; optional `jq`
- **GUI layout:** Node.js 22+, `npm ci`, Playwright Chromium (CI/dev only, not runtime)

## See also

- [ADR 0004 — Test gate before commit](../adr/0004-test-gate-before-commit.md)
- [Run locally](local-start.md)
- [REST API](../reference/api.md)
