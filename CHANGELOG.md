# Changelog

All notable changes. Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

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
