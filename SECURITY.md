# Security

## Reporting vulnerabilities

Please **do not open public issues** for security vulnerabilities. Report them
through GitHub instead: **Security → Report a vulnerability** (private security
report) in this repository. Helpful details are the affected version or
commit, steps to reproduce, and the expected impact.

Only the current state of `master` is supported. There are no guaranteed
response times, but reports are taken seriously and disclosed publicly once
fixed.

## Threat model in brief

The agent is built for a **trusted home network**. It controls speakers over
unencrypted UPnP/SOAP and itself only serves HTTP. The REST API is protected by
a static bearer token; the Sonos speakers themselves have no authentication on
the LAN, so anyone on the same network can control them directly anyway.

What the code does about it:

- The token is compared in constant time; an empty token and the placeholder
  `BITTE-AENDERN` are rejected; the token is never logged.
- The token is accepted only in the `Authorization` header (never in the URL);
  there are no cookies and no CORS allowances.
- API calls are rate-limited (response `429 rate_limited`).
- All SOAP arguments and metadata are XML-escaped.
- Stream URLs are allowed only with permitted schemes (`http(s)` and Sonos's
  own variants), never `file:`; SSDP responses are accepted only if the device
  URL matches the responding IP.
- `config.json` (which contains the token) and the backup `config.json.bak` are
  written atomically with mode `0600`.
- The web UI ships a Content Security Policy (`script-src 'self'`); the service
  worker never touches `/api/` and caches only the app shell.
- The systemd unit runs as its own user and is hardened (`ProtectSystem=strict`,
  `NoNewPrivileges`, empty capability set); the Docker image runs as `nonroot`.

## Notes for operators

1. **Strong token:** at least 24 random bytes, e.g.
   `openssl rand -hex 24`. Preferably set it via `SRA_API_TOKEN` in the systemd
   unit rather than in plain text in the config. If a device with the installed
   web app is lost, rotate the token (the UI stores it in the browser).
2. **Never expose directly to the internet:** never forward port `8080` on your
   router. The token would travel in clear text. For access from outside, use
   TLS **and** an authentication layer in front (reverse proxy with login,
   zero-trust tunnel, or VPN) — see
   [Remote access](docs/how-to/remote-access.md).
3. **Restrict the bind address:** if the reverse proxy or tunnel runs on the
   same host, set `"listen": "127.0.0.1:8080"`.
4. **`/api/v1/health` is readable without a token** and lists the names of the
   reachable speakers (intended for monitoring). If you don't want that, block
   the path in the reverse proxy for external access.
5. **Rate limit behind a proxy:** the limit applies per client address as the
   agent sees it. Behind a reverse proxy all clients share one counter — set
   limits in the proxy as well if needed.
6. **Installer and updates:** `scripts/install.sh` and `sonos-agent update`
   build the current state of `master` from Git (HTTPS, no signed releases).
   Read the installer before running it instead of blindly using `curl | bash`;
   if you want a fixed state, clone a tag or commit and install locally.
7. **Installation directory:** the systemd installation separates code and
   data. Source code, binary, and `.install-meta` are owned by root under
   `/usr/local/sonos-agent`; the service user `sonos` may write only to
   `/var/lib/sonos-agent` (config, backup; systemd `StateDirectory`, otherwise
   `ProtectSystem=strict`). The CLI reads `.install-meta` as a plain `KEY=VALUE`
   file and only if it is owned by root or the calling user. This way a
   compromised service can plant neither code nor build steps that
   `sonos-agent update` runs as admin. `sonos-agent migrate` converts older
   installations (the installer and `update` call it automatically) —
   [Migrate the installation layout](docs/how-to/install-layout-migration.md).
8. **Docker / NAS:** use the release `compose.yaml` (image pinned by digest,
   signed) or the same settings in your own file: `user: 65532`,
   `read_only`, `cap_drop: ALL`, `no-new-privileges`, limits, one data
   volume. Real speakers need `network_mode: host` — the agent then listens
   on all interfaces of the host (port 8790). `deploy/docker-compose.yml` is
   only for local tests with the mock driver.
9. **First-run setup:** finish the setup wizard right after installing, and
   do not enable remote access (QuickConnect, tunnels, port forwarding)
   before. See “First-run setup” below.

## Container image and supply chain

Details: [ADR 0021](docs/adr/0021-container-image-supply-chain.md).

- The image `ghcr.io/ohm-meter/hallido` is built only by the release workflow
  of this repository: static Go binary without third-party Go modules on
  distroless (no shell, no package manager), base images pinned by digest,
  user 65532, health check without a shell.
- The image digest, `compose.yaml` and `SHA256SUMS` are signed **keyless**
  with Sigstore cosign; the certificate names the workflow
  `.github/workflows/release.yml` and the tag, and every signature is in the
  public Rekor log. No signing key exists that could leak.
  [Verify the image](docs/how-to/verify-image.md).
- An SPDX SBOM is attached to every release and as a signed attestation;
  BuildKit adds SLSA provenance. A high/critical vulnerability with an
  available fix (Grype) stops the release before signing. CI runs
  `govulncheck` and Grype on every pull request.
- GitHub Actions are pinned by commit SHA; jobs get minimal permissions.
- At run time Hallido contacts only the speakers on the LAN, the stream
  addresses of your own scenes (format check) and — optional, can be turned
  off with `SRA_RADIO_SEARCH=0` — the Radio Browser directory. No telemetry,
  no update check. Full list:
  [What Hallido does](docs/explanation/what-hallido-does.md).

## First-run setup

Details and threat model: [ADR 0022](docs/adr/0022-first-run-setup.md).

The container starts without an access key. Until the setup wizard has
created one, every API endpoint except the setup status refuses requests.
`POST /api/v1/setup/token` creates the key **once** (160 random bits from the
OS, stored with mode 0600, shown once, never logged) and only if the request

- arrives within one hour after the start (restart to reopen),
- comes from a private, loopback or link-local address without proxy
  headers (`X-Forwarded-For`, `Forwarded`, `CF-Connecting-IP`, …),
- uses a local host name or IP (blocks DNS rebinding) and the same origin,
- carries the header `X-Hallido-Setup: 1` (other websites cannot send it
  without a CORS preflight, which is never answered).

**Known limits (accepted):** the home network is trusted — any device in it
can claim the key first during the window; the wizard then shows “already
set up” and the log records the client address. A port forwarder on the
same host that hides the client address and adds no proxy header (e.g. a
user-made SSH tunnel) would let a remote client pass the address check
during that hour; the NAS project file uses host networking and the guide
asks you to finish the setup before enabling remote access.

## Operations log

`/data/logs/hallido.log` (rotated, max. about 4 MB, mode 0600) and the web
app view (Speakers → Log, token required) contain events, scene and speaker
names and, for the setup, the client IP address. Values of attributes named
like secrets (token, password, authorization, …) are replaced by
`[redacted]` — [ADR 0023](docs/adr/0023-operations-log.md).

## Security review of the NAS installation (2026-10)

Reviewed: setup endpoint and its network checks, token generation and
storage, token check after setup, log redaction and the log view (no HTML
injection: texts are inserted as text), container rights, compose file,
release and CI workflows. Result: no exploitable finding; the two accepted
limits are listed under “First-run setup”.
