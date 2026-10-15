# ADR 0020: Product name “Hallido”

## Status

Accepted (2026-10-15)

## Context

The web app carried a personal brand name, and the first public export used
“Sonos Agent”. “Sonos” is a trademark of Sonos, Inc.;
a product name built on it invites confusion and possible trademark trouble,
and a personal name does not belong in a public product.

## Decision

- The product is called **Hallido** (an invented word). Tagline:
  “Scenes & alarms for Sonos”. Brand line in the web app and page title:
  “Hallido — Powered by siraris”.
- “Sonos” is only used descriptively (“for Sonos”, supported hardware) plus the
  trademark disclaimer in the README.
- The public repository is `ohm-meter/hallido`; release archives are named
  `hallido_<version>_<os>_<arch>.tar.gz`.
- **Unchanged for now:** binary `sonos-radio-agent`, CLI and systemd service
  `sonos-agent`, paths `/usr/local/sonos-agent` and `/var/lib/sonos-agent`,
  env vars `SRA_*`. Running installations (the author's Pi) keep working
  without migration. Renaming the binary/CLI to `hallido` is on the roadmap
  and needs its own migration plan.

## Consequences

- + Neutral, trademark-safe public name; the brand line stays as the author wants.
- − Mixed naming until the binary/CLI rename: docs say “Hallido”, commands
  still say `sonos-agent`.
