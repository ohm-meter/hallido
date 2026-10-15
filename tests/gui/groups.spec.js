// @ts-check
// Boxen-Tab nach aktueller Sonos-Gruppe (GET /api/v1/groups, ADR 0015):
// je Gruppe Kopf mit Status und Pause/Fortsetzen/Stop, Rest unter „Einzeln“.
const { test, expect } = require("@playwright/test");
const { gotoAppReady, gotoTab, EXPECTED_SPEAKERS } = require("./_layout-helpers");

/** Topologie wie aus der Sonos-App: eine Szene laeuft, eine Box pausiert, eine Gruppe still. */
function topology() {
  return [
    { coordinator: "Garage", members: ["Garage", "Keller"], state: "stopped" },
    { coordinator: "Terrasse", members: ["Terrasse"], state: "paused", title: "Podcast" },
    {
      coordinator: "Wohnzimmer", members: ["Wohnzimmer", "Bad", "Kueche"], state: "playing",
      title: "Hitradio Ö3", scene_id: "morgenradio",
    },
    { coordinator: "Buero", members: ["Buero"], state: "stopped" },
  ];
}

/**
 * GET /groups mit veraenderbarer Topologie; Gruppenbefehle aendern den Zustand
 * des Coordinators und werden protokolliert.
 * @param {import('@playwright/test').Page} page
 */
async function stubGroups(page) {
  const state = { groups: topology(), calls: /** @type {string[]} */ ([]) };
  await page.route(/\/api\/v1\/groups$/, (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ groups: state.groups }) }),
  );
  await page.route(/\/api\/v1\/speakers\/[^/]+\/(pause|resume|stop)$/, async (route) => {
    const parts = new URL(route.request().url()).pathname.split("/");
    const name = decodeURIComponent(parts[4]);
    const action = parts[5];
    state.calls.push(`${route.request().method()} ${name} ${action}`);
    const g = state.groups.find((x) => x.members.includes(name));
    if (g) g.state = { pause: "paused", resume: "playing", stop: "stopped" }[action];
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ speaker: name, coordinator: g ? g.coordinator : name, members: g ? g.members : [name], action, ok: true }),
    });
  });
  return state;
}

const block = (page, coord) => page.locator(`#groupList .group-block[data-coordinator="${coord}"]`);

test("Boxen-Tab: Boxen nach Gruppe, Coordinator vorne, Status + Szene im Kopf, Rest unter Einzeln", async ({ page }) => {
  await gotoAppReady(page);
  await stubGroups(page);
  await gotoTab(page, "boxen");

  await expect(page.locator("#groupList .group-block")).toHaveCount(3);
  await expect(page.locator("#groupList .group-block .group-name")).toHaveText(["Garage +1", "Terrasse", "Wohnzimmer +2"]);

  const wz = block(page, "Wohnzimmer");
  await expect(wz.locator(".group-name")).toHaveText("Wohnzimmer +2");
  await expect(wz.locator(".group-sub")).toHaveText("läuft · Morgenradio");
  await expect(wz.locator(".speaker-card .speaker-name")).toHaveText(["Wohnzimmer", "Bad", "Kueche"]);
  await expect(wz).toHaveAttribute("data-state", "playing");
  await expect(wz.getByRole("button", { name: "Pausieren: Gruppe Wohnzimmer" })).toBeVisible();
  await expect(wz.getByRole("button", { name: "Stoppen: Gruppe Wohnzimmer" })).toBeVisible();

  const ter = block(page, "Terrasse");
  await expect(ter.locator(".group-sub")).toHaveText("pausiert · Podcast");
  await expect(ter.getByRole("button", { name: "Fortsetzen: Gruppe Terrasse" })).toBeVisible();
  await expect(ter.getByRole("button", { name: "Stoppen: Gruppe Terrasse" })).toBeVisible();

  // Gestoppte Gruppe: nur Fortsetzen, kein Stop
  const gar = block(page, "Garage");
  await expect(gar.getByRole("button", { name: "Fortsetzen: Gruppe Garage" })).toBeVisible();
  await expect(gar.getByRole("button", { name: "Stoppen: Gruppe Garage" })).toBeHidden();

  // Einzeln: alle uebrigen Boxen (inkl. allein stehender, gestoppter Buero)
  await expect(page.locator("#singlesLabel")).toBeVisible();
  await expect(page.locator("#speakerList > .speaker-card")).toHaveCount(EXPECTED_SPEAKERS - 6);
  await expect(page.locator("#speakerList .speaker-card", { hasText: "Buero" })).toHaveCount(1);
  await expect(page.locator(".speaker-card")).toHaveCount(EXPECTED_SPEAKERS);

  // Amber nur fuer die laufende Gruppe
  const shadows = await page.evaluate(() =>
    [...document.querySelectorAll("#groupList .group-block")].map((b) => [b.dataset.coordinator, getComputedStyle(b).boxShadow !== "none"]),
  );
  expect(Object.fromEntries(shadows)).toEqual({ Garage: false, Terrasse: false, Wohnzimmer: true });
});

test("Gruppe: Pause, Fortsetzen und Stop gehen an den Coordinator und aktualisieren den Kopf", async ({ page }) => {
  await gotoAppReady(page);
  const state = await stubGroups(page);
  await gotoTab(page, "boxen");
  const wz = block(page, "Wohnzimmer");

  await wz.getByRole("button", { name: "Pausieren: Gruppe Wohnzimmer" }).click();
  await expect.poll(() => state.calls).toEqual(["POST Wohnzimmer pause"]);
  await expect(wz).toHaveAttribute("data-state", "paused");
  await expect(wz.locator(".group-state")).toHaveText("pausiert");

  await wz.getByRole("button", { name: "Fortsetzen: Gruppe Wohnzimmer" }).click();
  await expect.poll(() => state.calls.length).toBe(2);
  await expect(wz).toHaveAttribute("data-state", "playing");

  await wz.getByRole("button", { name: "Stoppen: Gruppe Wohnzimmer" }).click();
  await expect.poll(() => state.calls).toEqual(["POST Wohnzimmer pause", "POST Wohnzimmer resume", "POST Wohnzimmer stop"]);
  await expect(wz).toHaveAttribute("data-state", "stopped");
  await expect(wz.getByRole("button", { name: "Stoppen: Gruppe Wohnzimmer" })).toBeHidden();
});

test("Gruppe: Fehler der API erscheint unveraendert als Hinweis", async ({ page }) => {
  await gotoAppReady(page);
  await stubGroups(page);
  await page.route(/\/api\/v1\/speakers\/Wohnzimmer\/stop$/, (route) =>
    route.fulfill({
      status: 502,
      contentType: "application/json",
      body: JSON.stringify({ code: "speaker_unreachable", message: "Lautsprecher antwortet nicht", hint: "Box eingeschaltet?" }),
    }),
  );
  await gotoTab(page, "boxen");
  await block(page, "Wohnzimmer").getByRole("button", { name: "Stoppen: Gruppe Wohnzimmer" }).click();
  await expect(page.locator("#hint")).toHaveText("Lautsprecher antwortet nicht (Box eingeschaltet?)");
  await expect(page.locator("#hint")).toHaveClass(/error/);
  await expect(block(page, "Wohnzimmer")).toHaveAttribute("data-state", "playing");
});

test("Boxen-Tab ohne Gruppen-Antwort: flache Liste wie bisher, kein Einzeln-Label", async ({ page }) => {
  await gotoAppReady(page);
  await gotoTab(page, "boxen");
  await expect(page.locator("#speakerList > .speaker-card")).toHaveCount(EXPECTED_SPEAKERS);
  await expect(page.locator("#groupList .group-block")).toHaveCount(0);
  await expect(page.locator("#singlesLabel")).toBeHidden();
});
