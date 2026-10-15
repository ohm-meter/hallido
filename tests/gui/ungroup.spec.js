// @ts-check
// „Aus Gruppe lösen“ im Box-Detail (POST /api/v1/speakers/{name}/ungroup, ADR 0015):
// nur fuer gruppierte Boxen, immer mit Bestaetigung, Teilausfall je Box.
const { test, expect } = require("@playwright/test");
const { gotoAppReady, gotoTab } = require("./_layout-helpers");

/**
 * Topologie mit einer 3er-Gruppe; ungroup aendert sie wie die echte Box.
 * @param {import('@playwright/test').Page} page
 * @param {object} [result] feste Antwort statt der simulierten
 */
async function stubUngroup(page, result) {
  const state = {
    groups: [
      { coordinator: "Wohnzimmer", members: ["Wohnzimmer", "Bad", "Kueche"], state: "playing", title: "Ö3" },
      { coordinator: "Buero", members: ["Buero"], state: "stopped" },
    ],
    calls: /** @type {string[]} */ ([]),
  };
  await page.route(/\/api\/v1\/groups$/, (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ groups: state.groups }) }),
  );
  await page.route(/\/api\/v1\/speakers\/[^/]+\/ungroup$/, async (route) => {
    const name = decodeURIComponent(new URL(route.request().url()).pathname.split("/")[4]);
    state.calls.push(`${route.request().method()} ${name}`);
    const g = state.groups[0];
    const dissolved = g.coordinator === name;
    const leaving = dissolved ? g.members.filter((m) => m !== name) : [name];
    if (!result) {
      g.members = g.members.filter((m) => !leaving.includes(m));
      for (const m of leaving) state.groups.push({ coordinator: m, members: [m], state: "stopped" });
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(result || {
        speaker: name, coordinator: g.coordinator, dissolved, standalone: false,
        speakers: leaving.map((m) => ({ speaker: m, ok: true })),
      }),
    });
  });
  return state;
}

/** @param {import('@playwright/test').Page} page @param {string} name */
async function openBox(page, name) {
  await page.locator(".speaker-card", { hasText: name }).click();
  await expect(page.locator("#discoverPanel")).toBeVisible();
}

test("Mitglied: Aus Gruppe loesen nur mit Bestaetigung, danach unter Einzeln", async ({ page }) => {
  await gotoAppReady(page);
  const state = await stubUngroup(page);
  await gotoTab(page, "boxen");
  await expect(page.locator("#groupList .group-block")).toHaveCount(1);
  await openBox(page, "Kueche");

  const btn = page.getByRole("button", { name: "Aus Gruppe lösen: Kueche" });
  await expect(btn).toBeVisible();
  await btn.click();
  const dlg = page.locator("#ungroupDialog");
  await expect(dlg).toBeVisible();
  await expect(page.locator("#ungroupTitle")).toHaveText("Aus Gruppe lösen");
  await expect(page.locator("#ungroupMessage")).toContainText("„Kueche“ verlässt die Gruppe von „Wohnzimmer“");

  await dlg.getByRole("button", { name: "Abbrechen" }).click();
  await expect(dlg).toBeHidden();
  expect(state.calls).toEqual([]);

  await btn.click();
  await dlg.getByRole("button", { name: "Lösen" }).click();
  await expect.poll(() => state.calls).toEqual(["POST Kueche"]);
  await expect(page.locator("#hint")).toHaveText("„Kueche“ aus der Gruppe gelöst.");
  await expect(page.locator("#hint")).not.toHaveClass(/error/);
  await expect(page.locator("#groupList .group-block .speaker-name")).toHaveText(["Wohnzimmer", "Bad"]);
  await expect(page.locator("#speakerList .speaker-card", { hasText: "Kueche" })).toHaveCount(1);
  await expect(btn).toBeHidden();
});

test("Coordinator: Gruppe aufloesen, Teilausfall wird je Box gemeldet", async ({ page }) => {
  await gotoAppReady(page);
  const state = await stubUngroup(page, {
    speaker: "Wohnzimmer", coordinator: "Wohnzimmer", dissolved: true, standalone: false,
    speakers: [{ speaker: "Bad", ok: true }, { speaker: "Kueche", ok: false, error: "Lautsprecher antwortet nicht" }],
  });
  await gotoTab(page, "boxen");
  await openBox(page, "Wohnzimmer");

  await page.getByRole("button", { name: "Gruppe auflösen: Wohnzimmer" }).click();
  await expect(page.locator("#ungroupTitle")).toHaveText("Gruppe auflösen");
  await expect(page.locator("#ungroupMessage")).toContainText("Die ganze Gruppe wird aufgelöst");
  await expect(page.locator("#ungroupMessage")).toContainText("Bad, Kueche");
  await page.locator("#ungroupDialog").getByRole("button", { name: "Auflösen" }).click();
  await expect.poll(() => state.calls).toEqual(["POST Wohnzimmer"]);
  await expect(page.locator("#hint")).toHaveText("Nicht gelöst: Kueche (Lautsprecher antwortet nicht)");
  await expect(page.locator("#hint")).toHaveClass(/error/);
});

test("Allein stehende Box: kein Aufloesen-Knopf", async ({ page }) => {
  await gotoAppReady(page);
  await stubUngroup(page);
  await gotoTab(page, "boxen");
  await expect(page.locator("#groupList .group-block")).toHaveCount(1);
  await openBox(page, "Buero");
  await expect(page.locator("#discoverUngroupBtn")).toBeHidden();
});

test("Aufloesen scheitert komplett (502): Meldung + Boxen im Hinweis", async ({ page }) => {
  await gotoAppReady(page);
  await stubUngroup(page);
  await page.route(/\/api\/v1\/speakers\/Bad\/ungroup$/, (route) =>
    route.fulfill({
      status: 502,
      contentType: "application/json",
      body: JSON.stringify({
        code: "speaker_unreachable", message: "Keine Box konnte die Gruppe verlassen",
        result: { speaker: "Bad", coordinator: "Wohnzimmer", dissolved: false, speakers: [{ speaker: "Bad", ok: false, error: "Lautsprecher antwortet nicht" }] },
      }),
    }),
  );
  await gotoTab(page, "boxen");
  await openBox(page, "Bad");
  await page.getByRole("button", { name: "Aus Gruppe lösen: Bad" }).click();
  await page.locator("#ungroupDialog").getByRole("button", { name: "Lösen" }).click();
  await expect(page.locator("#hint")).toHaveText("Keine Box konnte die Gruppe verlassen — Bad (Lautsprecher antwortet nicht)");
  await expect(page.locator("#hint")).toHaveClass(/error/);
});
