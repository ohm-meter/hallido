# ADR 0008: Config sync on the Pi and scene import from Sonos

## Status

Accepted, partially implemented (as of 2026-10-14): speaker sync API (`POST /api/v1/config/speakers/sync`), the `config.json.bak` backup, and scene import from favorites (`from-favorite`) are in `master`; still open: CLI subcommand, merge modes `replace`/`Prune`. Plan: [feat-pi-config-sync-sonos-import.md](../plans/feat-pi-config-sync-sonos-import.md)

## Context

- ADR 0006 increment C explicitly ruled out **automatically overwriting `config.json`** and **generating scenes from the Sonos app**.
- ADR 0007 introduces token-protected config writing via the REST API (branch `feat/gui-scene-builder`).
- User request: set up the Pi without manual config maintenance; derive scenes from Sonos favorites/groups.

## Decision

Both topics are **back in scope**, but in a controlled way:

### Config sync (P1)

- Write only through the existing API (`internal/config/store.go`: temp + `rename`).
- Discovery result → speakers block; **merge strategy** instead of a blind full replace (details in the implementation).
- **Backup** of the existing file before every write; optional opt-in flag.
- No token in the repo; the config path still comes only from `-config` / the install path on the device.

### Scene import (P2)

- Depends on **increment 3** (ADR 0005): ContentDirectory `FV:2` against hardware.
- Import returns **suggestions** (`GET` endpoints); persistence only after a user/API action (`POST /api/v1/config/scenes`).
- No silent overwriting of existing scenes.

## Consequences

- The "Non-goals" section / increment C of ADR 0006 is **superseded** for these points (documented for history).
- Prioritization in `docs/todos.md`: config sync P1, scene import P2.
- The GUI scene editor (ADR 0007) is the natural entry point for import flows.

## Open (before acceptance)

- [ ] Merge rules for speaker sync (new vs. replace vs. IP overrides)
- [ ] Install/update hook vs. API-only use
- [ ] Hardware validation of ContentDirectory / favorites mapping
