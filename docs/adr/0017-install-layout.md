# ADR 0017: Install layout — code owned by root, data in /var/lib/sonos-agent

## Status

Accepted (2026-10-15)

## Context

Until now, `scripts/install.sh --system --systemd` put everything under
`/usr/local/sonos-agent` — the Git clone `src/` (with `Makefile`), `bin/`,
`.install-meta`, and `config.json` — and handed the tree to the service user
`sonos` via `chown -R`. The systemd unit allowed `ReadWritePaths` on the whole
directory, and `scripts/sonos-agent-cli.sh` ran `.install-meta` via `source`
and, on `update`, built in the tree as admin (with `sudo`).

Consequence (security review 2026-10-15, defense in depth): anyone who can run
code as `sonos` (a hole in the agent) can modify `.install-meta`, the
`Makefile`, or Git hooks and thereby have code run as admin or root on the next
`sonos-agent update`. The service only needs write access to its config
(`config.json`, backup `config.json.bak`, temp files during atomic writes — all
in the config's directory, `internal/config/store.go`).

## Decision

1. **Separate layout for the system installation**

   | Path | Contents | Owner |
   |------|----------|-------|
   | `/usr/local/sonos-agent/` | `bin/`, `src/` (remote install only), `.install-meta`, `config.example.json` | `root`, not group/world-writable |
   | `/var/lib/sonos-agent/` | `config.json`, `config.json.bak`, `.config-*.tmp` | `sonos:sonos`, `0750`, files `0600` |

   The unit sets `StateDirectory=sonos-agent` (`StateDirectoryMode=0750`),
   `WorkingDirectory` and `-config` point there; `ReadWritePaths` for the code
   is dropped — with `ProtectSystem=strict`, `/usr` is thus read-only for the
   service. No Go code is affected: the store already writes the temp file and
   `.bak` next to the config.

2. **`.install-meta` is data, not a script.** The CLI reads only known keys as
   `KEY=VALUE` (values verbatim, no expansion) and loads the file only if the
   file **and** directory belong to root or the calling user and are not
   writable by others. Otherwise it aborts with a hint to run
   `sudo sonos-agent migrate`.

3. **Migration as the CLI command `sonos-agent migrate`** (root), called by
   `install.sh` (system mode) and automatically by `sonos-agent update` when the
   old layout is detected:
   - Stop the service (if active), create `/var/lib/sonos-agent`, move
     `config.json`/`.bak` (`sonos`, `0600`), delete temp files
   - Symlinks instead of files are not carried over (no `chown` through a
     link planted by the service onto foreign files)
   - A service-owned clone under `src/` is checked beforehand for Git hooks and
     executable `.git/config` entries; if any are found → abort without changes.
     The remote installer sets such a clone aside and clones anew.
   - `REPO_DIR` from the old (service-owned) `.install-meta` applies only if the
     directory belongs to root or the `sudo` caller (Pi: `~/hallido`)
   - `/usr/local/sonos-agent` → `root`, `go-w`; rewrite `.install-meta`
     (`LAYOUT=2`, `DATA_DIR`)
   - **Rewrite the unit instead of replacing it** (custom lines such as
     `Environment=SRA_API_TOKEN=…` are kept), backup `…service.pre-migrate`
   - `daemon-reload`, restart the service if it was running; idempotent

   The installer no longer overwrites an existing unit.

4. **User mode unchanged** (`~/.local/share/sonos-agent`, config next to it).

## Consequences

- A compromised service can change neither the code, the binary, nor the
  metadata the admin uses on `update`; it can only write its own config.
- Remote installation (clone under `/usr/local/sonos-agent/src`, owned by root):
  `sudo sonos-agent update`. Own clone in the home directory (as on the Pi):
  still without `sudo`; only the binary, CLI, and metadata are replaced via
  `sudo install`.
- `config.json` is readable by the admin only with `sudo`; `sonos-agent doctor`
  reports this as OK instead of "missing".
- The first update run on an old system still uses the old CLI (it only installs
  the new one); the switch happens on the next `sonos-agent update` or
  immediately via `sudo sonos-agent migrate`.
- Rollback: [Migrate the install layout](../how-to/install-layout-migration.md#rollback).
- Tests: `scripts/test/install-layout.sh` (part of `make test`),
  `scripts/test/install-migration-docker.sh` (real paths in a container).
