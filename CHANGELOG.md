# Changelog

All notable changes. Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.4.0] — NAS installation

### Added

- NAS installation for non-technical users (Synology Container Manager, QNAP Container Station): release file `compose.yaml` with the image pinned by digest, guides in English and German (`docs/nas/`: install, update, uninstall, troubleshooting), [what Hallido does](docs/explanation/what-hallido-does.md) (ports, connections, files, rights, data) and [verify the image](docs/how-to/verify-image.md)
- Container image `ghcr.io/ohm-meter/hallido` (ADR 0021): linux/amd64, arm64, arm/v7; distroless without shell, user 65532, read-only root filesystem, no capabilities, built-in health check (`-healthcheck`); signed keyless with cosign, SBOM and provenance, vulnerability gate (Grype, govulncheck) in release and CI
- First-run setup in the web app (ADR 0022): without a token the container opens a wizard that creates a strong access key on the server and shows it once, searches the speakers and offers the first scene; open only from the home network, for one hour after the start and only once
- Setup and operations log (ADR 0023): web app Speakers → Log, rotated file in `/data/logs` (`-log-dir` / `SRA_LOG_DIR`), secrets redacted; `GET /api/v1/logs`
- `SRA_LISTEN` (listen address), `SRA_RADIO_SEARCH=0` (turn the Radio Browser search off)

### Changed

- Docker image defaults: real speakers (`SRA_DRIVER=sonos`), port **8790** instead of 8080, first-run setup on, log in `/data/logs`; `deploy/docker-compose.yml` is now a local mock test without a required token
- Release builds use the current Go toolchain (1.27); GitHub Actions pinned by commit SHA

### Fixed

- Faster scene start: `Play` gets its own 25 s timeout and is not sent twice when a large group's coordinator answers slowly; speakers already in the group are not regrouped, volumes are set in parallel, the log shows the duration of each phase

## [0.3.0] — first public release of Hallido

### Added

- Scenes: group speakers, start a stream, per-speaker volume; editing in the web app
- Schedules — group alarms the Sonos app can't do: start/stop scenes at fixed times, the group is rebuilt every time, catch-up after a restart
- Sonos driver over local UPnP/SOAP with SSDP discovery and fixed IPs as a fallback
- Group control: stop, pause, resume, ungroup — per scene or per speaker
- Radio search via Radio Browser; Sonos favorites as scenes (internet radio and TuneIn stations)
- Mobile-first web app (installable) in English and German (browser language), texts in `locales/<lang>.json`
- REST API with bearer token and OpenAPI contract
- Prebuilt binaries for Linux (amd64, arm64, armv7, armv6) and macOS with SHA256SUMS per release (`make dist`, GitHub workflow `release.yml`)
- Installer with systemd service and the `sonos-agent` CLI (`doctor`, `status`, `update`, `migrate`), Docker image

### Security

- The system installation separates code and data: `/usr/local/sonos-agent` is
  owned by root, `config.json` lives in `/var/lib/sonos-agent` (service user
  `sonos`); the CLI reads `.install-meta` as data only —
  [Migrate the installation layout](docs/how-to/install-layout-migration.md)

[Unreleased]: https://github.com/ohm-meter/hallido/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/ohm-meter/hallido/releases/tag/v0.3.0
