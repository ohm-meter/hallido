// @ts-check
// Play scheitert auf dem Coordinator (z. B. Stream-Format von Sonos nicht
// unterstuetzt): Fehler mit Hinweis anzeigen, nicht als laufend/teilweise
// gestartet werten und nicht endlos im Lade-Zustand pulsieren.
const { test, expect } = require("@playwright/test");
const { gotoAppReady } = require("./_layout-helpers");

const row = (page) => page.locator('#grid .scene-row[data-id="morgenradio"]');

const HINT =
  "Stream-Format von Sonos nicht unterstuetzt oder Stream fuer die Box nicht erreichbar (erkannt: audio/aac, Box-Status: STOPPED).";

test("Play mit playback_failed (502 + result): Fehler mit Hinweis, kein Teilstart, kein Mini-Player", async ({ page }) => {
  await gotoAppReady(page);
  await page.route(/\/api\/v1\/scenes\/status$/, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ scenes: [{ scene_id: "morgenradio", state: "stopped" }] }),
    }),
  );
  await page.route(/\/api\/v1\/scenes\/morgenradio\/play$/, (route) =>
    route.fulfill({
      status: 502,
      contentType: "application/json",
      body: JSON.stringify({
        error: "playback_failed",
        message: "Box startet die Wiedergabe nicht",
        speaker: "Wohnzimmer",
        hint: HINT,
        // Mitglieder wurden gruppiert, nur der Coordinator spielt nicht.
        result: {
          scene_id: "morgenradio",
          playing: false,
          speakers: [
            { speaker: "Wohnzimmer", ok: false, error: "playback_failed", hint: HINT },
            { speaker: "Kueche", ok: true },
            { speaker: "Bad", ok: true },
          ],
        },
      }),
    }),
  );
  await page.reload();
  await expect(row(page)).toHaveAttribute("data-state", "stopped");

  await row(page).locator(".scene-play").click();

  await expect(row(page)).toHaveAttribute("data-state", "error");
  await expect(page.locator("#hint")).toHaveText("Box startet die Wiedergabe nicht (" + HINT + ")");
  await expect(page.locator("#hint")).toHaveClass(/error/);
  await expect(page.locator("#hint")).not.toContainText("Teilweise");
  await expect(page.locator("#nowPlaying")).toBeHidden();
  await expect(row(page).locator(".scene-play")).toHaveAccessibleName("Abspielen: Morgenradio");

  // Kein Dauer-Pulsieren: der Knopf hat im Fehlerzustand keine Animation.
  const anim = await row(page).locator(".scene-play").evaluate((el) => getComputedStyle(el).animationName);
  expect(anim).toBe("none");

  // Nach kurzer Fehleranzeige uebernimmt wieder der Live-Status (gestoppt).
  await expect(row(page)).toHaveAttribute("data-state", "stopped", { timeout: 8_000 });
  await expect(page.locator("#hint")).toContainText("Box startet die Wiedergabe nicht");
});

test("Teilausfall bei HTTP 200 bleibt Teilstart", async ({ page }) => {
  await gotoAppReady(page);
  await page.route(/\/api\/v1\/scenes\/morgenradio\/play$/, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        scene_id: "morgenradio",
        playing: true,
        speakers: [
          { speaker: "Wohnzimmer", ok: true },
          { speaker: "Kueche", ok: true },
          { speaker: "Bad", ok: false, error: "speaker_unreachable" },
        ],
      }),
    }),
  );
  await row(page).locator(".scene-play").click();
  await expect(row(page)).toHaveAttribute("data-state", "partial");
  await expect(page.locator("#hint")).toContainText("Teilweise gestartet — nicht erreichbar: Bad");
});
