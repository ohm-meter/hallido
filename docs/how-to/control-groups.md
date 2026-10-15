# Control groups: pause, resume, stop, ungroup

Problem: Music is playing — from a scene or directly from the Sonos app — and you want to pause it, resume it, or split a group again from Homey, a script, or `curl`. Decision and details: [ADR 0015](../adr/0015-group-control.md).

**In the web GUI:** Pause/resume a scene from the "Now playing" bar or the scene's ⋯ menu. In the **Speakers** tab, speakers are listed by their current group; each group has Pause/Resume and Stop. You find "Leave group" and "Ungroup all" after tapping a speaker (with confirmation).

All `curl` examples use:

```bash
export AGENT=http://raspberrypi.local:8080
export TOKEN=your-token
```

## Which groups exist right now?

```bash
curl -s -H "Authorization: Bearer $TOKEN" $AGENT/api/v1/groups
```

```json
{
  "groups": [
    { "coordinator": "Bathroom", "members": ["Bathroom"], "state": "stopped" },
    { "coordinator": "Living Room", "members": ["Living Room", "Bedroom", "Kitchen"],
      "state": "playing", "title": "Hitradio Ö3", "uri": "x-rincon-mp3radio://…", "scene_id": "morning-radio" }
  ]
}
```

The coordinator is always first in `members`. `scene_id` is set when the running source matches a scene with that coordinator.

## Control a group through any speaker

You do not need to know which speaker leads the group — use any room in the group. The agent sends the command to the coordinator, and all members follow.

```bash
curl -s -X POST -H "Authorization: Bearer $TOKEN" $AGENT/api/v1/speakers/Kitchen/pause
curl -s -X POST -H "Authorization: Bearer $TOKEN" $AGENT/api/v1/speakers/Kitchen/resume
curl -s -X POST -H "Authorization: Bearer $TOKEN" $AGENT/api/v1/speakers/Kitchen/stop
```

Response:

```json
{ "speaker": "Kitchen", "coordinator": "Living Room", "members": ["Living Room", "Kitchen"], "action": "pause", "ok": true }
```

URL-encode room names containing spaces or special characters: `Basement: TV` → `Basement%3A%20TV`.

**Pause or stop?** For live radio, **stop** is usually better. Sonos stops some live streams (for example TuneIn) on pause anyway — the state is then `stopped`, not `paused`; `resume` still works. Pause is worthwhile for Spotify, podcasts, and audiobooks.

## Pause and resume a scene

```bash
curl -s -X POST -H "Authorization: Bearer $TOKEN" $AGENT/api/v1/scenes/morning-radio/pause
curl -s -X POST -H "Authorization: Bearer $TOKEN" $AGENT/api/v1/scenes/morning-radio/resume
```

`resume` continues whatever is loaded on the scene's coordinator — it does **not** reload the scene. If something else was started in the meantime, that keeps playing; use `…/play` in that case. If nothing is loaded at all, you get `502 playback_failed`.

## Ungroup

```bash
curl -s -X POST -H "Authorization: Bearer $TOKEN" $AGENT/api/v1/speakers/Kitchen/ungroup
```

| Speaker addressed | Effect |
|-------------------|--------|
| Member (for example `Kitchen`) | Only it leaves the group and is silent afterward; the rest keep playing. |
| Coordinator (for example `Living Room`) | The whole group is dissolved (`dissolved: true`): all members become standalone, and the coordinator keeps playing alone. |
| Standalone speaker | Nothing to do (`standalone: true`). |

```json
{
  "speaker": "Living Room", "coordinator": "Living Room", "dissolved": true, "standalone": false,
  "speakers": [ { "speaker": "Bedroom", "ok": true }, { "speaker": "Kitchen", "ok": false, "error": "Speaker is not responding" } ]
}
```

If a speaker fails, that is not a hard error (HTTP 200, error per speaker). Only if **no** speaker could leave the group do you get HTTP 502 with the same object under `result`.

## Errors

| HTTP | Code | Meaning |
|------|------|---------|
| 502 | `speaker_unreachable` | Speaker not in the topology (wrong name, offline), or no speaker answers the topology query |
| 502 | `playback_failed` | Speaker rejects the command, for example `resume` without a loaded source |
| 404 | `scene_not_found` | Unknown scene ID (`…/scenes/{id}/pause|resume`) |

For `…/speakers/{name}/…`, the affected group (as far as known) is in the error under `result`.

## Interaction with schedules

If you pause a scene that a schedule started, the schedule leaves it alone at stop time (`stop_skipped`) — and after an agent restart it does not restart it within the window either. See [Schedules](schedules.md).
