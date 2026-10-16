// @ts-check
// Akzeptanz Mobile-Redesign: jedes sichtbare Ziel >= 44 x 44 px, keine
// ueberlappenden Ziele, kein horizontaler Overflow — je Tab und je Sheet.
const { test, expect } = require("@playwright/test");
const { gotoAppReady, gotoTab, readTouchTargets } = require("./_layout-helpers");

/**
 * @param {import('@playwright/test').Page} page
 * @param {string} where
 */
async function expectTouchFriendly(page, where) {
  const t = await readTouchTargets(page);
  expect(t.count, `${where}: keine Ziele gefunden`).toBeGreaterThan(0);
  expect(t.small, `${where}: Ziele < 44 px`).toEqual([]);
  expect(t.overlaps, `${where}: ueberlappende Ziele`).toEqual([]);
  expect(t.outside, `${where}: Ziele ausserhalb des Viewports`).toEqual([]);
  expect(t.docOverflow, `${where}: horizontaler Overflow`).toBe(false);
}

/** Status-Poll: eine Szene laeuft bzw. ist pausiert (Mini-Player sichtbar). */
async function withPlayingScene(page, state = "playing") {
  await page.route(/\/api\/v1\/scenes\/status$/, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ scenes: [{ scene_id: "morgenradio", state }] }),
    }),
  );
  await page.reload();
  await expect(page.locator("#nowPlaying")).toBeVisible();
}

test("Touch-Ziele: drei Tabs (mit Mini-Player)", async ({ page }) => {
  await gotoAppReady(page);
  await withPlayingScene(page);
  await expectTouchFriendly(page, "Szenen");
  await gotoTab(page, "zeitplaene");
  await expectTouchFriendly(page, "Zeitpläne");
  await gotoTab(page, "boxen");
  await page.locator("#speakerList .speaker-card").first().click();
  await expect(page.locator("#discoverPanel")).toBeVisible();
  await expectTouchFriendly(page, "Boxen");
});

test("Touch-Ziele: ⋯-Sheet mit Lautstaerken, Loeschen-Bestaetigung", async ({ page }) => {
  await gotoAppReady(page);
  await page.getByRole("button", { name: "Aktionen: Morgenradio" }).click();
  await expect(page.locator("#sceneMenuVolumes .vol-row")).toHaveCount(3);
  await expectTouchFriendly(page, "⋯-Sheet mit Lautstärken");
  await page.keyboard.press("Escape");

  await page.getByRole("button", { name: "Aktionen: Morgenradio" }).click();
  await page.locator("#sceneMenuEdit").click();
  await page.locator("#sceneDeleteBtn").click();
  await expect(page.locator("#sceneDeleteDialog")).toBeVisible();
  await expectTouchFriendly(page, "Löschen bestätigen");
});

test("Touch-Ziele: Szenen-Editor in allen Quellen-Modi", async ({ page }) => {
  await gotoAppReady(page);
  await page.route(/\/api\/v1\/radio\/search/, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ stations: [
        { name: "FM4", url: "https://orf.at/fm4.mp3", country: "Austria", tags: "alternative,indie" },
        { name: "Ö1", url: "https://orf.at/oe1.mp3", country: "Austria" },
      ] }),
    }),
  );
  await page.getByRole("button", { name: "Neue Szene" }).click();
  await expect(page.locator("#sceneMembers .member-row").first()).toBeVisible();
  await page.locator("#sceneMembers .member-pick").first().click();
  await page.locator("#radioSearchInput").fill("FM4");
  await page.locator("#radioSearchBtn").click();
  await expect(page.locator(".radio-search-hit")).toHaveCount(2);
  await page.locator(".advanced > summary").click();
  await expectTouchFriendly(page, "Editor · Radio");
  for (const mode of ["Von Box", "URL"]) {
    await page.locator(".seg-opt", { hasText: mode }).click();
    await expectTouchFriendly(page, `Editor · ${mode}`);
  }
  await page.locator("#sourceFavoriteBtn").click();
  await expectTouchFriendly(page, "Editor · Favoriten");
});

test("Touch-Ziele: Zeitplan-Editor und Token-Dialog", async ({ page }) => {
  await gotoAppReady(page);
  await gotoTab(page, "zeitplaene");
  await page.getByRole("button", { name: "Zeitplan „Layout-Test“ 06:10 bearbeiten" }).click();
  await expect(page.locator("#scheduleDialog")).toBeVisible();
  await expectTouchFriendly(page, "Zeitplan-Editor");
  await page.locator("#scheduleCancelBtn").click();

  await gotoTab(page, "boxen");
  await page.locator("#tokenBtn").click();
  await expect(page.locator("#tokenDialog")).toBeVisible();
  await expectTouchFriendly(page, "Token-Dialog");
});

test("Touch-Ziele: pausierte Szene (Mini-Player mit Fortsetzen) und ⋯-Sheet mit Pause-Eintrag", async ({ page }) => {
  await gotoAppReady(page);
  await withPlayingScene(page, "paused");
  await expect(page.locator("#nowPlayingPause")).toBeVisible();
  await expectTouchFriendly(page, "Szenen · pausiert");
  await page.getByRole("button", { name: "Aktionen: Morgenradio" }).click();
  await expect(page.locator("#sceneMenuPause")).toBeVisible();
  await expectTouchFriendly(page, "⋯-Sheet · pausiert");
});

test("Touch-Ziele: Boxen-Tab nach Gruppen (Kopf mit Pause/Stop) und Box-Detail", async ({ page }) => {
  await gotoAppReady(page);
  await page.route(/\/api\/v1\/groups$/, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ groups: [
        { coordinator: "Wohnzimmer", members: ["Wohnzimmer", "Bad", "Kueche"], state: "playing", title: "Ein sehr langer Sendername, der nicht in die Zeile passt" },
        { coordinator: "Terrasse", members: ["Terrasse"], state: "paused" },
        { coordinator: "Garage", members: ["Garage", "Keller"], state: "stopped" },
      ] }),
    }),
  );
  await gotoTab(page, "boxen");
  await expect(page.locator("#groupList .group-block")).toHaveCount(3);
  await expectTouchFriendly(page, "Boxen · Gruppen");
  await page.locator("#groupList .speaker-card", { hasText: "Kueche" }).click();
  await expect(page.locator("#discoverPanel")).toBeVisible();
  await expect(page.locator("#discoverUngroupBtn")).toBeVisible();
  await expectTouchFriendly(page, "Boxen · Gruppen · Detail");
  await page.locator("#discoverUngroupBtn").click();
  await expect(page.locator("#ungroupDialog")).toBeVisible();
  await expectTouchFriendly(page, "Aus Gruppe lösen bestätigen");
});

test("Touch-Ziele: Protokoll-Sheet (Boxen → Protokoll)", async ({ page }) => {
  await gotoAppReady(page);
  await page.route(/\/api\/v1\/logs/, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ file: true, entries: [{ time: "2026-10-15T08:00:00Z", level: "INFO", msg: "agent started", attrs: { listen: ":8790" } }] }),
    }),
  );
  await gotoTab(page, "boxen");
  await page.locator("#logBtn").click();
  await expect(page.locator("#logSheet .log-entry")).toHaveCount(1);
  await expectTouchFriendly(page, "Protokoll-Sheet");
});

test("Touch-Ziele: Einrichtungsassistent (Willkommen)", async ({ page }) => {
  await page.route(/\/api\/v1\/setup$/, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ setup_required: true, open: true, expires_in_seconds: 3500 }),
    }),
  );
  await page.goto("/");
  await expect(page.locator("#setupCreateBtn")).toBeVisible();
  await expectTouchFriendly(page, "Einrichtung");
});
