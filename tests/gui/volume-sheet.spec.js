// @ts-check
const { test, expect } = require("@playwright/test");
const { gotoAppReady, settleAnimations } = require("./_layout-helpers");

/**
 * PUT /speakers/{name}/volume (live) und PUT /config/scenes/{id} (speichern) abfangen.
 * @param {import('@playwright/test').Page} page
 * @param {number} [liveStatus]
 */
async function captureVolume(page, liveStatus = 200) {
  /** @type {{ live: { name: string, body: any }[], saved: { id: string, body: any }[] }} */
  const calls = { live: [], saved: [] };
  await page.route(/\/api\/v1\/speakers\/[^/]+\/volume$/, async (route) => {
    const name = decodeURIComponent(new URL(route.request().url()).pathname.split("/")[4]);
    calls.live.push({ name, body: route.request().postDataJSON() });
    const body = liveStatus === 200
      ? { speaker: name, volume: route.request().postDataJSON().volume }
      : { error: "speaker_unreachable", message: "Lautsprecher antwortet nicht", speaker: name };
    await route.fulfill({ status: liveStatus, contentType: "application/json", body: JSON.stringify(body) });
  });
  await page.route(/\/api\/v1\/config\/scenes\/[^/]+$/, async (route) => {
    if (route.request().method() !== "PUT") return route.continue();
    const id = new URL(route.request().url()).pathname.split("/").pop() || "";
    calls.saved.push({ id, body: route.request().postDataJSON() });
    await route.fulfill({ status: 200, contentType: "application/json", body: '{"ok":true}' });
  });
  return calls;
}

/** Status-Poll: eine Szene laeuft. */
async function withPlaying(page, sceneId) {
  await page.route(/\/api\/v1\/scenes\/status$/, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ scenes: [{ scene_id: sceneId, state: "playing" }] }),
    }),
  );
  await page.reload();
}

test("⋯-Sheet: Bearbeiten oben, darunter Lautstaerke je Raum (Slider + −/+ 44 px)", async ({ page }) => {
  await gotoAppReady(page);
  await page.getByRole("button", { name: "Aktionen: Morgenradio" }).click();
  const sheet = page.locator("#sceneMenuSheet");
  await expect(sheet).toBeVisible();
  await expect(page.locator("#sceneMenuEdit")).toBeVisible();
  await expect(sheet.locator(".vol-row")).toHaveCount(3);
  await settleAnimations(page);

  const sizes = await page.evaluate(() =>
    [...document.querySelectorAll("#sceneMenuSheet .vol-step, #sceneMenuSheet input[type=range]")].map((el) => {
      const r = el.getBoundingClientRect();
      return { w: r.width, h: r.height };
    }),
  );
  for (const s of sizes) {
    expect(s.h).toBeGreaterThanOrEqual(44);
    expect(s.w).toBeGreaterThanOrEqual(44);
  }
  // Edit-Knopf oberhalb der Lautstaerken
  const editBottom = await page.locator("#sceneMenuEdit").evaluate((el) => el.getBoundingClientRect().bottom);
  const volTop = await sheet.locator(".vol-row").first().evaluate((el) => el.getBoundingClientRect().top);
  expect(editBottom).toBeLessThanOrEqual(volTop);
});

test("⋯-Sheet: −/+ aendern um genau 1 und speichern in der Szene (ohne Live, wenn Szene steht)", async ({ page }) => {
  await gotoAppReady(page);
  const calls = await captureVolume(page);
  await page.getByRole("button", { name: "Aktionen: Morgenradio" }).click();
  const row = page.locator('#sceneMenuSheet .vol-row[data-name="Kueche"]');
  await expect(row.locator(".vol-value")).toHaveText("15");
  await row.getByRole("button", { name: "Kueche lauter" }).click();
  await row.getByRole("button", { name: "Kueche lauter" }).click();
  await expect(row.locator(".vol-value")).toHaveText("17");
  await row.getByRole("button", { name: "Kueche leiser" }).click();
  await expect(row.locator(".vol-value")).toHaveText("16");

  // Entprellt: ein Speichern mit dem Endstand aller Raeume
  await expect.poll(() => calls.saved.length).toBe(1);
  expect(calls.saved[0].id).toBe("morgenradio");
  expect(calls.saved[0].body.members).toEqual([
    { speaker: "Wohnzimmer", volume: 15 },
    { speaker: "Kueche", volume: 16 },
    { speaker: "Bad", volume: 15 },
  ]);
  expect(calls.saved[0].body.name).toBe("Morgenradio");
  expect(calls.saved[0].body.source).toEqual({ type: "stream", uri: "https://example.com/morgenradio.mp3" });
  await expect(page.locator("#sceneMenuHint")).toContainText("gespeichert");
  // Szene laeuft nicht -> keine Box wird live veraendert
  expect(calls.live).toEqual([]);

  // Gespeicherter Wert erscheint im Bearbeiten-Dialog
  await page.locator("#sceneMenuEdit").click();
  await expect(page.locator('#sceneMembers .member-row[data-name="Kueche"] .vol-value')).toHaveText("16");
});

test("⋯-Sheet: bei laufender Szene wirkt die Aenderung sofort auf die Box", async ({ page }) => {
  await gotoAppReady(page);
  const calls = await captureVolume(page);
  await withPlaying(page, "morgenradio");
  await page.getByRole("button", { name: "Aktionen: Morgenradio" }).click();
  const row = page.locator('#sceneMenuSheet .vol-row[data-name="Bad"]');
  await row.locator('input[type="range"]').fill("40");
  await expect.poll(() => calls.live.length).toBe(1);
  expect(calls.live[0]).toEqual({ name: "Bad", body: { volume: 40 } });
  await expect.poll(() => calls.saved.length).toBe(1);
});

test("⋯-Sheet: Box offline zeigt Fehler am Raum", async ({ page }) => {
  await gotoAppReady(page);
  await captureVolume(page, 502);
  await withPlaying(page, "test-layout");
  await page.getByRole("button", { name: "Aktionen: Layout-Test" }).click();
  const row = page.locator('#sceneMenuSheet .vol-row[data-name="Wohnzimmer"]');
  await row.getByRole("button", { name: "Wohnzimmer leiser" }).click();
  await expect(row.locator(".vol-error")).toContainText("antwortet nicht");
});

test("Laeuft gerade: Tap oeffnet das ⋯-Sheet der laufenden Szene", async ({ page }) => {
  await gotoAppReady(page);
  await withPlaying(page, "party");
  await page.locator("#nowPlayingMain").click();
  await expect(page.locator("#sceneMenuSheet")).toBeVisible();
  await expect(page.locator("#sceneMenuTitle")).toHaveText("Party überall");
  await expect(page.locator("#sceneMenuSheet .vol-row")).toHaveCount(5);
});
