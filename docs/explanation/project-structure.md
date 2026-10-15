# Explanation: What lives where

Quick overview of the repository — for getting started and tidying up. Runtime architecture: [architecture.md](architecture.md).

## Start hier

| Goal | Start here |
|------|----------|
| Develop locally | `make dev` → [local-start.md](../how-to/local-start.md) |
| First time / tutorial | [first-run-local.md](../tutorials/first-run-local.md) |
| Pi in production | [install-one-liner.md](../how-to/install-one-liner.md) → [install-raspberry-pi.md](../how-to/install-raspberry-pi.md) |
| Docs index | [docs/README.md](../README.md) |
| Roadmap | [todos.md](../todos.md) (details: `AGENTS.md` § Open / Roadmap) |
| Contributors | [AGENTS.md](../../AGENTS.md) |

## Code

| Path | Contents |
|------|--------|
| `cmd/server/` | Binary entry point, driver selection (`--driver=mock\|sonos`) |
| `internal/api/` | REST handlers, OpenAPI counterpart: `docs/api/openapi.yaml` |
| `internal/scenes/` | Load scenes, play/stop/pause orchestration |
| `internal/schedule/` | Scheduler for schedules (ADR 0013) |
| `internal/radio/` | Radio Browser client (ADR 0011) |
| `internal/speaker/` | `SpeakerController`-Interface; `mock/`, `sonos/` |
| `internal/config/` | Load/save `config.json` |
| `internal/web/static/` | GUI (HTML/CSS/JS, no frameworks), web app manifest, service worker (`sw.js`, ADR 0016), app icons (`icons/`) |
| `internal/apperr/` | Stable error codes for the API |

## Configuration & deployment

| Path | Contents |
|------|--------|
| `config.example.json` | **Canonical** template for `config.json` (gitignored) |
| `docs/reference/config.md` | Field descriptions, no second example config |
| `.env.example` | Optional: `SRA_DRIVER`, token override |
| `deploy/` | systemd, install scripts for the Pi |
| `Makefile` | `make dev`, `make test`, `make build-pi`, … |

## Scripts (local)

All of them use `scripts/common.sh` (config, port, binary path).

| Script | Role |
|--------|--------|
| `scripts/dev.sh` | Development: `go run`, Sonos driver, `-debug`, stop/restart |
| `scripts/prod.sh` | Local production simulation: build + start the binary |
| `scripts/start.sh` | Wrapper: `start.sh dev` → `dev.sh`, `start.sh prod` → `prod.sh` |
| `scripts/install.sh` | One-liner installation (curl \| bash) |
| `scripts/sonos-agent-cli.sh` | CLI: `update`, `doctor`, `status`, `migrate`, … |
| `scripts/test/` | Shell API/GUI tests (`make test-scripts`), install layout (`make test-install`, `make test-install-docker`) |

Details and Makefile aliases: [local-start.md](../how-to/local-start.md).

## Documentation (Diátaxis)

| Path | Type | Note |
|------|-----|---------|
| `docs/tutorials/` | Learning path | One tutorial: first run |
| `docs/how-to/` | Recipes | One file per problem |
| `docs/reference/` | Reference | `config.md`, `api.md`; Homey: **`api-homey.md`** (table) |
| `docs/how-to/homey-integration.md` | How-to | Flow examples; points to the reference |
| `docs/explanation/` | Explanation | Architecture, this file |
| `docs/adr/` | Decisions | Numbered; do not duplicate with how-tos |
| `docs/plans/` | Implementation plans | Before implementation; linked from `todos.md` |
| `docs/todos.md` | Roadmap | Priorities; `AGENTS.md` has only a short open list |
| `docs/api/openapi.yaml` | API contract | Source of truth for REST |
| `docs/runbook.md` | Pi operations | After go-live |
| `docs/styleguide.md` | Web-GUI | Tokens, a11y |

## Agent & quality

| Path | Contents |
|------|--------|
| `.githooks/` | Pre-commit: tests |
| `.github/workflows/` | CI |

