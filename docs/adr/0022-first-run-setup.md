# ADR 0022: First-run setup in the web app

## Status

Accepted (2026-10-15)

## Context

Until now the API token had to be created by hand (`openssl rand -hex 24`)
and put into `config.json` or `SRA_API_TOKEN`. That is impossible for a
non-technical NAS user. At the same time an unprotected first start must not
let anybody take over the agent, and the token must never be known to anyone
but the person setting it up.

## Decision

With setup enabled (`-setup` / `SRA_SETUP=1`, default in the container image
only) the agent also starts **without `config.json` or without a token**
(`config.LoadForSetup`; zero scenes are allowed there). It then:

1. Rejects every protected endpoint (no token = fail closed).
2. Offers `GET /api/v1/setup` (status) and **one** unauthenticated action,
   `POST /api/v1/setup/token`, which works only if **all** of these hold:
   - setup is still open (no token created yet; the config file has no
     token either — `ErrAlreadySetUp` otherwise),
   - within the **time window** after the start (default 1 hour,
     `SRA_SETUP_WINDOW`); afterwards a restart reopens it,
   - the client address is **private, loopback or link-local** (RFC 1918,
     `fc00::/7`, `fe80::/10`; not CGNAT `100.64/10`),
   - **no proxy headers** (`Forwarded`, `X-Forwarded-*`, `X-Real-IP`,
     `CF-Connecting-IP`, `True-Client-IP`, `CF-Ray`): a tunnel on the same
     host would otherwise look like loopback,
   - the **Host** is an IP literal in the ranges above, a single-label name or
     ends in `.local`, `.lan`, `.home`, `.home.arpa`, `.internal`,
     `.localdomain`, `.fritz.box` (DNS rebinding through a public domain is
     refused),
   - an `Origin` header, if present, equals the Host (same origin),
   - the header **`X-Hallido-Setup: 1`** is present: a custom header forces a
     CORS preflight that the agent never answers, so other websites in the
     user's browser cannot trigger the request.
3. The token is generated **on the server**: 160 bits from `crypto/rand`,
   Crockford base32 in 8 groups of 4 (`XXXX-XXXX-…`, no I/L/O/U), written
   atomically with mode 0600 into `config.json` (existing scenes, schedules
   and speakers are kept), activated immediately and returned **once**
   (`201`, `Cache-Control: no-store`). It is never logged and never returned
   again. The browser's IANA time zone is stored if the config has none.
4. From then on the normal token requirement applies to everything; after a
   restart setup stays closed.

The web app (`setup.js`) checks `GET /api/v1/setup` before the normal start
and shows a four-step wizard: create key → show key once (copy, confirm
“saved”) → speaker search → optional first scene. Blocked states explain the
expired window, a request not from the home network and an existing key.

## Threat model and accepted risks

- **The LAN is trusted** (as in SECURITY.md: Sonos speakers themselves have
  no authentication on the LAN). Whoever in the LAN calls the endpoint first
  within the window gets the key. The user notices this because the wizard
  then says “already set up”; the log shows time and client address of the
  setup. Remedy: delete the container's data volume and reinstall.
- A local TCP forwarder that hides the real client address and adds no proxy
  header (e.g. an SSH/frp tunnel set up by the user on the NAS) would make an
  internet client look local during the window. The NAS project file uses
  host networking (real client addresses); users are told not to enable
  remote access before setup is complete.
- No one-time code from the container log: it would make the setup
  noticeably harder for the target group. Can be added later if needed.

## Alternatives considered

- **Token from an environment variable in the compose file:** users would
  have to invent a strong secret and store it in a file that is also used
  for updates. Rejected for the NAS; still supported (`SRA_API_TOKEN`).
- **Password chosen by the user:** weak passwords, would need hashing and a
  login flow. The bearer token stays the single mechanism (ADR 0003).
- **Setup open without time limit:** a forgotten fresh installation would stay
  claimable forever. Rejected.

## Consequences

- The systemd installation (Pi) does not set `-setup`: missing config or
  token still stops the agent as before.
- New error codes: `forbidden`, `setup_closed` (403), `already_set_up` (409).
- Losing the key on all devices means reinstalling (or editing
  `config.json` on hosts where the file is accessible).
