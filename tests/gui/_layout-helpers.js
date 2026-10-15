// @ts-check
const { expect } = require("@playwright/test");

const TOKEN = process.env.SRA_TEST_TOKEN || "integration-test-token";
const EXPECTED_SPEAKERS = 15;

/**
 * Prueft, ob zwei Rechtecke sich ueberlappen (inkl. Beruehrung).
 * @param {{ top: number, right: number, bottom: number, left: number }} a
 * @param {{ top: number, right: number, bottom: number, left: number }} b
 */
function rectsOverlap(a, b) {
  return !(a.right <= b.left || a.left >= b.right || a.bottom <= b.top || a.top >= b.bottom);
}

const GUI_SPEAKER_NAMES = [
  "Wohnzimmer", "Kueche", "Bad", "Schlafzimmer", "Buero", "Terrasse", "Garage",
  "Keller", "Gaeste", "Flur", "Kinderzimmer", "Esszimmer", "Ankleide", "Wintergarten", "Dachboden",
];

const GUI_SCENES = [
  {
    id: "test-layout",
    name: "Layout-Test",
    source: { type: "stream", uri: "https://example.com/stream.mp3" },
    members: [{ speaker: "Wohnzimmer", volume: 20 }],
  },
  ...[
    ["morgenradio", "Morgenradio", ["Wohnzimmer", "Kueche", "Bad"]],
    ["nachrichten", "Nachrichten Küche", ["Kueche"]],
    ["party", "Party überall", ["Wohnzimmer", "Kueche", "Bad", "Terrasse", "Buero"]],
    ["ruhe", "Ruhe Schlafzimmer", ["Schlafzimmer"]],
    ["wohnzimmer-jazz", "Wohnzimmer Jazz mit einem sehr langen Szenennamen", ["Wohnzimmer"]],
  ].map(([id, name, members]) => ({
    id,
    name,
    source: { type: "stream", uri: `https://example.com/${id}.mp3` },
    members: members.map((speaker) => ({ speaker, volume: 15 })),
  })),
];

const GUI_SCHEDULES = [
  {
    id: "test-layout-0610",
    scene: "test-layout",
    scene_name: "Layout-Test",
    days: ["mon", "tue", "wed", "thu", "fri"],
    start: "06:10",
    stop: "08:30",
    enabled: true,
    active: false,
    next_start: "2026-10-15T06:10:00+02:00",
    next_stop: "2026-10-15T08:30:00+02:00",
  },
  {
    id: "test-layout-wochenende-mit-sehr-langem-namen",
    scene: "test-layout",
    scene_name: "Layout-Test mit einem ziemlich langen Szenennamen",
    days: ["sat", "sun"],
    start: "09:00",
    enabled: false,
    active: false,
    last_run: { at: "2026-10-11T09:00:00+02:00", action: "start", ok: false, error: "Coordinator-Lautsprecher nicht erreichbar" },
  },
];

/**
 * Szenen-Zeilen: Groessen, Abstaende, sichtbare Anzahl ohne Scrollen.
 * @param {import('@playwright/test').Page} page
 */
async function readSceneRowsLayout(page) {
  return page.evaluate(() => {
    const rows = [...document.querySelectorAll("#grid .scene-row")];
    if (rows.length === 0) return { error: "keine Szenen-Zeilen" };
    const tabbar = document.querySelector(".tabbar")?.getBoundingClientRect();
    const np = document.querySelector("#nowPlaying");
    const npRect = np && !np.hidden ? np.getBoundingClientRect() : null;
    // Untere Grenze: Tab-Bar
    let limit = window.innerHeight;
    if (tabbar && tabbar.top > window.innerHeight / 2) limit = Math.min(limit, tabbar.top);
    if (npRect) limit = Math.min(limit, npRect.top);
    const r0 = rows[0].getBoundingClientRect();
    const play = rows[0].querySelector(".scene-play")?.getBoundingClientRect();
    const main = rows[0].querySelector(".scene-main")?.getBoundingClientRect();
    const more = rows[0].querySelector(".scene-more")?.getBoundingClientRect();
    if (!play || !main || !more) return { error: "Zeilen-Elemente fehlen" };
    return {
      count: rows.length,
      fullyVisible: rows.filter((r) => {
        const b = r.getBoundingClientRect();
        return b.top >= 0 && b.bottom <= limit + 0.5;
      }).length,
      rowHeight: r0.height,
      play: { w: play.width, h: play.height },
      playRound: getComputedStyle(rows[0].querySelector(".scene-play")).borderRadius,
      gapPlayMain: main.left - play.right,
      gapMainMore: more.left - main.right,
      more: { w: more.width, h: more.height },
      mainHeight: main.height,
    };
  });
}

/** @returns {object[]} */
function guiSpeakersPayload() {
  return GUI_SPEAKER_NAMES.map((name, i) => ({
    name,
    ip: `192.168.1.${10 + i}`,
    online: true,
  }));
}

/**
 * Stub fuer alle GUI-API-Aufrufe — entlastet Mock-Agent und Rate-Limit.
 * @param {import('@playwright/test').Page} page
 */
async function stubGuiApiRoutes(page) {
  const speakers = guiSpeakersPayload();
  await page.route(/\/api\/v1\//, async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    const json = (body) =>
      route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });

    if (path === "/api/v1/health") {
      return json({ status: "ok", speakers_online: speakers.length, speakers });
    }
    if (path === "/api/v1/scenes") {
      return json({ scenes: GUI_SCENES });
    }
    if (path === "/api/v1/speakers") {
      return json({ speakers, discovery_error: "" });
    }
    if (path === "/api/v1/schedules") {
      return json({ timezone: "Europe/Vienna", now: "2026-10-14T09:00:00+02:00", schedules: GUI_SCHEDULES });
    }
    if (path === "/api/v1/scenes/status") {
      return json({ scenes: [] });
    }
    if (path === "/api/v1/groups") {
      return json({ groups: [] });
    }
    if (path === "/api/v1/speakers/now-playing") {
      return json({ speakers: [] });
    }
    if (path === "/api/v1/speakers/stream-sources") {
      return json({ speakers: [{ name: "Wohnzimmer", speaker: "Wohnzimmer" }] });
    }
    if (path.startsWith("/api/v1/speakers/") && path.endsWith("/now-playing")) {
      return json({ playing: false });
    }
    return route.continue();
  });
}

/**
 * @param {import('@playwright/test').Page} page
 */
async function seedToken(page) {
  await stubGuiApiRoutes(page);
  await page.addInitScript((token) => {
    localStorage.setItem("sra_token", token);
  }, TOKEN);
}

/**
 * @param {import('@playwright/test').Page} page
 */
async function gotoAppReady(page) {
  await seedToken(page);
  await page.goto("/");
  await expect(page.locator("#addBtn")).toBeVisible({ timeout: 15_000 });
  await expect(page.locator("#grid .scene-row").first()).toBeVisible();
}

/**
 * Tab ueber die Tab-Bar wechseln (Hash-Routing).
 * @param {import('@playwright/test').Page} page
 * @param {"szenen"|"zeitplaene"|"boxen"} name
 */
async function gotoTab(page, name) {
  await page.locator(`.tabbar a[data-tab="${name}"]`).click();
  await expect(page).toHaveURL(new RegExp(`#${name}$`));
  await expect(page.locator(`[data-view="${name}"]`)).toBeVisible();
}

/**
 * @param {import('@playwright/test').Page} page
 * @param {number} [count]
 */
async function waitForSpeakerRows(page, count = EXPECTED_SPEAKERS) {
  await page.waitForFunction(
    (expected) => document.querySelectorAll("#sceneMembers .member-row").length === expected,
    count,
    { timeout: 15_000 },
  );
  await expect(page.locator("#sceneMembers .member-row")).toHaveCount(count);
}

/**
 * @param {import('@playwright/test').Page} page
 */
async function openSceneDialogWithSpeakers(page) {
  await gotoAppReady(page);
  await page.getByRole("button", { name: "Neue Szene" }).click();
  await expect(page.locator("#sceneDialog")).toBeVisible();
  await waitForSpeakerRows(page);
}

/**
 * Bearbeiten-Dialog fuer eine Stub-Szene oeffnen.
 * @param {import('@playwright/test').Page} page
 * @param {string} [sceneName]
 */
async function openSceneEditDialog(page, sceneName = "Layout-Test") {
  await gotoAppReady(page);
  await page.getByRole("button", { name: `Aktionen: ${sceneName}` }).click();
  await expect(page.locator("#sceneMenuSheet")).toBeVisible();
  await page.locator("#sceneMenuEdit").click();
  await expect(page.locator("#sceneDialog")).toBeVisible();
  await expect(page.locator("#sceneDialogTitle")).toHaveText("Szene bearbeiten");
  await waitForSpeakerRows(page);
}

/**
 * Wartet, bis laufende CSS-Animationen (z. B. Sheet-Einblendung) fertig sind,
 * damit Geometrie-Messungen nicht mitten in einer Transformation landen.
 * @param {import('@playwright/test').Page} page
 */
async function settleAnimations(page) {
  await page.evaluate(() =>
    Promise.all(
      document.getAnimations()
        .filter((a) => a.effect && a.effect.getComputedTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => null)),
    ),
  );
}

/**
 * Editor-Sheet: Geometrie, Segment-Control, verschachtelte Scrollbereiche.
 * @param {import('@playwright/test').Page} page
 */
async function readSceneEditorLayout(page) {
  await settleAnimations(page);
  return page.evaluate(() => {
    const dlg = document.querySelector("#sceneDialog");
    const body = dlg?.querySelector(".sheet-body");
    const save = document.querySelector("#sceneSaveBtn");
    const segs = [...document.querySelectorAll("#sceneDialog .seg .seg-opt")];
    if (!dlg || !body || !save || segs.length !== 3) return { error: "Editor-Elemente fehlen" };
    const d = dlg.getBoundingClientRect();
    const sv = save.getBoundingClientRect();
    // Scroll-in-Scroll: ausser dem Sheet-Body darf kein Element im Editor scrollen
    const nested = [...dlg.querySelectorAll("*")].filter((el) => {
      if (el === body) return false;
      const cs = getComputedStyle(el);
      return /(auto|scroll)/.test(cs.overflowY) && el.scrollHeight > el.clientHeight + 1;
    }).map((el) => el.id || el.className);
    return {
      dialog: { left: d.left, top: d.top, width: d.width, height: d.height },
      vw: document.documentElement.clientWidth,
      vh: window.innerHeight,
      saveInViewport: sv.top >= 0 && sv.bottom <= window.innerHeight + 1,
      saveRect: { top: Math.round(sv.top), bottom: Math.round(sv.bottom) },
      scroll: { dialog: dlg.scrollTop, body: body.scrollTop, doc: document.scrollingElement.scrollTop },
      saveHeight: sv.height,
      segHeights: segs.map((x) => x.getBoundingClientRect().height),
      nested,
      slugVisible: !!document.querySelector("#sceneId")?.checkVisibility(),
    };
  });
}

/** Toleranz fuer vertikale Zentrierung in der Kopfzeile (px). */
const HEADER_ALIGN_TOL = 6;

/**
 * Theme-Toggle und Status vertikal in der Bar ausrichten.
 * @param {import('@playwright/test').Page} page
 */
async function readHeaderLayout(page) {
  return page.evaluate((tol) => {
    const bar = document.querySelector("header.bar");
    const status = document.querySelector("#status");
    const themeToggle = document.querySelector("#addBtn");
    if (!bar || !status || !themeToggle) {
      return { error: "Header-Elemente fehlen" };
    }

    const barR = bar.getBoundingClientRect();
    const statusR = status.getBoundingClientRect();
    const themeR = themeToggle.getBoundingClientRect();
    const barCy = barR.top + barR.height / 2;
    const statusCy = statusR.top + statusR.height / 2;
    const themeCy = themeR.top + themeR.height / 2;

    return {
      statusThemeDelta: Math.abs(statusCy - themeCy),
      statusBarDelta: Math.abs(statusCy - barCy),
      themeBarDelta: Math.abs(themeCy - barCy),
      statusThemeAligned: Math.abs(statusCy - themeCy) <= tol,
    };
  }, HEADER_ALIGN_TOL);
}

/**
 * Horizontaler Overflow auf Viewport-Ebene (main, document, Szenen-Karten).
 * @param {import('@playwright/test').Page} page
 */
async function readHorizontalOverflow(page) {
  return page.evaluate(() => {
    const vw = document.documentElement.clientWidth;
    const tol = 1;
    const docOverflow = document.documentElement.scrollWidth > vw + tol;
    const main = document.querySelector("main.views");
    const mainOverflow = main ? main.scrollWidth > main.clientWidth + tol : false;

    /** @param {Element} el */
    function elOverflows(el) {
      const r = el.getBoundingClientRect();
      return r.left < -tol || r.right > vw + tol;
    }

    const scenes = [...document.querySelectorAll("#grid .scene-main")];
    const wraps = [...document.querySelectorAll("#grid .scene-row")];

    return {
      vw,
      docOverflow,
      mainOverflow,
      sceneOverflow: scenes.some(elOverflows),
      wrapOverflow: wraps.some(elOverflows),
      sceneCount: wraps.length,
      scrollWidth: document.documentElement.scrollWidth,
    };
  });
}

/** Design-Tokens aus docs/styleguide.md (px). */
const ICON_SIZES = { theme: 20, bar: 22, tab: 22, scene: 22 };

/**
 * Icon-Groessen und Szenen-Fuss aus Layout-Metriken.
 * @param {import('@playwright/test').Page} page
 */
async function readGuiMetrics(page) {
  return page.evaluate(() => {
    /** @param {Element | null | undefined} el */
    function iconSize(el) {
      if (!el) return null;
      const r = el.getBoundingClientRect();
      return { w: r.width, h: r.height };
    }

    const theme = document.querySelector("#themeIcon .theme-icon");
    const addBtn = document.querySelector("#addBtn");
    const tab = document.querySelector(".tabbar .tab-icon");
    const sceneIcon = document.querySelector("#grid .scene-action-icon");

    return {
      themeIcon: iconSize(theme),
      barIcon: iconSize(addBtn?.querySelector(".bar-icon")),
      tabIcon: iconSize(tab),
      sceneActionIcon: iconSize(sceneIcon),
    };
  });
}

/**
 * Zeitplan-Liste: Zeilen im Viewport, Aktionen ueberdecken Text nicht.
 * @param {import('@playwright/test').Page} page
 */
async function readScheduleListLayout(page) {
  return page.evaluate(() => {
    const vw = document.documentElement.clientWidth;
    const rows = [...document.querySelectorAll("#scheduleList .schedule-row")];
    if (rows.length === 0) return { error: "keine Zeitplan-Zeilen" };
    /** @param {DOMRect} a @param {DOMRect} b */
    const overlap = (a, b) => !(a.right <= b.left || a.left >= b.right || a.bottom <= b.top || a.top >= b.bottom);
    let overflow = false;
    let toggleOverlap = false;
    let minSwitchHit = Infinity;
    let minGap = Infinity;
    let minRowHeight = Infinity;
    for (const row of rows) {
      const r = row.getBoundingClientRect();
      minRowHeight = Math.min(minRowHeight, r.height);
      if (r.left < -1 || r.right > vw + 1) overflow = true;
      const main = row.querySelector(".schedule-main")?.getBoundingClientRect();
      const hit = row.querySelector(".switch-hit")?.getBoundingClientRect();
      if (!main || !hit) return { error: "Zeilen-Elemente fehlen" };
      if (overlap(main, hit)) toggleOverlap = true;
      minSwitchHit = Math.min(minSwitchHit, hit.width, hit.height);
      minGap = Math.min(minGap, hit.left - main.right);
    }
    return {
      rowCount: rows.length,
      overflow,
      toggleOverlap,
      minSwitchHit,
      minGap,
      minRowHeight,
      docOverflow: document.documentElement.scrollWidth > vw + 1,
    };
  });
}

/**
 * Zeitplan-Dialog: Chips in einer Zeile, Felder ohne Ueberlappung.
 * @param {import('@playwright/test').Page} page
 */
async function readScheduleDialogLayout(page) {
  await settleAnimations(page);
  return page.evaluate(() => {
    const dialog = document.querySelector("#scheduleDialog .sheet-body");
    const chips = [...document.querySelectorAll("#scheduleDays .day-chip span")];
    const start = document.querySelector("#scheduleStart");
    const stop = document.querySelector("#scheduleStop");
    const save = document.querySelector("#scheduleSaveBtn");
    if (!dialog || chips.length !== 7 || !start || !stop || !save) {
      return { error: "Dialog-Elemente fehlen" };
    }
    const d = dialog.getBoundingClientRect();
    const rects = chips.map((c) => c.getBoundingClientRect());
    const tops = rects.map((r) => Math.round(r.top));
    const s1 = start.getBoundingClientRect();
    const s2 = stop.getBoundingClientRect();
    const sv = save.getBoundingClientRect();
    /** @param {DOMRect} a @param {DOMRect} b */
    const overlap = (a, b) => !(a.right <= b.left || a.left >= b.right || a.bottom <= b.top || a.top >= b.bottom);
    let chipsOverlap = false;
    for (let i = 1; i < rects.length; i++) if (overlap(rects[i - 1], rects[i])) chipsOverlap = true;
    return {
      chipsOneRow: new Set(tops).size === 1,
      chipsInside: rects.every((r) => r.left >= d.left - 1 && r.right <= d.right + 1),
      chipsOverlap,
      minChipHeight: Math.min(...rects.map((r) => r.height)),
      minChipWidth: Math.min(...rects.map((r) => r.width)),
      chipRound: getComputedStyle(chips[0]).borderRadius,
      minPresetHeight: Math.min(
        ...[...document.querySelectorAll("#scheduleDialog .day-preset")].map((b) => b.getBoundingClientRect().height),
      ),
      timesOverlap: overlap(s1, s2),
      saveInViewport: sv.bottom <= window.innerHeight + 1 && sv.top >= 0,
      dialogInViewport: d.left >= -1 && d.right <= document.documentElement.clientWidth + 1,
    };
  });
}

/** Mindestgroesse fuer Touch-Ziele (Apple HIG 44 pt; Tokens liegen mit 48/56 px darueber). */
const MIN_TARGET = 44;

/**
 * Alle sichtbaren Bedienelemente pruefen: Groesse >= 44 x 44, keine
 * Ueberlappung, kein horizontaler Overflow. Ist ein modaler Dialog offen,
 * zaehlt nur dessen Inhalt (der Rest ist inert).
 * Checkbox/Radio zaehlen ueber ihr umschliessendes Label (eigentliche Trefferflaeche).
 * Ueberlappung nur innerhalb derselben Ebene (Kopfzeile, Inhalt, Mini-Player,
 * Tab-Bar) — Inhalt scrollt absichtlich unter den fixen Leisten durch.
 * @param {import('@playwright/test').Page} page
 */
async function readTouchTargets(page) {
  await settleAnimations(page);
  return page.evaluate((min) => {
    const openDialogs = [...document.querySelectorAll("dialog[open]")];
    const root = openDialogs.length ? openDialogs[openDialogs.length - 1] : document.body;
    const selector = [
      "button", "a[href]", "select", "summary", "textarea",
      'input:not([type="checkbox"]):not([type="radio"]):not([type="hidden"])',
      "label:has(> input[type=checkbox])", "label:has(> input[type=radio])",
    ].join(",");
    /** @param {Element} el */
    const visible = (el) =>
      el.checkVisibility({ opacityProperty: false, visibilityProperty: true }) &&
      !el.closest("[hidden], [inert]");
    /** @param {Element} el */
    const layer = (el) => {
      const fixed = el.closest("header.bar, #nowPlaying, .tabbar, dialog");
      return fixed ? (fixed.id || fixed.className || fixed.tagName) : "content";
    };
    /** @param {Element} el */
    const label = (el) =>
      (el.getAttribute("aria-label") || el.textContent || el.id || el.className || el.tagName)
        .replace(/\s+/g, " ").trim().slice(0, 40);
    /**
     * Sichtbare Flaeche: Rechteck auf den naechsten Scroll-Container zugeschnitten
     * (Sheet-Body scrollt unter Kopf/Footer durch).
     * @param {Element} el
     */
    const clipped = (el) => {
      const r = el.getBoundingClientRect();
      let box = { left: r.left, top: r.top, right: r.right, bottom: r.bottom };
      for (let p = el.parentElement; p && p !== document.body; p = p.parentElement) {
        if (!/(auto|scroll)/.test(getComputedStyle(p).overflowY)) continue;
        const c = p.getBoundingClientRect();
        box = {
          left: Math.max(box.left, c.left), top: Math.max(box.top, c.top),
          right: Math.min(box.right, c.right), bottom: Math.min(box.bottom, c.bottom),
        };
      }
      return box;
    };
    const targets = [...root.querySelectorAll(selector)].filter(visible).map((el) => {
      const r = el.getBoundingClientRect();
      return { el, r, vis: clipped(el), layer: layer(el), name: label(el) };
    }).filter((t) => t.r.width > 0 && t.r.height > 0);

    const small = targets
      .filter((t) => t.r.width < min - 0.5 || t.r.height < min - 0.5)
      .map((t) => `${t.name} (${Math.round(t.r.width)}x${Math.round(t.r.height)})`);

    const overlaps = [];
    for (let i = 0; i < targets.length; i++) {
      for (let j = i + 1; j < targets.length; j++) {
        const a = targets[i];
        const b = targets[j];
        if (a.layer !== b.layer) continue;
        if (a.el.contains(b.el) || b.el.contains(a.el)) continue;
        const ix = Math.min(a.vis.right, b.vis.right) - Math.max(a.vis.left, b.vis.left);
        const iy = Math.min(a.vis.bottom, b.vis.bottom) - Math.max(a.vis.top, b.vis.top);
        if (ix > 0.5 && iy > 0.5) overlaps.push(`${a.name} <> ${b.name}`);
      }
    }
    const vw = document.documentElement.clientWidth;
    const outside = targets
      .filter((t) => t.r.left < -1 || t.r.right > vw + 1)
      .map((t) => t.name);
    return {
      count: targets.length,
      small,
      overlaps,
      outside,
      docOverflow: document.documentElement.scrollWidth > vw + 1,
    };
  }, MIN_TARGET);
}

module.exports = {
  readTouchTargets,
  MIN_TARGET,
  TOKEN,
  GUI_SCHEDULES,
  readScheduleListLayout,
  readScheduleDialogLayout,
  EXPECTED_SPEAKERS,
  ICON_SIZES,
  HEADER_ALIGN_TOL,
  rectsOverlap,
  gotoAppReady,
  gotoTab,
  readSceneRowsLayout,
  GUI_SCENES,
  openSceneDialogWithSpeakers,
  openSceneEditDialog,
  readSceneEditorLayout,
  settleAnimations,
  readHeaderLayout,
  readHorizontalOverflow,
  readGuiMetrics,
};
