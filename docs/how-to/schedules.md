# Schedules: start and stop scenes automatically

Schedules double as group alarms: a Sonos alarm only plays on a single speaker or on a group you formed by hand beforehand, whereas a schedule rebuilds the group with per-room volume every time.

Example: **Mon–Fri 06:10–08:30 Ö3 in the Living Room, Kitchen and Bathroom.** Decision and details: [ADR 0013](../adr/0013-schedules.md).

## 1. Create a scene

A schedule starts an existing scene. For Ö3 (official ORF stream, MP3 192 kbit/s):

```json
{
  "id": "morning-radio",
  "name": "Ö3 – Morning",
  "source": { "type": "stream", "uri": "https://orf-live.ors-shoutcast.at/oe3-q2a" },
  "members": [
    { "speaker": "Living Room", "volume": 22 },
    { "speaker": "Kitchen", "volume": 20 },
    { "speaker": "Bathroom", "volume": 18 }
  ]
}
```

The first speaker (Living Room) leads the group and must be reachable.

## 2. Create a schedule

### In the GUI

Below the scenes: **Schedules → New schedule** → pick the scene, tap the weekdays (or "Weekdays"), start `06:10`, stop `08:30` → Save. The toggle in the list switches a schedule off and on again.

### In `config.json`

```json
"timezone": "Europe/Vienna",
"schedules": [
  {
    "id": "wake-up-weekdays",
    "scene": "morning-radio",
    "days": ["mon", "tue", "wed", "thu", "fri"],
    "start": "06:10",
    "stop": "08:30",
    "enabled": true
  }
]
```

Then restart the agent (changes saved via the GUI/API take effect immediately).

### Via the API

```bash
curl -s -X POST http://pi:8080/api/v1/config/schedules \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"schedule":{"scene":"morning-radio","days":["mon","tue","wed","thu","fri"],"start":"06:10","stop":"08:30"}}'
```

```bash
curl -s http://pi:8080/api/v1/schedules -H "Authorization: Bearer $TOKEN"
```

Returns `next_start`, `next_stop`, `active` and `last_run` for each schedule.

## Behavior

| Situation | What happens |
|-----------|--------------|
| 06:10 on a weekday | The scene starts (even if something else is playing) |
| 08:30, Ö3 still playing | The scene is stopped |
| 08:30, manually switched to Spotify | Nothing — it isn't cut off |
| 08:30, scene paused (`…/pause`) | Nothing — the pause stays, `stop_skipped` ([control groups](control-groups.md)) |
| Pi/agent restarts at 07:00 | The scene is caught up unless it is already playing or paused |
| Restart at 09:00 | Nothing until the next start |
| Pi boots with a wrong clock, NTP corrects it | Same as a restart: catch-up, no duplicate starts |
| Kitchen offline | Living Room and Bathroom play; the failure shows up in the log and in `last_run.detail` |
| Living Room (coordinator) offline | Start fails; error in `last_run.error` and in the log |
| No `stop` | The scene keeps playing, no catch-up after a restart |
| `stop` before `start` (e.g. 23:00–01:00) | The window ends the next day |


Scenes used by a schedule can only be deleted once the schedule is removed or reassigned.

## Time zone

`timezone` (IANA name) applies to all schedules; the default is `Europe/Vienna`. The time zone database is bundled in the binary — the Pi doesn't need the `tzdata` package. Daylight saving time is handled automatically. A changed time zone takes effect after a restart.

## Troubleshooting

```bash
journalctl -u sonos-agent | grep -E 'schedule|clock'
```

- `schedule: scene started` / `schedule: scene stopped` — normal
- `schedule: stop skipped` — the scene was no longer playing on its own (`playing_other`, paused, stopped, offline)
- `clock jump detected` — the clock was adjusted (NTP); catch-up instead of re-firing
