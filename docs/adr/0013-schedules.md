# ADR 0013: Schedules (time-controlled scenes)

## Status

Accepted (2026-10-14)

## Context

- Use case: Mon–Fri 06:10–08:30 Ö3 plays in Living Room, Kitchen, and Bathroom — without Homey/cron from outside.
- Scenes already exist (speakers + source + volume); a schedule should **only reference** them.
- The agent runs on a Pi (Austria) **without an RTC**: after booting, the clock jumps via NTP; tzdata is not guaranteed to be installed.
- Until now there was no way to stop playback (ADR 0009 phase B open).

## Decision

### Model (`config.json`)

```json
"timezone": "Europe/Vienna",
"schedules": [
  { "id": "wake-up-weekdays", "scene": "morning-radio",
    "days": ["mon","tue","wed","thu","fri"],
    "start": "06:10", "stop": "08:30", "enabled": true }
]
```

- `stop` is optional; if `stop` is before `start`, the window ends the next day (`days` = start day).
- Missing `enabled` → `true` (hand-written config); unknown fields are rejected.
- Time zone defaults to **Europe/Vienna**, `import _ "time/tzdata"` in package `internal/schedule` (+~450 KB binary, but no system dependency).
- Validation on load and save: weekdays, `HH:MM`, `stop != start`, scene exists, unique IDs, max. 64 schedules. A scene with a schedule cannot be deleted.

### Scheduler (`internal/schedule`)

- Runs in the same process, one goroutine, terminated via the signal context (SIGTERM/systemd).
- **Injectable clock** (`Clock`/`Timer`, `FakeClock`) → tests without sleeps.
- Wakes up at least every **30 s** and fires all events in the interval `(last check, now]`. Wall-clock times are computed per day with `time.Date` (DST-safe; nonexistent times such as 02:30 on the changeover day become 03:30).
- **Clock jump** (gap > 2.5 min, e.g. NTP after boot, suspend): do not fire old events retroactively; do a catch-up instead. Clock going backward: fire nothing.
- **Catch-up** only at startup (and after a clock jump): if "now" falls within a start/stop window, the scene is started — unless it is already playing (`playing`), something else is playing manually (`playing_other`), or it is paused (`paused`, added with [ADR 0015](0015-group-control.md)). Schedules without `stop` have no window → no catch-up. A hot reload does not catch up (otherwise saving within the window would immediately start music).
- **Stop** only if `SceneStatus == playing`. For `playing_other`, `stopped`, `paused`, `offline`, everything stays as it is (`stop_skipped`).
- **Partial failure** as with play: coordinator failure = error; failed secondary speakers are logged and noted in `last_run.detail`.
- **Hot reload** after `POST/PUT/DELETE /api/v1/config/schedules…`. Time zone changes require a restart.
- The last run per schedule is kept in memory only (`last_run`), not persisted.

### Stop in the driver

- `SpeakerController` extended with `Stop` and `Pause` (AVTransport `Stop`/`Pause`, `InstanceID 0`). UPnP error **701** (already stopped) counts as success.
- For live radio, the scene service uses `Stop` (pause would buffer and later continue with a time offset).
- `POST /api/v1/scenes/{id}/stop` stops without a status check (idempotent for Homey) — completes ADR 0009 phase B.

### API

| Endpoint | Purpose |
|----------|---------|
| `GET /api/v1/schedules` | Runtime: `next_start`, `next_stop`, `active`, `last_run` |
| `GET/POST /api/v1/config/schedules` | Read / upsert (without `id` → `<scene>-<hhmm>`) |
| `PUT/DELETE /api/v1/config/schedules/{id}` | Replace (e.g. on/off) / delete, `schedule_not_found` → 404 |
| `POST /api/v1/scenes/{id}/stop` | Stop scene |

## Alternatives

| Option | Con |
|--------|-----|
| systemd timer/cron + curl | Second place for configuration, no GUI, no `playing_other` protection |
| Homey flows | Depends on Homey; the agent should run autonomously |
| Cron expressions | Powerful, but needlessly complex for weekday + time and hard to represent in the GUI |
| Pause instead of stop | Stream buffers; resumes later with a time offset |
| Catch-up on reload too | Saving within the window would start playback immediately — surprising |

## Consequences

- No external module; the binary grows by the embedded tzdata.
- New error code `schedule_not_found`.
- How-to: [`schedules.md`](../how-to/schedules.md), OpenAPI updated, Homey reference extended with stop.
