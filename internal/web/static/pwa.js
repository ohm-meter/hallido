"use strict";

// Installierbare Web-App (ADR 0016): Service Worker, Updates, Fenstergroesse,
// Statusleisten-Farbe. Unabhaengig von app.js; ohne Service-Worker-Unterstuetzung
// (z. B. http:// im LAN ausser localhost) laeuft die GUI unveraendert.
(function () {
  const WINDOW_SIZED_KEY = "sonos-window-sized";
  const APP_WINDOW = { width: 420, height: 860 };
  const appVersion = document.querySelector('meta[name="app-version"]')?.content || "";

  /** theme-color folgt dem gewaehlten Design (nicht nur prefers-color-scheme). */
  function syncThemeColor() {
    const bg = getComputedStyle(document.documentElement).getPropertyValue("--bg").trim();
    if (!bg) return;
    for (const meta of document.querySelectorAll('meta[name="theme-color"]')) {
      meta.setAttribute("content", bg);
    }
  }

  /** Installierte App am PC: beim ersten Start Handy-Format, danach Nutzerwahl. */
  function sizeAppWindow() {
    if (!window.matchMedia("(display-mode: standalone)").matches) return;
    try {
      if (localStorage.getItem(WINDOW_SIZED_KEY)) return;
      localStorage.setItem(WINDOW_SIZED_KEY, "1");
    } catch (_) {
      return;
    }
    // Handy/Tablet: Fenster gehoert dem System.
    if (window.outerWidth <= APP_WINDOW.width + 100) return;
    try {
      window.resizeTo(APP_WINDOW.width, APP_WINDOW.height);
    } catch (_) {
      /* Browser erlaubt es nicht -- kein Fehler */
    }
  }

  let reloadPending = false;

  /** Neue Version aktiv: neu laden, aber nie mitten in einem offenen Dialog. */
  function reloadForUpdate() {
    if (document.querySelector("dialog[open]") || document.hidden) {
      reloadPending = true;
      return;
    }
    window.location.reload();
  }

  function registerServiceWorker() {
    if (!("serviceWorker" in navigator) || !window.isSecureContext) return;
    navigator.serviceWorker.addEventListener("message", (event) => {
      const data = event.data || {};
      if (data.type === "sw-activated" && appVersion && data.version !== appVersion) {
        reloadForUpdate();
      }
    });
    navigator.serviceWorker
      .register("sw.js", { scope: "/" })
      .then((reg) => {
        // App lange offen/im Hintergrund: beim Zurueckkehren nach Update fragen.
        document.addEventListener("visibilitychange", () => {
          if (document.hidden) return;
          if (reloadPending && !document.querySelector("dialog[open]")) {
            window.location.reload();
            return;
          }
          reg.update().catch(() => {});
        });
      })
      .catch(() => {
        /* z. B. Cloudflare-Access-Login abgelaufen -- GUI laeuft ohne SW */
      });
  }

  syncThemeColor();
  new MutationObserver(syncThemeColor).observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["data-theme"],
  });
  sizeAppWindow();
  window.addEventListener("load", registerServiceWorker);
})();
