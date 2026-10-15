# How-to: Install with the script (`scripts/install.sh`)

Problem: Install the agent on a Mac or Linux machine with a single command
and update it later.

## In short

```bash
curl -fsSLO https://raw.githubusercontent.com/ohm-meter/hallido/master/scripts/install.sh
less install.sh                          # read it first
bash install.sh                          # user mode (no root)
sudo bash install.sh --system --systemd  # Linux service (e.g. Raspberry Pi)
```

If you prefer `curl | bash`:

```bash
curl -fsSL https://raw.githubusercontent.com/ohm-meter/hallido/master/scripts/install.sh | bash -s -- --system --systemd
```

The script:

- detects the OS/architecture (darwin/linux, arm64/amd64/armv7)
- clones the repo to `INSTALL_ROOT/src` (source overridable with `SRA_REPO_URL`, e.g. for a fork)
- builds the binary with `make build` or `make build-pi`
- installs the CLI, config template, and optionally the systemd unit
- writes `.install-meta` for updates

## Options

| Option | Effect |
|--------|--------|
| `--user` | Default without root: `~/.local/share/sonos-agent/`, CLI in `~/.local/bin` |
| `--system` | Default with root: `/usr/local/sonos-agent/`, CLI in `/usr/local/bin` |
| `--systemd` | Install the `sonos-agent.service` unit (Linux, requires `--system`) |
| `--no-build` | Don't build; use the existing binary at `…/bin/sonos-radio-agent` |

## After installation

1. **Set the token** — `api_token` in the config or `SRA_API_TOKEN` (not `BITTE-AENDERN`); generate one with `openssl rand -hex 24`
2. **Driver** — for real speakers, `SRA_DRIVER=sonos` (the systemd unit already sets it)
3. **Verify:**

```bash
sonos-agent version
sonos-agent doctor    # config, binary, port
sonos-agent status    # GET /api/v1/health
```

Config:

- User: `~/.local/share/sonos-agent/config.json`
- System: `/var/lib/sonos-agent/config.json`

Layout of the system installation: code, binary, clone, and `.install-meta` are
owned by root under `/usr/local/sonos-agent/`; the service user `sonos` may
write only to `/var/lib/sonos-agent/` (config, backup, temp files). This way a
compromised service can change neither the code nor the metadata that
`sonos-agent update` uses as admin ([ADR 0017](../adr/0017-install-layout.md)).
The installer or `sonos-agent update` converts older installations
automatically: [Migrate the installation layout](install-layout-migration.md).

## Update

```bash
sonos-agent update               # git pull, rebuild, replace binary/CLI, restart the service
sonos-agent update --no-restart  # without restart (maintenance window)
```

`config.json` is overwritten neither on first install nor on `update`.
Back it up before larger updates anyway.

`update` needs the Git clone from `.install-meta` and **Go 1.22+** on the
host. If the installer did the cloning (`/usr/local/sonos-agent/src`,
owned by root), run the update with `sudo sonos-agent update`; for your own
clone in your home directory, run it without `sudo`. Without Go: build the binary elsewhere (`make build-pi`), copy it to
`…/bin/sonos-radio-agent`, and restart the service.

## Requirements

- **git**, **Go 1.22+** (for the build), **curl** (for `sonos-agent status`)
- Linux with `--systemd`: root/sudo

## Security

- No secrets in the installer; the token is written only locally into the config
- `set -euo pipefail`, fixed installation paths only
- It installs the current state of `master` — for a fixed state, clone a
  tag or commit and run `./scripts/install.sh` locally
- More notes: [SECURITY.md](../../SECURITY.md)

## See also

- [Install on a Raspberry Pi](install-raspberry-pi.md)
- [Start locally (dev)](local-start.md)
- [Operations & troubleshooting](../runbook.md)
