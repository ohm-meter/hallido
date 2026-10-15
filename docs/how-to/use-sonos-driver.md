# How-to: Use the Sonos driver

Problem: control real Sonos speakers on your LAN instead of the mock driver.

## Enable the driver

```bash
make dev
# or: ./scripts/dev.sh
```

Environment: `SRA_DRIVER=sonos` (the default in the scripts; override via `.env`).
The `-driver` CLI flag takes precedence, see `cmd/server/main.go`.

See also: [Run locally](local-start.md).

Production (Pi/systemd): set `SRA_DRIVER=sonos` in the unit — see `deploy/` and the [runbook](../runbook.md).

## Network

- The agent and the speakers must be on the **same LAN** (no cloud login).
- SSDP/UPnP must be reachable (multicast). This is often restricted in Docker or across VLANs.

## Names and config

- Room names in `members[].speaker` must match the Sonos app **exactly**.
- Optional: set fixed IPs under `speakers` as a fallback:

```json
"speakers": {
  "Living Room": { "ip": "192.168.1.10" }
}
```

An empty `"ip": ""` means discovery only.

## Check discovery

```bash
curl -H "Authorization: Bearer $SRA_API_TOKEN" http://<host>:8080/api/v1/speakers
```

GUI: the "Found" panel below the header.

## What works (increment 2)

- SSDP discovery, grouping, volume, stream playback
- Partial failure: reachable speakers play, errors are reported per speaker

## What doesn't work yet

- `type: favorite` — increment 3, see ADR 0005. Use stream URLs until then.

## Mock vs. Sonos

| | Mock | Sonos |
|---|------|-------|
| Hardware | no | yes, LAN |
| Discovery | fixed test list | SSDP + config fallback |
| Tests/CI | default | SOAP mock in unit tests |
