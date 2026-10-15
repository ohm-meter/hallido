// @ts-check
// Szenen pausieren/fortsetzen (ADR 0015): Mini-Player, ⋯-Sheet, Zustand paused.
const { test, expect } = require("@playwright/test");
const { gotoAppReady, rectsOverlap } = require("./_layout-helpers");

/**
 * Status-Poll mit veraenderbarem Zustand; pause/resume-Routen setzen ihn um
 * (wie die echte Box) und protokollieren die Aufrufe.
 * @param {import('@playwright/test').Page} page
 * @param {string} initial
 */
async function stubTransport(page, initial = "playing") {
  const state = { value: initial, calls: /** @type {string[]} */ ([]) };
  await page.route(/\/api\/v1\/scenes\/status$/, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ scenes: [{ scene_id: "morgenradio", state: state.value }] }),
    }),
  );
  await page.route(/\/api\/v1\/scenes\/morgenradio\/(pause|resume)$/, async (route) => {
    const action = route.request().url().endsWith("/pause") ? "pause" : "resume";
    state.calls.push(route.request().method() + " " + action);
    state.value = action === "pause" ? "paused" : "playing";
    const key = action === "pause" ? "paused" : "resumed";
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ scene_id: "morgenradio", [key]: true }),
    });
  });
  return state;
}

const row = (page) => page.locator('#grid .scene-row[data-id="morgenradio"]');

test("Mini-Player: Pause neben Stop (48 px, ohne Ueberlappung), Pause sendet POST und zeigt pausiert", async ({ page }) => {
  await gotoAppReady(page);
  const state = await stubTransport(page, "playing");
  await page.reload();

  const pause = page.locator("#nowPlayingPause");
  await expect(pause).toBeVisible();
  await expect(pause).toHaveAccessibleName("Pausieren: Morgenradio");
  const geo = await page.evaluate(() => {
    const r = (sel) => {
      const b = document.querySelector(sel).getBoundingClientRect();
      return { top: b.top, right: b.right, bottom: b.bottom, left: b.left, w: b.width, h: b.height };
    };
    return { pause: r("#nowPlayingPause"), stop: r("#nowPlayingStop"), main: r("#nowPlayingMain"), vw: document.documentElement.clientWidth };
  });
  expect(geo.pause.w).toBeGreaterThanOrEqual(48);
  expect(geo.pause.h).toBeGreaterThanOrEqual(48);
  expect(rectsOverlap(geo.pause, geo.stop)).toBe(false);
  expect(rectsOverlap(geo.pause, geo.main)).toBe(false);
  expect(geo.stop.left - geo.pause.right).toBeGreaterThanOrEqual(8);
  expect(geo.stop.right).toBeLessThanOrEqual(geo.vw + 1);

  await pause.click();
  await expect.poll(() => state.calls).toEqual(["POST pause"]);
  await expect(row(page)).toHaveAttribute("data-state", "paused");
  await expect(row(page).locator(".scene-state")).toContainText("pausiert");
  await expect(page.locator("#nowPlaying")).toBeVisible();
  await expect(page.locator("#nowPlaying")).toHaveAttribute("data-state", "paused");
  await expect(pause).toHaveAccessibleName("Fortsetzen: Morgenradio");

  await pause.click();
  await expect.poll(() => state.calls).toEqual(["POST pause", "POST resume"]);
  await expect(row(page)).toHaveAttribute("data-state", "playing");
  await expect(pause).toHaveAccessibleName("Pausieren: Morgenradio");
});

test("Pausierte Szene: neutral statt Amber, runder Knopf setzt fort (resume statt play)", async ({ page }) => {
  await gotoAppReady(page);
  const state = await stubTransport(page, "paused");
  /** @type {string[]} */
  const plays = [];
  await page.route(/\/api\/v1\/scenes\/morgenradio\/play$/, (route) => {
    plays.push(route.request().method());
    return route.fulfill({ status: 200, contentType: "application/json", body: '{"speakers":[]}' });
  });
  await page.reload();

  await expect(row(page)).toHaveAttribute("data-state", "paused");
  const play = row(page).locator(".scene-play");
  await expect(play).toHaveAccessibleName("Fortsetzen: Morgenradio");
  const colors = await page.evaluate(() => {
    const btn = document.querySelector('#grid .scene-row[data-id="morgenradio"] .scene-play');
    const probe = document.createElement("span");
    probe.style.color = "var(--amber)";
    document.body.append(probe);
    const amber = getComputedStyle(probe).color;
    probe.remove();
    return { bg: getComputedStyle(btn).backgroundColor, amber };
  });
  expect(colors.bg, "pausierter Knopf darf nicht Amber sein").not.toBe(colors.amber);
  await expect(page.locator("#nowPlaying")).toBeVisible();

  await play.click();
  await expect.poll(() => state.calls).toEqual(["POST resume"]);
  expect(plays).toEqual([]);
  await expect(row(page)).toHaveAttribute("data-state", "playing");
});

test("⋯-Sheet: Pausieren nur bei laufender Szene, Fortsetzen bei pausierter", async ({ page }) => {
  await gotoAppReady(page);
  const state = await stubTransport(page, "stopped");
  await page.reload();
  await expect(row(page)).toHaveAttribute("data-state", "stopped");

  await page.getByRole("button", { name: "Aktionen: Morgenradio" }).click();
  await expect(page.locator("#sceneMenuSheet")).toBeVisible();
  await expect(page.locator("#sceneMenuPause")).toBeHidden();
  await page.keyboard.press("Escape");

  state.value = "playing";
  await page.reload();
  await expect(row(page)).toHaveAttribute("data-state", "playing");
  await page.getByRole("button", { name: "Aktionen: Morgenradio" }).click();
  await expect(page.locator("#sceneMenuPause")).toBeVisible();
  await expect(page.locator("#sceneMenuPause")).toHaveText(/Pausieren/);
  await page.locator("#sceneMenuPause").click();
  await expect(page.locator("#sceneMenuSheet")).toBeHidden();
  await expect.poll(() => state.calls).toEqual(["POST pause"]);
  await expect(row(page)).toHaveAttribute("data-state", "paused");

  await page.getByRole("button", { name: "Aktionen: Morgenradio" }).click();
  await expect(page.locator("#sceneMenuPause")).toHaveText(/Fortsetzen/);
  await page.locator("#sceneMenuPause").click();
  await expect.poll(() => state.calls).toEqual(["POST pause", "POST resume"]);
});

test("Fortsetzen ohne geladene Quelle: Fehlertext der API unveraendert als Hinweis", async ({ page }) => {
  await gotoAppReady(page);
  await stubTransport(page, "paused");
  await page.route(/\/api\/v1\/scenes\/morgenradio\/resume$/, (route) =>
    route.fulfill({
      status: 502,
      contentType: "application/json",
      body: JSON.stringify({
        code: "playback_failed",
        message: "Box kann nicht fortsetzen (keine Quelle geladen)",
        hint: "Szene neu starten",
      }),
    }),
  );
  await page.reload();
  await expect(row(page)).toHaveAttribute("data-state", "paused");
  await page.locator("#nowPlayingPause").click();
  await expect(page.locator("#hint")).toHaveText("Box kann nicht fortsetzen (keine Quelle geladen) (Szene neu starten)");
  await expect(page.locator("#hint")).toHaveClass(/error/);
  await expect(row(page)).toHaveAttribute("data-state", "error");
});
