# ADR 0019: English as the project language

## Status

Accepted (2026-10-15)

## Context

The project started as a private tool with German docs, German code comments
and a German web app. It is now published as open source on GitHub
(`ohm-meter/hallido`) for anyone who runs Sonos speakers — most of them do
not read German. Keeping two languages for every doc would double the
maintenance work for each change.

## Decision

English is the source language of the project:

1. **Step 1 — documentation:** README, SECURITY, CONTRIBUTING, CHANGELOG, all
   of `docs/` (tutorial, how-tos, reference, explanation, ADRs, plans,
   OpenAPI) and `AGENTS.md` are English. Doc file names are English.
   Private operations notes that are not exported (runbook of the author's
   installation, hardware validation, Pi/Forgejo/Cloudflare how-tos, handover)
   may stay German.
2. **Step 2 — web app, API and CLI texts:**
   - The web app ships English and German texts (`i18n.js`); English is the
     default, German is used when the browser language starts with `de`.
     Hash routes (`#szenen`, `#zeitplaene`, `#boxen`) stay unchanged so
     bookmarks and installed apps keep working.
   - API error messages and hints, log lines and the installer/CLI output are
     English only (they are read by automations and admins; no `Accept-Language`
     handling).
   - Terms follow [the glossary](../reference/glossary.md).
3. **From now on:** commit messages and new code comments are English. Existing
   German code comments are translated when the code is touched.
4. Example data uses English room names (`Living Room`, `Kitchen`, …) and the
   scene IDs from `config.example.json`.

## Consequences

- + One language for contributors, issues and docs; the public README can be
  read by everyone.
- + The glossary keeps docs, UI and API wording consistent.
- − Until step 2 is done, the docs use the English UI labels while the web app
  still shows German ones.
- − Doc file names changed; old links to German file names break (the project
  had no public links yet).
