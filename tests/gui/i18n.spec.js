// @ts-check
// Bilingual web app (ADR 0019): English by default, German when the browser
// language starts with "de". The other specs run in de-DE (playwright.config.js).
const { test, expect } = require("@playwright/test");
const { gotoAppReady, gotoTab } = require("./_layout-helpers");

test.describe("English (en-US)", () => {
  test.use({ locale: "en-US" });

  test("tabs, header, scene editor, schedules and speakers show English labels", async ({ page }) => {
    await gotoAppReady(page);
    await expect(page.locator("html")).toHaveAttribute("lang", "en");

    // Header and tabs
    await expect(page.locator("#viewTitle")).toHaveText("Scenes");
    await expect(page.locator("#addBtn")).toHaveAttribute("aria-label", "New scene");
    await expect(page.locator(".tabbar")).toHaveAttribute("aria-label", "Main navigation");
    await expect(page.locator(".tabbar .tab-label")).toHaveText(["Scenes", "Schedules", "Speakers"]);
    await expect(page.locator("#statusText")).toHaveText("15 speakers");
    await expect(page.getByRole("button", { name: "Actions: Morgenradio" })).toBeVisible();
    await expect(page.locator('.scene-row[data-id="morgenradio"] .scene-play')).toHaveAttribute("aria-label", "Play: Morgenradio");
    await expect(page.locator("#meta")).toHaveText("6 scenes");

    // Scene editor
    await page.getByRole("button", { name: "New scene" }).click();
    await expect(page.locator("#sceneDialog")).toBeVisible();
    await expect(page.locator("#sceneDialogTitle")).toHaveText("New scene");
    await expect(page.locator(".seg-opt")).toHaveText(["Search radio", "From speaker", "URL"]);
    await expect(page.locator("#radioSearchInput")).toHaveAttribute("placeholder", "Search stations, e.g. FM4");
    await expect(page.locator("#sourceFavoriteBtn")).toHaveText(/From Sonos favorites/);
    await expect(page.locator("#sceneMembersLabel")).toHaveText("Rooms & volume");
    await expect(page.locator("#sceneSaveBtn")).toHaveText("Save & play");
    await expect(page.locator("#sceneCancelBtn")).toHaveText("Cancel");
    await expect(page.getByRole("button", { name: "Kueche louder", includeHidden: true })).toBeAttached();
    await expect(page.getByRole("slider", { name: "Volume Kueche", includeHidden: true })).toBeAttached();
    await page.locator("#sceneCancelBtn").click();

    // Schedules
    await gotoTab(page, "zeitplaene");
    await expect(page.locator("#viewTitle")).toHaveText("Schedules");
    await expect(page.locator("#addBtn")).toHaveAttribute("aria-label", "New schedule");
    await expect(page.locator("#scheduleList .schedule-when").first()).toHaveText("Layout-Test · Mo–Fr");
    await expect(page.locator("#scheduleList .schedule-next").first()).toContainText("Next start");
    await expect(page.locator("#scheduleList .schedule-next").nth(1)).toHaveText("disabled");
    await expect(page.getByRole("switch", { name: "Schedule “Layout-Test” 06:10 enabled" })).toBeAttached();
    await page.getByRole("button", { name: "New schedule" }).click();
    await expect(page.locator("#scheduleDialogTitle")).toHaveText("New schedule");
    await expect(page.locator(".day-preset")).toHaveText(["Mon–Fri", "Weekends", "Daily"]);
    await expect(page.locator("#scheduleDays .day-chip span")).toHaveText(["Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"]);
    await expect(page.getByRole("checkbox", { name: "Wednesday" })).toBeAttached();
    await expect(page.locator("#scheduleSaveBtn")).toHaveText("Save schedule");
    await page.locator("#scheduleCancelBtn").click();

    // Speakers and settings
    await gotoTab(page, "boxen");
    await expect(page.locator("#viewTitle")).toHaveText("Speakers");
    await expect(page.locator("#refreshSpeakersBtn")).toHaveAttribute("aria-label", "Refresh found speakers");
    await expect(page.locator("#refreshSpeakersBtn .btn-label")).toHaveText("Refresh");
    await expect(page.locator("#tokenBtn")).toHaveText("Change API token");
    await expect(page.locator("#saveSpeakersConfigBtn .btn-label")).toHaveText("Save speakers to config");
    await expect(page.locator("#themeToggle .btn-label")).toHaveText("Appearance");
    await expect(page.locator("#themeLabel")).toHaveText(/^(Dark|Light)$/);
    await expect(page.locator("#themeToggle")).toHaveAttribute("aria-label", /^Switch to (light|dark) theme$/);
  });
});

test.describe("German (de-DE)", () => {
  test.use({ locale: "de-DE" });

  test("German labels and html lang=de", async ({ page }) => {
    await gotoAppReady(page);
    await expect(page.locator("html")).toHaveAttribute("lang", "de");
    await expect(page.locator("#viewTitle")).toHaveText("Szenen");
    await expect(page.locator("#addBtn")).toHaveAttribute("aria-label", "Neue Szene");
    await expect(page.locator(".tabbar")).toHaveAttribute("aria-label", "Hauptnavigation");
    await expect(page.locator(".tabbar .tab-label")).toHaveText(["Szenen", "Zeitpläne", "Boxen"]);
    await expect(page.locator("#statusText")).toHaveText("15 Boxen");
    await expect(page.locator('.scene-row[data-id="morgenradio"] .scene-play')).toHaveAttribute("aria-label", "Abspielen: Morgenradio");

    await page.getByRole("button", { name: "Neue Szene" }).click();
    await expect(page.locator(".seg-opt")).toHaveText(["Radio suchen", "Von Box", "URL"]);
    await expect(page.locator("#sceneSaveBtn")).toHaveText("Speichern & abspielen");
    await expect(page.locator("#sceneName")).toHaveAttribute("placeholder", "z. B. FM4 Wohnzimmer");
    await page.locator("#sceneCancelBtn").click();

    await gotoTab(page, "zeitplaene");
    await page.getByRole("button", { name: "Neuer Zeitplan" }).click();
    await expect(page.locator(".day-preset")).toHaveText(["Mo–Fr", "Wochenende", "Täglich"]);
    await expect(page.locator("#scheduleDays .day-chip span")).toHaveText(["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"]);
    await page.locator("#scheduleCancelBtn").click();

    await gotoTab(page, "boxen");
    await expect(page.locator("#tokenBtn")).toHaveText("API-Token ändern");
    await expect(page.locator("#saveSpeakersConfigBtn")).toHaveAttribute("aria-label", "Gefundene Boxen in config.json speichern");
  });
});

/** Dictionary as served (locales/<lang>.json, ADR 0019). */
async function fetchDict(request, lang) {
  const res = await request.get(`/locales/${lang}.json`);
  expect(res.status(), `locales/${lang}.json`).toBe(200);
  return res.json();
}

test("dictionaries: locales/en.json and de.json have the same keys, no empty values", async ({ page, request }) => {
  const en = await fetchDict(request, "en");
  const de = await fetchDict(request, "de");
  await gotoAppReady(page);
  const result = await page.evaluate(async ({ en, de }) => {
    // Keys used in app.js (string literals with a known key prefix).
    const src = await (await fetch(document.querySelector('script[src^="app.js"]').src)).text();
    const prefixes = new Set(Object.keys(en).map((k) => k.split(".")[0]));
    const jsKeys = [...src.matchAll(/"([a-z]+\.[A-Za-z_.]+)"/g)]
      .map((m) => m[1])
      .filter((k) => prefixes.has(k.split(".")[0]));
    const enKeys = Object.keys(en);
    const deKeys = Object.keys(de);
    const empty = [];
    for (const [lang, dict] of [["en", en], ["de", de]]) {
      for (const [k, v] of Object.entries(dict)) {
        if (typeof v !== "string" || v.trim() === "") empty.push(lang + ":" + k);
      }
    }
    const used = [...document.querySelectorAll("[data-i18n],[data-i18n-aria],[data-i18n-title],[data-i18n-placeholder]")]
      .flatMap((n) => [n.dataset.i18n, n.dataset.i18nAria, n.dataset.i18nTitle, n.dataset.i18nPlaceholder])
      .filter(Boolean);
    return {
      count: enKeys.length,
      missingInDe: enKeys.filter((k) => !(k in de)),
      missingInEn: deKeys.filter((k) => !(k in en)),
      empty,
      unknownInMarkup: used.filter((k) => !(k in en)),
      jsKeyCount: jsKeys.length,
      unknownInJs: jsKeys.filter((k) => !(k in en) && !(k + ".one" in en)),
      pluralsComplete: enKeys
        .filter((k) => k.endsWith(".one") || k.endsWith(".other"))
        .filter((k) => !(k.replace(/\.(one|other)$/, k.endsWith(".one") ? ".other" : ".one") in en)),
    };
  }, { en, de });
  expect(result.jsKeyCount).toBeGreaterThan(100);
  expect(result.unknownInJs).toEqual([]);
  expect(result.count).toBeGreaterThan(100);
  expect(result.missingInDe).toEqual([]);
  expect(result.missingInEn).toEqual([]);
  expect(result.empty).toEqual([]);
  expect(result.unknownInMarkup).toEqual([]);
  expect(result.pluralsComplete).toEqual([]);
});

test.describe("fallback", () => {
  test.use({ locale: "de-DE" });

  test("key missing in de.json shows English; missing everywhere shows the key", async ({ page }) => {
    const warnings = [];
    page.on("console", (msg) => {
      if (msg.type() === "warning") warnings.push(msg.text());
    });
    const strip = (removed) => async (route) => {
      const res = await route.fetch();
      const dict = await res.json();
      for (const k of removed) delete dict[k];
      await route.fulfill({ response: res, json: dict });
    };
    await page.route(/\/locales\/de\.json/, strip(["tab.scenes", "scene.new", "tab.speakers"]));
    await page.route(/\/locales\/en\.json/, strip(["tab.speakers"]));
    await gotoAppReady(page);

    await expect(page.locator("html")).toHaveAttribute("lang", "de");
    // English fallback for the removed German keys …
    await expect(page.locator("#viewTitle")).toHaveText("Scenes");
    await expect(page.locator("#addBtn")).toHaveAttribute("aria-label", "New scene");
    // … German for the rest, the key itself when English is missing too.
    await expect(page.locator(".tabbar .tab-label")).toHaveText(["Scenes", "Zeitpläne", "tab.speakers"]);
    expect(warnings.filter((w) => w.includes("missing key tab.speakers"))).toHaveLength(1);
  });

  test("I18N.ready resolves and loads de + en for German browsers", async ({ page }) => {
    await gotoAppReady(page);
    const loaded = await page.evaluate(async () => {
      await window.I18N.ready;
      return { lang: window.I18N.lang, langs: Object.keys(window.I18N.dict).sort() };
    });
    expect(loaded).toEqual({ lang: "de", langs: ["de", "en"] });
  });
});

test("index.html: every visible text and label is translatable (data-i18n*)", async ({ page, request }) => {
  const html = await (await request.get("/")).text();
  await gotoAppReady(page);
  const offenders = await page.evaluate((html) => {
    const doc = new DOMParser().parseFromString(html, "text/html");
    // Allowlist: brand, data filled in by app.js, text without words (numbers,
    // symbols, punctuation) and URLs.
    const ALLOWED_HOSTS = [".brand-text", "#speakerCount", "#scheduleTz"];
    const allowedText = (s) => !/[A-Za-zÄÖÜäöüß]{2,}/.test(s) || /^https?:\/\//.test(s);
    const out = [];
    const walker = doc.createTreeWalker(doc.body, NodeFilter.SHOW_TEXT);
    for (let n = walker.nextNode(); n; n = walker.nextNode()) {
      const text = n.nodeValue.trim();
      const host = n.parentElement;
      if (!text || host.closest("script, style, template")) continue;
      if (host.hasAttribute("data-i18n") || allowedText(text)) continue;
      if (ALLOWED_HOSTS.some((sel) => host.closest(sel))) continue;
      out.push(`<${host.tagName.toLowerCase()} id="${host.id}">: ${text}`);
    }
    const ATTRS = { "aria-label": "data-i18n-aria", title: "data-i18n-title", placeholder: "data-i18n-placeholder" };
    for (const [attr, marker] of Object.entries(ATTRS)) {
      for (const node of doc.body.querySelectorAll(`[${attr}]`)) {
        const value = node.getAttribute(attr);
        if (node.hasAttribute(marker) || allowedText(value)) continue;
        out.push(`<${node.tagName.toLowerCase()} id="${node.id}"> ${attr}="${value}" without ${marker}`);
      }
    }
    return out;
  }, html);
  expect(offenders).toEqual([]);
});
