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
8. **Docker:** `deploy/docker-compose.yml` requires `SRA_API_TOKEN` from the
   environment and is meant for testing; real speakers need
   `network_mode: host` — the agent then listens on all interfaces of the host.
