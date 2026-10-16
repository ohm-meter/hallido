// @ts-check
// First-run setup wizard (ADR 0022) and log sheet (ADR 0023). The agent under
// test runs without setup; the setup endpoints are stubbed per test.
const { test, expect } = require("@playwright/test");
const { gotoAppReady, gotoTab, readTouchTargets, readHorizontalOverflow } = require("./_layout-helpers");

const SAMPLE_ACCESS_KEY = "7KQ2-M9XP-4RTV-8WHC-2DJN-6PFB-9ZQS-3MEA";

/**
 * Stubs a fresh installation: setup open, token creation, speaker search.
 * @param {import('@playwright/test').Page} page
 * @param {{ open?: boolean, reason?: string, tokenStatus?: number, tokenBody?: object }} [opts]
 */
async function stubSetup(page, opts = {}) {
  const state = { done: false, tokenRequests: [], speakerAuth: [] };
  const json = (route, status, body) =>
    route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
  await page.route(/\/api\/v1\//, async (route) => {
    const req = route.request();
    const path = new URL(req.url()).pathname;
    if (path === "/api/v1/setup") {
      if (state.done) return json(route, 200, { setup_required: false, open: false });
      const open = opts.open !== false;
      return json(route, 200, { setup_required: true, open, reason: open ? undefined : opts.reason, expires_in_seconds: 3500 });
    }
    if (path === "/api/v1/setup/token") {
      state.tokenRequests.push({ headers: req.headers(), body: req.postDataJSON() });
      if (opts.tokenStatus) return json(route, opts.tokenStatus, opts.tokenBody || {});
      state.done = true;
      return json(route, 201, { token: SAMPLE_ACCESS_KEY });
    }
    if (path === "/api/v1/speakers" || path === "/api/v1/discovery/refresh") {
      state.speakerAuth.push(req.headers()["authorization"]);
      return json(route, 200, {
        speakers: [
          { name: "Wohnzimmer", ip: "192.168.1.10", reachable: true },
          { name: "Bad", ip: "192.168.1.11", reachable: false },
        ],
        discovery_error: "",
      });
    }
    if (path === "/api/v1/health") return json(route, 200, { status: "ok", speakers_online: 1, speakers: ["Wohnzimmer"] });
    if (path === "/api/v1/scenes") return json(route, 200, { scenes: [] });
    if (path === "/api/v1/scenes/status") return json(route, 200, { scenes: [] });
    if (path === "/api/v1/schedules") return json(route, 200, { timezone: "Europe/Vienna", now: "2026-10-15T09:00:00+02:00", schedules: [] });
    if (path === "/api/v1/groups") return json(route, 200, { groups: [] });
    return route.continue();
  });
  return state;
}

async function expectTouchFriendly(page, where) {
  const t = await readTouchTargets(page);
  expect(t.count, `${where}: no targets`).toBeGreaterThan(0);
  expect(t.small, `${where}: targets < 44 px`).toEqual([]);
  expect(t.overlaps, `${where}: overlapping targets`).toEqual([]);
  expect(t.outside, `${where}: targets outside the viewport`).toEqual([]);
  expect(t.docOverflow, `${where}: horizontal overflow`).toBe(false);
}

test("Setup: Schlüssel erstellen, einmal anzeigen, Boxen suchen, erste Szene", async ({ page }) => {
  const state = await stubSetup(page);
  await page.goto("/");
  const dlg = page.locator("#setupDialog");
  await expect(dlg).toBeVisible();
  await expect(dlg.getByRole("heading", { name: "Willkommen bei Hallido" })).toBeVisible();
  await expect(page.locator("#setupStepNo")).toHaveText("Schritt 1 von 4");
  await expectTouchFriendly(page, "Setup: Willkommen");

  // Escape does not close the wizard.
  await page.keyboard.press("Escape");
  await expect(dlg).toBeVisible();

  await page.getByRole("button", { name: "Zugangsschlüssel erstellen" }).click();
  await expect(page.locator("#setupToken")).toHaveText(SAMPLE_ACCESS_KEY);
  expect(state.tokenRequests).toHaveLength(1);
  expect(state.tokenRequests[0].headers["x-hallido-setup"]).toBe("1");
  expect(typeof state.tokenRequests[0].body.timezone).toBe("string");
  expect(await page.evaluate(() => localStorage.getItem("sra_token"))).toBe(SAMPLE_ACCESS_KEY);
  await expectTouchFriendly(page, "Setup: Schlüssel");
  expect((await readHorizontalOverflow(page)).docOverflow).toBe(false);

  // "Next" only after confirming that the key was saved.
  const next = page.locator("#setupTokenNext");
  await expect(next).toBeDisabled();
  await page.getByText("Ich habe den Zugangsschlüssel gespeichert").click();
  await expect(next).toBeEnabled();
  await next.click();

  await expect(page.locator("#setupSpeakerList li")).toHaveCount(2);
  await expect(page.locator("#setupSpeakerHint")).toHaveText("2 Boxen gefunden.");
  expect(state.speakerAuth[0]).toBe("Bearer " + SAMPLE_ACCESS_KEY);
  await expectTouchFriendly(page, "Setup: Boxen");
  await page.getByRole("button", { name: "Erneut suchen" }).click();
  await expect.poll(() => state.speakerAuth.length).toBe(2);

  await page.locator("#setupSpeakersNext").click();
  await expect(dlg.getByRole("heading", { name: "Hallido ist bereit" })).toBeVisible();
  await expectTouchFriendly(page, "Setup: Fertig");
  await page.getByRole("button", { name: "Erste Szene anlegen" }).click();

  await expect(dlg).toBeHidden();
  await expect(page.locator("#sceneDialog")).toBeVisible();

  // Reload: setup is done, the key is still in the browser, no wizard.
  await page.reload();
  await expect(page.locator("#addBtn")).toBeVisible();
  await expect(dlg).toBeHidden();
  expect(state.tokenRequests).toHaveLength(1);
});

test("Setup: Zeitfenster abgelaufen zeigt Neustart-Hinweis", async ({ page }) => {
  await stubSetup(page, { open: false, reason: "expired" });
  await page.goto("/");
  await expect(page.locator("#setupDialog").getByRole("heading", { name: "Einrichtungszeit abgelaufen" })).toBeVisible();
  await expect(page.locator("#setupBlockedText")).toContainText("Container");
  await expect(page.locator("#setupCreateBtn")).toBeHidden();
  await expectTouchFriendly(page, "Setup: abgelaufen");
});

test("Setup: nicht aus dem Heimnetz (Server lehnt ab)", async ({ page }) => {
  await stubSetup(page, {
    tokenStatus: 403,
    tokenBody: { error: "forbidden", message: "Setup is only possible from your home network" },
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Zugangsschlüssel erstellen" }).click();
  await expect(page.locator("#setupDialog").getByRole("heading", { name: "Nur aus dem Heimnetz" })).toBeVisible();
  expect(await page.evaluate(() => localStorage.getItem("sra_token"))).toBeNull();
});

test("Setup: Fehler beim Speichern bleibt im Schritt mit Meldung", async ({ page }) => {
  await stubSetup(page, {
    tokenStatus: 500,
    tokenBody: { error: "internal", message: "Could not save the configuration" },
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Zugangsschlüssel erstellen" }).click();
  await expect(page.locator("#setupError")).toHaveText("Could not save the configuration");
  await expect(page.locator("#setupCreateBtn")).toBeEnabled();
});

test("Setup: English texts", async ({ browser }) => {
  const context = await browser.newContext({ locale: "en-US" });
  const page = await context.newPage();
  await stubSetup(page);
  await page.goto("/");
  await expect(page.getByRole("button", { name: "Create access key" })).toBeVisible();
  await expect(page.locator("#setupStepNo")).toHaveText("Step 1 of 4");
  await context.close();
});

test("Protokoll: Boxen → Protokoll zeigt Einträge, neueste zuerst", async ({ page }) => {
  await gotoAppReady(page);
  await page.route(/\/api\/v1\/logs/, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        file: true,
        entries: [
          { time: "2026-10-15T08:00:00Z", level: "INFO", msg: "setup: access key created, setup closed", attrs: { event: "setup", client: "192.168.1.50" } },
          { time: "2026-10-15T08:05:00Z", level: "WARN", msg: "speaker unreachable", attrs: { speaker: "Bad" } },
        ],
      }),
    }),
  );
  await gotoTab(page, "boxen");
  await page.getByRole("button", { name: "Protokoll", exact: true }).click();
  const sheet = page.locator("#logSheet");
  await expect(sheet).toBeVisible();
  await expect(sheet.locator(".log-entry")).toHaveCount(2);
  await expect(sheet.locator(".log-entry").first()).toContainText("speaker unreachable");
  await expect(sheet.locator(".log-entry").first().locator(".log-level")).toHaveText("Warnung");
  await expect(sheet.locator(".log-entry").nth(1)).toContainText("client=192.168.1.50");
  await expect(page.locator("#logHint")).toContainText("NAS");
  await expectTouchFriendly(page, "Protokoll-Sheet");
  await sheet.getByRole("button", { name: "Schließen" }).click();
  await expect(sheet).toBeHidden();
});
