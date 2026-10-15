# How-to: Run locally (dev and prod simulation)

Problem: start the agent without running `SRA_DRIVER=sonos make run` by hand.

| Script / Make | Behavior |
|---------------|----------|
| `make dev` / `scripts/dev.sh` | `go run`, Sonos driver, `-debug`, stop/restart |
| `make prod-local` / `scripts/prod.sh` | Binary built to `build/`, no debug |
| `scripts/start.sh dev\|prod` | Wrapper → `dev.sh` or `prod.sh` |

Shared logic: `scripts/common.sh`. Overview: [project-structure.md](../explanation/project-structure.md).

## Prerequisites

- Go 1.22+
- `config.json` (copied from `config.example.json` if missing)
- Optional: `.env` from `.env.example` (driver, token)

## Development (Sonos + debug)

```bash
make dev
# or: ./scripts/dev.sh
# or: ./scripts/start.sh dev
```

- Driver: `SRA_DRIVER=sonos` (override via `.env` or the environment)
- `-debug` flag (verbose logging)
- Checks: `config.json` exists, the port from `listen` is free (default 8080)
- Start: `go run ./cmd/server`

GUI: http://localhost:8080

### Restart after a code change

```bash
./scripts/dev.sh restart
# or: ./scripts/dev.sh stop && ./scripts/dev.sh start
# or: ./scripts/start.sh dev restart
```

- `stop` ends the agent via `.run/sonos-agent-dev.pid` or `lsof` on the `listen` port
- `restart` stops first, also kills a stale `go run` on the port, then starts again

## Local prod simulation

```bash
make prod-local
# or: ./scripts/prod.sh
# or: ./scripts/start.sh prod
```

- Builds the binary to `build/sonos-radio-agent`
- No `-debug`
- Same config/port checks as dev

## Mock driver (no hardware)

```bash
SRA_DRIVER=mock make run
```

Or set `SRA_DRIVER=mock` in `.env` and run `./scripts/dev.sh`.

## Real production (Pi)

The scripts are for **local development**. On the Raspberry Pi:

1. `sudo ./scripts/install.sh --system --systemd`
2. `sudo systemctl enable --now sonos-agent`

See [Raspberry Pi go-live](install-raspberry-pi.md), the [runbook](../runbook.md) and `deploy/systemd/sonos-agent.service`.

## Token

- Set it in `config.json` (`api_token` ≠ `BITTE-AENDERN`)
- Or set `SRA_API_TOKEN` in `.env` / the shell (takes precedence)

## Port in use

- `./scripts/dev.sh stop` or `./scripts/dev.sh restart` — ends your own dev agent
- Otherwise `start` aborts and shows the blocking process (`lsof`)
- Alternative: change `listen` in `config.json`

## See also

- [Use the Sonos driver](use-sonos-driver.md)
- [First run tutorial](../tutorials/first-run-local.md)
- [Open items](../todos.md)
