# Hallido

*Scenes & alarms for Sonos — Powered by siraris*

[![CI](https://github.com/ohm-meter/hallido/actions/workflows/ci.yml/badge.svg)](https://github.com/ohm-meter/hallido/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ohm-meter/hallido?sort=semver)](https://github.com/ohm-meter/hallido/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/ohm-meter/hallido)](go.mod)
[![License: MIT](https://img.shields.io/github/license/ohm-meter/hallido)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%C2%B7%20Raspberry%20Pi%20%C2%B7%20macOS%20%C2%B7%20Docker-ff9d2f)](docs/how-to/install-linux.md)

**Group alarms, saved speaker groups and one-tap radio for Sonos — running
locally on your own Linux box, no cloud account required.**

<p align="center">
  <img src="docs/demo/hallido-demo.gif" alt="Demo: starting a scene, per-room volume, schedules and groups in the Hallido web app" width="720">
</p>
<p align="center"><a href="docs/demo/hallido-demo.mp4">Watch the demo as video (MP4)</a></p>

Hallido is a small self-hosted service that controls your Sonos speakers
directly in your home network (UPnP/SOAP). One static Go binary with a built-in
web app — it runs on **any Linux machine** (PC, server, NAS, Raspberry Pi;
x86_64 and ARM), on macOS or in Docker.

## The missing Sonos feature: alarms for groups

The Sonos app sets an alarm for **one speaker**. To wake up to music in several
rooms you have to group the speakers by hand the night before and enable
“Include grouped rooms” — and the alarm falls back to a single room as soon as
the group was split (TV on, Wi-Fi hiccup, someone used the app). There are no
saved groups and only one volume for the whole alarm.

Hallido closes that gap. A **schedule** starts a **scene**, and a scene is a
saved group:

- **Builds the group fresh every time** — Kitchen, Bathroom and Living Room
  play together at 6:10, no matter how the speakers were grouped last night.
- **Volume per room** — quiet in the bathroom, a bit louder in the kitchen.
- **Stop time** — e.g. Monday to Friday 06:10–08:30; it only stops the music
  if your scene is still playing (it leaves alone whatever you switched to).
- **Catch-up after a restart** — if the agent was down at start time, it
  starts the scene as soon as it is back — as long as the window is still open
  and nothing else is playing.

```json
{ "id": "wake-up-weekdays", "scene": "morning-radio",
  "days": ["mon", "tue", "wed", "thu", "fri"], "start": "06:10", "stop": "08:30" }
```

Set it up in the **Schedules** tab of the web app or in `config.json` —
see [Schedules](docs/how-to/schedules.md).

## Features

- **Scenes:** named presets — which speakers, which station, which volume per
  speaker. One tap groups the speakers and starts the stream.
- **Schedules (group alarms):** start and stop scenes at fixed times, see above.
- **Group control:** stop, pause, resume, ungroup — per scene or per speaker,
  also for music started from the Sonos app.
- **Radio search:** find stations in the free
  [Radio Browser](https://www.radio-browser.info/) directory and save them as
  scenes.
- **Web app:** mobile-first UI with tabs for scenes, schedules and speakers;
  installable on phones and desktops.
- **REST API:** the same features for home automation (Homey, Home Assistant,
  curl); OpenAPI contract in [`docs/api/openapi.yaml`](docs/api/openapi.yaml).
- **Discovery:** finds speakers via SSDP on your LAN; fixed IPs as fallback.

Not included: streaming services that require a login (Spotify, Sonos Radio,
…) usually cannot be started directly — see
[Music services & now playing](docs/how-to/music-services-and-now-playing.md).

## Requirements

- Sonos speakers and a computer **on the same LAN/subnet** — any Linux
  (x86_64, arm64, armv7, armv6), macOS or Docker
- Prebuilt binaries on the [Releases](https://github.com/ohm-meter/hallido/releases)
  page; to build yourself you need **Go 1.22+** and **git** (no other Go modules)
- Multicast (SSDP, UDP 1900) allowed on the LAN — otherwise configure fixed IPs

## Quick start (try it locally)

```bash
git clone https://github.com/ohm-meter/hallido.git
cd hallido
cp config.example.json config.json   # set api_token, e.g. openssl rand -hex 24
make run                             # mock driver, no speakers needed
```

Open <http://localhost:8080> and enter the token. With real speakers:
`make dev` (Sonos driver). Guided version: [First run tutorial](docs/tutorials/first-run-local.md).

## Installation (Linux, including Raspberry Pi)

**Without Go:** download the prebuilt binary for your architecture from
[Releases](https://github.com/ohm-meter/hallido/releases) and set it up as a
systemd service — step by step:
[Install on any Linux machine](docs/how-to/install-linux.md).

**With the installer** (clones, builds, sets up service and CLI; needs Go):

```bash
curl -fsSLO https://raw.githubusercontent.com/ohm-meter/hallido/master/scripts/install.sh
less install.sh                      # read it first, then run it
sudo bash install.sh --system --systemd
```

Then set the token and your scenes in `/var/lib/sonos-agent/config.json` and
start the service. Raspberry Pi details incl. go-live checklist:
[Install on a Raspberry Pi](docs/how-to/install-raspberry-pi.md) ·
installer details: [Install with the script](docs/how-to/install-one-liner.md) ·
Docker: [Run with Docker](docs/how-to/run-with-docker.md).

CLI after installation:

```bash
sonos-agent doctor     # check config, binary, port
sonos-agent status     # health check
sonos-agent update     # build the latest version from Git and restart
```

## Configuration

A single `config.json` (template: [`config.example.json`](config.example.json)):
bind address, API token, optional fixed IPs, scenes, schedules, time zone.
Scenes and schedules can also be edited in the web app.
Reference: [Configuration](docs/reference/config.md).

## Security in brief

- Use a **long random API token** (`openssl rand -hex 24`).
- Do **not expose the agent directly to the internet**: it speaks plain HTTP.
  For access on the go, use a reverse proxy with TLS and a login in front, or a
  VPN — [Remote access](docs/how-to/remote-access.md).
- Details and how to report vulnerabilities: [SECURITY.md](SECURITY.md).

## API

Every call except `GET /api/v1/health` needs `Authorization: Bearer <token>`.

```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://raspberrypi.local:8080/api/v1/scenes/morning-radio/play
```

Overview: [REST API](docs/reference/api.md) · Homey flows:
[Homey integration](docs/how-to/homey-integration.md).

## Documentation

Start at [`docs/README.md`](docs/README.md) (tutorial, how-to guides,
reference, explanation). Architecture: [`docs/explanation/architecture.md`](docs/explanation/architecture.md),
decisions: [`docs/adr/`](docs/adr/).

## Contributing

Bug reports and ideas are welcome as issues, pull requests too — please read
[CONTRIBUTING.md](CONTRIBUTING.md) first. Tests run without hardware:

```bash
make test       # go vet + all Go tests (mock driver)
make test-gui   # web UI: static checks + Playwright (run npm ci first)
```

## License and disclaimer

[MIT](LICENSE). This is an independent open-source project and is not
affiliated with, endorsed by or sponsored by Sonos, Inc. “Sonos” is a trademark
of Sonos, Inc.; the name is used only to describe the supported hardware.
