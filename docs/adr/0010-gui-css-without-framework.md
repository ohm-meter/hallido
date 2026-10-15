# ADR 0010: Web GUI without a CSS framework

## Status

Accepted (2026-05-28); partially superseded for the base stylesheets by [ADR 0014](0014-pico-css-base.md) (Pico.css embedded locally). The principles (no build step, no CDN, no Tailwind/Bootstrap) still apply.

## Context

The embedded GUI (`internal/web/static/`) is embedded into the Go binary as static files. There is no npm/Vite build step. Occasionally the question comes up whether Tailwind CSS, Bootstrap, or a comparable UI kit would simplify maintenance.

Requirements remain:

- zero external runtime dependencies in the frontend
- small binary, suitable for the Pi
- CSP-friendly (no CDN styles/scripts)
- Styleguide with design tokens (`:root`)
- Accessibility and responsiveness without framework overhead

## Decision

**No CSS framework.** Continue with native CSS: custom properties, Flexbox, and CSS Grid, added incrementally in the existing `style.css` and `docs/styleguide.md`.

Specifically:

- Layout and components via project-specific classes (`.scene-wrap`, `.scenes-toolbar`, `.grid`, …)
- Breakpoints via `@media` in `style.css` (including 320px, 768px, 1200px)
- No Tailwind/Bootstrap build, no CDN link in `index.html`

## Rationale

| Criterion | Vanilla CSS | Tailwind / Bootstrap |
|-----------|-------------|----------------------|
| Binary / deploy | Embedded bytes only | CDN = network + CSP; build = toolchain |
| Dependencies | 0 (AGENTS.md) | Package or CDN |
| Look | "Control Panel" styleguide | Risk of a generic kit look |
| GUI maintenance | ~3 files, clearly bounded | Class soup or overrides |

Tailwind only pays off when the UI surface grows considerably **and** a build step is accepted (PostCSS in CI). Bootstrap fits poorly with the dark, token-based panel look and would mostly mean override work.

## Consequences

- GUI changes: tokens + styleguide, `make test-gui` after CSS/markup changes
- A framework only after a new ADR, if scope or team requirements justify it
- Reference: `docs/styleguide.md` (section "No CSS framework")

## Alternatives (rejected)

1. **Tailwind (CLI in CI)** — build pipeline, more diff noise; disproportionate for the current GUI size.
2. **Bootstrap 5 (CDN or vendored)** — different design vocabulary, conflicts with the styleguide, CSP/CDN drawbacks.
3. **Pico.css / Water.css** — less control over states (scene `data-state`, amber theme).
