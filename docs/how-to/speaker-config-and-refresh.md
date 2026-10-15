# How-to: Speaker config and refreshing speakers in the GUI

Two approaches complement each other: **fixed IPs in the config** and **live discovery in the GUI**.

## `speakers` in config.json

- Map: room name (exactly as in the Sonos app) → `{ "ip": "…" }`
- Empty IP (`""`): SSDP/discovery only, no fixed-IP fallback
- IP set: fallback when discovery doesn't find the speaker (Docker, VLAN, SSDP blocked)
- **Wrong or outdated IP** (different subnet): skipped after a short timeout; SSDP still finds the speaker — fix or remove the entry
- Changes to `config.json` require an agent **restart**

Template: `config.example.json`. Your own `config.json` is in `.gitignore`.

## GUI: "Refresh" speakers

With a valid API token:

1. The **Found** panel is visible
2. The **Refresh** button calls `POST /api/v1/discovery/refresh` (bearer token) — the **only way to trigger SSDP/live discovery**
3. `GET /api/v1/speakers` returns config/cache speakers without network discovery (background poll about every 30 s)

**Error hints:**

- **401** → check the token in the dialog (exactly as in `config.json`, Latin-1)
- **`discovery_error`** in the JSON → hint below the scenes; if needed, set a fixed IP in the config and restart the agent

Health (`/api/v1/health`) polls without SSDP (about every 15 s). Names from `config.json` (`speakers` + scene members) count as known at startup.

## Scene names

`members[].speaker` must match the Sonos room names. The GUI warns **only after a manual speaker refresh** if a member isn't in the discovery list. At startup, config names are accepted.
