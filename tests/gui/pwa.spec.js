// @ts-check
// Installierbare Web-App (ADR 0016): Manifest, Service Worker, Handy-Spalte.
const { test, expect } = require("@playwright/test");
const { gotoAppReady, readHorizontalOverflow, TOKEN } = require("./_layout-helpers");

const COL_W = 460;

test("Manifest: erreichbar, gueltig, Icons ladbar", async ({ page, request }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop", "Viewport-unabhaengig, einmal genuegt");
  await gotoAppReady(page);
  const link = page.locator('link[rel="manifest"]');
  await expect(link).toHaveAttribute("crossorigin", "use-credentials");
  await expect(page.locator('link[rel="apple-touch-icon"]')).toHaveCount(1);
  await expect(page.locator('meta[name="theme-color"]')).toHaveCount(2);

  const res = await request.get("/manifest.webmanifest");
  expect(res.status()).toBe(200);
  expect(res.headers()["content-type"]).toContain("application/manifest+json");
  const m = await res.json();
  expect(m.name).toBe("Hallido");
  expect(m.short_name).toBe("Hallido");
  expect(m.display).toBe("standalone");
  expect(m.start_url).toBe("/");
  expect(m.scope).toBe("/");
  expect(m.theme_color).toMatch(/^#[0-9a-f]{6}$/i);
  expect(m.background_color).toMatch(/^#[0-9a-f]{6}$/i);

  const sizes = m.icons.filter((i) => i.purpose === "any").map((i) => i.sizes);
  expect(sizes).toEqual(expect.arrayContaining(["192x192", "512x512"]));
  expect(m.icons.some((i) => i.purpose === "maskable" && i.sizes === "512x512")).toBe(true);
  for (const icon of [...m.icons, { src: "icons/apple-touch-icon.png", type: "image/png" }]) {
    const r = await request.get("/" + icon.src);
    expect(r.status(), icon.src).toBe(200);
    expect(r.headers()["content-type"], icon.src).toContain(icon.type);
  }
});

test.describe("Service Worker", () => {
  test.use({ serviceWorkers: "allow" });

  test("registriert, App-Shell aus dem Cache, /api/ immer Netzwerk", async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== "desktop", "Viewport-unabhaengig, einmal genuegt");
    await page.addInitScript((token) => localStorage.setItem("sra_token", token), TOKEN);

    const sw = await page.request.get("/sw.js");
    expect(sw.headers()["content-type"]).toContain("javascript");
    expect(sw.headers()["cache-control"]).toBe("no-cache");

    await page.goto("/");
    await page.waitForFunction(() => navigator.serviceWorker && navigator.serviceWorker.controller !== null, null, {
      timeout: 15_000,
    });

    /** @type {{ url: string, viaSW: boolean }[]} */
    const seen = [];
    page.on("response", (r) => seen.push({ url: r.url(), viaSW: r.fromServiceWorker() }));
    await page.reload();
    await expect(page.locator("#grid .scene-row").first()).toBeVisible();
    await page.evaluate(() => fetch("/api/v1/health").then((r) => r.json()));

    const nav = seen.find((r) => new URL(r.url).pathname === "/");
    expect(nav?.viaSW, "Seite laeuft ueber den Service Worker").toBe(true);
    const api = seen.filter((r) => new URL(r.url).pathname.startsWith("/api/"));
    expect(api.length).toBeGreaterThan(0);
    expect(api.filter((r) => r.viaSW).map((r) => r.url), "API-Antworten aus dem SW").toEqual([]);

    const keys = await page.evaluate(async () => {
      const all = [];
      for (const name of await caches.keys()) {
        const cache = await caches.open(name);
        for (const req of await cache.keys()) all.push(new URL(req.url).pathname + new URL(req.url).search);
      }
      return all;
    });
    expect(keys).toContain("/");
    expect(keys.some((k) => /^\/app\.js\?v=[0-9a-f]+$/.test(k))).toBe(true);
    expect(keys.some((k) => k.startsWith("/manifest.webmanifest"))).toBe(true);
    expect(keys.filter((k) => k.startsWith("/api/")), "API im Cache").toEqual([]);
  });
});

test("Handy-Spalte: zentriert, max. 460 px, kein Overflow", async ({ page }) => {
  await gotoAppReady(page);
  const col = await page.evaluate(() => {
    const vw = document.documentElement.clientWidth;
    const f = document.querySelector(".frame").getBoundingClientRect();
    const t = document.querySelector(".tabbar").getBoundingClientRect();
    return { vw, left: f.left, right: f.right, width: f.width, tabLeft: t.left, tabWidth: t.width };
  });
  const expected = Math.min(col.vw, COL_W);
  expect(Math.abs(col.width - expected)).toBeLessThanOrEqual(1);
  expect(Math.abs(col.left - (col.vw - col.right))).toBeLessThanOrEqual(1);
  expect(Math.abs(col.tabWidth - expected)).toBeLessThanOrEqual(1);
  const layout = await readHorizontalOverflow(page);
  expect(layout.docOverflow).toBe(false);
  expect(layout.wrapOverflow).toBe(false);

  // Bottom-Sheet bleibt in der Spalte
  await page.getByRole("button", { name: "Aktionen: Morgenradio" }).click();
  const sheet = await page.locator("#sceneMenuSheet").boundingBox();
  expect(sheet).toBeTruthy();
  if (sheet) {
    expect(sheet.width).toBeLessThanOrEqual(expected + 1);
    expect(Math.abs(sheet.x - col.left)).toBeLessThanOrEqual(1);
  }
});

test("theme-color folgt dem gewaehlten Design", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile", "einmal genuegt");
  await gotoAppReady(page);
  const colors = () =>
    page.locator('meta[name="theme-color"]').evaluateAll((m) => m.map((x) => x.getAttribute("content")));
  const theme = await page.evaluate(() => document.documentElement.dataset.theme);
  expect(await colors()).toEqual(theme === "light" ? ["#f4efe3", "#f4efe3"] : ["#0d0c0a", "#0d0c0a"]);
  await page.locator("#themeToggle").evaluate((b) => /** @type {HTMLElement} */ (b).click());
  await expect.poll(colors).toEqual(theme === "light" ? ["#0d0c0a", "#0d0c0a"] : ["#f4efe3", "#f4efe3"]);
});
