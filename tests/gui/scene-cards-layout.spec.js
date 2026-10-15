// @ts-check
const { test, expect } = require("@playwright/test");
const { gotoAppReady, readHorizontalOverflow, readSceneRowsLayout, GUI_SCENES } = require("./_layout-helpers");

test("Szenen-Zeilen: sichtbar ohne horizontalen Overflow", async ({ page }) => {
  await gotoAppReady(page);

  const layout = await readHorizontalOverflow(page);
  expect(layout.sceneCount).toBe(GUI_SCENES.length);
  expect(layout.docOverflow).toBe(false);
  expect(layout.mainOverflow).toBe(false);
  expect(layout.sceneOverflow).toBe(false);
  expect(layout.wrapOverflow).toBe(false);
  expect(layout.scrollWidth).toBeLessThanOrEqual(layout.vw + 1);
});

test("Szenen-Zeilen: 72 px, Play 56 px rund, Abstaende >= 8 px, >= 5 ohne Scrollen", async ({ page }) => {
  await gotoAppReady(page);

  const rows = await readSceneRowsLayout(page);
  expect(rows.error, rows.error || "").toBeUndefined();
  expect(rows.rowHeight).toBeGreaterThanOrEqual(72);
  expect(rows.play.w).toBeGreaterThanOrEqual(56);
  expect(rows.play.h).toBeGreaterThanOrEqual(56);
  expect(rows.playRound).toBe("50%");
  expect(rows.mainHeight).toBeGreaterThanOrEqual(56);
  expect(rows.more.w).toBeGreaterThanOrEqual(48);
  expect(rows.more.h).toBeGreaterThanOrEqual(48);
  expect(rows.gapPlayMain).toBeGreaterThanOrEqual(8);
  expect(rows.gapMainMore).toBeGreaterThanOrEqual(8);
  expect(rows.fullyVisible).toBeGreaterThanOrEqual(5);
});

test("Szene: Tap auf Zeile startet, zweiter Tap stoppt", async ({ page }) => {
  await gotoAppReady(page);
  /** @type {string[]} */
  const calls = [];
  await page.route(/\/api\/v1\/scenes\/test-layout\/(play|stop)$/, async (route) => {
    const action = route.request().url().endsWith("/play") ? "play" : "stop";
    calls.push(action);
    const body = action === "play"
      ? { scene_id: "test-layout", speakers: [{ speaker: "Wohnzimmer", ok: true }] }
      : { scene_id: "test-layout", stopped: true };
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
  });
  const row = page.locator('#grid .scene-row[data-id="test-layout"]');
  await row.locator(".scene-main").click();
  await expect(row).toHaveAttribute("data-state", "playing");
  await expect(row.locator(".scene-play")).toHaveAttribute("aria-label", "Stoppen: Layout-Test");
  await expect(row.locator(".scene-state")).toContainText("läuft");

  await row.locator(".scene-play").click();
  await expect(row).toHaveAttribute("data-state", "stopped");
  await expect(row.locator(".scene-play")).toHaveAttribute("aria-label", "Abspielen: Layout-Test");
  expect(calls).toEqual(["play", "stop"]);
});

test("Szene: playing_other bleibt neutral (Play statt Stop, kein Amber)", async ({ page }) => {
  await gotoAppReady(page);
  // Nach dem Basis-Stub registriert -> hat Vorrang; Reload loest den Poll sofort aus.
  await page.route(/\/api\/v1\/scenes\/status$/, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ scenes: [{ scene_id: "morgenradio", state: "playing_other", now_playing_title: "Spotify" }] }),
    }),
  );
  await page.reload();
  const row = page.locator('#grid .scene-row[data-id="morgenradio"]');
  await expect(row).toHaveAttribute("data-state", "playing_other");
  await expect(row.locator(".scene-state")).toContainText("läuft anders");
  await expect(row.locator(".scene-play")).toHaveAttribute("aria-label", "Abspielen: Morgenradio");
});

test("Szene loeschen: nur im Bearbeiten-Sheet, mit Bestaetigung", async ({ page }) => {
  await gotoAppReady(page);
  await expect(page.locator("#grid").getByRole("button", { name: /löschen/ })).toHaveCount(0);

  /** @type {string | null} */
  let deleted = null;
  await page.route(/\/api\/v1\/config\/scenes\/ruhe$/, async (route) => {
    deleted = route.request().method();
    await route.fulfill({ status: 200, contentType: "application/json", body: '{"ok":true}' });
  });
  await page.getByRole("button", { name: "Aktionen: Ruhe Schlafzimmer" }).click();
  await expect(page.locator("#sceneMenuSheet")).toBeVisible();
  await page.locator("#sceneMenuEdit").click();
  await expect(page.locator("#sceneDialog")).toBeVisible();
  await page.locator("#sceneDeleteBtn").click();
  await expect(page.locator("#sceneDeleteDialog")).toBeVisible();
  await expect(page.locator("#sceneDeleteMessage")).toContainText("Ruhe Schlafzimmer");
  await page.locator('#sceneDeleteDialog button[value="delete"]').click();
  await expect.poll(() => deleted).toBe("DELETE");
  await expect(page.locator("#sceneDialog")).toBeHidden();
});

test("Szenen-Zeilen: kein Overflow an den Breakpoint-Raendern (Handy-Spalte)", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop", "Breiten werden einmalig im Desktop-Projekt durchlaufen");
  await gotoAppReady(page);
  for (const width of [320, 459, 460, 480, 599, 600, 899, 900, 1280, 1920]) {
    await page.setViewportSize({ width, height: 800 });
    const layout = await readHorizontalOverflow(page);
    expect(layout.docOverflow, `${width}px: Dokument breiter als Viewport`).toBe(false);
    expect(layout.wrapOverflow, `${width}px: Szenen-Zeile ragt aus dem Viewport`).toBe(false);
  }
});
