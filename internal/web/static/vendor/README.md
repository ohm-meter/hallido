# Vendor — frontend libraries

Static third-party assets, embedded via `go:embed` (`internal/web/static/`).

## Pico.css

| Field | Value |
|------|------|
| Version | **2.1.1** (2025-03-15) |
| Source | https://github.com/picocss/pico/releases/tag/v2.1.1 |
| License | [MIT](https://github.com/picocss/pico/blob/main/LICENSE.md) |
| Files | `pico.min.css` (runtime), `pico.css` (reference, unminified) |

Loaded in `index.html` **before** `style.css`. Theme via `html[data-theme="dark"|"light"]`
(kept in sync with `sonos-theme` in `localStorage`). Project overrides for `--pico-*` live in
`style.css` (section “Pico.css — Token-Mapping”).

Update: download the new release files, update the version here and in `docs/styleguide.md`,
run `make test-gui`.

## Lucide icons

| Field | Value |
|------|------|
| Version | **1.17.0** |
| Source | https://github.com/lucide-icons/lucide |
| License | [ISC](https://github.com/lucide-icons/lucide/blob/main/LICENSE) |
| Files | `lucide/*.svg` (reference), runtime helper `icons.js` |

Details: `lucide/README.md`. Loaded in `index.html` **before** `app.js`.
