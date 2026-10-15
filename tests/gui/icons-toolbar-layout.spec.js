// @ts-check
const { test, expect } = require("@playwright/test");
const { gotoAppReady, gotoTab, readGuiMetrics, readHorizontalOverflow, ICON_SIZES } = require("./_layout-helpers");

const TOL = 1.5;

/** @param {{ w: number, h: number } | null | undefined} size */
function expectIconSize(size, expected, label) {
  expect(size, `${label} fehlt`).toBeTruthy();
  expect(size.w, label).toBeGreaterThanOrEqual(expected - TOL);
  expect(size.w, label).toBeLessThanOrEqual(expected + TOL);
  expect(size.h, label).toBeGreaterThanOrEqual(expected - TOL);
  expect(size.h, label).toBeLessThanOrEqual(expected + TOL);
}

test("Design-Tokens: Icon-Groessen", async ({ page }, testInfo) => {
  await gotoAppReady(page);

  const metrics = await readGuiMetrics(page);
  expectIconSize(metrics.barIcon, ICON_SIZES.bar, "Kopfzeilen-Icon");
  expectIconSize(metrics.tabIcon, ICON_SIZES.tab, "Tab-Icon");
  expectIconSize(metrics.sceneActionIcon, ICON_SIZES.scene, "Szenen-Aktions-Icon");

  if (testInfo.project.name === "mobile") {
    const overflow = await readHorizontalOverflow(page);
    expect(overflow.docOverflow).toBe(false);
    expect(overflow.sceneOverflow).toBe(false);
  }

  await gotoTab(page, "boxen");
  const boxen = await readGuiMetrics(page);
  expectIconSize(boxen.themeIcon, ICON_SIZES.theme, "Theme-Icon");
});
