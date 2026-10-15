// @ts-check
const { test, expect } = require("@playwright/test");
const { gotoAppReady, gotoTab } = require("./_layout-helpers");

test("Tabs: Hash-Routing, Titel, Plus-Knopf und Zurueck-Taste", async ({ page }) => {
  await gotoAppReady(page);
  await expect(page).toHaveURL(/#szenen$/);
  await expect(page.locator("#viewTitle")).toHaveText("Szenen");
  await expect(page.locator('.tabbar a[data-tab="szenen"]')).toHaveAttribute("aria-current", "page");
  await expect(page.locator("#addBtn")).toHaveAttribute("aria-label", "Neue Szene");

  await gotoTab(page, "zeitplaene");
  await expect(page.locator("#viewTitle")).toHaveText("Zeitpläne");
  await expect(page.locator("#addBtn")).toHaveAttribute("aria-label", "Neuer Zeitplan");
  await expect(page.locator('[data-view="szenen"]')).toBeHidden();

  await gotoTab(page, "boxen");
  await expect(page.locator("#addBtn")).toBeHidden();
  await expect(page.locator("#speakerList .speaker-card")).toHaveCount(15);
  await expect(page.locator("#tokenBtn")).toBeVisible();

  await page.goBack();
  await expect(page).toHaveURL(/#zeitplaene$/);
  await expect(page.locator('[data-view="zeitplaene"]')).toBeVisible();
  await page.goBack();
  await expect(page.locator('[data-view="szenen"]')).toBeVisible();
});

test("Deep-Link: #boxen oeffnet direkt den Boxen-Tab", async ({ page }) => {
  await gotoAppReady(page);
  await page.goto("/#boxen");
  await expect(page.locator('[data-view="boxen"]')).toBeVisible();
  await expect(page.locator("#viewTitle")).toHaveText("Boxen");
});

test("Tab-Bar: unten in der Handy-Spalte (alle Breiten), Ziele >= 48 px", async ({ page }) => {
  await gotoAppReady(page);
  const nav = await page.evaluate(() => {
    const bar = document.querySelector(".tabbar");
    if (!bar) return null;
    const r = bar.getBoundingClientRect();
    const frame = document.querySelector(".frame").getBoundingClientRect();
    const tabs = [...bar.querySelectorAll(".tab")].map((t) => t.getBoundingClientRect());
    return {
      top: r.top, bottom: r.bottom, left: r.left, right: r.right,
      frameLeft: frame.left, frameRight: frame.right,
      vh: window.innerHeight,
      minTab: Math.min(...tabs.map((t) => Math.min(t.width, t.height))),
    };
  });
  expect(nav).toBeTruthy();
  expect(nav.minTab).toBeGreaterThanOrEqual(48);
  expect(Math.round(nav.bottom)).toBe(nav.vh);
  expect(nav.top).toBeGreaterThan(nav.vh - 80);
  // Tab-Bar deckt genau die Spalte ab
  expect(Math.abs(nav.left - nav.frameLeft)).toBeLessThanOrEqual(1);
  expect(Math.abs(nav.right - nav.frameRight)).toBeLessThanOrEqual(1);
});
