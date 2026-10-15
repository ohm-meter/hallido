# Plan: GUI mobile-first redesign

As of 2026-10-14 · Status: **implemented** (branch `claude/gui-mobile-redesign-5b21c8`, steps 1–8 one commit each).
For deviations from the concept see the section "Implementation — deviations" at the end.

Clickable mockup: [`gui-mobile-mockup.html`](gui-mobile-mockup.html) (open in a browser, phone width).

## Brief

```text
Designing: Web GUI of the hallido agent (start scenes, schedules, speakers) for the household.
Platform: Web, primarily phone (390×844), secondarily tablet/desktop.
Goal: Music on/off in 1 tap — no aiming, no scrolling, even half asleep at 6:10.
Tone: Device/instrument, not SaaS. Existing identity (dark, amber, mono) stays.
Risk: Admin functions (new group, save config, speaker list) displace playback.
Constraints: Vanilla JS + local Pico.css, no web fonts, no build, offline Pi, Playwright layout tests.
Research: Styles (visual direction), screens (player/device patterns).
Path: Concept + mockup → approval → implementation in small commits.
```

## Findings (current GUI, 375×812, mock)

| Problem | Effect on a phone |
|---|---|
| Header ~125 px (two-line brand + status + theme) | First scene only visible from ~230 px |
| Full-width "+ New group" **above** the scenes | The admin action is the most prominent element; play is not |
| Scene card ~370 px tall, lots of empty space | Only **one** scene per screen visible |
| Edit and delete icons right next to each other in the card footer | Mis-tapping delete is likely; destructive action without spacing |
| No stop/volume on the card | Switching off only possible via the Sonos app |
| "Found speakers" + save config + refresh on the same page | Long scroll page, maintenance mixes with operation |
| Scene dialog: nested scroll area for speakers (~4 rows visible), number fields 5 % | Small targets, keyboard for volume, scroll-in-scroll |
| "ID (slug)" field visible | Technical term, generated from the name anyway |
| Title "Powerd" | Typo → "Powered" |

## Research summary

- **Styles (2 full, 10 skimmed):** *Superlative* (precision instrument, matte surfaces, signal orange only as an indicator), *Inngest* (midnight console, amber exclusively for CTA/active, pill buttons). Skimmed: Elektron, teenage engineering, Bang & Olufsen, Umbrel.
- **Screens (iOS):** *Vinyls* tracklist (round play/pause button on the left, active row with progress ring), *Neuecast* (mini player above a fixed tab bar), *Spotify* device sheet (bottom sheet with device list + volume slider at the bottom), *Apple Music* queue (bottom nav with 3 icons).

## Reference lock

```text
Primary: "Precision instrument" (Superlative) — fits the existing device/radio identity.
Preserve: matte near-black canvas · mono uppercase for labels/metadata · flat surfaces with 1px edges
          instead of shadows · a single signal accent (amber) · tight, disciplined spacing.
Borrow only: (1) Inngest — pill shape (9999px) for primary touch actions; amber only for CTA + "playing".
             (2) Vinyls/Neuecast — round play/stop button per row + "Now playing" bar above the tab bar.
Role rules: Amber = "active/playing" + exactly one primary action per screen. Red only for delete/offline.
            Green only for online status. Mono only for metadata, never body text/main button text.
Media: no images; Lucide icons (local), outline, currentColor.
Reject: grid/gradient decoration in the background · card shadows · multiple accent colors · centered
        huge headers · hidden gestures (long-press, swipe-to-delete) as the only way.
Tokens: see below (palette from docs/styleguide.md stays, new size/touch tokens).
```

## Concept

### Navigation: 3 tabs at the bottom (thumb zone)

| Tab | Content | Primary action |
|---|---|---|
| **Scenes** (start) | List of all scenes as rows | Tap on a row = play/stop |
| **Schedules** | List with next start + switch | `+` schedule |
| **Speakers** | Found speakers, refresh, save config, token, theme | Refresh |

- Tab bar fixed at the bottom, 64 px + `safe-area-inset-bottom`, icon + label, active tab amber.
- Header shrinks to **one** 48 px row: status dot + "15 speakers" on the left, page title, on the right `+` (context-dependent: New scene / New schedule).
- Tab switching without reload (hash routing `#szenen`, `#zeitplaene`, `#boxen`) → the phone's back button works, deep link for home-screen bookmarks.
- ~~From 900 px width: the tab bar becomes a left rail (same targets), scenes in 2 columns.~~ Replaced by the phone column at all widths (ADR 0016, 2026-10-15).

### Scenes tab

- **Scene row** instead of a tall card: 72 px high, round play button 56 px on the left, name in the middle (17 px, 600) + mono line "Living Room · Kitchen · Bathroom", on the right `⋯` (48×48) → sheet "Edit / Delete".
- **The whole row is the tap target** for play; `⋯` is separate on the right with ≥ 8 px spacing.
- "Playing" state: button turns amber with a stop symbol, the row gets an amber edge on the left (3 px), status label "playing". "Playing something else" (`playing_other`): neutral hint, no amber.
- **"Now playing" bar** (like a mini player) above the tab bar as soon as a scene is playing: name, rooms, large stop button, tap opens the volume sheet.
- **Volume sheet** (Spotify pattern): one row per room with slider (track 48 px high) + `−`/`+` (44 px). *Needs a volume endpoint, see backend.*
- Delete only in the edit sheet now, at the bottom, red, with confirmation.
- Goal: **5–6 scenes without scrolling** on 390×844.

### Create/edit a scene (bottom sheet → full screen on a phone)

1. **Name** (slug automatic; "ID" only under "Advanced", read-only when editing).
2. **Source** as a segmented control with 3 pill segments (48 px): *Search radio* · *From speaker* · *URL*; Sonos favorites as a 4th option in the overflow. Radio search is the default (most common case).
3. **Rooms**: full rows (56 px) with checkbox **and** inline slider, visible only when selected; no scroll-in-scroll — the sheet scrolls as a whole.
4. **Sticky footer**: "Cancel" (ghost) + "Save & play" (amber pill, full width on the right).

### Schedules tab (builds on `feat/zeitplaene`)

- Row: time large (mono, 22 px) "06:10–08:30", below it scene + day abbreviations "Mon Tue Wed Thu Fri", on the right a switch (51×31, hit area 48 px).
- Top row "Next start: tomorrow 06:10 · Morning Radio".
- Editor: scene (list), weekdays as 7 round chips at 44 px each + quick picks "Mon–Fri / Weekends / Daily", start/stop as native `<input type="time">` (phone wheel).

### Speakers tab

- List of speakers (name, model/IP in mono, online dot), "Refresh" as a pill at the top.
- "Settings" section: theme, change API token, "Save speakers to config" — maintenance lives here, not on the start page.

## Design tokens (new/changed)

| Token | Value | Role |
|---|---|---|
| `--tap-min` | `48px` | Minimum size of all touch targets (Material 48dp ≥ Apple 44pt) |
| `--tap-primary` | `56px` | Play/stop button, primary footer button |
| `--tap-gap` | `8px` | Minimum spacing between adjacent targets |
| `--row-h` | `72px` | Scene/schedule row |
| `--bar-h` | `48px` | Header |
| `--tabbar-h` | `64px` | Tab bar (+ safe area) |
| `--radius-pill` | `9999px` | Buttons, segments, chips |
| `--radius-row` | `12px` | Row/sheet corners |
| `--space-1..6` | `4/8/12/16/24/32px` | Spacing scale |
| UI font | `17px` body on phone (≥ 16 px prevents iOS zoom on inputs) | |

Palette (`--bg`, `--panel`, `--amber`, …) stays unchanged from `docs/styleguide.md`.
Grid background and header glow are dropped (reject list).

## Decision ledger

| Decision | Source | Rule/role | Why |
|---|---|---|---|
| Bottom tab bar, 3 tabs | Neuecast, Apple Music (screens) | Navigation in the thumb zone | One-handed use; separates operation from maintenance |
| Scenes as rows with a round play button on the left | Vinyls tracklist | Play circle = main action, active row marked | 5–6 scenes per screen instead of 1 |
| "Now playing" mini player | Neuecast, Spotify | Above the tab bar, always reachable | Stop in 1 tap from anywhere |
| Volume sheet with sliders | Spotify device sheet | Slider at the bottom of the sheet | Slider instead of number keyboard |
| Amber only for active + 1 CTA | Inngest "Amber exclusively for CTA/active", Superlative "signal only" | Token role | Status readable at a glance |
| Pill buttons | Inngest | 9999px for interactive elements | Large, clearly tappable shapes |
| Flat, 1px edges, no shadows | Superlative "no heavy shadows" | Surfaces | Calmer, device character |
| Mono only for metadata | Style guide + Superlative | Type role | Names stay readable in sans |
| 48/56 px touch targets, 8 px spacing | Craft (Apple HIG 44pt, Material 48dp) | Touch | Core requirement from the user |
| Delete only in the edit sheet | Craft (separate destructive actions) | Red = destructive | No mis-taps |
| Hash routing | User constraint (no build/framework) | — | Back button + bookmarks |

## Backend dependencies

| Need | Status |
|---|---|
| `POST /api/v1/scenes/{id}/stop` | comes with `feat/zeitplaene` (session D) |
| Schedule API | comes with `feat/zeitplaene` |
| Live volume per room (`PUT /api/v1/speakers/{name}/volume`) | **new** — `SetVolume` exists in the controller, endpoint missing |

## Implementation (phase 2, after merging `feat/zeitplaene`)

One commit per step, `make test-gui` green, Playwright tests in `tests/gui/` (390×844, 820×1180, 1280×800):

1. Tokens + 48 px header + title typo.
2. Tab bar + hash routing, move speakers/settings into their own tab.
3. Scene rows (play/stop, `⋯` sheet, delete moved into the edit sheet).
4. "Now playing" bar + stop.
5. Scene editor as a sheet (segmented control, room rows, sticky footer, slug under "Advanced").
6. Schedules tab in the new pattern.
7. Volume endpoint + volume sheet.
8. Playwright: test "all buttons ≥ 44×44 px" and "no target overlap" for all viewports; update the style guide.

**Acceptance:** 390×844 shows ≥ 5 scenes without scrolling · each scene on/off in 1 tap · no interactive area < 44 px · no horizontal overflow · Lighthouse accessibility ≥ 95.

## Implementation — deviations

| Concept | Implementation | Reason |
|---|---|---|
| ⋯ sheet "Edit / Delete" | ⋯ sheet "Edit / Volume"; delete only in the edit sheet | Request: destructive action two steps away (decision ledger "Delete only in the edit sheet") |
| Favorites "in the overflow" | its own text button "From Sonos favorites" below the segmented control | No extra menu needed, stays tappable at 44 px and easy to find |
| Starting volume live from the speaker | last set value, otherwise the scene volume | `SpeakerController` has no `GetVolume`; the Sonos driver stays unchanged |
| Day chips 44 px | 44 px round in a 7-column grid; on 390 px about 7–8 px gap | All 7 days in one row, acceptance "≥ 44 px" |
| Switch 51×31 | 44×26 in a 48 px hit area | Pico switch proportions; the hit area counts |
