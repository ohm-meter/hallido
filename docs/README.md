# Documentation — Hallido

**Start here:** [Project README](../README.md) · [First-run tutorial](tutorials/first-run-local.md) · [Install on Linux](how-to/install-linux.md) · [What lives where](explanation/project-structure.md) · [Contributing](../CONTRIBUTING.md)

Structured after [Diátaxis](https://diataxis.fr/): four types for different goals.

| Type | Goal | Contents |
|------|------|----------|
| **Tutorial** | Learn successfully | Guided first run |
| **How-to** | Solve a specific problem | Recipes for operations and configuration |
| **Reference** | Look things up | Config fields, API contract |
| **Explanation** | Understand | Architecture, decisions |

## Tutorials

- [First run locally](tutorials/first-run-local.md) — mock and Sonos, token, GUI, `/api/v1/speakers`

## How-to

Installation and operations:

- [Install on a NAS (Synology, QNAP)](nas/README.md) — step by step for non-technical users, English and German
- [Verify the image before installing](how-to/verify-image.md) — cosign signature, digest, SBOM
- [Install on any Linux machine](how-to/install-linux.md) — prebuilt binaries (x86_64/ARM), systemd, firewall
- [Install on a Raspberry Pi](how-to/install-raspberry-pi.md) — systemd, token, go-live checklist
- [Install with the script](how-to/install-one-liner.md) — `scripts/install.sh`, paths, `sonos-agent update`
- [Migrate the installation layout](how-to/install-layout-migration.md) — move older installations to the config in `/var/lib/sonos-agent`
- [Run with Docker](how-to/run-with-docker.md) — signed image, host networking for SSDP, environment variables
- [Remote access](how-to/remote-access.md) — reverse proxy, tunnel, or VPN instead of port forwarding
- [Operations & troubleshooting](runbook.md) — logs, updates, common errors
- [Start locally (dev & prod)](how-to/local-start.md)
- [Add a language](how-to/add-a-language.md) — translate the web app
- [Run the tests](how-to/testing.md) — Go, shell scripts, Playwright

Usage:

- [Create a scene](how-to/create-scene.md)
- [GUI scene editor](how-to/gui-scene-editor.md) — create a group, save speakers to the config
- [Schedules](how-to/schedules.md) — start/stop scenes automatically
- [Control groups](how-to/control-groups.md) — pause/resume/stop per speaker, ungroup
- [Radio Browser search](how-to/radio-browser-search.md)
- [Import Sonos favorites](how-to/sonos-favorites.md) — API + GUI, limits for Spotify/radio
- [Music services & now playing](how-to/music-services-and-now-playing.md) — limits, API, script
- [Install as an app](how-to/install-as-app.md) — laptop, iPhone, Android
- [Homey integration](how-to/homey-integration.md) — REST API for flows

Speakers and network:

- [Use the Sonos driver](how-to/use-sonos-driver.md)
- [Speaker config & GUI refresh](how-to/speaker-config-and-refresh.md)
- [Troubleshoot discovery](how-to/troubleshoot-discovery.md)
- [UPnP error 714 (stream)](how-to/troubleshoot-upnp-714.md)

## Reference

- [Configuration](reference/config.md) — `config.json` fields
- [REST API](reference/api.md) — overview, link to the [OpenAPI](api/openapi.yaml) spec
- [API for Homey](reference/api-homey.md) — quick reference for the flow builder
- [Glossary](reference/glossary.md) — terms and web app labels (English/German)

## Explanation

- [What Hallido does on your network and your NAS](explanation/what-hallido-does.md) — ports, connections, files, rights, data
- [Architecture](explanation/architecture.md) — layers, drivers, error model
- [What lives where](explanation/project-structure.md) — repo overview, scripts, startup path
- [ADRs](adr/) — documented architecture decisions

## More resources

- [Roadmap](todos.md)
- [Style guide (web GUI)](styleguide.md)
- [Plans](plans/) — implementation notes for larger features, e.g. the [i18n roadmap](plans/i18n-roadmap.md)
