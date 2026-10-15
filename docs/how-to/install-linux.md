# How-to: Install on any Linux machine

Problem: Run Hallido permanently on a Linux machine in your home network —
whether a Raspberry Pi, mini PC, home server, Linux NAS, or VM.

The agent is a **single, static Go binary** with no runtime dependencies
(no glibc requirement, no libraries). It runs on any common distribution
(Debian, Ubuntu, Raspberry Pi OS, Fedora, Arch, Alpine …) and these architectures:

| `uname -m` | Release archive | Typical devices |
|------------|-----------------|-----------------|
| `x86_64` | `linux_amd64` | PC, mini PC, server, NAS, VM |
| `aarch64` | `linux_arm64` | Raspberry Pi 3/4/5/Zero 2 W (64-bit OS), ARM servers |
| `armv7l` | `linux_armv7` | Raspberry Pi 2/3/4 with 32-bit OS |
| `armv6l` | `linux_armv6` | Raspberry Pi 1 / Zero (W) |

For Raspberry Pi-specific notes, see [Install on a Raspberry Pi](install-raspberry-pi.md).

## Prerequisites

- Machine and Sonos speakers **on the same LAN/subnet** (no VLAN boundary, no guest Wi-Fi in between)
- systemd for permanent operation (otherwise use Docker, see [Run with Docker](run-with-docker.md))
- A VM needs a **bridge** to the LAN (no NAT), otherwise discovery won't work

## Option A: Prebuilt binary (no Go)

1. Download the matching archive from the [Releases page](https://github.com/ohm-meter/hallido/releases)
   and verify it (example `x86_64`, adjust the version):

   ```bash
   VERSION=v0.3.0
   ARCH=linux_amd64          # see table above
   curl -fsSLO "https://github.com/ohm-meter/hallido/releases/download/${VERSION}/hallido_${VERSION}_${ARCH}.tar.gz"
   curl -fsSLO "https://github.com/ohm-meter/hallido/releases/download/${VERSION}/SHA256SUMS"
   sha256sum --ignore-missing -c SHA256SUMS
   tar -xzf "hallido_${VERSION}_${ARCH}.tar.gz"
   cd "hallido_${VERSION}_${ARCH}"
   ```

2. Create the user and files — code owned by root under `/usr/local/sonos-agent`,
   config in the data directory `/var/lib/sonos-agent` (the only place the
   service may write, see [SECURITY.md](../../SECURITY.md)):

   ```bash
   sudo useradd --system --no-create-home --shell /usr/sbin/nologin sonos
   sudo install -d -o root -g root -m 0755 /usr/local/sonos-agent/bin
   sudo install -m 0755 sonos-radio-agent /usr/local/sonos-agent/bin/
   sudo install -d -o sonos -g sonos -m 0750 /var/lib/sonos-agent
   sudo install -m 0600 -o sonos -g sonos config.example.json /var/lib/sonos-agent/config.json
   sudo install -m 0644 sonos-agent.service /etc/systemd/system/
   ```

3. Set the token and add scenes:

   ```bash
   openssl rand -hex 24                         # generate a token
   sudo nano /var/lib/sonos-agent/config.json   # api_token, scenes, optional fixed IPs
   ```

4. Start the service:

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now sonos-agent
   curl http://localhost:8080/api/v1/health
   ```

**Update:** download the new archive, replace the binary, run `sudo systemctl restart sonos-agent`.
The config stays untouched.

## Option B: Installer (builds from source)

Requires `git` and **Go 1.22+**; also installs the `sonos-agent` CLI
(`doctor`, `status`, `update`):

```bash
curl -fsSLO https://raw.githubusercontent.com/ohm-meter/hallido/master/scripts/install.sh
less install.sh
sudo bash install.sh --system --systemd
```

Distribution Go packages: `apt install golang` (Debian/Ubuntu, may be too old —
then get it from <https://go.dev/dl/>), `dnf install golang`, `pacman -S go`.
Details: [Install with the script](install-one-liner.md).

## Build it yourself (cross-compile)

On any machine with Go, build for any target:

```bash
make dist            # all archives + SHA256SUMS into dist/
# or individually, e.g. for an x86 server:
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o sonos-radio-agent ./cmd/server
```

## Firewall

The agent needs the following on the LAN:

| Direction | Port | Purpose |
|-----------|------|---------|
| inbound | `8080/tcp` (or `listen`) | Web app and API — LAN only |
| inbound | UDP replies to SSDP (multicast `239.255.255.250:1900`) | Speaker discovery |
| outbound | `1400/tcp` to the speakers | Control (UPnP/SOAP) |

- **ufw** (Debian/Ubuntu): `sudo ufw allow from 192.168.1.0/24 to any port 8080 proto tcp`
- **firewalld** (Fedora/RHEL): put the machine in the `home` zone or run
  `sudo firewall-cmd --permanent --add-port=8080/tcp && sudo firewall-cmd --reload`.
  If the firewall blocks the SSDP replies, enter fixed IPs under `speakers` —
  [Troubleshoot discovery](troubleshoot-discovery.md).

Never expose port 8080 to the internet — [Remote access](remote-access.md).

## See also

- [Operations & troubleshooting](../runbook.md)
- [Configuration](../reference/config.md)
- [SECURITY.md](../../SECURITY.md)
