# Style guide — web GUI

Applies to `internal/web/static/` (`index.html`, `style.css`, `app.js`).

## CSS base: Pico.css (local)

- **Pico.css v2.1.1** lives at `vendor/pico.min.css` (no CDN); metadata in `vendor/README.md` (MIT).
- Load order: `vendor/pico.min.css` → `style.css` (overrides for `--pico-*` + components).
- **Theme:** `html[data-theme="dark"|"light"]` — Pico and hallido tokens together; toggle as before (`sonos-theme`).
- **Direction: semantic HTML.** Build forms, dialogs and new UI step by step with native markup (`<button>`, `<dialog>`, `<label>`, `<input>`) instead of purely div-based classes — Pico styles tags directly; existing `.scene-*` components stay until refactored.
- Tailwind/Bootstrap remain excluded ([ADR 0010](adr/0010-gui-css-without-framework.md)); Pico is a deliberate, minimal exception (one embedded CSS file, no build).

## Icons: Lucide (local)

- **Lucide 1.17.0** — individual SVGs under `vendor/lucide/` (reference), runtime via `icons.js` (no CDN, no npm).
- Helper: `createLucideIcon(name, className)`; static markup uses `[data-lucide="…"]` (replaced on load).
- **Sizes:** header/tab/⋯ 22px (`.bar-icon`, `.tab-icon`, `.scene-action-icon`), play/stop 22px filled (`.play-icon`), theme 20px, buttons 14–18px — color via `currentColor`.
- **Icon-only buttons:** always set `aria-label` (e.g. "Actions: scene", "Stop: scene", "Living Room louder"); SVG with `aria-hidden="true"`.
- Icons in use: `circle-play`, `clock`, `speaker` (tabs), `play`, `pause`, `square`, `unlink`, `ellipsis`, `plus`, `minus`, `x`, `search`, `volume-2`, `pencil`, `trash-2`, `save`, `chevron-right`, `refresh-cw`, `sun`, `moon`.

Updating: copy SVGs from the Lucide repo, keep `LUCIDE_PATHS` in `icons.js` in sync, update `vendor/lucide/README.md` + the version here.

## Design principles

Inspired by established guidelines, adapted to a **minimal control panel** (not a marketing site):

| Reference | Relevant for us |
|-----------|-----------------|
| [web.dev — Learn Accessibility / Design](https://web.dev/learn/accessibility/) | Contrast, focus, semantic HTML, touch targets |
| [Material Design 3 — Density & layout](https://m3.material.io/foundations/layout/understanding-layout/overview) | Compact dashboard density, clear hierarchy, cards as units |
| [Apple HIG — Layout](https://developer.apple.com/design/human-interface-guidelines/layout) | Readable spacing, safe areas, sufficient tap targets (~44pt) |

**Character:** "precision instrument" (reference lock, [plan](plans/gui-mobile-redesign.md)) — matte near-black canvas, flat surfaces with 1px edges instead of shadows, **one** signal accent (amber), mono uppercase for labels/metadata. Warm dark theme (default), light theme via the toggle in the Speakers tab. **Mobile-first:** it is mostly used on a phone, one-handed — large targets, navigation in the thumb zone.

**Color roles:** amber = "playing/active" + exactly one primary action per screen · red only for delete/offline/error · green only for online status · mono only for metadata, never body text or main button text.

## Design tokens (`:root` / `[data-theme="light"]`)

Dark is the default (`:root`). The light theme overrides the palette on `html[data-theme="light"]`.

| Token | Dark | Light | Use |
|-------|------|-------|-----|
| `--bg` | `#0d0c0a` | `#f4efe3` | Page background |
| `--panel` | `#16140f` | `#faf7f0` | Surfaces, dialog |
| `--panel-2` | `#1f1c14` | `#f0ebe0` | Cards, chips |
| `--edge` | `#2c2820` | `#d4ccc0` | Borders, dividers |
| `--ink` | `#f4ead3` | `#1c1610` | Body text |
| `--muted` | `#9a8f73` | `#6b6152` | Secondary text |
| `--amber` | `#ff9d2f` | `#c96a00` | Primary accent (light: darker for AA) |
| `--amber-dim` | `#b56d18` | `#a85800` | Hover borders |
| `--green` | `#6fcf6b` | `#2a7a3a` | Online, success |
| `--red` | `#e8634a` | `#c0392b` | Offline, error |

Additional theme tokens: `--primary-on-amber`, `--surface-hover`, `--badge-neutral-bg`, `--shadow-*`, `--backdrop`.

Also `--amber-edge` (border of playing rows/mini player, per theme).

**Theme toggle:** "Appearance" settings row in the Speakers tab (`#themeToggle`, value "Dark/Light"); the choice is stored in `localStorage` (`sonos-theme`); without a stored choice `prefers-color-scheme` applies, otherwise dark.

### Touch and layout tokens

| Token | Value | Role |
|-------|-------|------|
| `--tap-min` | `48px` | Minimum size of all touch targets (Material 48dp ≥ Apple 44pt) |
| `--tap-primary` | `56px` | Play/stop button, primary footer button |
| `--tap-gap` | `8px` | Minimum spacing between adjacent targets |
| `--row-h` | `72px` | Scene/schedule row |
| `--bar-h` | `48px` | Header bar |
| `--tabbar-h` | `64px` | Tab bar (+ `safe-area-inset-bottom`) |
| `--col-w` | `460px` | Width of the phone column (all viewports) |
| `--col-inset` | `max(0, (100% − col-w)/2)` | Side offset of fixed elements (tab bar, mini player) from the column |
| `--radius-pill` | `9999px` | Buttons, segments, chips |
| `--radius-row` | `12px` | Rows, sheets, mini player |
| `--space-1…6` | `4/8/12/16/24/32px` | Spacing scale |

Body font 17 px (inputs ≥ 16 px prevent iOS zoom). Hard minimum for every visible target: **44×44 px** (`touch-targets.spec.js`).

| Token | Value | Use |
|-------|-------|-----|
| `--icon-theme` | `20px` | Theme toggle (`.theme-icon`) |
| `--icon-btn` | `14px` | Base size of `.btn-icon` (18px in pills) |
| `--icon-scene-action` | `16px` | Legacy token; `⋯` uses 22px |
| `--sans` | System UI stack | UI text |
| `--mono` | Monospace stack | Brand, status, metadata |

Add new colors only if no token fits — keep the palette small.

## Typography

- **Header title:** `.bar-title` — sans 17px, 650.
- **Scene name:** `.scene-name` — sans 17px, 600; rooms below in mono 12.5px (`.scene-sub`).
- **Schedule time:** `.schedule-time` — mono 22px, 600.
- **Labels/metadata:** mono 11–12px uppercase (`.section-label`, `.field-label`, `.tab`, status).
- **Brand:** `.brand-text` in the footer of the Speakers tab — mono 11px uppercase, amber on `<em>`.
- **Hints:** `.hint` — mono 12.5px; `.hint.error` in `--red`.

Don't load additional web fonts (performance, offline Pi).

## Layout & navigation

- **Three tabs** (hash routing, no reload): `#szenen` (start) · `#zeitplaene` · `#boxen` (Scenes · Schedules · Speakers). Back button and bookmarks work; an unknown hash → `#szenen`.
- **Tab bar** (`nav.tabbar`) fixed at the bottom, 64 px + safe area, icon + mono label, active tab amber (`aria-current="page"`). At all widths it sits in the **phone column** (see below) — no desktop rail anymore.
- **Header** (`header.bar`) sticky, one 48 px row: status dot + "15 speakers" · title · `+` (context-dependent "New scene"/"New schedule", off in the Speakers tab).
- **Content** (`main.views`) scrolls with the document; space at the bottom for the tab bar and mini player (`--np-space`).
- **Phone column (ADR 0016):** The GUI is at most 460 px wide (`--col-w`) everywhere and centered — on a PC it looks like a phone. Tab bar, "Now playing" and bottom sheets stay in the column; from 600 px, 1px edges (`--edge`) frame the column, tab bar and sheets on the sides.
- **Maintenance separate from playback:** speaker list, saving the config, token and appearance live only in the Speakers tab.

## Components

### Scene row (`.scene-row`)

72 px; on the left a round play/stop button (`.scene-play`, 56 px), in the middle name + rooms (`.scene-main`, whole area tappable), on the right `⋯` (`.scene-more`, 48 px) → sheet with "Edit" and the rooms' volumes. Tap starts; tapping a playing scene stops it (`POST …/stop`); tapping a paused scene resumes it (`POST …/resume`, doesn't reload). States via `data-state`: `playing`/`partial` → amber button with stop + 3px amber edge; `paused` → neutral, status "paused" in body-text color, play button ("Resume: …"); `playing_other` → neutral "playing something else" (no amber, button stays play); `loading`/`stopping`/`pausing`/`resuming` → pulsing border; `error`/`offline` → red. The ⋯ sheet shows "Pause" or "Resume" only while the scene is playing or paused, respectively. Polling `GET /api/v1/scenes/status` every 8 s.

### "Now playing" (`#nowPlaying`)

Mini player above the tab bar as soon as a scene is playing or paused (a playing one takes precedence): equalizer, name, rooms, neutral pause/resume button (48 px, `--edge` border) and amber stop (48 px) as the only amber action. Paused: `--edge` border instead of amber, equalizer stands still. Tapping the text opens the scene's ⋯ sheet (edit + volumes). Visible in all tabs.

### Bottom sheet (`dialog.sheet`)

Native `<dialog>` + `showModal()` (focus trap, Esc). Docked at the bottom, handle, header with title + close (48 px), scrolling body (`.sheet-body`, the **only** scroll area), optional sticky footer (`.sheet-foot`, Cancel + amber primary). `.sheet-full` = full screen within the column (editors). Docked at the bottom at all widths, at most `--col-w` wide. Tapping the dimmed backdrop closes it.

### Scene editor (`#sceneDialog`)

Name → source as a segmented control (`.seg`, 44 px pill segments: Search radio · From speaker · URL; "From Sonos favorites" as a text button) → rooms as 56 px rows (`.member-row`), with slider + −/+ once selected (`createVolumeControl`, step 1) → "Advanced" (`<details>`, slug) → Delete (only when editing, red, with confirmation). Footer "Save & play".

### ⋯ sheet of a scene (`#sceneMenuSheet`)

"Edit" at the top, below it the volume per room: name, −/+ (44 px, step **1**), slider with a 28 px thumb and filled track (`--fill`, `touch-action: pan-y`) and value. Changes are saved to the scene with a debounce (`PUT /api/v1/config/scenes/{id}`) and show up in the edit dialog; if the scene is playing, they're also applied live to the speaker (`PUT /api/v1/speakers/{name}/volume`). Tapping "Now playing" opens the same sheet.

### Schedules

Row (`.schedule-row`): large time, below it scene + days, next run, last error if any; tap opens the editor. Switch (`input[role=switch]`) in a 48 px hit area (`.switch-hit`). "Next start" at the top (`#scheduleSummary`). Editor sheet: 7 round day chips at 44 px each, quick picks Mon–Fri / Weekends / Daily, native `<input type="time">`, Delete in the sheet. Polling every 60 s.

### Speakers tab

"Refresh" (amber pill, the tab's one primary action), list of `.speaker-card` (60 px, online dot, IP · now playing in mono); tap → now-playing panel (`.discover-panel`). Settings as 56 px rows (`.setting`).

**Groups** (`GET /api/v1/groups`, every 8 s only while the Speakers tab is open): speakers appear in their current Sonos group (`.group-block`, coordinator first). Groups with several speakers or with playing/paused playback get a header (`.group-head`, 64 px): coordinator "+N", mono status + scene or title, on the right neutral 48 px buttons (`.group-btn`) Pause/Resume and Stop (Stop only while playing/paused). A playing group gets a 3px amber edge like the scene row. All remaining speakers are listed under "Standalone" (`#speakerList`); without a groups response the flat list remains.

**Leave group:** in the speaker detail, `.pill.danger` (`#discoverUngroupBtn`), only for speakers in a group of ≥ 2 speakers — member: "Leave group", coordinator: "Ungroup all". Always with a confirmation dialog (`#ungroupDialog`) that states the effect; partial failure per speaker is shown in the hint.

### Buttons

`.pill` / `.primary` / `.ghost`: pill shape, min. 48 px, sans 15px. `.pill.danger` has a red outline. `.icon-btn` is 48×48, round.

## Error states

| UI | Meaning |
|----|---------|
| `.hint.error` | Global hint (token, play error, name warning) |
| `data-state="playing"` | Scene is playing (amber button, stop) |
| `data-state="paused"` | Scene is paused (neutral, button resumes) |
| `data-state="playing_other"` | Coordinator is playing something else (neutral) |
| `data-state="stopped"` | Stopped / no playback |
| `data-state="offline"` | Coordinator unreachable |
| `data-state="error"` | Scene failed |
| `data-state="partial"` | Partial failure |
| Status `.dot.off` | No speaker online / agent down |

Show error texts from the API (`message`, `hint`) unchanged — no rephrasing in the GUI.

## Accessibility

- **Contrast:** check `--ink` on `--panel` and `--amber` on a dark button background for new colors (aim for WCAG AA).
- **Focus:** `:focus-visible` on interactive elements (cards, buttons, inputs, links).
- **Live region:** `#grid`, `#scheduleList` with `aria-live="polite"`.
- **Motion:** sheet entrance and equalizer respect `prefers-reduced-motion`.
- **Language:** `<html lang="en">`.
- **Touch:** every visible target ≥ 44×44 px (tokens 48/56 px), ≥ 8 px spacing, no overlap — checked automatically in `tests/gui/touch-targets.spec.js`.

No `outline: none` without a visible replacement.

## Internationalization

The web app is bilingual (ADR 0019): English by default, German when
`navigator.language` starts with `de`.

**Files:**

- `internal/web/static/locales/en.json`, `de.json` — the dictionaries: flat
  JSON objects `"key": "text"`, keys sorted, 2-space indent, UTF-8. English is
  the reference and the fallback.
- `internal/web/static/i18n.js` — only the loader: picks the language
  (`LANGUAGES`), fetches `locales/<lang>.json?v=<app version>` (plus `en.json`
  as fallback), sets `<html lang>` and translates static markup. It exposes
  `I18N.ready` (a Promise that never rejects); `app.js` renders only after it
  resolves. Until then the page shows the English static texts of `index.html`.
- The agent embeds `locales/*.json` (`internal/web/web.go`); the service worker
  keeps them offline with the same versioned URLs.

**Lookup:** `t(key)` uses the chosen language, then English, then the key
itself (with one `console.warn` per missing key).

To add a text:

1. Add the key to **every** locale file (same key, no empty value, keep the
   keys sorted). English and German wording follow
   [the glossary](reference/glossary.md).
2. Static markup: write the English text in `index.html` (the page reads
   English without JS) and add `data-i18n="key"` (text),
   `data-i18n-aria`, `data-i18n-title` or `data-i18n-placeholder`.
3. JavaScript: `t("key", { name })` fills `{name}` placeholders;
   `tn("key", n)` picks `key.one` (n = 1) or `key.other` and fills `{n}` —
   plurals are always these two keys. Dates and sorting use `I18N.locale`
   with `Intl` / `localeCompare`.
4. Never call `t()` at module load time in `app.js` (the dictionaries are not
   loaded yet): module-level label tables use `translated({ name: "key" })`,
   which translates on access.
5. API error messages (`message`, `hint`) are shown as delivered — not translated in the GUI.

**No hard-coded visible text:**

- Every element in `index.html` with visible text has `data-i18n`, and every
  `aria-label` / `title` / `placeholder` has its `data-i18n-*` counterpart.
  Exceptions (allowlist in `tests/gui/i18n.spec.js`): the brand line,
  values filled in by `app.js` (`#speakerCount`, `#scheduleTz`), text without
  words (numbers, symbols) and URLs.
- `app.js` has no string literal with German umlauts/ß and assigns no literal
  words to `textContent`, `innerText`, `title`, `placeholder`, `ariaLabel` or
  `setAttribute("aria-label" | "title" | "placeholder", …)`. Checked by
  `scripts/test/i18n-lint.js` (part of `gui-static.sh`; comments are ignored,
  `ALLOWED_LITERALS` lists the few exceptions).
- The product name **Hallido** and "Powered by siraris" are not translated.

Hash routes (`#szenen`, `#zeitplaene`, `#boxen`) stay unchanged.
`tests/gui/i18n.spec.js` checks English and German labels, key parity, the
English fallback and the markup rule; `go test ./internal/web/` checks that
the locale files are embedded and in parity. The other specs run with
`locale: "de-DE"`. Adding a language:
[how-to](how-to/add-a-language.md).

## Mobile

- `viewport-fit=cover`, `100dvh`, safe-area padding (header at the top, tab bar at the bottom).
- Breakpoints: < 480 px (frame padding 14 px), ≥ 600 px (column framed on the sides). Nothing changes at wider widths.

## Web app (PWA)

- Manifest `manifest.webmanifest`, service worker `sw.js` (filled by the agent with version + app shell), logic in `pwa.js` — [ADR 0016](adr/0016-web-app-pwa.md), [how-to](how-to/install-as-app.md).
- **Icon:** Lucide `radio` in `--amber` on `--bg`, source `icons/icon.svg` / `icons/icon-maskable.svg`; re-render PNGs with `node scripts/dev/render-icons.js`.
- `theme-color` (two metas for light/dark) follows the chosen appearance (`pwa.js`).
- Standalone: safe area as in the browser (`viewport-fit=cover`, header/tab bar with `env(safe-area-inset-*)`); iOS status bar `black-translucent`.
- Add new files that should be available offline to `shellFiles` (`internal/web/web.go`) or `versionedAssets`. Never put `/api/` in the service worker cache.
- Acceptance at 390×844: ≥ 5 scenes without scrolling, each scene on/off in 1 tap, no horizontal overflow.

## JavaScript conventions

- `"use strict"`, no build step.
- Relative API paths (`/api/v1/...`).
- Token in `localStorage`; Latin-1 validation before fetch (fetch header limit).
- Minimal DOM updates; keep state in `data-*` attributes and classes.

## What to avoid

- External UI kits, icon fonts, large animations.
- Generic "AI slop" aesthetics (Inter + purple gradient).
- Inline styles except where dynamically unavoidable.
- Grid/gradient decoration, card shadows, multiple accent colors, huge centered headers.
- Hidden gestures (long-press, swipe-to-delete) as the only way to an action.
- Custom scroll areas inside sheets (scroll-in-scroll).

## Checklist for GUI changes

1. Use tokens from `:root`; don't scatter hard-coded hex values.
2. Check `:focus-visible` and touch size.
3. Test error and loading states (token, 401, partial failure).
4. Layout: `make test-gui` (Playwright — mobile/tablet/desktop, see `docs/how-to/testing.md`). Add new sheets/tabs to `touch-targets.spec.js`.
5. Check ~320px mobile width manually if the Playwright test doesn't cover it yet.
6. No new npm/build dependencies; update Pico only via `vendor/` (see `vendor/README.md`).
