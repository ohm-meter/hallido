# Operations & troubleshooting

Initial installation: [Install on a Raspberry Pi](how-to/install-raspberry-pi.md).
Paths refer to the system installation: code under `/usr/local/sonos-agent`
(owned by root), config under `/var/lib/sonos-agent/config.json` (service user `sonos`).

## Start, stop, logs

- Status: `systemctl status sonos-agent`
- Logs: `journalctl -u sonos-agent -f`
- Restart: `sudo systemctl restart sonos-agent` (required after manual edits to `config.json`)
- Update: `sonos-agent update` (back up `config.json` first)
- Health: `curl http://<HOST>:8080/api/v1/health`
- Self-test: `sonos-agent doctor`

## Common cases

**Agent doesn't start, log: "api_token missing or still the placeholder"**
Set `api_token` in `config.json` (random, e.g. `openssl rand -hex 24`) or
`Environment=SRA_API_TOKEN=…` in the unit. A set `SRA_API_TOKEN` takes precedence.

**Update completes, but old behavior persists**
Browser cache or installed web app: reload the page; the app reloads
automatically after an update. Otherwise check `sonos-agent version` and look at the log.
There is no `-race` on 32-bit ARM; `make build-pi` doesn't use it.

**`sonos-agent update` fails at `git pull`**
Local changes in the clone (`REPO_DIR` from `sonos-agent version`/`.install-meta`)
or no network. Check `git status` in the clone. If the clone is owned by root
(`/usr/local/sonos-agent/src`): `sudo sonos-agent update`.

**`… .install-meta is not loaded`**
Old layout; the service user still owns `/usr/local/sonos-agent`.
`sudo sonos-agent migrate` — [Migrate the installation layout](how-to/install-layout-migration.md).

**GUI shows "Agent unreachable"**
The service isn't running or the host/port is wrong — check health (above). When
accessing through a proxy/tunnel, check its status too.

**Speaker not found**
SSDP is blocked (VLAN, guest Wi-Fi, multicast filter) — enter a fixed IP under `speakers`:
[Troubleshoot discovery](how-to/troubleshoot-discovery.md).

**Scene reports a partial failure / `speaker_unreachable`**
Speaker is off, on another network, or the IP points to a **bonded** partner
(right side of a stereo pair, surround, sub). Enter the IP of the visible player.
The remaining speakers keep playing.

**Coordinator unreachable (502)**
The first speaker of a scene leads the group and must be reachable —
change the order of `members`.

**Stream doesn't start (UPnP 714)**
[Troubleshoot UPnP 714](how-to/troubleshoot-upnp-714.md).

**Music is playing and should stop — no matter where it was started**
`POST /api/v1/speakers/{name}/stop` (or `/pause`) stops the speaker's current
group — [Control groups](how-to/control-groups.md).

## Driver

- Test/development: `--driver=mock` (default of the binary, not of the systemd unit)
- Production: `SRA_DRIVER=sonos` (set in the unit)

Favorites (`type: favorite`) only play with a playable URI; TuneIn, Spotify,
and Sonos Radio favorites generally don't → use a stream URL (radio search
in the web app).
