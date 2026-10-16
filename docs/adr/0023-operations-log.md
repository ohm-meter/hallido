# ADR 0023: Setup and operations log

## Status

Accepted (2026-10-15)

## Context

NAS users cannot read the container log comfortably, and support questions
need a traceable record of what Hallido did (setup, scene starts, speakers
offline). The log must never contain secrets.

## Decision

- `internal/applog` wraps `log/slog`: every record still goes to stdout
  (journald, container log). Records at level **Info and above** are also
  kept in memory (last 500) and, if `-log-dir` / `SRA_LOG_DIR` is set
  (container: `/data/logs`), appended as JSON lines to `hallido.log`.
- **Rotation:** 1 MiB per file, three old files (`hallido.log.1` … `.3`),
  i.e. at most about 4 MiB. Files and directory are `0600`/`0700`.
- **Redaction:** attributes whose key contains `token`, `password`,
  `passwd`, `secret`, `authorization`, `cookie`, `api_key` or `apikey` are
  replaced by `[redacted]` — in the file, in memory and on stdout. Values are
  cut at 500 characters. The setup handler logs only time and client address.
- After a restart the memory buffer is filled from the current file, so the
  setup stays visible.
- `GET /api/v1/logs?limit=N` (token required, max. 500) returns the entries;
  the web app shows them under **Speakers → Log**, newest first.
- An unusable log directory (read-only, full) is not fatal: logging continues
  to stdout and memory, with a warning.

## Consequences

- The log contains client IP addresses of the home network (setup) and
  speaker/scene names. It stays on the NAS; documented in
  `docs/explanation/what-hallido-does.md`.
- The Pi does not set `-log-dir`; its log view shows entries since the last
  start (journald keeps the full history).
