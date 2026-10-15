// @ts-check
const { test, expect } = require("@playwright/test");
const { gotoAppReady, gotoTab, rectsOverlap } = require("./_layout-helpers");

/** Status-Poll: eine Szene laeuft. */
async function stubPlaying(page, sceneId = "morgenradio") {
  await page.route(/\/api\/v1\/scenes\/status$/, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ scenes: [{ scene_id: sceneId, state: "playing" }] }),
    }),
  );
}

test("Laeuft gerade: erscheint ueber der Tab-Bar, ueberdeckt nichts, Stop sendet POST", async ({ page }) => {
  await gotoAppReady(page);
  await expect(page.locator("#nowPlaying")).toBeHidden();
  await stubPlaying(page);
  /** @type {string | null} */
  let stopped = null;
  await page.route(/\/api\/v1\/scenes\/morgenradio\/stop$/, async (route) => {
    stopped = route.request().method();
    await route.fulfill({ status: 200, contentType: "application/json", body: '{"stopped":true}' });
  });
  await page.reload();

  const bar = page.locator("#nowPlaying");
  await expect(bar).toBeVisible();
  await expect(page.locator("#nowPlayingName")).toHaveText("Morgenradio");
  await expect(page.locator("#nowPlayingSub")).toContainText("Wohnzimmer · Kueche · Bad");

  const geo = await page.evaluate(() => {
    const np = document.querySelector("#nowPlaying").getBoundingClientRect();
    const tab = document.querySelector(".tabbar").getBoundingClientRect();
    const stop = document.querySelector("#nowPlayingStop").getBoundingClientRect();
    const rows = [...document.querySelectorAll("#grid .scene-row")];
    const last = rows[rows.length - 1];
    last.scrollIntoView({ block: "end" });
    const lastRect = last.getBoundingClientRect();
    return {
      np: { top: np.top, right: np.right, bottom: np.bottom, left: np.left },
      tab: { top: tab.top, right: tab.right, bottom: tab.bottom, left: tab.left },
      lastRow: { top: lastRect.top, right: lastRect.right, bottom: lastRect.bottom, left: lastRect.left },
      stop: { w: stop.width, h: stop.height },
      vw: document.documentElement.clientWidth,
    };
  });
  expect(rectsOverlap(geo.np, geo.tab)).toBe(false);
  expect(rectsOverlap(geo.np, geo.lastRow), "Leiste verdeckt letzte Szene").toBe(false);
  expect(geo.stop.w).toBeGreaterThanOrEqual(48);
  expect(geo.stop.h).toBeGreaterThanOrEqual(48);
  expect(geo.np.right).toBeLessThanOrEqual(geo.vw + 1);

  await page.locator("#nowPlayingStop").click();
  await expect.poll(() => stopped).toBe("POST");
});

test("Laeuft gerade: auch in anderen Tabs sichtbar", async ({ page }) => {
  await gotoAppReady(page);
  await stubPlaying(page);
  await page.reload();
  await expect(page.locator("#nowPlaying")).toBeVisible();
  await gotoTab(page, "boxen");
  await expect(page.locator("#nowPlaying")).toBeVisible();
});
