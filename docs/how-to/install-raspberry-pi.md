# How-to: Install on a Raspberry Pi

Problem: Run Hallido permanently on your LAN — with the real Sonos driver,
token protection, and systemd. Applies equally to other Linux machines with systemd.

## Prerequisites

- Raspberry Pi (3/4/5/Zero 2 W) with Raspberry Pi OS, 64-bit recommended; 32-bit (`armv7l`) works too
- Pi and Sonos speakers **on the same LAN/subnet** (no guest Wi-Fi, no VLAN boundary in between)
- SSH access to the Pi, `sudo` rights
- `git` and **Go 1.22+** on the Pi (alternatively build the binary on another machine, see below)

## 1. Install packages

```bash
sudo apt update
sudo apt install -y git golang
go version   # at least go1.22
```

If the `golang` package is too old, extract Go from <https://go.dev/dl/> to
`/usr/local/go`. The installer finds `/usr/local/go/bin` even under `sudo`
(which often resets `PATH`).

## 2. Run the installer

```bash
curl -fsSLO https://raw.githubusercontent.com/ohm-meter/hallido/master/scripts/install.sh
less install.sh                        # skim it first
sudo bash install.sh --system --systemd
```

Alternatively from a clone (permanent path, not `/tmp`):

```bash
git clone https://github.com/ohm-meter/hallido.git ~/hallido
cd ~/hallido
sudo ./scripts/install.sh --system --systemd
```

The installer detects the architecture (on 32-bit it builds with
`make build-pi PI_ARCH=arm PI_ARM=7`) and sets up:

- Binary: `/usr/local/sonos-agent/bin/sonos-radio-agent`
- Config: `/var/lib/sonos-agent/config.json` (never overwritten on updates; the only path the service may write to)
- CLI: `/usr/local/bin/sonos-agent`
- systemd unit: `/etc/systemd/system/sonos-agent.service` (user `sonos`, driver `sonos`)

Details on options and paths: [Install with the script](install-one-liner.md).

## 3. Set the token and scenes

The installer asks interactively for a token (empty = generate a random one).
Otherwise set it yourself:

```bash
openssl rand -hex 24                         # generate a token
sudo nano /var/lib/sonos-agent/config.json # enter api_token, create scenes
```

Better than plain text in the config: store the token in the unit.

```bash
sudo systemctl edit sonos-agent
```

```ini
[Service]
Environment=SRA_API_TOKEN=your-long-random-token
```

Scenes and speaker names: [Create a scene](create-scene.md),
[Speaker config](speaker-config-and-refresh.md), reference: [Configuration](../reference/config.md).
You can also edit scenes later in the web app.

## 4. Start the service

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now sonos-agent
sudo systemctl status sonos-agent
```

## 5. Check reachability

On the LAN the Pi is usually reachable via mDNS at `http://<HOSTNAME>.local:8080`
(example hostname `raspberrypi`), or via its IP.

```bash
curl http://raspberrypi.local:8080/api/v1/health
curl -H "Authorization: Bearer $TOKEN" http://raspberrypi.local:8080/api/v1/speakers
```

Web app in the browser: `http://raspberrypi.local:8080`, enter the token.

## Network, firewall, SSDP

- **SSDP/UPnP** (multicast, UDP 1900) must work on the LAN. If it doesn't,
  enter fixed IPs under `speakers` — [Troubleshoot discovery](troubleshoot-discovery.md).
- Open port **8080/tcp** on the LAN only, never forward it to the internet.
  Access from outside: [Remote access](remote-access.md).

## Update

```bash
sonos-agent update        # git pull, rebuild, restart the service
```

Back up `config.json` first: `sudo cp /var/lib/sonos-agent/config.json ~/config.json.bak`.

Installations from before the separated layout (config under
`/usr/local/sonos-agent/`) are converted automatically by `update` —
[Migrate the installation layout](install-layout-migration.md).

## Without Go on the Pi

Build on another machine and copy:

```bash
make build-pi                        # arm64; 32-bit: make build-pi PI_ARCH=arm PI_ARM=7
scp build/sonos-radio-agent-pi pi@raspberrypi.local:/tmp/sonos-radio-agent
```

On the Pi, put the binary at `/usr/local/sonos-agent/bin/sonos-radio-agent`
and run `sudo ./scripts/install.sh --system --systemd --no-build` in the clone.

## Rollback

1. `sudo systemctl stop sonos-agent`
2. Copy the previous binary back to `/usr/local/sonos-agent/bin/sonos-radio-agent`
   (or `git checkout <commit>` in the clone and rebuild)
3. Restore `config.json` from the backup
4. `sudo systemctl start sonos-agent`

## Go-live checklist

- [ ] `api_token` set or `SRA_API_TOKEN` in the unit (no placeholder, ≥ 24 random characters)
- [ ] `SRA_DRIVER=sonos` in the unit (default in the bundled unit)
- [ ] `GET /api/v1/health` reports the expected speakers
- [ ] `GET /api/v1/speakers` lists all names the scenes use
- [ ] Test scene plays successfully via the web app or `POST /api/v1/scenes/{id}/play`
- [ ] Backup of `config.json` created
- [ ] Port 8080 not reachable from the internet
- [ ] Token and config **not** committed to a Git repo

## See also

- [Operations & troubleshooting](../runbook.md)
- [Run with Docker](run-with-docker.md)
- [Use the Sonos driver](use-sonos-driver.md)
- [Homey integration](homey-integration.md)
