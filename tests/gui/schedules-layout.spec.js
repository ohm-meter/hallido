// @ts-check
const { test, expect } = require("@playwright/test");
const {
  gotoAppReady,
  gotoTab,
  readScheduleListLayout,
  readScheduleDialogLayout,
} = require("./_layout-helpers");

test("Zeitplaene: Zeilen mit Uhrzeit, Schalter-Trefferflaeche 48 px, kein Ueberlauf", async ({ page }) => {
  await gotoAppReady(page);
  await gotoTab(page, "zeitplaene");
  await expect(page.locator("#scheduleList .schedule-row")).toHaveCount(2);
  await expect(page.locator("#scheduleList .schedule-time").first()).toHaveText("06:10–08:30");
  await expect(page.locator("#scheduleList .schedule-when").first()).toHaveText("Layout-Test · Mo–Fr");
  await expect(page.locator("#scheduleList .schedule-next").first()).toContainText("Nächster Start");
  await expect(page.locator("#scheduleList .schedule-next").nth(1)).toHaveText("deaktiviert");
  await expect(page.locator("#scheduleList .schedule-last.error")).toContainText("nicht erreichbar");
  await expect(page.locator("#scheduleSummary")).toContainText("Nächster Start");
  await expect(page.locator("#scheduleSummary")).toContainText("Layout-Test");

  const layout = await readScheduleListLayout(page);
  expect(layout.error, layout.error || "").toBeUndefined();
  expect(layout.overflow).toBe(false);
  expect(layout.docOverflow).toBe(false);
  expect(layout.toggleOverlap).toBe(false);
  expect(layout.minSwitchHit).toBeGreaterThanOrEqual(48);
  expect(layout.minGap).toBeGreaterThanOrEqual(8);
  expect(layout.minRowHeight).toBeGreaterThanOrEqual(72);
});

test("Neuer Zeitplan: runde Tage-Chips 44 px in einer Zeile, Schnellwahl, Speichern sichtbar", async ({ page }) => {
  await gotoAppReady(page);
  await gotoTab(page, "zeitplaene");
  await page.getByRole("button", { name: "Neuer Zeitplan" }).click();
  await expect(page.locator("#scheduleDialog")).toBeVisible();
  await expect(page.locator("#scheduleDialogTitle")).toHaveText("Neuer Zeitplan");
  await expect(page.locator("#scheduleScene")).toHaveValue("test-layout");
  await expect(page.locator("#scheduleStart")).toHaveAttribute("type", "time");
  // Szene als Dropdown erkennbar: Chevron als Hintergrundbild
  const chevron = await page.locator("#scheduleScene").evaluate((el) => getComputedStyle(el).backgroundImage);
  expect(chevron).toContain("svg");
  await expect(page.locator("#scheduleDeleteBtn")).toBeHidden();

  const layout = await readScheduleDialogLayout(page);
  expect(layout.error, layout.error || "").toBeUndefined();
  expect(layout.chipsOneRow).toBe(true);
  expect(layout.chipsInside).toBe(true);
  expect(layout.chipsOverlap).toBe(false);
  expect(layout.minChipHeight).toBeGreaterThanOrEqual(44);
  expect(layout.minChipWidth).toBeGreaterThanOrEqual(44);
  expect(layout.chipRound).toBe("50%");
  expect(layout.minPresetHeight).toBeGreaterThanOrEqual(44);
  expect(layout.timesOverlap).toBe(false);
  expect(layout.saveInViewport).toBe(true);
  expect(layout.dialogInViewport).toBe(true);

  await page.getByRole("button", { name: "Wochenende" }).click();
  await expect(page.locator('input[name="scheduleDay"]:checked')).toHaveCount(2);
  await page.getByRole("button", { name: "Täglich" }).click();
  await expect(page.locator('input[name="scheduleDay"]:checked')).toHaveCount(7);
});

test("Zeitplan bearbeiten: Tap auf Zeile oeffnet vorbelegtes Sheet, Speichern sendet PUT", async ({ page }) => {
  await gotoAppReady(page);
  await gotoTab(page, "zeitplaene");
  /** @type {any} */
  let sent = null;
  await page.route(/\/api\/v1\/config\/schedules\//, async (route) => {
    sent = { method: route.request().method(), body: route.request().postDataJSON() };
    await route.fulfill({ status: 200, contentType: "application/json", body: '{"ok":true}' });
  });
  await page.getByRole("button", { name: "Zeitplan „Layout-Test“ 06:10 bearbeiten" }).click();
  await expect(page.locator("#scheduleDialogTitle")).toHaveText("Zeitplan bearbeiten");
  await expect(page.locator("#scheduleStart")).toHaveValue("06:10");
  await expect(page.locator("#scheduleStop")).toHaveValue("08:30");
  await expect(page.locator('input[name="scheduleDay"]:checked')).toHaveCount(5);
  await expect(page.locator("#scheduleDeleteBtn")).toBeVisible();

  await page.locator("#scheduleStart").fill("06:20");
  await page.locator("#scheduleSaveBtn").click();
  await expect(page.locator("#scheduleDialog")).toBeHidden();
  expect(sent?.method).toBe("PUT");
  expect(sent?.body).toEqual({
    scene: "test-layout",
    days: ["mon", "tue", "wed", "thu", "fri"],
    start: "06:20",
    stop: "08:30",
    enabled: true,
  });
});

test("Zeitplan loeschen: nur im Bearbeiten-Sheet, mit Bestaetigung", async ({ page }) => {
  await gotoAppReady(page);
  await gotoTab(page, "zeitplaene");
  await expect(page.locator("#scheduleList").getByRole("button", { name: /löschen/ })).toHaveCount(0);
  /** @type {string | null} */
  let method = null;
  await page.route(/\/api\/v1\/config\/schedules\/test-layout-0610$/, async (route) => {
    method = route.request().method();
    await route.fulfill({ status: 200, contentType: "application/json", body: '{"ok":true}' });
  });
  await page.getByRole("button", { name: "Zeitplan „Layout-Test“ 06:10 bearbeiten" }).click();
  await page.locator("#scheduleDeleteBtn").click();
  await expect(page.locator("#scheduleDeleteDialog")).toBeVisible();
  await page.locator('#scheduleDeleteDialog button[value="delete"]').click();
  await expect.poll(() => method).toBe("DELETE");
  await expect(page.locator("#scheduleDialog")).toBeHidden();
});

test("Zeitplan-Schalter: Ausschalten sendet PUT mit enabled=false", async ({ page }) => {
  await gotoAppReady(page);
  await gotoTab(page, "zeitplaene");
  /** @type {any} */
  let sent = null;
  await page.route(/\/api\/v1\/config\/schedules\//, async (route) => {
    sent = route.request().postDataJSON();
    await route.fulfill({ status: 200, contentType: "application/json", body: '{"ok":true}' });
  });
  const toggle = page.getByRole("switch", { name: "Zeitplan „Layout-Test“ 06:10 aktiv" });
  await expect(toggle).toBeChecked();
  await toggle.click();
  await expect.poll(() => sent && sent.enabled).toBe(false);
});
