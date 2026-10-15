// @ts-check
const { test, expect } = require("@playwright/test");
const { gotoAppReady, gotoTab, readHeaderLayout, HEADER_ALIGN_TOL } = require("./_layout-helpers");

test("Kopfzeile: Status und Plus-Knopf vertikal ausgerichtet", async ({ page }) => {
  await gotoAppReady(page);

  const layout = await readHeaderLayout(page);
  expect(layout.error, layout.error || "").toBeUndefined();
  expect(
    layout.statusThemeAligned,
    `Status/Plus Y-Abweichung ${layout.statusThemeDelta}px (max ${HEADER_ALIGN_TOL}px)`,
  ).toBe(true);
});

test("Boxen-Tab: Theme-Umschalter zeigt genau ein Icon und schaltet um", async ({ page }) => {
  await gotoAppReady(page);
  await gotoTab(page, "boxen");

  const icons = await page.evaluate(() => {
    const toggle = document.querySelector("#themeToggle");
    const slot = document.querySelector("#themeIcon");
    return {
      svgCount: toggle ? toggle.querySelectorAll("svg").length : 0,
      slotChildCount: slot ? slot.childElementCount : 0,
    };
  });
  expect(icons.svgCount, "Theme-Toggle darf nur ein SVG enthalten").toBe(1);
  expect(icons.slotChildCount, "Theme-Icon-Slot darf nur ein Kind haben").toBe(1);

  const before = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
  await page.locator("#themeToggle").click();
  const after = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
  expect(after).not.toBe(before);
  await expect(page.locator("#themeLabel")).toHaveText(after === "light" ? "Hell" : "Dunkel");
});

test("Kopfzeile: eine 48-px-Zeile ohne Ueberlauf", async ({ page }) => {
  await gotoAppReady(page);

  const bar = await page.evaluate(() => {
    const el = document.querySelector("header.bar");
    const title = document.querySelector("#viewTitle");
    if (!el || !title) return null;
    const r = el.getBoundingClientRect();
    const t = title.getBoundingClientRect();
    return {
      height: r.height,
      right: r.right,
      vw: document.documentElement.clientWidth,
      titleInside: t.top >= r.top && t.bottom <= r.bottom,
    };
  });
  expect(bar, "Kopfzeile fehlt").toBeTruthy();
  expect(bar.height).toBeGreaterThanOrEqual(47);
  expect(bar.height).toBeLessThanOrEqual(49);
  expect(bar.titleInside).toBe(true);
  expect(bar.right).toBeLessThanOrEqual(bar.vw + 1);
  await expect(page).toHaveTitle(/^Scenes · Hallido — Powered by siraris$|^Szenen · Hallido — Powered by siraris$/);
});
