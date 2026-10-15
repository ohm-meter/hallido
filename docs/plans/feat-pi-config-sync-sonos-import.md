# Implementation plan: Pi config sync & Sonos scene import

**Branch:** done/merged into `master` (plan carried over from the old branch)  
**ADR:** [`docs/adr/0008-config-sync-and-scene-import.md`](../adr/0008-config-sync-and-scene-import.md)  
**Status (2026-10-14):** Backup (`.bak`) and `POST /api/v1/config/speakers/sync` implemented; open: merge modes `replace`/`Prune`, CLI `sonos-agent config sync-speakers`.

## Prerequisites (before phase 1)

- [ ] Review branch `feat/gui-scene-builder` and merge it into `master` (ADR 0007)
- [ ] Rebase `feat/pi-config-sync-sonos-import` onto the current `master`
- [ ] Hardware smoke test: discovery + stream on a real speaker (in parallel with P1 in todos)

---

## Phase 1 — Pi speaker sync

### 1.1 Config store: backup

| # | Task | File |
|---|------|------|
| 1 | `BackupBeforeSave(path string) error` — copy to `path + ".bak"`, chmod 0600 | `internal/config/store.go` |
| 2 | Call it in `SaveAtomic` before the temp write (config flag `sync.backup_before_write`, default true) | `internal/config/store.go` |
| 3 | Tests: backup created, missing source, restore documented manually | `internal/config/store_test.go` |

### 1.2 Extend merge

| # | Task | File |
|---|------|------|
| 4 | `SpeakerSyncMode` (`merge`, `replace`) + optional `Prune bool` | `internal/config/store.go` |
| 5 | `MergeSpeakersFromDiscovery` → mode/prune; with `replace` the discovery set replaces the existing one, prune deletes missing entries | `internal/config/store.go` |
| 6 | Request body `{ "speakers", "mode", "prune" }` in the handler | `internal/api/config_handlers.go` |

### 1.3 Server-side discovery sync

| # | Task | File |
|---|------|------|
| 7 | `POST /api/v1/config/speakers/sync-from-discovery?refresh=true` or body `"use_discovery": true` | `internal/api/config_handlers.go`, `server.go` |
| 8 | Flow: optional `ctrl.Refresh`/ListSpeakers → merge → backup → SaveAtomic | `internal/api/config_handlers.go` |
| 9 | Rate limit (existing pattern) | `internal/api/server.go` |
| 10 | Handler tests: mock + missing discovery | `internal/api/config_test.go` |

### 1.4 CLI

| # | Task | File |
|---|------|------|
| 11 | Env `SRA_CONFIG_PATH` in the CLI + `load_meta` fallback | `scripts/sonos-agent-cli.sh` |
| 12 | Subcommand `config sync-speakers [--mode merge\|replace] [--prune] [--via-api]` | `scripts/sonos-agent-cli.sh` |
| 13 | `--via-api`: HTTP POST with the token from config/env; otherwise a hint to stop the service | `scripts/sonos-agent-cli.sh` |
| 14 | `sonos-agent update --sync-speakers` (opt-in) | `scripts/sonos-agent-cli.sh` |
| 15 | `doctor`: checks that the config is writable and that `.bak` exists after a sync | `scripts/sonos-agent-cli.sh` |

### 1.5 Config schema (optional, phase 1)

| # | Task | File |
|---|------|------|
| 16 | Struct `SyncConfig` in `Config` (`speakers_enabled`, `backup_before_write`) | `internal/config/config.go` |
| 17 | `config.example.json` + `docs/reference/config.md` | various |

### 1.6 Docs & API

| # | Task | File |
|---|------|------|
| 18 | OpenAPI: extended speaker-sync parameters, new endpoint if separate | `docs/api/openapi.yaml` |
| 19 | How-to: first start on the Pi, backup/restore, CLI | `docs/how-to/pi-config-sync.md` (new) |
| 20 | Link `install-raspberry-pi.md` | `docs/how-to/install-raspberry-pi.md` |
| 21 | CHANGELOG `[Unreleased]` | `CHANGELOG.md` |

### Phase 1 done criterion

`make test` green; manually: discovery → sync → `.bak` exists → restart hint visible.

---

## Phase 2 — Export favorites (read-only)

### 2.1 Sonos ContentDirectory

| # | Task | File |
|---|------|------|
| 22 | **Hardware spike:** browse `FV:2` on a real speaker, save DIDL samples | `docs/how-to/` or `.agent/` |
| 23 | `BrowseFavorites(ctx, coordinatorIP) ([]Favorite, error)` | `internal/speaker/sonos/contentdirectory.go` (new) |
| 24 | SOAP mock for browse in tests | `internal/speaker/sonos/*_test.go` |
| 25 | Interface extension `SpeakerController` or a narrower `FavoritesReader` | `internal/speaker/controller.go` |

### 2.2 API

| # | Task | File |
|---|------|------|
| 26 | `GET /api/v1/sonos/favorites` | `internal/api/sonos_import_handlers.go` (new) |
| 27 | Mock driver: fixed test favorites | `internal/speaker/mock/mock.go` |
| 28 | OpenAPI + tests | `docs/api/openapi.yaml`, `internal/api/*_test.go` |

### Phase 2 done criterion

Hardware-validated favorites list via the API; no persistence.

---

## Phase 3 — Group scene suggestions

| # | Task | File |
|---|------|------|
| 29 | `BuildSceneSuggestions(ctx)` from discovery + groups + now playing | `internal/scenes/import.go` (new) |
| 30 | `GET /api/v1/sonos/scene-suggestions` | `internal/api/sonos_import_handlers.go` |
| 31 | Map source → `stream` / `favorite`; URI normalization | `internal/scenes/import.go` |
| 32 | GUI: import panel, checkbox selection, "Save to config" | `internal/web/static/app.js`, `index.html` |
| 33 | Confirmation on ID collision | GUI + API |
| 34 | How-to scene import | `docs/how-to/sonos-szenen-import.md` (new) |

### Phase 3 done criterion

The user sees suggestions, picks a scene, `POST /api/v1/config/scenes` — scene is playable.

---

## Phase 4 — Optional auto-merge (background)

| # | Task | File |
|---|------|------|
| 35 | Background goroutine when `sync.speakers_enabled` + interval | `cmd/server/main.go` or `internal/config/sync.go` |
| 36 | systemd timer `deploy/systemd/sonos-agent-sync-speakers.timer` | `deploy/systemd/` |
| 37 | Doctor warning if enabled without backup | `scripts/sonos-agent-cli.sh` |

---

## Order for agents

1. Merge ADR 0007 (`feat/gui-scene-builder`)
2. Phase 1.1 → 1.3 → 1.4 → 1.6 (backup, API, CLI, docs)
3. Increment 3 / ADR 0005 in parallel or earlier (hardware)
4. Phase 2 after hardware validation of FV:2
5. Phase 3 (GUI synergy)
6. Phase 4 only if needed

## Not part of this feature

- Starting Spotify playback
- Sonos Cloud
- Automatic full scene import without a UI
