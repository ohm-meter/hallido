// Service Worker der Web-App (ADR 0016): nur die App-Shell offline vorhalten.
//
// Regeln:
//   - /api/ wird nie angefasst (kein respondWith) -> immer direkt Netzwerk.
//   - Seite (Navigation) und unversionierte Dateien: Netzwerk zuerst, Cache nur
//     als Rueckfall ohne Verbindung -> ein "sonos-agent update" ist sofort sichtbar.
//   - Assets mit ?v=<Hash>: Cache zuerst (URL aendert sich mit dem Inhalt).
// VERSION und SHELL setzt der Agent beim Ausliefern ein (internal/web/web.go).
"use strict";

const VERSION = "__SW_VERSION__";
const SHELL = __SW_SHELL__;
const PREFIX = "sonos-shell-";
const CACHE = PREFIX + VERSION;

self.addEventListener("install", (event) => {
  event.waitUntil(
    (async () => {
      const cache = await caches.open(CACHE);
      // Einzeln statt addAll: eine fehlende Datei (z. B. Access-Login) bricht
      // die Installation nicht ab.
      await Promise.allSettled(
        SHELL.map(async (url) => {
          const res = await fetch(url, { cache: "no-cache" });
          if (cacheable(res)) await cache.put(url, res);
        })
      );
      await self.skipWaiting();
    })()
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      for (const key of await caches.keys()) {
        if (key.startsWith(PREFIX) && key !== CACHE) await caches.delete(key);
      }
      await self.clients.claim();
      for (const client of await self.clients.matchAll({ type: "window" })) {
        client.postMessage({ type: "sw-activated", version: VERSION });
      }
    })()
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;
  if (url.pathname.startsWith("/api/")) return; // nie cachen, immer Netzwerk
  if (url.pathname === "/sw.js") return;

  if (req.mode === "navigate") {
    event.respondWith(networkFirst(req, "/"));
  } else if (url.searchParams.has("v")) {
    event.respondWith(cacheFirst(req));
  } else {
    event.respondWith(networkFirst(req, url.pathname.slice(1)));
  }
});

/** Nur eigene, erfolgreiche Antworten (keine Weiterleitung zu Cloudflare Access). */
function cacheable(res) {
  return res && res.ok && res.type === "basic" && !res.redirected;
}

async function networkFirst(req, key) {
  const cache = await caches.open(CACHE);
  try {
    const res = await fetch(req);
    if (cacheable(res) && SHELL.includes(key)) await cache.put(key, res.clone());
    // Agent nicht erreichbar (Tunnel liefert 5xx): App-Shell statt Fehlerseite.
    if (req.mode === "navigate" && res.status >= 500) return (await cache.match(key)) || res;
    return res;
  } catch (err) {
    const hit = await cache.match(key);
    if (hit) return hit;
    throw err;
  }
}

async function cacheFirst(req) {
  const cache = await caches.open(CACHE);
  const hit = await cache.match(req);
  if (hit) return hit;
  const res = await fetch(req);
  if (cacheable(res)) await cache.put(req, res.clone());
  return res;
}
