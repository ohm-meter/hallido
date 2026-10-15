// @ts-check
const { test, expect } = require("@playwright/test");
const {
  EXPECTED_SPEAKERS,
  openSceneDialogWithSpeakers,
  openSceneEditDialog,
  readSceneEditorLayout,
} = require("./_layout-helpers");

test("Neue Szene: Sheet (Handy Vollbild), Segment-Control, kein Scroll-in-Scroll", async ({ page }, testInfo) => {
  await openSceneDialogWithSpeakers(page);
  await expect(page.locator("#sceneDialogTitle")).toHaveText("Neue Szene");
  await expect(page.locator('input[name="sourceMode"][value="radio"]')).toBeChecked();
  await expect(page.locator("#sceneMembers .member-row")).toHaveCount(EXPECTED_SPEAKERS);

  const layout = await readSceneEditorLayout(page);
  expect(layout.error, layout.error || "").toBeUndefined();
  if (testInfo.project.name === "mobile") {
    expect(layout.dialog.width).toBeGreaterThanOrEqual(layout.vw - 1);
    expect(layout.dialog.height).toBeGreaterThanOrEqual(layout.vh - 1);
  }
  expect(layout.saveInViewport, `Sticky Footer: Speichern sichtbar ${JSON.stringify(layout)}`).toBe(true);
  expect(layout.saveHeight).toBeGreaterThanOrEqual(48);
  for (const h of layout.segHeights) expect(h).toBeGreaterThanOrEqual(44);
  expect(layout.nested, `verschachtelte Scrollbereiche: ${layout.nested.join(", ")}`).toEqual([]);
  expect(layout.slugVisible, "Slug nur unter Erweitert").toBe(false);
  await expect(page.locator("#sceneSaveBtn")).toHaveText("Speichern & abspielen");

  // Raum waehlen -> Slider + −/+ erscheinen
  const row = page.locator('#sceneMembers .member-row[data-name="Kueche"]');
  await expect(row.locator('input[type="range"]')).toBeHidden();
  await row.locator(".member-pick").click();
  await expect(row.locator('input[type="range"]')).toBeVisible();
  await row.getByRole("button", { name: "Kueche lauter" }).click();
  await expect(row.locator(".vol-value")).toHaveText("6");
  await row.getByRole("button", { name: "Kueche leiser" }).click();
  await row.getByRole("button", { name: "Kueche leiser" }).click();
  await expect(row.locator(".vol-value")).toHaveText("4");

  // Quelle wechseln
  await page.locator(".seg-opt", { hasText: "Von Box" }).click();
  await expect(page.locator('[data-source-panel="box"]')).toBeVisible();
  await page.locator("#sourceFavoriteBtn").click();
  await expect(page.locator('[data-source-panel="favorite"]')).toBeVisible();
  await expect(page.locator("#sourceFavoriteBtn")).toHaveAttribute("aria-pressed", "true");
  await page.locator(".seg-opt", { hasText: "URL" }).click();
  await expect(page.locator("#sourceUrl")).toBeVisible();

  await page.locator(".advanced > summary").click();
  await expect(page.locator("#sceneId")).toBeVisible();
});

test("Neue Szene: Radio suchen, Treffer antippen, Speichern & abspielen", async ({ page }) => {
  await openSceneDialogWithSpeakers(page);
  /** @type {any} */
  let saved = null;
  let played = false;
  await page.route(/\/api\/v1\/radio\/search/, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ stations: [{ name: "FM4", url: "https://orf.at/fm4.mp3", country: "Austria", tags: "alternative" }] }),
    }),
  );
  await page.route(/\/api\/v1\/config\/scenes$/, async (route) => {
    saved = route.request().postDataJSON();
    await route.fulfill({ status: 200, contentType: "application/json", body: '{"ok":true}' });
  });
  await page.route(/\/api\/v1\/scenes\/fm4\/play$/, async (route) => {
    played = true;
    await route.fulfill({ status: 404, contentType: "application/json", body: '{"error":"scene_not_found","message":"x"}' });
  });

  await page.locator("#radioSearchInput").fill("FM4");
  await page.locator("#radioSearchBtn").click();
  await page.locator(".radio-search-hit", { hasText: "FM4" }).click();
  await expect(page.locator("#radioPicked")).toContainText("FM4");
  await expect(page.locator("#sceneName")).toHaveValue("FM4");
  await page.locator('#sceneMembers .member-row[data-name="Wohnzimmer"] .member-pick').click();
  await page.locator("#sceneSaveBtn").click();

  await expect.poll(() => saved).not.toBeNull();
  expect(saved.scene).toEqual({
    id: "fm4",
    name: "FM4",
    source: { type: "stream", uri: "https://orf.at/fm4.mp3" },
    members: [{ speaker: "Wohnzimmer", volume: 5 }],
  });
  await expect(page.locator("#sceneDialog")).toBeHidden();
  // Stub-Liste kennt fm4 nicht -> kein Play; echter Agent laedt die neue Szene
  expect(played).toBe(false);
});

test("Szene bearbeiten: URL vorbelegt, Slug fest, kein Favoriten-Import", async ({ page }) => {
  await openSceneEditDialog(page);
  await expect(page.locator('input[name="sourceMode"][value="url"]')).toBeChecked();
  await expect(page.locator("#sourceUrl")).toHaveValue("https://example.com/stream.mp3");
  await expect(page.locator("#sourceFavoriteBtn")).toBeHidden();
  await expect(page.locator("#sceneDeleteBtn")).toBeVisible();
  const row = page.locator('#sceneMembers .member-row[data-name="Wohnzimmer"]');
  await expect(row.locator('input[type="checkbox"]')).toBeChecked();
  await expect(row.locator(".vol-value")).toHaveText("20");
  await page.locator(".advanced > summary").click();
  await expect(page.locator("#sceneId")).toHaveAttribute("readonly", "");

  const layout = await readSceneEditorLayout(page);
  expect(layout.nested).toEqual([]);
  expect(layout.saveInViewport).toBe(true);
});

test("Radio-Suchfeld: kein Pico-Lupen-Hintergrund unter dem Platzhalter", async ({ page }) => {
  await openSceneDialogWithSpeakers(page);
  const input = page.locator("#radioSearchInput");
  await expect(input).toBeVisible();
  const css = await input.evaluate((el) => {
    const st = getComputedStyle(el);
    return { bg: st.backgroundImage, pad: parseFloat(st.paddingLeft) };
  });
  expect(css.bg).toBe("none");
  expect(css.pad).toBeLessThanOrEqual(16);
});
