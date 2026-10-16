# ADR 0021: Hardened container image, signed releases and the NAS project file

## Status

Accepted (2026-10-15)

## Context

Non-technical users with a Synology (Container Manager) or QNAP (Container
Station) NAS should be able to install Hallido from a single project file,
safely and without third-party sources. Before this ADR the `Dockerfile` was a
test aid: single architecture, base images by tag, no published image.

Requirements:

- Source is exclusively our own releases of `ohm-meter/hallido`.
- No downloads at install time or run time apart from the Sonos speakers on
  the LAN and the optional Radio Browser search (ADR 0011), which can be
  turned off.
- Checksum and signature must be verifiable by anyone, without keys we have
  to manage.
- Sonos discovery needs SSDP multicast, so the container uses the host
  network.

## Decision

**Image** (`Dockerfile`):

- Static Go binary (`CGO_ENABLED=0`, `-tags timetzdata`) on
  `gcr.io/distroless/static-debian13:nonroot`: no shell, no package manager.
- Base images (Go builder and distroless) pinned by digest.
- `linux/amd64`, `linux/arm64`, `linux/arm/v7`, built by Go cross-compilation
  (`--platform=$BUILDPLATFORM`), no emulation.
- User `65532:65532`; only `/data` is owned by it (config, backup, `logs/`).
  Designed for `read_only`, `cap_drop: ALL`, `no-new-privileges`.
- `HEALTHCHECK` through the binary itself (`-healthcheck`, GET `/` on
  127.0.0.1), because there is no shell or curl.
- Defaults for the NAS: `SRA_DRIVER=sonos`, `SRA_SETUP=1` (ADR 0022),
  `SRA_LOG_DIR=/data/logs` (ADR 0023), port `8790` (8080 is the QNAP admin
  port).

**Registry and signature:** the GitHub release workflow (tag `v*`) builds all
platforms, pushes to `ghcr.io/ohm-meter/hallido` (tags `<version>` and
`latest`) and signs the **digest** keyless with Sigstore cosign, using the
workflow's GitHub OIDC identity. The signature certificate names the exact
workflow file and tag (`…/.github/workflows/release.yml@refs/tags/vX.Y.Z`),
the signature is recorded in the public transparency log (Rekor). The same
applies to `SHA256SUMS` and `compose.yaml` (`cosign sign-blob --bundle`).

**SBOM and vulnerability gate:** BuildKit attaches an SBOM and SLSA provenance
(`provenance: mode=max`) to the image; Syft creates an SPDX SBOM that is
attached to the release and as a signed cosign attestation. Grype scans the
SBOM; a high or critical finding **with an available fix** stops the
workflow before the signature, so no `compose.yaml` points to an unsigned or
vulnerable build. CI additionally runs `govulncheck` (Go code and standard
library) and Grype on every pull request.

**NAS project file:** `deploy/nas/compose.template.yaml` is rendered by
`scripts/render-compose.sh` in the release workflow into `compose.yaml` with
the image **pinned by digest** and attached to the release (plus
`compose.yaml.sha256` and a Sigstore bundle). Settings: `network_mode: host`,
`user: 65532:65532`, `read_only`, `cap_drop: ALL`,
`no-new-privileges:true`, `mem_limit: 128m`, `pids_limit: 64`,
`cpu_shares: 512`, named volume `hallido-data:/data`. Optional variables with
defaults (`HALLIDO_PORT`, `HALLIDO_RADIO_SEARCH`) that users do not have to
touch. `scripts/test/nas-compose.sh` checks the hardening in `make test`.

**Actions pinned by commit SHA** (with the version as comment), minimal
`permissions` per job.

## Alternatives considered

- **cosign with our own key pair:** the private key would have to be stored as
  a repository secret and rotated; a leaked key would allow valid-looking
  signatures. Keyless binds the signature to the workflow identity instead
  and needs no secret. Rejected.
- **GitHub artifact attestations only (`gh attestation verify`):** equivalent
  in strength (also Sigstore), but tied to the GitHub CLI and login for
  verification. cosign works without a GitHub account. Could be added later.
- **Docker Content Trust / Notary v1:** deprecated in practice, not supported
  by GHCR. Rejected.
- **Alpine or scratch base:** Alpine brings a shell and package manager;
  scratch lacks `/etc/passwd`, CA certificates (Radio Browser over HTTPS) and
  a non-root user. distroless static has exactly those and nothing else.
- **Trivy instead of Grype:** equivalent scanner. Grype comes from the same
  vendor as the SBOM tool (Anchore), which keeps the number of third parties
  in the release pipeline small.
- **Bind mount of a NAS folder for `/data`:** the folder would belong to the
  NAS admin user and be read-only for UID 65532; laypeople cannot fix
  ownership. A named volume inherits the ownership from the image. Trade-off:
  the data is not visible in File Station (the log is visible in the web app
  instead).
- **CPU limit `cpus`:** several NAS kernels lack CFS quota support and Docker
  then refuses to start the container. `cpu_shares` only sets a weight and
  degrades to a warning.

## Consequences

- One-time manual step after the first release: set the GHCR package
  `hallido` to **public** and link it to the repository.
- The image runs only with host networking for real speakers; the agent then
  listens on all interfaces of the NAS (documented in SECURITY.md and
  `docs/explanation/what-hallido-does.md`).
- Updating the base images is a deliberate change (digest in the
  `Dockerfile`, CHANGELOG entry).
- The systemd installation (Pi) is unaffected.
