# How-to: Run with Docker

Problem: run Hallido in Docker — on a NAS, a Linux server or for testing.

**On a Synology or QNAP NAS** use the step-by-step guide instead:
[Install on a NAS](../nas/README.md). This page is for people who use
Docker or Docker Compose directly.

## The image

`ghcr.io/ohm-meter/hallido` — built and signed by the release workflow for
`linux/amd64`, `linux/arm64` and `linux/arm/v7`
([ADR 0021](../adr/0021-container-image-supply-chain.md)). Distroless, no
shell, user `65532`, health check built in. Verify it before use:
[Verify the image](verify-image.md). Everything it does:
[What Hallido does](../explanation/what-hallido-does.md).

Prefer the digest over a tag. Each release has `IMAGE_DIGEST.txt` and a
ready `compose.yaml` pinned to that digest.

## Host networking is required for real speakers

Speakers are found by SSDP multicast, which the default bridge network
blocks. Use `network_mode: host`; `ports:` then does not apply and Hallido
listens directly on the host's port **8790** (`SRA_LISTEN`).

## Compose

The release file `compose.yaml` (template:
[`deploy/nas/compose.template.yaml`](../../deploy/nas/compose.template.yaml))
works with plain Docker Compose as well:

```bash
docker compose -f compose.yaml up -d
```

Then open `http://<host>:8790` from a device in your home network: the setup
wizard creates the access key ([ADR 0022](../adr/0022-first-run-setup.md)).
Alternatively set a token yourself with `SRA_API_TOKEN` (setup is then
skipped).

Hardening used there (keep it when you write your own file): `user:
"65532:65532"`, `read_only: true`, `cap_drop: [ALL]`,
`security_opt: [no-new-privileges:true]`, `mem_limit`, `pids_limit`, one
named volume for `/data`.

**Data:** `/data` must be writable for UID 65532. A named volume handles this
automatically. A host directory needs `sudo chown 65532:65532 /path/to/dir`.

## Environment variables

| Variable | Image default | Meaning |
|---|---|---|
| `SRA_DRIVER` | `sonos` | `mock` for tests without speakers |
| `SRA_LISTEN` | `:8790` | Listen address |
| `SRA_SETUP` | `1` | First-run setup in the web app while no token exists |
| `SRA_SETUP_WINDOW` | `1h` | Setup window after the start |
| `SRA_LOG_DIR` | `/data/logs` | Rotated log, also shown in the web app (Speakers → Log) |
| `SRA_RADIO_SEARCH` | on | `0` = no connection to the Radio Browser directory |
| `SRA_API_TOKEN` | — | Fixed token instead of the setup wizard |

## Local test with the mock driver

```bash
docker compose -f deploy/docker-compose.yml up --build
```

Builds the image from the repository and starts it with the mock driver on
`http://localhost:8790`. Smoke test of an image with the full hardening:
`make image && scripts/test/container-smoke.sh hallido:dev`.

## Logs and health

```bash
docker logs -f hallido
docker inspect -f '{{.State.Health.Status}}' hallido
```

## Firewall

Allow TCP 8790 from your LAN and the speakers' SSDP answers (UDP from the
LAN). Never forward the port to the internet — see SECURITY.md and
[Remote access](remote-access.md) instead.

## See also

- [Troubleshoot discovery](troubleshoot-discovery.md)
- [Install on a NAS](../nas/README.md)
