# ADR 0016: Installable web app (manifest + service worker) and phone column

## Status

Accepted (2026-10-15)

## Context

- The GUI should be installable **as an app** on laptop and phone (own window/icon, no browser tab) and open in **phone format** on a PC — operated just like on a phone.
- Externally it runs through Cloudflare Tunnel + Access (`sonos.example.com`); the zone sets a browser cache TTL of 4 h. That is why the agent already serves ETag + `no-cache` and appends `?v=<hash>` to asset URLs.
- A service worker (SW) is a **new runtime component** in the browser: if built wrongly, it can serve stale files — exactly the problem the asset hash solved. And it must never serve API responses (status, volume, groups) from a cache.
- No build step, no new dependency (ADR 0001/0010).

## Decision

### Manifest and icons

- `internal/web/static/manifest.webmanifest` (`name` "Hallido", `display: standalone`, `start_url`/`scope` `/`, colors = token `--bg`); the agent sets the content type `application/manifest+json` itself (Go does not know the extension).
- `<link rel="manifest" … crossorigin="use-credentials">`: otherwise Chrome loads the manifest without cookies — behind Cloudflare Access this ends up on the login page.
- Icons (Lucide "radio" in amber on `--bg`) as an SVG source plus checked-in PNGs (192/512, maskable, Apple touch 180). Re-render only when the icon changes: `node scripts/dev/render-icons.js` (Playwright, dev only).

### Service worker (`static/sw.js`, served as `/sw.js`)

- At startup the agent injects the **version** (hash over all embedded files) and the **app shell list** (page, assets with `?v=`, manifest, icons) into `sw.js`. Every change to the GUI thus changes `sw.js` byte for byte → the browser installs the new SW on the next visit.
- Strategy:
  - `/api/`: **no** `respondWith` — always straight to the network, never cached.
  - Page (navigation) and unversioned files: **network first**, cache only when offline (or on a 5xx from the tunnel for the page).
  - Assets with `?v=`: cache first (the URL changes with the content).
  - Only the app's own successful responses are stored — no redirects to Cloudflare Access.
- Update: `skipWaiting` + `clients.claim`, old caches are deleted. The SW reports its version; if it differs from that of the loaded page (`<meta name="app-version">`), `pwa.js` reloads — never in the middle of an open dialog, but on the next return to the app. On return (`visibilitychange`), the app actively asks for a new SW.
- `sw.js` itself gets ETag + `no-cache`; browsers bypass the HTTP cache for the SW script anyway.
- Without a secure context (e.g. `http://<Pi-IP>:8080` on the LAN), no SW registers — the GUI runs unchanged.

### Phone column instead of desktop rail

- On all widths, the GUI is a single column of at most `--col-w` (460 px), centered; tab bar, "Now playing", and bottom sheets stay within this column. The rail view from 900 px (left navigation, 2 scene columns) is removed without replacement (user's decision, 2026-10-15).
- Installed app on a PC: on first start `window.resizeTo(420, 860)` where the browser allows it (Chrome/Edge app window); afterward the window keeps the size the user chooses.

## Consequences

- + Installable on laptop (Chrome/Edge), Android, and iPhone; opens the app shell offline instead of an error page.
- + `sonos-agent update` is visible on the next open — the page always comes from the network first.
- − New source of errors in the browser. Emergency fix: clear the website data for `sonos.example.com` in the browser settings or uninstall the app (how-to `install-as-app.md`).
- − iOS: status bar `black-translucent` (fits the default dark theme; in the light theme the status bar text stays white).
- Playwright tests block service workers by default (`serviceWorkers: "block"`) so that `page.route` stubs take effect; `pwa.spec.js` allows it deliberately.
