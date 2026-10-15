# Plan: Internationalization roadmap

**Status:** step 1 and 2 of [ADR 0019](../adr/0019-english-project-language.md) done (2026-10-15);
this plan lists what exists and what comes next. How-to for contributors:
[Add a language](../how-to/add-a-language.md). Terms: [glossary](../reference/glossary.md).

## Current state

| Area | Language(s) | Where the texts live |
|------|-------------|----------------------|
| Web app (labels, hints, dialogs, dates) | English (default), German | `internal/web/static/locales/<lang>.json`, loaded by `i18n.js` |
| API error messages and hints | English | Go code (`internal/apperr`, handlers, drivers) |
| Logs | English | Go code (`slog`) |
| Installer and CLI (`install.sh`, `sonos-agent`) | English | shell scripts |
| Docs, ADRs, README | English | `docs/`, overlay `scripts/release/public/` |

The web app picks German when the browser language starts with `de`,
otherwise English. A key missing in the chosen language falls back to English,
a key missing in English shows the key itself (and logs a warning once).

## File format and conventions

- One flat JSON object per language: `locales/en.json`, `locales/de.json`, …
  English is the reference file; every other file has exactly the same keys
  (checked by the Go test in `internal/web` and the Playwright `i18n.spec.js`).
- **Keys:** `area.name` in lowerCamelCase segments, grouped by screen or
  feature (`tab.scenes`, `scene.delete`, `fav.none`, `schedule.nextStart`). Never
  reuse a key for a different meaning; rename instead.
- **Placeholders:** `{name}` — same names in every language, never translated.
- **Plurals:** separate keys `key.one` (exactly 1) and `key.other`, used via `tn(key, n)` with `{n}`;
  languages with more plural forms add their own forms.
- **Dates, times, weekdays:** formatted with `Intl` in the active locale, not stored as text.
- **No visible text in markup or JS:** `index.html` carries English defaults
  with `data-i18n*` attributes; `app.js` only calls `t()`/`tn()`. A static
  check fails the build on new hard-coded texts.
- Wording follows the glossary; new terms go into the glossary first.

## Workflow for a new language

1. Copy `locales/en.json` to `locales/<code>.json`, translate the values.
2. Register the code in `i18n.js` (language detection).
3. Run `make test` and `make test-gui` — the coverage tests show missing keys.
4. Open a pull request; a second native speaker reviews if possible.

Details: [Add a language](../how-to/add-a-language.md).

## Next steps

| Priority | Item | Notes |
|----------|------|-------|
| P2 | **Language switch in the web app** | Settings → Language (Automatic / English / Deutsch / …), stored in `localStorage`; "Automatic" keeps the browser language. |
| P2 | **API errors translated in the GUI** | The API returns stable error codes; the GUI could map `code` → locale key and show the API `message` only as a fallback. Keeps the API English for automations. |
| P3 | **Installer and CLI** | Stay English. Translating shell output (gettext) is not planned — admins read logs and docs in English. |
| P3 | **Docs in other languages** | Only English is maintained. Community translations would live under `docs/<lang>/` and be marked as possibly outdated. |
| P3 | **Binary/CLI name `hallido`** | Product name is Hallido ([ADR 0020](../adr/0020-product-name-hallido.md)); renaming `sonos-agent`/`sonos-radio-agent` needs a migration (service, paths, CLI symlink) and its own ADR. |
| P3 | **Right-to-left languages** | Not planned; the layout would need `dir="rtl"` support and checks. |
