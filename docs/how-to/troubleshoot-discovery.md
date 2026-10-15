# How-to: Troubleshoot discovery

Problem: No speakers, or the wrong ones, in the GUI/API; scenes fail because of names.

## Symptoms and checks

### `discovery_error` is set

Health and `/api/v1/speakers` can return `discovery_error` (for example, SSDP is blocked).

```bash
curl http://localhost:8080/api/v1/health
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/speakers
```

**Typical causes:** Docker without host networking, a VLAN without multicast, a firewall, or **stale fixed IPs** (a wrong subnet used to block SSDP through long timeouts).

**Fix:** Check the fixed IP under `speakers.<name>.ip` in `config.json` and remove or correct wrong entries (see the [config reference](../reference/config.md)). Since the fix, wrong fixed IPs are skipped quickly and SSDP still runs.

### GUI: "Scene names not in discovery"

Appears **only after a manual "Refresh"** of the speakers (no longer automatically after a restart).

Cause: The scene's `speaker` name differs from the Sonos room name found via SSDP. Compare the list under "Found" speakers with the app names.

**Fix:** Adjust `config.json` (`speakers` + `members[].speaker`) and restart the agent. Fixed IPs under `speakers.<name>.ip` are accepted as known at startup; SSDP only runs on refresh or play.

### `scene_misconfigured` (400) on play

A member name is not in the last discovery. The response contains a hint.

**Fix:** Correct the name, or switch the speaker on / bring it onto the network.

### Speaker offline / partial failure

A "X offline" badge or a red border. Health shows fewer speakers online.

**Fix:** Check power, Wi-Fi, same subnet; the coordinator (first `members` element) must be reachable.

### `speaker_unreachable` / `grouping_failed` despite discovery

Discovery only checks the device description (`device_description.xml`), not SOAP control. All affected speakers can be listed in `/api/v1/speakers` and still fail on play.

**Typical causes:**

- Speaker in sleep mode — the first SOAP action wakes it; play again
- Many speakers in the `speakers` config — older versions refreshed all IPs before every play (slow, timeouts)
- Portable speaker (`Mobile: …`) cannot be grouped (Bluetooth, no Wi-Fi)

**Fix:**

1. Restart the agent (`./scripts/dev.sh`) and play the scene again
2. Test a single-speaker scene with only the coordinator (`fm4-living-room`)
3. Test a multi-room scene without portable speakers
4. Read the error hint (`hint`) — it distinguishes "unreachable" (IP, possibly "timeout"), "bonded/invisible" (change the config IP to the visible player) and "rejected the grouping" (SOAP rejection)

### Agent unreachable

GUI: "Agent unreachable".

```bash
systemctl status sonos-radio-agent   # Pi
curl http://<host>:8080/api/v1/health
```

## Checklist

1. Driver `--driver=sonos` (not mock)?
2. Token valid?
3. `/api/v1/speakers` — expected names + IPs?
4. Scene names exactly (byte for byte) as in discovery?
5. For SSDP problems: fixed-IP fallback

More operational cases: [runbook](../runbook.md).
