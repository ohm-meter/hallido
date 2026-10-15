# ADR 0014: Pico.css as a locally embedded base

## Status

Accepted (2026-10-14), supplements ADR 0010.

## Context

ADR 0010 prohibited CSS frameworks. Since then, the GUI uses Pico.css v2.1.1 (`vendor/pico.min.css`) as a semantic base, with project-specific overrides and tokens in `style.css`.

## Decision

- Pico.css stays embedded as a **local, minified file** (MIT license, see `internal/web/static/vendor/README.md`).
- Unchanged from ADR 0010: no npm/build step for the GUI, no CDN, CSP `style-src 'self'`, design tokens in `style.css`.
- Only runtime assets are shipped via `go:embed` (`index.html`, `app.js`, `icons.js`, `style.css`, `vendor/pico.min.css`). `vendor/pico.css` and `vendor/lucide/*.svg` remain in the repo as a reference, but not in the binary.

## Consequences

- Pico updates: replace the file, update the version in `vendor/README.md` and `docs/styleguide.md`, run `make test-gui`.
- No further framework without a new ADR.
