// @ts-check
// Sonos-Favoriten als Quelle (ADR 0018): TuneIn-Sender lassen sich als
// Favoriten-Szene anlegen; beim Bearbeiten bleibt die Favoriten-Quelle
// erhalten, solange keine neue URL eingegeben wird.
const { test, expect } = require("@playwright/test");
const { gotoAppReady, openSceneDialogWithSpeakers } = require("./_layout-helpers");

const FAVORITES = {
  speaker: "Wohnzimmer",
  favorites: [
    { id: "FV:2/1", title: "Test Radio", uri: "https://stream.example/live", integratable: true, playable: true },
    { id: "FV:2/2", title: "Spotify Track", uri: "x-sonos-spotify:spotify:track:abc", integratable: false, playable: false },
    {
      id: "FV:2/78",
      title: "Radio Superfly",
      uri: "x-sonosapi-stream:tunein%3a17315?sid=303&flags=8232&sn=9",
      integratable: false,
      playable: true,
    },
  ],
};

test("Neue Szene aus TuneIn-Favorit: wählbar, Hinweis, from-favorite mit favorite_id", async ({ page }) => {
  // Eigene Routen nach dem Standard-Stub registrieren (spaetere Routen gewinnen).
  await openSceneDialogWithSpeakers(page);
  await page.route(/\/api\/v1\/sonos\/favorites/, (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(FAVORITES) }),
  );
  /** @type {any} */
  let saved = null;
  await page.route(/\/api\/v1\/config\/scenes\/from-favorite$/, async (route) => {
    saved = route.request().postDataJSON();
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ok: true, scene_id: "superfly", source_type: "favorite" }),
    });
  });

  await page.locator("#sourceFavoriteBtn").click();
  const select = page.locator("#sourceFavoriteSelect");
  await expect(select).toBeEnabled();
  // Spotify bleibt draussen, TuneIn ist als Sonos-Favorit markiert.
  await expect(select.locator("option")).toHaveText(["Favorit wählen…", "Test Radio", "Radio Superfly (Sonos-Favorit)"]);

  await select.selectOption("FV:2/78");
  await expect(page.locator("#sourceFavoritePreview")).toContainText("als Sonos-Favorit gespeichert");

  await page.locator("#sceneName").fill("Superfly");
  await page.locator('#sceneMembers .member-row[data-name="Wohnzimmer"] .member-pick').click();
  await page.locator("#sceneSaveBtn").click();

  await expect.poll(() => saved).not.toBeNull();
  expect(saved.favorite_id).toBe("FV:2/78");
  expect(saved.scene_id).toBe("superfly");
  expect(saved.members).toEqual([{ speaker: "Wohnzimmer", volume: 5 }]);
});

test("Favoriten-Szene bearbeiten: Quelle bleibt erhalten, neue URL ersetzt sie", async ({ page }) => {
  const favScene = {
    id: "superfly-fm",
    name: "Superfly Fm",
    source: { type: "favorite", name: "Radio Superfly", uri: "x-sonosapi-stream:tunein%3a17315?sid=303&flags=8232&sn=9" },
    members: [{ speaker: "Wohnzimmer", volume: 3 }],
  };
  await gotoAppReady(page);
  await page.route(/\/api\/v1\/scenes$/, (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ scenes: [favScene] }) }),
  );
  /** @type {any[]} */
  const puts = [];
  await page.route(/\/api\/v1\/config\/scenes\/superfly-fm$/, async (route) => {
    puts.push(route.request().postDataJSON());
    await route.fulfill({ status: 200, contentType: "application/json", body: '{"ok":true}' });
  });
  await page.route(/\/api\/v1\/scenes\/superfly-fm\/play$/, (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: '{"scene_id":"superfly-fm","playing":true,"speakers":[]}' }),
  );
  await page.reload();
  await expect(page.locator('#grid .scene-row[data-id="superfly-fm"]')).toBeVisible();

  const openEdit = async () => {
    await page.getByRole("button", { name: "Aktionen: Superfly Fm" }).click();
    await page.locator("#sceneMenuEdit").click();
    await expect(page.locator("#sceneDialogTitle")).toHaveText("Szene bearbeiten");
  };

  await openEdit();
  await expect(page.locator("#sourceUrl")).toHaveValue("");
  await expect(page.locator("#sourceUrlHint")).toBeVisible();
  await expect(page.locator("#sourceUrlHint")).toContainText("Radio Superfly");
  await page.locator("#sceneSaveBtn").click();
  await expect.poll(() => puts.length).toBe(1);
  expect(puts[0].source).toEqual(favScene.source);

  await openEdit();
  await page.locator("#sourceUrl").fill("https://stream.example/neu");
  await page.locator("#sceneSaveBtn").click();
  await expect.poll(() => puts.length).toBe(2);
  expect(puts[1].source).toEqual({ type: "stream", uri: "https://stream.example/neu" });
});
