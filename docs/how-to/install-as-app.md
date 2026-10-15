# Install as an app (laptop, iPhone, Android)

The web GUI is an installable web app ([ADR 0016](../adr/0016-web-app-pwa.md)): its own icon, its own window without a browser bar, and a phone-sized layout on desktop.

**Prerequisite:** Open it over **HTTPS** (`https://sonos.example.com`) or `http://localhost`. Over `http://<Pi-IP>:<port>` on the LAN the GUI works but cannot be installed (browsers require a secure context).

Cloudflare Access: sign in first (or open it from the home network), then install. If the Access session expires, the app shows the Access sign-in again the next time you open it.

## Laptop — Chrome or Edge

1. Open `https://sonos.example.com`.
2. Click the install icon on the right of the address bar (monitor with an arrow), or use the menu **⋮ → Cast, save, and share → Install page as app** (Edge: **… → Apps → Install this site as an app**).
3. Confirm "Hallido".

On first launch the app opens a phone-sized window (about 420 × 860). The operating system remembers its size and position afterward. In a normal browser tab the GUI also appears as a centered phone-width column.

To uninstall: in the app, use **⋮ → Uninstall "Hallido"** (Chrome), or go to `chrome://apps` / `edge://apps`.

## iPhone — Safari

1. Open `https://sonos.example.com` in **Safari**.
2. Tap Share → **Add to Home Screen** → name it "Sonos" → **Add**.

The app starts without Safari's bars; the header and tab bar keep clear of the notch and the home indicator. Note: the status bar is transparent with white text, which is hard to read in the light appearance.

## Android — Chrome

1. Open `https://sonos.example.com` in Chrome.
2. Use the menu **⋮ → Install app** (or the "Install Hallido" banner).

## Updates

After `sonos-agent update` on the Pi, the app shows the new version the **next time you open it**. The page always comes from the agent first and from the cache only when there is no connection. If the app stayed open the whole time, it reloads when you return to it (not in the middle of an open dialog).

API calls (status, volume, groups) are never cached.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| No install icon | Using HTTPS? Signed in to Cloudflare Access? Reload the page once. In Chrome DevTools, *Application → Manifest* shows errors. |
| Old interface despite an update | Close and reopen the app. If that does not help: DevTools → *Application → Service Workers → Unregister*, or clear the site data (iPhone: Settings → Safari → Advanced → Website Data). |
| Empty scene list without a network | Expected: offline you only get the interface, no data. The status shows "offline". |
