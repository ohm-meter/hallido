# How-to: Deploy with Docker

Problem: Run the agent in Docker — with the real Sonos driver and SSDP on the LAN.

## When Docker makes sense

- Test/staging environment on a Linux host on the same LAN
- You do not want systemd on the host

**Recommendation for continuous operation on a Pi:** systemd via [`install-raspberry-pi.md`](install-raspberry-pi.md) — fewer SSDP pitfalls.

## Prerequisites

- Docker and Docker Compose on a host **on the same LAN** as the Sonos speakers
- Your own `config.json` (token, scenes, optionally `speakers`)

## Important: host networking for SSDP

The default bridge network blocks SSDP multicast. For real Sonos operation, use **`network_mode: host`**.

Example change in `deploy/docker-compose.yml`:

```yaml
services:
  sonos-radio-agent:
    build:
      context: ..
      dockerfile: Dockerfile
    network_mode: host
    environment:
      - SRA_API_TOKEN=your-long-random-token
      - SRA_DRIVER=sonos
    volumes:
      - /path/to/sonos-data:/data   # contains config.json, must be writable for UID 65532
```

Notes:

- With `network_mode: host`, `ports:` no longer applies — the agent listens directly on the host's `:8080`
- Mount the config **directory** as a volume (not read-only, not a single file): the API writes the config atomically via a temp file + rename in the same directory. Host directory: `sudo chown 65532:65532 /path/to/sonos-data`. Pass secrets only via `SRA_API_TOKEN`, never in the image

## Start

In the repo (the Compose file requires `SRA_API_TOKEN` from the environment or from a `.env` file next to where you run it):

```bash
export SRA_API_TOKEN="$(openssl rand -hex 24)"   # generate once and keep it safe
docker compose -f deploy/docker-compose.yml up -d --build
```

Logs:

```bash
docker compose -f deploy/docker-compose.yml logs -f
```

Health:

```bash
curl http://localhost:8080/api/v1/health
```

## Mock vs. Sonos

The bundled Compose file uses `SRA_DRIVER=mock` by default — only for tests without hardware. For production, set `SRA_DRIVER=sonos` and host networking.

## Firewall

On the Docker host: allow port 8080/tcp on the LAN. No port forwarding to the internet — the API is meant for the local network only.

## Rollback

```bash
docker compose -f deploy/docker-compose.yml down
# Restore the previous image/config, then run up again
```

## Go-live checklist

Same items as the [Pi go-live](install-raspberry-pi.md#go-live-checklist), plus:

- [ ] `network_mode: host` is active (Sonos driver)
- [ ] `/data` is a writable volume containing `config.json`, not in the image
- [ ] `SRA_API_TOKEN` is set

## See also

- [Raspberry Pi deployment](install-raspberry-pi.md)
- [Troubleshoot discovery](troubleshoot-discovery.md)
