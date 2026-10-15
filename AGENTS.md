# AGENTS.md

Instructions for every (AI) agent and human working on this project. Follow
these rules so the code stays consistent and traceable across many
contributions.

## Ground rules

- **English is the project language** ([ADR 0019](docs/adr/0019-english-project-language.md)):
  docs, ADRs, commit messages, new code comments, web-app texts and API
  messages. Older German code comments are translated when you touch the code.
  Terms and UI labels (English + German translation for the web app):
  [`docs/reference/glossary.md`](docs/reference/glossary.md).
- **Product name: Hallido** ([ADR 0020](docs/adr/0020-product-name-hallido.md)) — never use "Sonos" in the product name; binary/CLI/service/paths keep `sonos-agent` for now.
- **One backend language: Go.** Do not introduce a second backend language.
  Rationale in `docs/adr/0001-language-go.md`.
- **Prefer the standard library.** External dependencies only for a good
  reason and documented as an ADR. Runtime: stdlib + **one external HTTP API**
  (Radio Browser, ADR 0011) — no additional Go module.
- **Small, focused changes.** One commit = one topic.
- **Commit only with green tests.** `make test` must pass, otherwise no
  commit. Changes under `internal/web/static/*`: additionally `make test-gui`
  (static checks + Playwright layout, three viewports). The pre-commit hook
  (`.githooks/`) enforces this for staged GUI files. Conventional Commits
  (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`).
- **Record decisions as ADRs.** Every architecture or technology decision gets
  a new file in `docs/adr/`.
- **No secrets in code.** Tokens/keys only via config file or environment
  variable.

## Architecture in one sentence

The GUI and external tools (Homey) use the same **REST API** (`internal/api`);
the API uses the **scene service** (`internal/scenes`), which the **scheduler**
(`internal/schedule`) also calls for schedules; the service controls speakers
exclusively through the **SpeakerController interface** (`internal/speaker`).
Concrete drivers (mock and Sonos) sit behind this interface and are selected in
`cmd/server/main.go` (`buildController`, `--driver=mock|sonos`).

## Extending — typical cases

- **New scene:** configuration only (`config.json` or web app), no code.
- **New speaker driver (e.g. another brand):** new package under
  `internal/speaker/<name>` implementing `SpeakerController`; wire it up in
  `buildController`. Test it against a matching mock (example:
  `internal/speaker/sonos`, tested with an HTTP test server).
- **New API endpoint:** handler in `internal/api`, route in `Handler()`, update
  the OpenAPI spec in `docs/api/openapi.yaml`, add a handler test.

## Error handling

Every error that leaves the agent is an `*apperr.Error` with a stable code.
Devices can be offline — methods that talk to a device get a context with a
timeout and return `speaker_unreachable` or similar with a concrete hint. A
partial failure (one of several speakers offline) is NOT a hard error: the
other speakers play, and the failure is reported per speaker.

## Tests

Backend: `go test ./...`, entirely without hardware (mock driver). Every new
code path gets a test, error paths included. The real Sonos driver is tested
against a SOAP mock (HTTP test server), never against real speakers.

**Web GUI** (`internal/web/static/`): mobile-first (tab bar, bottom sheets,
touch targets ≥ 44 px). New or changed UI (sheets, tabs, responsive layout)
needs Playwright layout tests in `tests/gui/` — add new sheets/tabs to
`touch-targets.spec.js` — and a green `make test-gui` (mobile 390×844, tablet
820×1180, desktop 1280×800). CI and pre-commit run it for staged GUI files.
Details: `docs/how-to/testing.md`, `docs/styleguide.md`.

## Current state

**Increment 1 (done):** architecture, scene model, REST API with token
protection, GUI (playback only), mock driver, deployment (systemd/Docker), CI
and pre-commit gate. Runs and is tested entirely without hardware.

**Increment 2 (done):** real Sonos driver (`internal/speaker/sonos`) over local
UPnP/SOAP, enabled with `--driver=sonos`. Implemented:
- SSDP discovery + parsing of the device description, fixed-IP fallback from config
- lazy discovery when resolving unknown speakers
- grouping via `x-rincon`, `SetVolume`, **stream playback**
- all SOAP arguments XML-escaped (injection protection)
- structured errors: `speaker_unreachable` (speaker offline) vs.
  `playback_failed` (speaker refuses), each with a hint
- tests against an HTTP-simulated Sonos (no real speaker needed)

**Increment 3 (done, October 2026):**
- **Schedules** (`internal/schedule`, ADR 0013): start/stop scenes on a timetable — group alarms the Sonos app cannot do; catch-up after restart
- **Group control** (ADR 0015): stop/pause/resume/ungroup per scene or per speaker (acts on the current Sonos group), `GET /api/v1/groups`; GUI: pause/resume per scene (mini player, ⋯ menu), Speakers tab grouped with pause/stop, “Leave group” in the speaker detail
- **Code audit 2026-10** implemented (data race, rate limit `rate_limited`, `.bak` backup, single install path)
- **Install layout** (ADR 0017): code root-owned in `/usr/local/sonos-agent`, config in `/var/lib/sonos-agent`
- **Stream probe + check after play** (#17): AAC as `aac://`, play without `PLAYING` → `playback_failed` with hint — `docs/how-to/troubleshoot-stream-not-playing.md`

No external module, one static binary. Pi build: `make build-pi`
(arm64; 32-bit via `make build-pi PI_ARCH=arm PI_ARM=7`); all platforms: `make dist`.


## Open / roadmap

Canonical list: [`docs/todos.md`](docs/todos.md). Repository overview:
[`docs/explanation/project-structure.md`](docs/explanation/project-structure.md).


## Conventions checklist before every commit

1. `make test` green? (vet + all tests) — otherwise no commit.
2. `internal/web/static/*` changed? → `make test-gui` green (layout tests).
3. New code paths incl. error paths tested?
4. Decision made? → ADR in `docs/adr/`.
5. API changed? → `docs/api/openapi.yaml` updated.
6. State/open items kept current here in AGENTS.md.
