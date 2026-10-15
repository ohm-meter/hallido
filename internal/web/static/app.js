"use strict";

// Hallido -- web app (mobile first). Texts: i18n.js (t/tn).
// Drei Tabs per Hash-Routing (#szenen, #zeitplaene, #boxen); Boxen-Tab buendelt
// Discovery, Now-Playing je Box und Einstellungen.

const TOKEN_KEY = "sra_token";
const THEME_KEY = "sonos-theme";
// Product name in the page title (brand, not translated).
const APP_TITLE = "Hallido — Powered by siraris";
const STATUS_POLL_MS = 8000;
const DISCOVER_POLL_MS = 8000;
const POLL_STAGGER_MS = 200;
const STREAM_SOURCES_TTL_MS = 30000;
const SPEAKERS_LIST_TTL_MS = 30000;
const RADIO_SEARCH_DEBOUNCE_MS = 300;
const DEFAULT_MEMBER_VOLUME = 5;
const SCHEDULES_POLL_MS = 60000;

const el = {
  grid: document.getElementById("grid"),
  hint: document.getElementById("hint"),
  statusDot: document.getElementById("statusDot"),
  statusText: document.getElementById("statusText"),
  meta: document.getElementById("meta"),
  tokenBtn: document.getElementById("tokenBtn"),
  dialog: document.getElementById("tokenDialog"),
  tokenInput: document.getElementById("tokenInput"),
  tokenForm: document.getElementById("tokenForm"),
  speakersPanel: document.getElementById("speakersPanel"),
  speakerCount: document.getElementById("speakerCount"),
  speakerList: document.getElementById("speakerList"),
  groupList: document.getElementById("groupList"),
  singlesLabel: document.getElementById("singlesLabel"),
  refreshSpeakersBtn: document.getElementById("refreshSpeakersBtn"),
  discoverPanel: document.getElementById("discoverPanel"),
  discoverTitle: document.getElementById("discoverTitle"),
  discoverMeta: document.getElementById("discoverMeta"),
  discoverClose: document.getElementById("discoverClose"),
  discoverUngroupBtn: document.getElementById("discoverUngroupBtn"),
  ungroupDialog: document.getElementById("ungroupDialog"),
  ungroupForm: document.getElementById("ungroupForm"),
  ungroupTitle: document.getElementById("ungroupTitle"),
  ungroupMessage: document.getElementById("ungroupMessage"),
  ungroupConfirmBtn: document.getElementById("ungroupConfirmBtn"),
  addBtn: document.getElementById("addBtn"),
  viewTitle: document.getElementById("viewTitle"),
  themeLabel: document.getElementById("themeLabel"),
  saveSpeakersConfigBtn: document.getElementById("saveSpeakersConfigBtn"),
  sceneDialog: document.getElementById("sceneDialog"),
  sceneDialogTitle: document.getElementById("sceneDialogTitle"),
  sceneForm: document.getElementById("sceneForm"),
  sceneName: document.getElementById("sceneName"),
  sceneId: document.getElementById("sceneId"),
  sceneMembers: document.getElementById("sceneMembers"),
  sourceBoxSelect: document.getElementById("sourceBoxSelect"),
  sourceBoxPreview: document.getElementById("sourceBoxPreview"),
  sourceUrl: document.getElementById("sourceUrl"),
  radioSearchInput: document.getElementById("radioSearchInput"),
  radioSearchBtn: document.getElementById("radioSearchBtn"),
  radioSearchResults: document.getElementById("radioSearchResults"),
  radioSearchHint: document.getElementById("radioSearchHint"),
  radioPicked: document.getElementById("radioPicked"),
  sourceFavoriteBtn: document.getElementById("sourceFavoriteBtn"),
  sourceFavoriteSpeakerSelect: document.getElementById("sourceFavoriteSpeakerSelect"),
  sourceFavoriteSelect: document.getElementById("sourceFavoriteSelect"),
  sourceFavoritePreview: document.getElementById("sourceFavoritePreview"),
  sourceUrlHint: document.getElementById("sourceUrlHint"),
  sceneSaveHint: document.getElementById("sceneSaveHint"),
  sceneCancelBtn: document.getElementById("sceneCancelBtn"),
  sceneDeleteDialog: document.getElementById("sceneDeleteDialog"),
  sceneDeleteBtn: document.getElementById("sceneDeleteBtn"),
  sceneMenuSheet: document.getElementById("sceneMenuSheet"),
  nowPlaying: document.getElementById("nowPlaying"),
  nowPlayingMain: document.getElementById("nowPlayingMain"),
  nowPlayingName: document.getElementById("nowPlayingName"),
  nowPlayingSub: document.getElementById("nowPlayingSub"),
  nowPlayingStop: document.getElementById("nowPlayingStop"),
  nowPlayingPause: document.getElementById("nowPlayingPause"),
  sceneMenuPause: document.getElementById("sceneMenuPause"),
  sceneMenuTitle: document.getElementById("sceneMenuTitle"),
  sceneMenuEdit: document.getElementById("sceneMenuEdit"),
  sceneMenuVolumes: document.getElementById("sceneMenuVolumes"),
  sceneMenuHint: document.getElementById("sceneMenuHint"),
  sceneDeleteForm: document.getElementById("sceneDeleteForm"),
  sceneDeleteMessage: document.getElementById("sceneDeleteMessage"),
  themeToggle: document.getElementById("themeToggle"),
  schedulesSection: document.getElementById("schedulesSection"),
  scheduleList: document.getElementById("scheduleList"),
  scheduleHint: document.getElementById("scheduleHint"),
  scheduleDialog: document.getElementById("scheduleDialog"),
  scheduleDialogTitle: document.getElementById("scheduleDialogTitle"),
  scheduleForm: document.getElementById("scheduleForm"),
  scheduleScene: document.getElementById("scheduleScene"),
  scheduleTz: document.getElementById("scheduleTz"),
  scheduleStart: document.getElementById("scheduleStart"),
  scheduleStop: document.getElementById("scheduleStop"),
  scheduleEnabled: document.getElementById("scheduleEnabled"),
  scheduleSaveHint: document.getElementById("scheduleSaveHint"),
  scheduleCancelBtn: document.getElementById("scheduleCancelBtn"),
  scheduleDeleteDialog: document.getElementById("scheduleDeleteDialog"),
  scheduleDeleteBtn: document.getElementById("scheduleDeleteBtn"),
  scheduleSummary: document.getElementById("scheduleSummary"),
  scheduleDeleteForm: document.getElementById("scheduleDeleteForm"),
  scheduleDeleteMessage: document.getElementById("scheduleDeleteMessage"),
};

function getPreferredTheme() {
  const stored = localStorage.getItem(THEME_KEY);
  if (stored === "light" || stored === "dark") return stored;
  if (window.matchMedia && window.matchMedia("(prefers-color-scheme: light)").matches) {
    return "light";
  }
  return "dark";
}

/** Icon fuer Theme-Umschalter: Sonne = zu Hell wechseln, Mond = zu Dunkel wechseln. */
function themeIconName(theme) {
  return theme === "light" ? "moon" : "sun";
}

function updateThemeIcon(theme) {
  const slot = document.getElementById("themeIcon");
  if (!slot) return;
  slot.replaceChildren(createLucideIcon(themeIconName(theme), "lucide-icon theme-icon"));
}

function applyTheme(theme) {
  const root = document.documentElement;
  root.setAttribute("data-theme", theme === "light" ? "light" : "dark");
  updateThemeIcon(theme);
  if (el.themeToggle) {
    const isLight = theme === "light";
    el.themeToggle.setAttribute("aria-label", t(isLight ? "theme.toDark" : "theme.toLight"));
    el.themeToggle.setAttribute("aria-pressed", isLight ? "true" : "false");
  }
  if (el.themeLabel) el.themeLabel.textContent = t(theme === "light" ? "theme.light" : "theme.dark");
}

function toggleTheme() {
  const isLight = document.documentElement.getAttribute("data-theme") === "light";
  const next = isLight ? "dark" : "light";
  localStorage.setItem(THEME_KEY, next);
  applyTheme(next);
}

function initTheme() {
  applyTheme(getPreferredTheme());
}

/**
 * Label map whose values are translated on access (keys -> i18n keys).
 * The dictionaries load asynchronously (I18N.ready), so module-level label
 * tables must not call t() at load time.
 */
function translated(keys) {
  const out = {};
  for (const [name, key] of Object.entries(keys)) {
    Object.defineProperty(out, name, { get: () => t(key), enumerable: true });
  }
  return out;
}

// ---- Navigation: drei Tabs per Hash-Routing (Zurueck-Taste, Lesezeichen) ----

// Hash routes stay German (bookmarks, installed apps); labels come from i18n.js.
const TABS = translated({ szenen: "tab.scenes", zeitplaene: "tab.schedules", boxen: "tab.speakers" });
const ADD_LABELS = translated({ szenen: "scene.new", zeitplaene: "schedule.new" });
let currentTab = "szenen";
let sceneBuilderEnabled = false;
let schedulesEnabled = false;

function tabFromHash() {
  const name = (location.hash || "").slice(1);
  return Object.prototype.hasOwnProperty.call(TABS, name) ? name : "szenen";
}

/** Plus-Knopf in der Kopfzeile: kontextabhaengig (Szene / Zeitplan), im Boxen-Tab aus. */
function updateAddButton() {
  if (!el.addBtn) return;
  const label = ADD_LABELS[currentTab];
  el.addBtn.hidden = !label || !sceneBuilderEnabled;
  if (label) el.addBtn.setAttribute("aria-label", label);
}

function showTab(name) {
  const changed = name !== currentTab;
  currentTab = name;
  for (const view of document.querySelectorAll("[data-view]")) {
    view.hidden = view.dataset.view !== name;
  }
  for (const tab of document.querySelectorAll("[data-tab]")) {
    if (tab.dataset.tab === name) tab.setAttribute("aria-current", "page");
    else tab.removeAttribute("aria-current");
  }
  if (el.viewTitle) el.viewTitle.textContent = TABS[name];
  document.title = TABS[name] + " · " + APP_TITLE;
  updateAddButton();
  if (changed) window.scrollTo(0, 0);
  if (changed && name === "boxen" && sceneBuilderEnabled) loadGroups();
}

function initRouting() {
  if (!Object.prototype.hasOwnProperty.call(TABS, (location.hash || "").slice(1))) {
    history.replaceState(null, "", "#szenen");
  }
  currentTab = "";
  showTab(tabFromHash());
  window.addEventListener("hashchange", () => showTab(tabFromHash()));
}

let pendingSceneDelete = null;

let discoveredSpeakers = [];
let sceneIdManual = false;
let sceneEditId = null;
// Quelle einer bearbeiteten Favoriten-Szene (TuneIn): bleibt erhalten, solange
// keine neue URL eingegeben wird.
let sceneEditFavoriteSource = null;
let pickedStreamUri = "";
let pickedRadio = null; // { name, url } aus der Radio-Suche

let speakersLoading = false;
let statusPollTimer = null;
let discoverPollTimer = null;
let discoverSpeaker = null;

let discoveredNames = new Set();
let loadedScenes = [];
const sceneCards = new Map();
const sceneById = new Map();
const speakerCards = new Map();
const nowPlayingCache = new Map();

let streamSourcesCache = [];
let streamSourcesLoadedAt = 0;
let favoritesCache = [];
let favoritesLoadedAt = 0;
let radioSearchTimer = 0;
let radioSearchAbort = null;
let pickedFavoriteId = "";
let favoriteProxySpeaker = "";
let favoritesUsedSpeaker = "";
const FAVORITES_TTL_MS = 30000;
let speakersListFingerprint = "";
let statusPollInFlight = false;
let speakersPollTimer = null;

function getToken() {
  return (localStorage.getItem(TOKEN_KEY) || "").trim();
}

function tokenHeaderSafe(s) {
  if (!s) return false;
  for (let i = 0; i < s.length; i++) {
    if (s.charCodeAt(i) > 0xff) return false;
  }
  return true;
}

function setHint(text, isError) {
  el.hint.textContent = text || "";
  el.hint.classList.toggle("error", !!isError);
}

async function api(path, opts = {}) {
  const token = getToken();
  if (!tokenHeaderSafe(token)) {
    localStorage.removeItem(TOKEN_KEY);
    setHint(t("token.invalidChars"), true);
    openTokenDialog();
    return { ok: false, status: 0, body: null };
  }
  const headers = new Headers(opts.headers || {});
  headers.set("Authorization", "Bearer " + token);
  const res = await fetch(path, Object.assign({}, opts, { headers }));
  let body = null;
  try { body = await res.json(); } catch (_) { /* leerer Body ok */ }
  return { ok: res.ok, status: res.status, body };
}

function encodeSpeakerPath(name) {
  return encodeURIComponent(name);
}

async function refreshStatus() {
  if (document.hidden || Date.now() < pollBackoffUntil) return;
  try {
    const res = await fetch("/api/v1/health");
    const data = await res.json();
    const online = data.speakers_online || 0;
    el.statusDot.className = "dot " + (online > 0 ? "on" : "off");
    // Short form for the 48 px header; long form as tooltip.
    el.statusText.textContent = tn("status.speakers", online);
    el.statusDot.parentElement.title = tn("status.speakersOnline", online);
    if (data.discovery_error && !el.hint.classList.contains("error")) {
      setHint(t("status.discoveryHint", { msg: data.discovery_error }), false);
    }
  } catch (_) {
    el.statusDot.className = "dot off";
    el.statusText.textContent = t("status.offline");
    el.statusDot.parentElement.title = t("status.unreachable");
  }
}

function setSpeakersPanelVisible(visible) {
  el.speakersPanel.hidden = !visible;
}

function updateSpeakerSelection() {
  for (const [name, card] of speakerCards) {
    card.classList.toggle("selected", name === discoverSpeaker);
    card.setAttribute("aria-pressed", name === discoverSpeaker ? "true" : "false");
  }
}

function renderDiscoverMeta(data) {
  el.discoverMeta.innerHTML = "";
  if (!data) return;

  const rows = [
    [t("np.status"), PLAYBACK_LABELS[data.state] || data.state || "—"],
    [t("np.title"), data.title || "—"],
    [t("np.artist"), data.artist || "—"],
    [t("np.album"), data.album || "—"],
    [t("np.uri"), data.uri || "—"],
  ];
  if (data.raw) {
    rows.push([t("np.raw"), data.raw]);
  }

  for (const [label, value] of rows) {
    const dt = document.createElement("dt");
    dt.textContent = label;
    const dd = document.createElement("dd");
    dd.textContent = value;
    el.discoverMeta.append(dt, dd);
  }
}

async function loadNowPlaying(name) {
  const path = "/api/v1/speakers/" + encodeSpeakerPath(name) + "/now-playing";
  const { ok, status, body } = await api(path);
  if (status === 401) {
    setHint(t("token.invalidReenter"), true);
    openTokenDialog();
    return null;
  }
  if (!ok) {
    const msg = (body && body.message) || t("np.loadFailed");
    renderDiscoverMeta(null);
    el.discoverMeta.innerHTML = "";
    const err = document.createElement("p");
    err.className = "discover-error";
    err.textContent = msg;
    el.discoverMeta.appendChild(err);
    return null;
  }
  renderDiscoverMeta(body);
  return body;
}

function startDiscoverPolling() {
  stopDiscoverPolling();
  if (!discoverSpeaker) return;
  discoverPollTimer = setInterval(() => {
    if (document.hidden || !discoverSpeaker) return;
    loadNowPlaying(discoverSpeaker);
  }, DISCOVER_POLL_MS);
}

function stopDiscoverPolling() {
  if (discoverPollTimer) {
    clearInterval(discoverPollTimer);
    discoverPollTimer = null;
  }
}

function enterDiscoverMode(name) {
  discoverSpeaker = name;
  el.discoverPanel.hidden = false;
  el.discoverTitle.textContent = t("np.titleFor", { name });
  updateSpeakerSelection();
  updateDiscoverUngroup();
  loadNowPlaying(name);
  startDiscoverPolling();
}

function exitDiscoverMode() {
  discoverSpeaker = null;
  el.discoverPanel.hidden = true;
  el.discoverMeta.innerHTML = "";
  updateSpeakerSelection();
  updateDiscoverUngroup();
  stopDiscoverPolling();
}


function anyDialogOpen() {
  return (
    (el.dialog && el.dialog.open) ||
    (el.sceneDialog && el.sceneDialog.open) ||
    (el.sceneDeleteDialog && el.sceneDeleteDialog.open) ||
    (el.sceneMenuSheet && el.sceneMenuSheet.open) ||
    (el.scheduleDialog && el.scheduleDialog.open) ||
    (el.scheduleDeleteDialog && el.scheduleDeleteDialog.open) ||
    (el.ungroupDialog && el.ungroupDialog.open)
  );
}

/** ⋯-Knopf je Szene nur mit gueltigem Token (Bearbeiten/Lautstaerke). */
function setSceneMoreButtonsVisible(visible) {
  for (const row of sceneCards.values()) {
    const btn = row.querySelector(".scene-more");
    if (btn) btn.hidden = !visible;
  }
}

function isIntegratableNowPlaying(np) {
  if (!np) return false;
  if (np.integratable === true) return true;
  if (np.integratable === false) return false;
  return isIntegratableStreamURI(np.uri);
}

function speakersFingerprint(list) {
  return (list || [])
    .map((s) => `${s.name}\t${s.ip || ""}\t${s.reachable !== false}`)
    .join("\n");
}

function isIntegratableStreamURI(uri) {
  const u = (uri || "").trim();
  if (!u) return false;
  const lower = u.toLowerCase();
  if (lower.startsWith("spotify:") || lower.startsWith("x-sonos-spotify:")) return false;
  if (lower.startsWith("x-sonos-vli:")) return false;
  if (lower === "x-rincon") return false;
  if (lower.startsWith("x-rincon:") && !lower.startsWith("x-rincon-mp3radio:")) return false;
  if (lower.startsWith("http://") || lower.startsWith("https://")) return true;
  if (lower.startsWith("x-rincon-mp3radio://")) return true;
  if (lower.startsWith("aac://")) {
    const inner = lower.slice(6);
    if (inner.startsWith("http://") || inner.startsWith("https://")) return true;
    return inner.length > 0 && !inner.startsWith("x-rincon");
  }
  if (lower.startsWith("x-sonos-http://") || lower.startsWith("x-sonos-htastream://")) {
    return true;
  }
  return false;
}

function displayStreamURI(uri) {
  const u = (uri || "").trim();
  if (!u) return "";
  const lower = u.toLowerCase();
  if (lower.startsWith("http://") || lower.startsWith("https://")) return u;
  if (lower.startsWith("aac://")) {
    const inner = u.slice(6);
    if (/^https?:\/\//i.test(inner)) return inner;
  }
  if (lower.startsWith("x-rincon-mp3radio://")) {
    return "https://" + u.slice("x-rincon-mp3radio://".length);
  }
  return u;
}

function slugFromName(name) {
  const map = { ä: "ae", ö: "oe", ü: "ue", ß: "ss", Ä: "ae", Ö: "oe", Ü: "ue" };
  let s = (name || "").trim();
  for (const [k, v] of Object.entries(map)) s = s.split(k).join(v);
  return s
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .replace(/-+/g, "-")
    .slice(0, 64);
}

function setSceneBuilderVisible(visible) {
  sceneBuilderEnabled = visible;
  updateAddButton();
  if (el.saveSpeakersConfigBtn) el.saveSpeakersConfigBtn.hidden = !visible;
  setSceneMoreButtonsVisible(visible);
}

function renderSceneMemberFields(speakers, opts = {}) {
  if (!el.sceneMembers) return;
  el.sceneMembers.innerHTML = "";
  const { loading = false, selected = null } = opts;
  const selectedMap = selected || {};
  const list = speakers || [];
  if (loading && list.length === 0) {
    const p = document.createElement("p");
    p.className = "field-hint";
    p.textContent = t("speakers.loading");
    el.sceneMembers.appendChild(p);
    return;
  }
  if (list.length === 0) {
    const p = document.createElement("p");
    p.className = "field-hint";
    p.textContent = t("members.none");
    el.sceneMembers.appendChild(p);
    return;
  }
  for (const sp of list) {
    // 56-px-Zeile: Checkbox + Name; bei Auswahl darunter Slider mit −/+ (Spotify-Sheet)
    const row = document.createElement("div");
    row.className = "member-row";
    row.dataset.name = sp.name;
    const pick = document.createElement("label");
    pick.className = "member-pick";
    const cb = document.createElement("input");
    cb.type = "checkbox";
    cb.value = sp.name;
    cb.dataset.name = sp.name;
    const name = document.createElement("span");
    name.className = "member-name";
    name.textContent = sp.name;
    pick.append(cb, name);
    const start = selectedMap[sp.name] != null ? selectedMap[sp.name] : DEFAULT_MEMBER_VOLUME;
    const vol = createVolumeControl(sp.name, start);
    row.append(pick, vol);
    cb.checked = selectedMap[sp.name] != null;
    row.classList.toggle("is-selected", cb.checked);
    cb.addEventListener("change", () => row.classList.toggle("is-selected", cb.checked));
    el.sceneMembers.appendChild(row);
  }
}

/**
 * Lautstaerke-Regler: −/+ (44 px) und Slider (44 px Trefferhoehe) mit Wertanzeige.
 * onChange wird bei jeder Aenderung mit dem neuen Wert aufgerufen.
 */
function createVolumeControl(name, value, onChange) {
  const wrap = document.createElement("div");
  wrap.className = "member-vol";
  const range = document.createElement("input");
  range.type = "range";
  range.min = "0";
  range.max = "100";
  range.step = "1";
  range.value = String(Math.min(100, Math.max(0, Number(value) || 0)));
  range.dataset.for = name;
  range.setAttribute("aria-label", t("volume.label", { name }));
  const out = document.createElement("output");
  out.className = "vol-value";
  out.textContent = range.value;
  // Gefuellte Spur (CSS --fill) folgt dem Wert.
  const paint = () => range.style.setProperty("--fill", range.value + "%");
  const set = (v) => {
    const next = Math.min(100, Math.max(0, v));
    range.value = String(next);
    out.textContent = range.value;
    paint();
    if (onChange) onChange(next);
  };
  paint();
  const step = (delta, labelKey, icon) => {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "icon-btn vol-step";
    btn.setAttribute("aria-label", t(labelKey, { name }));
    btn.append(createLucideIcon(icon, "lucide-icon btn-icon"));
    btn.addEventListener("click", () => set(Number(range.value) + delta));
    return btn;
  };
  range.addEventListener("input", () => set(Number(range.value)));
  wrap.append(step(-1, "volume.quieter", "minus"), range, out, step(1, "volume.louder", "plus"));
  return wrap;
}

function fillSourceBoxSelect(sources, opts = {}) {
  if (!el.sourceBoxSelect) return;
  const { loading = false } = opts;
  const prev = el.sourceBoxSelect.value;
  el.sourceBoxSelect.innerHTML = "";
  if (loading) {
    const opt = document.createElement("option");
    opt.value = "";
    opt.textContent = t("source.loadingStreams");
    el.sourceBoxSelect.appendChild(opt);
    el.sourceBoxSelect.disabled = true;
    return;
  }
  const list = sources || [];
  if (list.length === 0) {
    const opt = document.createElement("option");
    opt.value = "";
    opt.textContent = t("source.noStreamBox");
    el.sourceBoxSelect.appendChild(opt);
    el.sourceBoxSelect.disabled = true;
    return;
  }
  for (const sp of list) {
    const name = sp.name || sp.speaker;
    if (!name) continue;
    const opt = document.createElement("option");
    opt.value = name;
    opt.textContent = name;
    el.sourceBoxSelect.appendChild(opt);
  }
  el.sourceBoxSelect.disabled = false;
  if (prev && [...el.sourceBoxSelect.options].some((o) => o.value === prev)) {
    el.sourceBoxSelect.value = prev;
  }
}

async function ensureStreamSources(opts = {}) {
  const { force = false } = opts;
  if (
    !force &&
    streamSourcesLoadedAt &&
    Date.now() - streamSourcesLoadedAt < STREAM_SOURCES_TTL_MS
  ) {
    fillSourceBoxSelect(streamSourcesCache);
    return streamSourcesCache;
  }
  if (!el.sceneDialog || !el.sceneDialog.open) return streamSourcesCache;
  fillSourceBoxSelect([], { loading: true });
  const { ok, status, body } = await api("/api/v1/speakers/stream-sources");
  if (status === 401) return [];
  if (!ok) {
    fillSourceBoxSelect([]);
    return [];
  }
  streamSourcesCache = (body && body.speakers) || [];
  streamSourcesLoadedAt = Date.now();
  for (const np of streamSourcesCache) {
    if (np.speaker) nowPlayingCache.set(np.speaker, np);
  }
  if (el.sceneDialog && el.sceneDialog.open) {
    fillSourceBoxSelect(streamSourcesCache);
  }
  return streamSourcesCache;
}

function updateSourceModeUI(opts = {}) {
  const { loadPreview = true } = opts;
  const mode = document.querySelector('input[name="sourceMode"]:checked');
  const activeMode = mode ? mode.value : "radio";
  const isBox = activeMode === "box";
  const isUrl = activeMode === "url";
  const isRadio = activeMode === "radio";
  const isFavorite = activeMode === "favorite";
  for (const panel of document.querySelectorAll("[data-source-panel]")) {
    panel.hidden = panel.dataset.sourcePanel !== activeMode;
  }
  if (el.sourceFavoriteBtn) el.sourceFavoriteBtn.setAttribute("aria-pressed", isFavorite ? "true" : "false");
  if (el.radioSearchInput) el.radioSearchInput.disabled = !isRadio;
  if (el.radioSearchBtn) el.radioSearchBtn.disabled = !isRadio;
  if (el.sourceBoxSelect) {
    el.sourceBoxSelect.disabled =
      !isBox || el.sourceBoxSelect.options.length === 0 || el.sourceBoxSelect.disabled;
  }
  if (el.sourceUrl) el.sourceUrl.disabled = !isUrl;
  if (el.sourceFavoriteSelect) {
    el.sourceFavoriteSelect.disabled = !isFavorite;
  }
  if (el.sourceFavoriteSpeakerSelect) {
    el.sourceFavoriteSpeakerSelect.disabled = !isFavorite;
    if (isFavorite) {
      fillSourceFavoriteSpeakerSelect(discoveredSpeakers, {
        loading: speakersLoading && discoveredSpeakers.length === 0,
      });
    }
  }
  if (isBox && el.sceneDialog && el.sceneDialog.open && !streamSourcesLoadedAt) {
    ensureStreamSources();
  }
  if (isFavorite && el.sceneDialog && el.sceneDialog.open && !favoritesLoadedAt) {
    ensureSonosFavorites();
  }
  if (loadPreview && isBox && el.sourceBoxSelect && el.sourceBoxSelect.value) {
    loadStreamFromBox(el.sourceBoxSelect.value);
  }
  if (loadPreview && isFavorite) {
    applyFavoritePreview();
  }
}

function applyStreamPreviewFromNP(body) {
  pickedStreamUri = "";
  if (!body || !body.uri) {
    if (el.sourceBoxPreview) {
      el.sourceBoxPreview.textContent = t("source.noStreamOnBox");
      el.sourceBoxPreview.classList.add("error");
    }
    return;
  }
  if (!isIntegratableNowPlaying(body)) {
    if (el.sourceBoxPreview) {
      el.sourceBoxPreview.textContent = t("source.notIntegratable");
      el.sourceBoxPreview.classList.add("error");
    }
    return;
  }
  pickedStreamUri = body.uri;
  if (el.sourceBoxPreview) {
    el.sourceBoxPreview.textContent = displayStreamURI(body.uri);
    el.sourceBoxPreview.classList.remove("error");
  }
}

async function loadStreamFromBox(name) {
  if (el.sourceBoxPreview) {
    el.sourceBoxPreview.textContent = t("common.loading");
    el.sourceBoxPreview.classList.remove("error");
  }
  const cached = nowPlayingCache.get(name);
  if (cached && cached.uri && isIntegratableNowPlaying(cached)) {
    applyStreamPreviewFromNP(cached);
    return;
  }
  const { ok, body } = await api(
    "/api/v1/speakers/" + encodeSpeakerPath(name) + "/now-playing"
  );
  if (!ok) {
    applyStreamPreviewFromNP(null);
    return;
  }
  if (body && body.speaker) nowPlayingCache.set(body.speaker, body);
  applyStreamPreviewFromNP(body);
}

function speakersForSceneEdit(sc) {
  const names = new Set(discoveredSpeakers.map((s) => s.name));
  const extra = (sc.members || [])
    .filter((m) => m.speaker && !names.has(m.speaker))
    .map((m) => ({ name: m.speaker, reachable: false }));
  return [...discoveredSpeakers, ...extra];
}

function memberSelectionFromScene(sc) {
  const selected = {};
  for (const m of sc.members || []) {
    if (m.speaker) selected[m.speaker] = m.volume;
  }
  return selected;
}

function setSceneIdFieldMode(editing) {
  if (!el.sceneId) return;
  el.sceneId.readOnly = editing;
  el.sceneId.classList.toggle("readonly", editing);
  if (editing) {
    el.sceneId.setAttribute("aria-readonly", "true");
  } else {
    el.sceneId.removeAttribute("aria-readonly");
  }
}

function openSceneDialog(opts = {}) {
  if (!el.sceneDialog) return;
  const { loadingSpeakers = false, edit = null } = opts;
  sceneEditId = edit ? edit.id : null;
  if (el.sceneDialogTitle) {
    el.sceneDialogTitle.textContent = t(edit ? "scene.edit" : "scene.new");
  }
  if (el.sceneDeleteBtn) el.sceneDeleteBtn.hidden = !edit;
  const speakerList = edit ? speakersForSceneEdit(edit) : discoveredSpeakers;
  const selectedMembers = edit ? memberSelectionFromScene(edit) : null;
  renderSceneMemberFields(speakerList, {
    loading: loadingSpeakers && speakerList.length === 0,
    selected: selectedMembers,
  });
  fillSourceBoxSelect([], { loading: !streamSourcesLoadedAt });
  if (el.sceneName) el.sceneName.value = edit ? edit.name || "" : "";
  if (el.sceneId) {
    el.sceneId.value = edit ? edit.id || "" : "";
    sceneIdManual = !!edit;
    setSceneIdFieldMode(!!edit);
  }
  pickedStreamUri = "";
  setPickedRadio(null);
  pickedFavoriteId = "";
  favoriteProxySpeaker = "";
  favoritesUsedSpeaker = "";
  favoritesLoadedAt = 0;
  sceneEditFavoriteSource =
    edit && edit.source && edit.source.type === "favorite" ? edit.source : null;
  if (el.sourceUrl) {
    el.sourceUrl.value =
      !sceneEditFavoriteSource && edit && edit.source && edit.source.uri ? edit.source.uri : "";
  }
  if (el.sourceUrlHint) {
    el.sourceUrlHint.hidden = !sceneEditFavoriteSource;
    el.sourceUrlHint.textContent = sceneEditFavoriteSource
      ? t("source.favoriteKeepHint", { name: sceneEditFavoriteSource.name || "" })
      : "";
  }
  if (el.sourceFavoritePreview) {
    el.sourceFavoritePreview.textContent = "";
    el.sourceFavoritePreview.classList.remove("error");
  }
  if (el.sourceBoxPreview) {
    el.sourceBoxPreview.textContent = "";
    el.sourceBoxPreview.classList.remove("error");
  }
  resetRadioSearchUI();
  if (el.sceneSaveHint) {
    el.sceneSaveHint.hidden = true;
    el.sceneSaveHint.textContent = "";
  }
  // Neu: Radio-Suche ist der haeufigste Fall; Bearbeiten zeigt die gespeicherte URL.
  const modeRadio = edit
    ? document.querySelector('input[name="sourceMode"][value="url"]')
    : document.querySelector('input[name="sourceMode"][value="radio"]');
  if (el.sourceFavoriteBtn) el.sourceFavoriteBtn.hidden = !!edit;
  const advanced = el.sceneDialog.querySelector("details.advanced");
  if (advanced) advanced.open = false;
  if (modeRadio) modeRadio.checked = true;
  updateSourceModeUI({ loadPreview: false });
  el.sceneDialog.showModal();
  if (el.sceneName) el.sceneName.focus();
}

function openSceneEditDialog(sc) {
  const needsLoad = discoveredSpeakers.length === 0;
  openSceneDialog({ edit: sc, loadingSpeakers: needsLoad });
  loadSpeakers().then(() => {
    if (!el.sceneDialog || !el.sceneDialog.open || sceneEditId !== sc.id) return;
    renderSceneMemberFields(speakersForSceneEdit(sc), {
      selected: memberSelectionFromScene(sc),
    });
  });
}

function isTVLikeZone(name) {
  const lower = String(name || "").trim().toLowerCase();
  if (lower.includes("fernseher")) return true;
  if (lower.includes(" tv") || lower.endsWith(" tv") || lower.startsWith("tv ") || lower === "tv") {
    return true;
  }
  return lower.split(/[\s:/.-]+/).includes("tv");
}

function orderedFavoriteSpeakers(list) {
  const regular = [];
  const tvLike = [];
  for (const sp of list || []) {
    if (!sp || !sp.name) continue;
    if (isTVLikeZone(sp.name)) tvLike.push(sp);
    else regular.push(sp);
  }
  regular.sort((a, b) => a.name.localeCompare(b.name, I18N.locale));
  tvLike.sort((a, b) => a.name.localeCompare(b.name, I18N.locale));
  return [...regular, ...tvLike];
}

function fillSourceFavoriteSpeakerSelect(speakers, opts = {}) {
  if (!el.sourceFavoriteSpeakerSelect) return;
  const { loading = false } = opts;
  const prev = el.sourceFavoriteSpeakerSelect.value;
  el.sourceFavoriteSpeakerSelect.innerHTML = "";
  if (loading) {
    const opt = document.createElement("option");
    opt.value = "";
    opt.textContent = t("speakers.loading");
    el.sourceFavoriteSpeakerSelect.appendChild(opt);
    el.sourceFavoriteSpeakerSelect.disabled = true;
    return;
  }
  const ordered = orderedFavoriteSpeakers(speakers);
  if (ordered.length === 0) {
    const opt = document.createElement("option");
    opt.value = "";
    opt.textContent = t("fav.noSpeakers");
    el.sourceFavoriteSpeakerSelect.appendChild(opt);
    el.sourceFavoriteSpeakerSelect.disabled = true;
    return;
  }
  const auto = document.createElement("option");
  auto.value = "";
  auto.textContent = t("fav.auto");
  el.sourceFavoriteSpeakerSelect.appendChild(auto);
  for (const sp of ordered) {
    const opt = document.createElement("option");
    opt.value = sp.name;
    opt.textContent = sp.name + (isTVLikeZone(sp.name) ? " (TV)" : "");
    el.sourceFavoriteSpeakerSelect.appendChild(opt);
  }
  el.sourceFavoriteSpeakerSelect.disabled = false;
  if (prev && [...el.sourceFavoriteSpeakerSelect.options].some((o) => o.value === prev)) {
    el.sourceFavoriteSpeakerSelect.value = prev;
  }
}

function fillSourceFavoriteSelect(favorites, opts = {}) {
  if (!el.sourceFavoriteSelect) return;
  const { loading = false, error = "" } = opts;
  el.sourceFavoriteSelect.innerHTML = "";
  if (loading) {
    const opt = document.createElement("option");
    opt.value = "";
    opt.textContent = t("fav.loading");
    el.sourceFavoriteSelect.appendChild(opt);
    el.sourceFavoriteSelect.disabled = true;
    return;
  }
  if (error) {
    const opt = document.createElement("option");
    opt.value = "";
    opt.textContent = error;
    el.sourceFavoriteSelect.appendChild(opt);
    el.sourceFavoriteSelect.disabled = true;
    return;
  }
  const list = (favorites || []).filter((f) => f.integratable || f.playable);
  if (list.length === 0) {
    const opt = document.createElement("option");
    opt.value = "";
    opt.textContent = t("fav.none");
    el.sourceFavoriteSelect.appendChild(opt);
    el.sourceFavoriteSelect.disabled = true;
    return;
  }
  const placeholder = document.createElement("option");
  placeholder.value = "";
  placeholder.textContent = t("fav.choose");
  el.sourceFavoriteSelect.appendChild(placeholder);
  for (const f of list) {
    const opt = document.createElement("option");
    opt.value = f.id;
    opt.textContent = f.integratable ? f.title : t("fav.sonosFavoriteOption", { title: f.title });
    opt.dataset.uri = f.uri || "";
    opt.dataset.kind = f.integratable ? "stream" : "favorite";
    el.sourceFavoriteSelect.appendChild(opt);
  }
  el.sourceFavoriteSelect.disabled = false;
}

async function ensureSonosFavorites(opts = {}) {
  const { force = false, speaker = "" } = opts;
  if (
    !force &&
    favoritesLoadedAt &&
    Date.now() - favoritesLoadedAt < FAVORITES_TTL_MS &&
    (speaker || "") === (favoriteProxySpeaker || "")
  ) {
    fillSourceFavoriteSelect(favoritesCache);
    return favoritesCache;
  }
  if (!el.sceneDialog || !el.sceneDialog.open) return favoritesCache;
  fillSourceFavoriteSelect([], { loading: true });
  const q = speaker ? "?speaker=" + encodeURIComponent(speaker) : "";
  const { ok, status, body } = await api("/api/v1/sonos/favorites" + q);
  if (status === 401) return [];
  if (!ok) {
    const msg = (body && body.message) || t("fav.unavailable");
    fillSourceFavoriteSelect([], { error: msg });
    return [];
  }
  favoritesCache = (body && body.favorites) || [];
  favoritesLoadedAt = Date.now();
  favoriteProxySpeaker = speaker || "";
  favoritesUsedSpeaker = (body && body.speaker) || "";
  if (el.sceneDialog && el.sceneDialog.open) {
    fillSourceFavoriteSelect(favoritesCache);
  }
  if (el.sourceFavoritePreview) {
    if (body && body.discovery_error) {
      el.sourceFavoritePreview.textContent = t("fav.note", { msg: body.discovery_error });
      el.sourceFavoritePreview.classList.add("error");
    } else if (favoritesUsedSpeaker) {
      el.sourceFavoritePreview.textContent = t("fav.from", { name: favoritesUsedSpeaker });
      el.sourceFavoritePreview.classList.remove("error");
    } else {
      el.sourceFavoritePreview.textContent = "";
      el.sourceFavoritePreview.classList.remove("error");
    }
  }
  return favoritesCache;
}

function applyFavoritePreview() {
  pickedFavoriteId = "";
  if (!el.sourceFavoriteSelect || !el.sourceFavoritePreview) return;
  const opt = el.sourceFavoriteSelect.selectedOptions[0];
  if (!opt || !opt.value) {
    if (favoritesUsedSpeaker) {
      el.sourceFavoritePreview.textContent = t("fav.fromChoose", { name: favoritesUsedSpeaker });
      el.sourceFavoritePreview.classList.remove("error");
    } else {
      el.sourceFavoritePreview.textContent = t("fav.chooseImportable");
      el.sourceFavoritePreview.classList.add("error");
    }
    return;
  }
  pickedFavoriteId = opt.value;
  const uri = opt.dataset.uri || "";
  let text = t("fav.noUri");
  if (uri && opt.dataset.kind === "favorite") {
    text = t("fav.willSaveFavorite");
  } else if (uri) {
    text = t("fav.willSave", { uri: displayStreamURI(uri) });
  }
  el.sourceFavoritePreview.textContent = text;
  el.sourceFavoritePreview.classList.toggle("error", !uri);
}

function resetRadioSearchUI() {
  if (radioSearchAbort) {
    radioSearchAbort.abort();
    radioSearchAbort = null;
  }
  if (el.radioSearchInput) el.radioSearchInput.value = "";
  if (el.radioSearchResults) {
    el.radioSearchResults.innerHTML = "";
    el.radioSearchResults.hidden = true;
  }
  if (el.radioSearchHint) {
    el.radioSearchHint.textContent = "";
    el.radioSearchHint.classList.remove("error");
  }
}

function setRadioSearchHint(text, isError = false) {
  if (!el.radioSearchHint) return;
  el.radioSearchHint.textContent = text || "";
  el.radioSearchHint.classList.toggle("error", !!isError);
}

function radioSearchMetaHTML(st) {
  const tags = [];
  if (st.country) tags.push(st.country);
  if (st.tags) {
    for (const part of String(st.tags).split(",")) {
      const t = part.trim();
      if (t) tags.push(t);
    }
  }
  if (tags.length === 0) return "";
  const chips = tags
    .map((t) => `<span class="radio-search-tag">${escapeHtml(t)}</span>`)
    .join("");
  return `<span class="radio-search-meta">${chips}</span>`;
}

function renderRadioSearchResults(stations) {
  if (!el.radioSearchResults) return;
  el.radioSearchResults.innerHTML = "";
  const list = (stations || []).filter((st) => st.url && isIntegratableStreamURI(st.url));
  if (list.length === 0) {
    el.radioSearchResults.hidden = true;
    setRadioSearchHint(t("radio.noResults"), true);
    return;
  }
  for (const st of list) {
    const li = document.createElement("li");
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "radio-search-hit";
    btn.dataset.url = st.url;
    btn.setAttribute("aria-pressed", pickedRadio && pickedRadio.url === st.url ? "true" : "false");
    btn.innerHTML =
      `<span class="radio-search-name">${escapeHtml(st.name)}</span>` +
      radioSearchMetaHTML(st);
    btn.addEventListener("click", () => applyRadioSearchResult(st));
    li.appendChild(btn);
    el.radioSearchResults.appendChild(li);
  }
  el.radioSearchResults.hidden = false;
  setRadioSearchHint(t("radio.hits", { n: list.length }), false);
}

function setPickedRadio(station) {
  pickedRadio = station && station.url ? { name: station.name || "", url: station.url } : null;
  if (!el.radioPicked) return;
  el.radioPicked.hidden = !pickedRadio;
  el.radioPicked.textContent = pickedRadio
    ? t("radio.picked", { name: pickedRadio.name, url: displayStreamURI(pickedRadio.url) })
    : "";
}

function applyRadioSearchResult(station) {
  if (!station || !station.url) return;
  setPickedRadio(station);
  if (el.sceneName && !(el.sceneName.value || "").trim() && station.name) {
    el.sceneName.value = station.name;
    if (el.sceneId && !sceneIdManual) {
      el.sceneId.value = slugFromName(station.name);
    }
  }
  if (el.radioSearchResults) {
    for (const btn of el.radioSearchResults.querySelectorAll(".radio-search-hit")) {
      btn.setAttribute("aria-pressed", btn.dataset.url === station.url ? "true" : "false");
    }
  }
  setRadioSearchHint("", false);
}

async function searchRadioStations(opts = {}) {
  const { q = "", debounced = false } = opts;
  const query = (q || (el.radioSearchInput && el.radioSearchInput.value) || "").trim();
  if (query.length < 2) {
    if (!debounced) setRadioSearchHint(t("radio.minChars"), true);
    if (el.radioSearchResults) {
      el.radioSearchResults.innerHTML = "";
      el.radioSearchResults.hidden = true;
    }
    return;
  }
  if (radioSearchAbort) radioSearchAbort.abort();
  radioSearchAbort = new AbortController();
  const signal = radioSearchAbort.signal;
  setRadioSearchHint(t("radio.searching"), false);
  if (el.radioSearchResults) {
    el.radioSearchResults.innerHTML = "";
    el.radioSearchResults.hidden = true;
  }
  const path = "/api/v1/radio/search?q=" + encodeURIComponent(query) + "&limit=10";
  const { ok, status, body } = await api(path, { signal });
  if (signal.aborted) return;
  radioSearchAbort = null;
  if (status === 401) {
    setRadioSearchHint(t("radio.noToken"), true);
    return;
  }
  if (!ok) {
    setRadioSearchHint((body && (body.hint || body.message)) || t("radio.failed"), true);
    return;
  }
  renderRadioSearchResults((body && body.stations) || []);
}

function scheduleRadioSearch() {
  if (radioSearchTimer) window.clearTimeout(radioSearchTimer);
  radioSearchTimer = window.setTimeout(() => {
    radioSearchTimer = 0;
    searchRadioStations({ debounced: true });
  }, RADIO_SEARCH_DEBOUNCE_MS);
}

function escapeHtml(text) {
  return String(text || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function refreshOpenSceneDialog() {
  if (!el.sceneDialog || !el.sceneDialog.open) return;
  renderSceneMemberFields(discoveredSpeakers);
  fillSourceBoxSelect(streamSourcesCache);
  updateSourceModeUI({ loadPreview: false });
}

function collectScenePayload() {
  const id = (el.sceneId && el.sceneId.value.trim()) || "";
  const name = (el.sceneName && el.sceneName.value.trim()) || "";
  const members = [];
  if (el.sceneMembers) {
    for (const row of el.sceneMembers.querySelectorAll(".member-row")) {
      const cb = row.querySelector('input[type="checkbox"]');
      const vol = row.querySelector('input[type="range"]');
      if (!cb || !cb.checked) continue;
      members.push({
        speaker: cb.value,
        volume: Math.min(100, Math.max(0, parseInt(vol && vol.value, 10) || 0)),
      });
    }
  }
  const mode = document.querySelector('input[name="sourceMode"]:checked');
  let uri = "";
  let favoriteId = "";
  if (mode && mode.value === "box") {
    uri = pickedStreamUri;
  } else if (mode && mode.value === "radio") {
    uri = pickedRadio ? pickedRadio.url : "";
  } else if (mode && mode.value === "favorite") {
    favoriteId = pickedFavoriteId;
  } else if (el.sourceUrl) {
    uri = el.sourceUrl.value.trim();
  }
  return { id, name, members, uri, favoriteId };
}

async function saveSceneToConfig() {
  const { id, name, members, uri, favoriteId } = collectScenePayload();
  const editing = !!sceneEditId;
  if (!name) {
    setSceneFormHint(t("scene.nameMissing"), true);
    return;
  }
  if (!editing && (!id || !/^[a-z0-9][a-z0-9-]*$/.test(id))) {
    setSceneFormHint(t("scene.idSlug"), true);
    return;
  }
  if (members.length === 0) {
    setSceneFormHint(t("scene.needSpeaker"), true);
    return;
  }
  const mode = document.querySelector('input[name="sourceMode"]:checked');
  const fromFavorite = mode && mode.value === "favorite";
  if (editing && fromFavorite) {
    setSceneFormHint(t("scene.favOnlyNew"), true);
    return;
  }
  if (fromFavorite && !favoriteId) {
    setSceneFormHint(t("scene.favMissing"), true);
    return;
  }
  const keepFavorite =
    editing && !!sceneEditFavoriteSource && !uri && (!mode || mode.value === "url");
  if (!fromFavorite && !uri && !keepFavorite) {
    const msg = mode && mode.value === "radio"
      ? t("scene.stationMissing")
      : t("scene.uriMissing");
    setSceneFormHint(msg, true);
    return;
  }
  setSceneFormHint(t("common.saving"), false);
  let path = "/api/v1/config/scenes";
  let method = "POST";
  let payload;
  if (editing) {
    path = "/api/v1/config/scenes/" + encodeURIComponent(sceneEditId);
    method = "PUT";
    payload = {
      name,
      source: keepFavorite ? sceneEditFavoriteSource : { type: "stream", uri },
      members,
    };
  } else if (fromFavorite) {
    path = "/api/v1/config/scenes/from-favorite";
    payload = { scene_id: id, name, favorite_id: favoriteId, members };
    if (favoriteProxySpeaker) payload.speaker = favoriteProxySpeaker;
  } else {
    payload = {
      scene: {
        id,
        name,
        source: { type: "stream", uri },
        members,
      },
    };
  }
  const { ok, status, body } = await api(path, {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (status === 401) {
    setSceneFormHint(t("token.invalid"), true);
    openTokenDialog();
    return;
  }
  if (!ok) {
    const msg = (body && body.message) || t("common.saveFailed");
    setSceneFormHint(msg, true);
    return;
  }
  // "Save & play": reload the list, then start the scene right away.
  const savedId = editing ? sceneEditId : id;
  setSceneFormHint(t("common.saved"), false);
  sceneEditId = null;
  sceneEditFavoriteSource = null;
  if (el.sceneDialog) el.sceneDialog.close();
  await loadScenes();
  const row = sceneCards.get(savedId);
  const sc = sceneById.get(savedId);
  if (row && sc) playScene(sc, row);
}

function setSceneFormHint(text, isError) {
  if (!el.sceneSaveHint) return;
  el.sceneSaveHint.hidden = !text;
  el.sceneSaveHint.textContent = text;
  el.sceneSaveHint.classList.toggle("error", !!isError);
}

function setSpeakerActionLabel(btn, text) {
  if (!btn) return;
  const label = btn.querySelector(".btn-label");
  if (label) label.textContent = text;
  else btn.textContent = text;
}

async function saveSpeakersToConfig() {
  if (!discoveredSpeakers.length) {
    setHint(t("speakers.noneToSave"), true);
    return;
  }
  const btn = el.saveSpeakersConfigBtn;
  if (btn) {
    btn.disabled = true;
    btn.setAttribute("aria-busy", "true");
    setSpeakerActionLabel(btn, t("speakers.savingLabel"));
  }
  const speakers = discoveredSpeakers.map((s) => ({ name: s.name, ip: s.ip || "" }));
  const { ok, status, body } = await api("/api/v1/config/speakers/sync", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ speakers }),
  });
  if (btn) {
    btn.disabled = false;
    btn.setAttribute("aria-busy", "false");
    setSpeakerActionLabel(btn, t("speakers.saveConfig"));
  }
  if (status === 401) {
    setHint(t("token.invalidReenter"), true);
    openTokenDialog();
    return;
  }
  if (!ok) {
    setHint((body && body.message) || t("common.saveFailed"), true);
    return;
  }
  const hint = (body && body.restart_hint) || t("speakers.saved");
  setHint(hint, false);
}

function createSpeakerCard(sp) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "speaker-card";
  if (!sp.reachable) btn.classList.add("offline");
  btn.dataset.name = sp.name;
  btn.setAttribute("aria-pressed", "false");

  const label = document.createElement("span");
  label.className = "speaker-name";
  label.textContent = sp.name;
  btn.appendChild(label);

  // Eine Mono-Zeile: IP · Now-Playing (bzw. offline)
  const subtitle = document.createElement("span");
  subtitle.className = "speaker-sub";
  btn.appendChild(subtitle);

  btn.addEventListener("click", () => {
    if (discoverSpeaker === sp.name) return;
    enterDiscoverMode(sp.name);
  });
  return btn;
}

function renderSpeakerSub(card) {
  const sub = card.querySelector(".speaker-sub");
  if (!sub) return;
  const parts = [];
  if (card.dataset.ip) parts.push(card.dataset.ip);
  if (card.classList.contains("offline")) parts.push(t("state.offline"));
  else if (card.dataset.now) parts.push(card.dataset.now);
  const text = parts.join(" · ") || "—";
  if (sub.textContent !== text) sub.textContent = text;
}

function patchSpeakerCardMeta(card, sp) {
  card.classList.toggle("offline", sp.reachable === false);
  card.dataset.ip = sp.ip || "";
  renderSpeakerSub(card);
}

function syncSpeakersDOM(speakers, opts = {}) {
  const list = speakers || [];
  discoveredSpeakers = list;
  discoveredNames = new Set(list.map((s) => s.name));
  const names = new Set(discoveredNames);

  if (el.speakerCount) el.speakerCount.textContent = String(list.length);
  if (list.length === 0) {
    for (const [, card] of speakerCards) card.remove();
    speakerCards.clear();
    if (discoverSpeaker) exitDiscoverMode();
    return;
  }

  for (const [name, card] of speakerCards) {
    if (!names.has(name)) {
      card.remove();
      speakerCards.delete(name);
      nowPlayingCache.delete(name);
    }
  }

  for (const sp of list) {
    let card = speakerCards.get(sp.name);
    if (!card) {
      card = createSpeakerCard(sp);
      speakerCards.set(sp.name, card);
      el.speakerList.appendChild(card);
    }
    patchSpeakerCardMeta(card, sp);
    const cached = nowPlayingCache.get(sp.name);
    if (cached) updateSpeakerSubtitle(card, cached);
  }
  updateSpeakerSelection();
  renderGroups();
  checkSceneMemberNames({ strict: opts.strictValidation === true });
}

// ---- Gruppen im Boxen-Tab (GET /api/v1/groups, ADR 0015) ----

let groupsData = null; // letzte Topologie; null = noch nicht geladen
let groupsFingerprint = "";
const groupBlocks = new Map(); // Coordinator -> <section>
const groupOfSpeaker = new Map(); // Box -> Gruppe (fuer „Aus Gruppe lösen“)
const GROUP_ACTIVE_STATES = ["playing", "paused", "transitioning"];
const GROUP_STATE_LABELS = translated({ transitioning: "state.transitioning" });

/** Gruppe mit eigenem Kopf: mehrere Boxen oder es laeuft/pausiert etwas. */
function groupHasControls(g) {
  return (g.members || []).length > 1 || GROUP_ACTIVE_STATES.includes(g.state);
}

function groupSubtitle(g) {
  const state = GROUP_STATE_LABELS[g.state] || PLAYBACK_LABELS[g.state] || g.state || "—";
  const sc = g.scene_id && sceneById.get(g.scene_id);
  const what = g.state === "offline" ? "" : (sc && sc.name) || g.title || g.artist || "";
  return { state, what };
}

function createGroupButton(cls, onClick) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "icon-btn group-btn " + cls;
  btn.addEventListener("click", onClick);
  return btn;
}

function setGroupButtonIcon(btn, icon) {
  if (btn.dataset.icon === icon) return;
  btn.dataset.icon = icon;
  btn.replaceChildren(createLucideIcon(icon, "lucide-icon play-icon"));
}

function createGroupBlock(coordinator) {
  const block = document.createElement("section");
  block.className = "group-block";
  block.dataset.coordinator = coordinator;
  const head = document.createElement("div");
  head.className = "group-head";
  const info = document.createElement("div");
  info.className = "group-info";
  const name = document.createElement("span");
  name.className = "group-name";
  const sub = document.createElement("span");
  sub.className = "group-sub";
  info.append(name, sub);
  const toggle = createGroupButton("group-toggle", () => {
    const g = groupBlockData(block);
    if (g) groupAction(g, g.state === "playing" || g.state === "transitioning" ? "pause" : "resume", block);
  });
  const stop = createGroupButton("group-stop", () => {
    const g = groupBlockData(block);
    if (g) groupAction(g, "stop", block);
  });
  setGroupButtonIcon(stop, "square");
  head.append(info, toggle, stop);
  const members = document.createElement("ul");
  members.className = "speakers-list group-members";
  block.append(head, members);
  return block;
}

function groupBlockData(block) {
  return (groupsData || []).find((g) => g.coordinator === block.dataset.coordinator) || null;
}

function patchGroupBlock(block, g) {
  const members = g.members || [];
  block.dataset.state = g.state || "unknown";
  block.setAttribute("aria-label", t("group.aria", { name: g.coordinator }));
  const name = block.querySelector(".group-name");
  const count = members.length > 1 ? " +" + (members.length - 1) : "";
  const nameKey = g.coordinator + count;
  if (name.dataset.key !== nameKey) {
    name.dataset.key = nameKey;
    name.textContent = g.coordinator;
    if (count) {
      const c = document.createElement("span");
      c.className = "group-count";
      c.textContent = count;
      name.append(c);
    }
  }
  const { state, what } = groupSubtitle(g);
  const sub = block.querySelector(".group-sub");
  const subKey = state + "\t" + what;
  if (sub.dataset.key !== subKey) {
    sub.dataset.key = subKey;
    const st = document.createElement("span");
    st.className = "group-state";
    st.textContent = state;
    sub.replaceChildren(st);
    if (what) sub.append(" · " + what);
  }
  const busy = block.dataset.busy === "true";
  const running = g.state === "playing" || g.state === "transitioning";
  const toggle = block.querySelector(".group-toggle");
  const stop = block.querySelector(".group-stop");
  const offline = g.state === "offline";
  setGroupButtonIcon(toggle, running ? "pause" : "play");
  toggle.setAttribute("aria-label", t(running ? "group.pause" : "group.resume", { name: g.coordinator }));
  toggle.hidden = offline;
  toggle.disabled = busy;
  stop.setAttribute("aria-label", t("group.stop", { name: g.coordinator }));
  // Gestoppt: nur Fortsetzen anbieten (setzt fort, was auf der Box geladen ist).
  stop.hidden = offline || !(running || g.state === "paused");
  stop.disabled = busy;
}

/** Ordnet die Box-Karten den aktuellen Gruppen zu; Rest unter „Einzeln“. */
function renderGroups() {
  if (!el.groupList || !el.speakerList) return;
  groupOfSpeaker.clear();
  const active = [];
  for (const g of groupsData || []) {
    for (const m of g.members || []) groupOfSpeaker.set(m, g);
    if (groupHasControls(g)) active.push(g);
  }
  const keep = new Set(active.map((g) => g.coordinator));
  for (const [coord, block] of groupBlocks) {
    if (!keep.has(coord)) {
      block.remove();
      groupBlocks.delete(coord);
    }
  }
  const placed = new Set();
  for (const g of active) {
    let block = groupBlocks.get(g.coordinator);
    if (!block) {
      block = createGroupBlock(g.coordinator);
      groupBlocks.set(g.coordinator, block);
    }
    el.groupList.append(block); // haelt die API-Reihenfolge
    patchGroupBlock(block, g);
    const list = block.querySelector(".group-members");
    for (const m of g.members || []) {
      const card = speakerCards.get(m);
      if (!card || placed.has(m)) continue;
      placed.add(m);
      list.append(card);
    }
  }
  let singles = 0;
  for (const sp of discoveredSpeakers) {
    const card = speakerCards.get(sp.name);
    if (!card || placed.has(sp.name)) continue;
    singles++;
    el.speakerList.append(card);
  }
  if (el.singlesLabel) el.singlesLabel.hidden = active.length === 0 || singles === 0;
  updateDiscoverUngroup();
}

async function loadGroups() {
  if (!shouldRunStatusPoll()) return;
  const { ok, status, body } = await api("/api/v1/groups");
  noteRateLimit(status);
  if (status === 401 || !ok) return;
  const groups = (body && body.groups) || [];
  const fp = JSON.stringify(groups);
  if (fp === groupsFingerprint && groupsData) return;
  groupsFingerprint = fp;
  groupsData = groups;
  renderGroups();
}

const GROUP_ACTION_STATES = { pause: "paused", resume: "playing", stop: "stopped" };
const GROUP_ACTION_ERRORS = translated({
  pause: "group.pauseFailed",
  resume: "group.resumeFailed",
  stop: "group.stopFailed",
});

/** Befehl an die Gruppe; der Agent schickt ihn an den aktuellen Coordinator. */
async function groupAction(g, action, block) {
  if (block.dataset.busy === "true") return;
  block.dataset.busy = "true";
  patchGroupBlock(block, g);
  setHint("");
  const { ok, status, body } = await api(
    "/api/v1/speakers/" + encodeSpeakerPath(g.coordinator) + "/" + action,
    { method: "POST" }
  );
  delete block.dataset.busy;
  if (status === 401) { patchGroupBlock(block, g); openTokenDialog(); return; }
  if (!ok) {
    const msg = (body && body.message) || GROUP_ACTION_ERRORS[action];
    setHint(msg + (body && body.hint ? " (" + body.hint + ")" : ""), true);
  } else {
    g.state = GROUP_ACTION_STATES[action];
    groupsFingerprint = "";
  }
  renderGroups();
  loadGroups();
  runStatusPollTick();
}

// ---- „Aus Gruppe lösen“ im Box-Detail (POST /api/v1/speakers/{name}/ungroup) ----

let pendingUngroup = null; // { name, group }

/** Knopf nur, wenn die Box gerade in einer Gruppe mit mehreren Boxen steht. */
function updateDiscoverUngroup() {
  const btn = el.discoverUngroupBtn;
  if (!btn) return;
  const g = discoverSpeaker && groupOfSpeaker.get(discoverSpeaker);
  const grouped = !!g && (g.members || []).length > 1;
  btn.hidden = !grouped;
  if (!grouped) return;
  const label = btn.querySelector(".btn-label");
  const text = t(g.coordinator === discoverSpeaker ? "group.ungroupAll" : "group.leave");
  if (label && label.textContent !== text) label.textContent = text;
  btn.setAttribute("aria-label", text + ": " + discoverSpeaker);
}

function ungroupMessage(name, g) {
  const others = (g.members || []).filter((m) => m !== name);
  if (g.coordinator === name) {
    return t("group.msgDissolve", { name, others: others.join(", ") });
  }
  return t("group.msgLeave", { name, coordinator: g.coordinator });
}

function openUngroupDialog(name) {
  const g = groupOfSpeaker.get(name);
  if (!g || !el.ungroupDialog) return;
  pendingUngroup = { name, group: g };
  const dissolve = g.coordinator === name;
  if (el.ungroupTitle) el.ungroupTitle.textContent = t(dissolve ? "group.ungroupAll" : "group.leave");
  if (el.ungroupConfirmBtn) el.ungroupConfirmBtn.textContent = t(dissolve ? "group.btnDissolve" : "group.btnUngroup");
  if (el.ungroupMessage) el.ungroupMessage.textContent = ungroupMessage(name, g);
  el.ungroupDialog.showModal();
}

/** Teilausfall ist kein harter Fehler: fehlgeschlagene Boxen einzeln nennen. */
function ungroupFailures(result) {
  return ((result && result.speakers) || [])
    .filter((s) => !s.ok)
    .map((s) => s.speaker + (s.error ? " (" + s.error + ")" : ""));
}

async function ungroupSpeaker({ name }) {
  setHint("");
  const btn = el.discoverUngroupBtn;
  if (btn) btn.disabled = true;
  const { ok, status, body } = await api(
    "/api/v1/speakers/" + encodeSpeakerPath(name) + "/ungroup",
    { method: "POST" }
  );
  if (btn) btn.disabled = false;
  if (status === 401) { openTokenDialog(); return; }
  const result = ok ? body : body && body.result;
  const failed = ungroupFailures(result);
  if (!ok) {
    const msg = (body && body.message) || t("group.ungroupFailed");
    const detail = failed.length ? " — " + failed.join(", ") : "";
    setHint(msg + (body && body.hint ? " (" + body.hint + ")" : "") + detail, true);
  } else if (failed.length) {
    setHint(t("group.notUngrouped", { list: failed.join(", ") }), true);
  } else if (result && result.standalone) {
    setHint(t("group.alreadyAlone", { name }), false);
  } else {
    setHint(result && result.dissolved ? t("group.dissolved") : t("group.left", { name }), false);
  }
  groupsFingerprint = "";
  await loadGroups();
  runStatusPollTick();
}

if (el.discoverUngroupBtn) {
  el.discoverUngroupBtn.addEventListener("click", () => {
    if (discoverSpeaker) openUngroupDialog(discoverSpeaker);
  });
}
if (el.ungroupForm) {
  el.ungroupForm.addEventListener("submit", (e) => {
    e.preventDefault();
    const action = e.submitter && e.submitter.value;
    el.ungroupDialog.close();
    if (action === "ungroup" && pendingUngroup) ungroupSpeaker(pendingUngroup);
    pendingUngroup = null;
  });
}

function checkSceneMemberNames(opts = {}) {
  const strict = opts.strict === true;
  if (discoveredNames.size === 0 || loadedScenes.length === 0) return;
  const unknown = [];
  for (const sc of loadedScenes) {
    for (const m of sc.members || []) {
      if (m.speaker && !discoveredNames.has(m.speaker)) {
        unknown.push(m.speaker);
      }
    }
  }
  if (unknown.length === 0) return;
  if (!strict) return;
  const uniq = [...new Set(unknown)];
  setHint(t("scenes.namesNotInDiscovery", { names: uniq.join(", ") }), true);
}

function setSpeakersRefreshBusy(busy) {
  speakersLoading = busy;
  if (!el.refreshSpeakersBtn) return;
  el.refreshSpeakersBtn.disabled = busy;
  el.refreshSpeakersBtn.setAttribute("aria-busy", busy ? "true" : "false");
  setSpeakerActionLabel(el.refreshSpeakersBtn, t(busy ? "common.loading" : "common.refresh"));
}

async function loadSpeakers(opts = {}) {
  const { showErrors = false, fromManual = false } = opts;
  const token = getToken();
  if (!token || !tokenHeaderSafe(token)) {
    if (fromManual) openTokenDialog();
    return false;
  }
  if (fromManual) setSpeakersRefreshBusy(true);
  try {
    const fetchOpts = fromManual ? { method: "POST" } : {};
    const path = fromManual ? "/api/v1/discovery/refresh" : "/api/v1/speakers";
    const { ok, status, body } = await api(path, fetchOpts);
    if (status === 401) {
      if (showErrors || fromManual) {
        setHint(t("token.invalidReenter"), true);
        openTokenDialog();
      }
      return false;
    }
    if (!ok) {
      if (showErrors || fromManual) {
        setHint(t("speakers.listFailed"), true);
      }
      return false;
    }
    const speakers = (body && body.speakers) || [];
    const fp = speakersFingerprint(speakers);
    if (fp !== speakersListFingerprint) {
      speakersListFingerprint = fp;
      syncSpeakersDOM(speakers, { strictValidation: fromManual });
    }
    if (body && body.discovery_error) {
      if (fromManual || !el.hint.classList.contains("error")) {
        setHint(t("status.discoveryHint", { msg: body.discovery_error }), false);
      }
    } else if (fromManual && !el.hint.classList.contains("error")) {
      setHint("");
    }
    if (discoverSpeaker && speakerCards.has(discoverSpeaker)) {
      loadNowPlaying(discoverSpeaker);
    }
    return true;
  } finally {
    if (fromManual) setSpeakersRefreshBusy(false);
  }
}

async function refreshSpeakersAndStatus() {
  await refreshStatus();
  await loadSpeakers({ showErrors: true, fromManual: true });
  groupsFingerprint = "";
  await loadGroups();
}

async function loadScenes() {
  const token = getToken();
  if (!token) { setSpeakersPanelVisible(false); openTokenDialog(); return; }
  if (!tokenHeaderSafe(token)) {
    localStorage.removeItem(TOKEN_KEY);
    setHint(t("token.latin1"), true);
    openTokenDialog();
    return;
  }
  const { ok, status, body } = await api("/api/v1/scenes");
  if (status === 401) { setHint(t("token.invalidReenter"), true); openTokenDialog(); return; }
  if (!ok) {
    setHint(t("scenes.loadFailed"), true);
    setSpeakersPanelVisible(false);
    setSceneBuilderVisible(false);
    return;
  }

  setSpeakersPanelVisible(true);
  setSceneBuilderVisible(true);
  const scenes = (body && body.scenes) || [];
  loadedScenes = scenes;
  syncScenesDOM(scenes);
  schedulesEnabled = true;
  loadSchedules();
  startSchedulesPolling();
  if (scenes.length === 0) {
    setHint(t("scenes.none"), false);
    return;
  }
  if (!el.hint.classList.contains("error")) setHint("");
  el.meta.textContent = tn("scenes.count", scenes.length);
  loadSpeakers();
  runStatusPollTick();
  startStatusPolling();
  startSpeakersListPolling();
}

function sceneCardFingerprint(sc) {
  const members = (sc.members || [])
    .map((m) => `${m.speaker}:${m.volume}`)
    .join(",");
  return `${sc.id}|${sc.name}|${sc.source.type}|${sc.source.uri || ""}|${sc.source.name || ""}|${members}`;
}

function sceneMembersText(sc) {
  return (sc.members || []).map((m) => m.speaker).join(" · ");
}

function patchSceneCard(row, sc) {
  const name = row.querySelector(".scene-name");
  const members = row.querySelector(".scene-rooms");
  if (name && name.textContent !== sc.name) name.textContent = sc.name;
  const membersText = sceneMembersText(sc);
  if (members && members.textContent !== membersText) members.textContent = membersText;
  row.dataset.fingerprint = sceneCardFingerprint(sc);
  sceneById.set(sc.id, sc);
  updateScenePlayButton(row);
}

function syncScenesDOM(scenes) {
  const list = scenes || [];
  const ids = new Set(list.map((s) => s.id));

  for (const [id, row] of sceneCards) {
    if (!ids.has(id)) {
      row.remove();
      sceneCards.delete(id);
      sceneById.delete(id);
    }
  }
  updateNowPlayingBar();

  for (const sc of list) {
    const fp = sceneCardFingerprint(sc);
    let row = sceneCards.get(sc.id);
    if (!row) {
      row = renderScene(sc);
      row.dataset.fingerprint = fp;
      sceneCards.set(sc.id, row);
      el.grid.appendChild(row);
      continue;
    }
    if (row.dataset.fingerprint !== fp) patchSceneCard(row, sc);
  }
}

/**
 * Szenen-Zeile (Vinyls-Tracklist): runder Play/Stop links, Name + Raeume in der
 * Mitte (ganze Flaeche tappbar), ⋯ rechts fuer Bearbeiten/Lautstaerke.
 */
function renderScene(sc) {
  sceneById.set(sc.id, sc);
  const row = document.createElement("div");
  row.className = "scene-row";
  row.dataset.id = sc.id;
  row.dataset.state = "";

  const play = document.createElement("button");
  play.type = "button";
  play.className = "scene-play";
  play.addEventListener("click", () => toggleScene(sc.id));

  const main = document.createElement("button");
  main.type = "button";
  main.className = "scene-main";
  const name = document.createElement("span");
  name.className = "scene-name";
  name.textContent = sc.name;
  const sub = document.createElement("span");
  sub.className = "scene-sub";
  const state = document.createElement("span");
  state.className = "scene-state";
  const members = document.createElement("span");
  members.className = "scene-rooms";
  members.textContent = sceneMembersText(sc);
  sub.append(state, members);
  main.append(name, sub);
  main.addEventListener("click", () => toggleScene(sc.id));

  const more = document.createElement("button");
  more.type = "button";
  more.className = "icon-btn scene-more";
  more.hidden = !sceneBuilderEnabled;
  more.setAttribute("aria-label", t("scene.actions", { name: sc.name }));
  more.append(createLucideIcon("ellipsis", "lucide-icon scene-action-icon"));
  more.addEventListener("click", () => openSceneMenu(sceneById.get(sc.id) || sc));

  row.append(play, main, more);
  updateScenePlayButton(row);
  return row;
}

/** Zustaende, bei denen der runde Knopf Stop statt Play zeigt. */
function sceneIsRunning(state) {
  return state === "playing" || state === "partial";
}

/** Uebergangszustaende waehrend eines Befehls: Poll ueberschreibt sie nicht. */
const SCENE_BUSY_STATES = ["loading", "stopping", "pausing", "resuming"];
function sceneIsBusy(state) {
  return SCENE_BUSY_STATES.includes(state);
}

function updateScenePlayButton(row) {
  const sc = sceneById.get(row.dataset.id);
  const play = row.querySelector(".scene-play");
  if (!sc || !play) return;
  const running = sceneIsRunning(row.dataset.state);
  const icon = running ? "square" : "play";
  if (play.dataset.icon !== icon) {
    play.dataset.icon = icon;
    play.replaceChildren(createLucideIcon(icon, "lucide-icon play-icon"));
  }
  const key = running ? "scene.stop" : row.dataset.state === "paused" ? "scene.resume" : "scene.play";
  const label = t(key, { name: sc.name });
  if (play.getAttribute("aria-label") !== label) play.setAttribute("aria-label", label);
}

function setSceneRowState(row, state, label, title) {
  if (!row) return;
  row.dataset.state = state || "";
  const st = row.querySelector(".scene-state");
  if (st) {
    const text = label ? label + " · " : "";
    if (st.textContent !== text) st.textContent = text;
  }
  const main = row.querySelector(".scene-main");
  if (main) {
    if (title) main.title = title;
    else main.removeAttribute("title");
  }
  updateScenePlayButton(row);
  updateNowPlayingBar();
}

// ---- „Läuft gerade“-Leiste (Mini-Player ueber der Tab-Bar) ----

let nowPlayingSceneId = "";

/** Szene gilt fuer die Leiste als aktiv: laeuft, pausiert oder Befehl unterwegs. */
function sceneIsActive(state) {
  return sceneIsRunning(state) || state === "paused" || (sceneIsBusy(state) && state !== "loading");
}

/**
 * Erste laufende Szene in Listenreihenfolge (sonst erste pausierte);
 * weitere aktive Szenen werden mitgezaehlt.
 */
function updateNowPlayingBar() {
  if (!el.nowPlaying) return;
  const active = [...sceneCards.values()].filter((row) => sceneIsActive(row.dataset.state));
  const running = [
    ...active.filter((row) => row.dataset.state !== "paused"),
    ...active.filter((row) => row.dataset.state === "paused"),
  ];
  const row = running[0];
  const sc = row && sceneById.get(row.dataset.id);
  if (!sc) {
    nowPlayingSceneId = "";
    el.nowPlaying.hidden = true;
    document.body.classList.remove("has-now-playing");
    return;
  }
  nowPlayingSceneId = sc.id;
  el.nowPlaying.hidden = false;
  el.nowPlaying.dataset.state = row.dataset.state;
  document.body.classList.add("has-now-playing");
  if (el.nowPlayingName.textContent !== sc.name) el.nowPlayingName.textContent = sc.name;
  const more = running.length > 1 ? " · " + t("np.more", { n: running.length - 1 }) : "";
  const sub = sceneMembersText(sc) + more;
  if (el.nowPlayingSub.textContent !== sub) el.nowPlayingSub.textContent = sub;
  const busy = sceneIsBusy(row.dataset.state);
  el.nowPlayingStop.setAttribute("aria-label", t("scene.stop", { name: sc.name }));
  el.nowPlayingStop.disabled = busy;
  if (el.nowPlayingPause) {
    const paused = row.dataset.state === "paused" || row.dataset.state === "resuming";
    const icon = paused ? "play" : "pause";
    if (el.nowPlayingPause.dataset.icon !== icon) {
      el.nowPlayingPause.dataset.icon = icon;
      el.nowPlayingPause.replaceChildren(createLucideIcon(icon, "lucide-icon play-icon"));
    }
    el.nowPlayingPause.setAttribute("aria-label", t(paused ? "scene.resume" : "scene.pause", { name: sc.name }));
    el.nowPlayingPause.disabled = busy;
  }
  el.nowPlayingMain.setAttribute("aria-label", t("np.mainAria", { name: sc.name }));
}

if (el.nowPlayingStop) {
  el.nowPlayingStop.addEventListener("click", () => {
    const row = sceneCards.get(nowPlayingSceneId);
    const sc = sceneById.get(nowPlayingSceneId);
    if (!row || !sc) return;
    if (sceneIsRunning(row.dataset.state) || row.dataset.state === "paused") stopScene(sc, row);
  });
}
if (el.nowPlayingPause) {
  el.nowPlayingPause.addEventListener("click", () => {
    const row = sceneCards.get(nowPlayingSceneId);
    const sc = sceneById.get(nowPlayingSceneId);
    if (row && sc) togglePauseScene(sc, row);
  });
}
if (el.nowPlayingMain) {
  el.nowPlayingMain.addEventListener("click", () => {
    const sc = sceneById.get(nowPlayingSceneId);
    if (sc) openSceneMenu(sc);
  });
}

function toggleScene(id) {
  const row = sceneCards.get(id);
  const sc = sceneById.get(id);
  if (!row || !sc) return;
  if (sceneIsBusy(row.dataset.state)) return;
  if (sceneIsRunning(row.dataset.state)) stopScene(sc, row);
  else if (row.dataset.state === "paused") resumeScene(sc, row);
  else playScene(sc, row);
}

/** Pause bzw. Fortsetzen je nach aktuellem Zustand (Mini-Player, ⋯-Sheet). */
function togglePauseScene(sc, row) {
  if (sceneIsBusy(row.dataset.state)) return;
  if (row.dataset.state === "paused") resumeScene(sc, row);
  else if (sceneIsRunning(row.dataset.state)) pauseScene(sc, row);
}

const PLAYBACK_LABELS = translated({
  playing: "state.playing",
  playing_other: "state.playing_other",
  stopped: "state.stopped",
  paused: "state.paused",
  unknown: "state.unknown",
  offline: "state.offline",
});

function formatSpeakerSubtitle(np) {
  if (!np) return "—";
  if (np.state === "playing" && np.title) return np.title;
  if (np.state === "playing" && np.artist) return np.artist;
  if (np.state === "paused") return PLAYBACK_LABELS.paused;
  if (np.state === "stopped") return PLAYBACK_LABELS.stopped;
  return PLAYBACK_LABELS[np.state] || "—";
}

function applyPlaybackState(row, state, nowPlayingTitle) {
  if (!row || sceneIsBusy(row.dataset.state)) return;
  const st = state || "stopped";
  const label = st === "stopped" || st === "unknown" ? "" : PLAYBACK_LABELS[st] || st;
  const title =
    st === "playing_other" && nowPlayingTitle ? t("scene.playingTitle", { title: nowPlayingTitle }) : "";
  const next = st + "\t" + label + "\t" + title;
  if (row.dataset.playbackPoll === next && row.dataset.state === st) return;
  row.dataset.playbackPoll = next;
  setSceneRowState(row, st, label, title);
}

function updateSpeakerSubtitle(card, np) {
  const text = formatSpeakerSubtitle(np);
  card.dataset.now = text === "—" ? "" : text;
  renderSpeakerSub(card);
}

// Nach HTTP 429 (rate_limited) Polling kurz aussetzen.
let pollBackoffUntil = 0;
function noteRateLimit(status) {
  if (status === 429) pollBackoffUntil = Date.now() + 30000;
}

function shouldRunStatusPoll() {
  return Date.now() >= pollBackoffUntil && !document.hidden && !anyDialogOpen() && getToken() && tokenHeaderSafe(getToken());
}

async function pollSceneStatus() {
  if (!shouldRunStatusPoll()) return;
  const { ok, status, body } = await api("/api/v1/scenes/status");
  noteRateLimit(status);
  if (status === 401 || !ok) return;
  const statuses = (body && body.scenes) || [];
  for (const st of statuses) {
    applyPlaybackState(sceneCards.get(st.scene_id), st.state, st.now_playing_title);
  }
}

async function pollSpeakersNowPlaying() {
  if (!shouldRunStatusPoll()) return;
  const { ok, status, body } = await api("/api/v1/speakers/now-playing");
  noteRateLimit(status);
  if (status === 401 || !ok) return;
  const list = (body && body.speakers) || [];
  for (const np of list) {
    const key = np.speaker;
    if (!key) continue;
    const prev = nowPlayingCache.get(key);
    const prevKey = prev
      ? `${prev.state}|${prev.title}|${prev.artist}|${prev.uri}|${prev.integratable}`
      : "";
    const nextKey = `${np.state}|${np.title}|${np.artist}|${np.uri}|${np.integratable}`;
    nowPlayingCache.set(key, np);
    if (prevKey === nextKey) continue;
    const card = speakerCards.get(key);
    if (card) updateSpeakerSubtitle(card, np);
  }
}

async function runStatusPollTick() {
  if (!shouldRunStatusPoll() || statusPollInFlight) return;
  statusPollInFlight = true;
  try {
    await pollSceneStatus();
    await new Promise((r) => setTimeout(r, POLL_STAGGER_MS));
    await pollSpeakersNowPlaying();
    if (currentTab === "boxen") {
      await new Promise((r) => setTimeout(r, POLL_STAGGER_MS));
      await loadGroups();
    }
  } finally {
    statusPollInFlight = false;
  }
}

function startStatusPolling() {
  stopStatusPolling();
  statusPollTimer = setInterval(runStatusPollTick, STATUS_POLL_MS);
}

function stopSpeakersListPolling() {
  if (speakersPollTimer) {
    clearInterval(speakersPollTimer);
    speakersPollTimer = null;
  }
}

function startSpeakersListPolling() {
  stopSpeakersListPolling();
  speakersPollTimer = setInterval(() => {
    if (document.hidden || anyDialogOpen()) return;
    loadSpeakers();
  }, SPEAKERS_LIST_TTL_MS);
}

function stopStatusPolling() {
  if (statusPollTimer) {
    clearInterval(statusPollTimer);
    statusPollTimer = null;
  }
}

for (const dlg of [el.dialog, el.sceneDialog, el.sceneDeleteDialog, el.sceneMenuSheet, el.scheduleDialog, el.scheduleDeleteDialog, el.ungroupDialog]) {
  if (!dlg) continue;
  dlg.addEventListener("close", () => {
    if (!document.hidden && !anyDialogOpen()) runStatusPollTick();
  });
}

function openSceneDeleteDialog(sc) {
  pendingSceneDelete = { sc };
  if (el.sceneDeleteMessage) {
    el.sceneDeleteMessage.textContent = t("scene.deleteConfirm", { name: sc.name });
  }
  el.sceneDeleteDialog.showModal();
}

async function deleteSceneFromConfig({ sc }) {
  setHint("");
  const { ok, status, body } = await api(
    "/api/v1/config/scenes/" + encodeURIComponent(sc.id),
    { method: "DELETE" }
  );
  if (status === 401) {
    openTokenDialog();
    return;
  }
  if (!ok) {
    const msg = (body && body.message) || t("scene.deleteFailed");
    setHint(msg, true);
    return;
  }
  const row = sceneCards.get(sc.id);
  if (row) row.remove();
  sceneCards.delete(sc.id);
  sceneById.delete(sc.id);
  if (el.sceneDialog && el.sceneDialog.open) el.sceneDialog.close();
  sceneEditId = null;
  loadScenes();
}

if (el.sceneDeleteForm) {
  el.sceneDeleteForm.addEventListener("submit", (e) => {
    e.preventDefault();
    const action = e.submitter && e.submitter.value;
    el.sceneDeleteDialog.close();
    if (action === "delete" && pendingSceneDelete) {
      deleteSceneFromConfig(pendingSceneDelete);
    }
    pendingSceneDelete = null;
  });
}

document.addEventListener("visibilitychange", () => {
  if (document.hidden) {
    stopStatusPolling();
    stopSpeakersListPolling();
    stopDiscoverPolling();
    stopSchedulesPolling();
  } else {
    runStatusPollTick();
    if (schedulesEnabled) {
      loadSchedules();
      startSchedulesPolling();
    }
    startStatusPolling();
    startSpeakersListPolling();
    if (discoverSpeaker) {
      loadNowPlaying(discoverSpeaker);
      startDiscoverPolling();
    }
  }
});

/** Kurz angezeigter Fehler-/Teilzustand, danach wieder Live-Status vom Poll. */
function resetSceneRowLater(row, states) {
  setTimeout(() => {
    if (states.includes(row.dataset.state)) {
      row.dataset.playbackPoll = "";
      setSceneRowState(row, "", "", "");
      runStatusPollTick();
    }
  }, 4000);
}

async function playScene(sc, row) {
  setSceneRowState(row, "loading", t("scene.starting"), "");
  setHint("");

  const { ok, status, body } = await api(
    "/api/v1/scenes/" + encodeURIComponent(sc.id) + "/play",
    { method: "POST" }
  );

  if (status === 401) { setSceneRowState(row, "", "", ""); openTokenDialog(); return; }

  const result = ok ? body : (body && body.result);
  const speakers = (result && result.speakers) || [];
  const failed = speakers.filter((s) => !s.ok);
  row.dataset.playbackPoll = "";

  if (ok && failed.length === 0) {
    setSceneRowState(row, "playing", PLAYBACK_LABELS.playing, "");
  } else if (ok && speakers.length > failed.length) {
    // Teilausfall nur bei HTTP 200: scheitert die Wiedergabe auf dem
    // Coordinator (z. B. Stream-Format), spielt nichts — auch wenn die
    // uebrigen Boxen gruppiert wurden (Antwort 502 mit result).
    setSceneRowState(row, "partial", t("scene.partialOffline", { n: failed.length }), "");
    setHint(t("scene.partial", { names: failed.map((s) => s.speaker).join(", ") }), true);
    resetSceneRowLater(row, ["partial"]);
  } else {
    setSceneRowState(row, "error", t("common.error"), "");
    const msg = (body && body.message) || t("scene.startFailed");
    const hint = (body && body.hint) ? " (" + body.hint + ")" : "";
    setHint(msg + hint, true);
    resetSceneRowLater(row, ["error"]);
  }
  refreshStatus();
  loadSpeakers();
  runStatusPollTick();
}

async function stopScene(sc, row) {
  setSceneRowState(row, "stopping", t("scene.stopping"), "");
  setHint("");
  const { ok, status, body } = await api(
    "/api/v1/scenes/" + encodeURIComponent(sc.id) + "/stop",
    { method: "POST" }
  );
  row.dataset.playbackPoll = "";
  if (status === 401) { setSceneRowState(row, "", "", ""); openTokenDialog(); return; }
  if (!ok) {
    setSceneRowState(row, "error", t("common.error"), "");
    const msg = (body && body.message) || t("scene.stopFailed");
    setHint(msg + (body && body.hint ? " (" + body.hint + ")" : ""), true);
    resetSceneRowLater(row, ["error"]);
    return;
  }
  setSceneRowState(row, "stopped", "", "");
  runStatusPollTick();
}

/**
 * Pause/Fortsetzen ueber die Gruppen-API (ADR 0015). Fortsetzen laedt die
 * Szene nicht neu; ohne geladene Quelle meldet die Box playback_failed.
 */
async function sceneTransport(sc, row, action) {
  const pausing = action === "pause";
  const prev = row.dataset.state;
  setSceneRowState(row, pausing ? "pausing" : "resuming", t(pausing ? "scene.pausing" : "scene.resuming"), "");
  setHint("");
  const { ok, status, body } = await api(
    "/api/v1/scenes/" + encodeURIComponent(sc.id) + "/" + action,
    { method: "POST" }
  );
  row.dataset.playbackPoll = "";
  if (status === 401) { setSceneRowState(row, prev, "", ""); openTokenDialog(); return; }
  if (!ok) {
    setSceneRowState(row, "error", t("common.error"), "");
    const fallback = t(pausing ? "scene.pauseFailed" : "scene.resumeFailed");
    const msg = (body && body.message) || fallback;
    setHint(msg + (body && body.hint ? " (" + body.hint + ")" : ""), true);
    resetSceneRowLater(row, ["error"]);
    return;
  }
  if (pausing) setSceneRowState(row, "paused", PLAYBACK_LABELS.paused, "");
  else setSceneRowState(row, "playing", PLAYBACK_LABELS.playing, "");
  runStatusPollTick();
}

function pauseScene(sc, row) {
  return sceneTransport(sc, row, "pause");
}

function resumeScene(sc, row) {
  return sceneTransport(sc, row, "resume");
}

// ---- ⋯-Sheet je Szene ----

let menuScene = null;

function openSceneMenu(sc) {
  if (!el.sceneMenuSheet) return;
  menuScene = sc;
  if (el.sceneMenuTitle) el.sceneMenuTitle.textContent = sc.name;
  updateSceneMenuPause();
  setSceneMenuHint("", false);
  renderSceneMenuVolumes(sc);
  el.sceneMenuSheet.showModal();
}

/** „Pausieren“/„Fortsetzen“ im ⋯-Sheet nur, wenn die Szene laeuft bzw. pausiert ist. */
function updateSceneMenuPause() {
  const btn = el.sceneMenuPause;
  if (!btn) return;
  const row = menuScene && sceneCards.get(menuScene.id);
  const state = row ? row.dataset.state : "";
  const paused = state === "paused";
  btn.hidden = !(paused || sceneIsRunning(state));
  if (btn.hidden) return;
  const icon = paused ? "play" : "pause";
  if (btn.dataset.icon !== icon) {
    btn.dataset.icon = icon;
    const svg = btn.querySelector("svg");
    const next = createLucideIcon(icon, "lucide-icon setting-icon");
    if (svg) svg.replaceWith(next);
    else btn.prepend(next);
  }
  const label = btn.querySelector(".btn-label");
  if (label) label.textContent = t(paused ? "action.resume" : "action.pause");
}

function closeSheetOnBackdrop(dlg) {
  // Tap auf die abgedunkelte Flaeche schliesst das Sheet (wie native Sheets).
  dlg.addEventListener("click", (e) => {
    if (e.target === dlg) dlg.close();
  });
}

for (const btn of document.querySelectorAll("[data-close-dialog]")) {
  btn.addEventListener("click", () => {
    const dlg = btn.closest("dialog");
    if (dlg) dlg.close();
  });
}
for (const dlg of document.querySelectorAll("dialog.sheet")) closeSheetOnBackdrop(dlg);

// ---- Lautstaerken im ⋯-Sheet (Spotify-Geraete-Sheet): Slider + −/+ je Raum ----
// Aenderungen werden in der Szene gespeichert (PUT config/scenes/{id}) und wirken
// bei laufender Szene sofort auf die Box (PUT speakers/{name}/volume).

const VOLUME_DEBOUNCE_MS = 250;
const SCENE_SAVE_DEBOUNCE_MS = 700;
const volumeTimers = new Map();
let sceneSaveTimer = 0;

function setVolumeRowError(row, text) {
  let err = row.querySelector(".vol-error");
  if (!text) {
    if (err) err.remove();
    return;
  }
  if (!err) {
    err = document.createElement("p");
    err.className = "field-hint error vol-error";
    row.append(err);
  }
  err.textContent = text;
}

function setSceneMenuHint(text, isError) {
  if (!el.sceneMenuHint) return;
  el.sceneMenuHint.textContent = text || "";
  el.sceneMenuHint.classList.toggle("error", !!isError);
}

async function sendSpeakerVolume(name, volume, row) {
  const { ok, status, body } = await api(
    "/api/v1/speakers/" + encodeSpeakerPath(name) + "/volume",
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ volume }),
    }
  );
  if (status === 401) { openTokenDialog(); return; }
  if (!ok) {
    const msg = (body && body.message) || t("volume.setFailed");
    setVolumeRowError(row, msg + (body && body.hint ? " (" + body.hint + ")" : ""));
    return;
  }
  setVolumeRowError(row, "");
}

function queueSpeakerVolume(name, volume, row) {
  if (volumeTimers.has(name)) clearTimeout(volumeTimers.get(name));
  volumeTimers.set(name, setTimeout(() => {
    volumeTimers.delete(name);
    sendSpeakerVolume(name, volume, row);
  }, VOLUME_DEBOUNCE_MS));
}

/** Szene mit geaenderten Lautstaerken speichern (Name/Quelle unveraendert). */
async function saveSceneVolumes(sc) {
  setSceneMenuHint(t("common.saving"), false);
  const { ok, status, body } = await api(
    "/api/v1/config/scenes/" + encodeURIComponent(sc.id),
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: sc.name, source: sc.source, members: sc.members }),
    }
  );
  if (status === 401) { openTokenDialog(); return; }
  if (!ok) {
    setSceneMenuHint((body && body.message) || t("volume.saveFailed"), true);
    return;
  }
  setSceneMenuHint(t("volume.saved"), false);
}

function queueSceneVolumeSave(sc) {
  if (sceneSaveTimer) clearTimeout(sceneSaveTimer);
  sceneSaveTimer = setTimeout(() => {
    sceneSaveTimer = 0;
    saveSceneVolumes(sc);
  }, SCENE_SAVE_DEBOUNCE_MS);
}

function renderSceneMenuVolumes(sc) {
  if (!el.sceneMenuVolumes) return;
  el.sceneMenuVolumes.replaceChildren();
  const row0 = sceneCards.get(sc.id);
  for (const m of sc.members || []) {
    if (!m.speaker) continue;
    const row = document.createElement("div");
    row.className = "vol-row";
    row.dataset.name = m.speaker;
    const name = document.createElement("div");
    name.className = "vol-name";
    name.textContent = m.speaker;
    const control = createVolumeControl(m.speaker, m.volume, (v) => {
      m.volume = v;
      // Live nur, wenn die Szene gerade laeuft -- sonst nicht in fremde Wiedergabe eingreifen.
      if (row0 && sceneIsRunning(row0.dataset.state)) queueSpeakerVolume(m.speaker, v, row);
      queueSceneVolumeSave(sc);
    });
    row.append(name, control);
    el.sceneMenuVolumes.append(row);
  }
}

if (el.sceneMenuPause) {
  el.sceneMenuPause.addEventListener("click", () => {
    const sc = menuScene;
    el.sceneMenuSheet.close();
    const row = sc && sceneCards.get(sc.id);
    if (sc && row) togglePauseScene(sc, row);
  });
}
if (el.sceneMenuEdit) {
  el.sceneMenuEdit.addEventListener("click", () => {
    const sc = menuScene;
    el.sceneMenuSheet.close();
    if (sc) openSceneEditDialog(sc);
  });
}

// ---- Zeitplaene ----

const DAY_ORDER = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"];
const DAY_LABELS = translated(Object.fromEntries(DAY_ORDER.map((d) => [d, "day." + d])));

let loadedSchedules = [];
let schedulesTimezone = "Europe/Vienna";
let scheduleEditId = null;
let pendingScheduleDelete = null;
let schedulesPollTimer = null;

function setScheduleHint(text, isError) {
  if (!el.scheduleHint) return;
  el.scheduleHint.textContent = text || "";
  el.scheduleHint.classList.toggle("error", !!isError);
}

/** Compact days: Mo–Fr, Sa, Su, daily. */
function formatScheduleDays(days) {
  const idx = DAY_ORDER.map((d) => (days || []).includes(d));
  if (idx.every(Boolean)) return t("schedule.daily");
  const parts = [];
  let i = 0;
  while (i < 7) {
    if (!idx[i]) { i++; continue; }
    let j = i;
    while (j + 1 < 7 && idx[j + 1]) j++;
    const a = DAY_LABELS[DAY_ORDER[i]];
    const b = DAY_LABELS[DAY_ORDER[j]];
    if (j - i >= 2) parts.push(a + "–" + b);
    else if (j === i) parts.push(a);
    else parts.push(a, b);
    i = j + 1;
  }
  return parts.join(", ");
}

function formatScheduleWhen(iso) {
  if (!iso) return "";
  try {
    return new Intl.DateTimeFormat(I18N.locale, {
      weekday: "short", day: "2-digit", month: "2-digit",
      hour: "2-digit", minute: "2-digit", timeZone: schedulesTimezone,
    }).format(new Date(iso));
  } catch (_) {
    return iso;
  }
}

function scheduleSceneName(st) {
  if (st.scene_name) return st.scene_name;
  const sc = loadedScenes.find((s) => s.id === st.scene);
  return sc ? sc.name : st.scene;
}

function scheduleNextText(st) {
  if (!st.enabled) return t("schedule.disabled");
  if (st.active && st.next_stop) return t("schedule.runningStop", { when: formatScheduleWhen(st.next_stop) });
  if (st.next_start) return t("schedule.nextStart") + ": " + formatScheduleWhen(st.next_start);
  return t("schedule.none");
}

function scheduleLastRunText(st) {
  const lr = st.last_run;
  if (!lr) return { text: "", error: false };
  if (!lr.ok && lr.error) return { text: t("schedule.error", { msg: lr.error }), error: true };
  if (lr.detail) return { text: lr.detail, error: lr.action === "start" || lr.action === "catch_up" };
  return { text: "", error: false };
}

function scheduleTimeText(st) {
  return st.start + (st.stop ? "–" + st.stop : "");
}

/**
 * Zeitplan-Zeile: Uhrzeit gross (Mono), darunter Szene + Tage; ganze Flaeche
 * oeffnet den Editor. Schalter rechts mit 48-px-Trefferflaeche.
 */
function renderScheduleRow(st) {
  const li = document.createElement("li");
  li.className = "schedule-row";
  li.dataset.id = st.id;
  li.dataset.active = st.active ? "true" : "false";
  li.classList.toggle("is-disabled", !st.enabled);

  const main = document.createElement("button");
  main.type = "button";
  main.className = "schedule-main";
  main.setAttribute("aria-label", t("schedule.editAria", { scene: scheduleSceneName(st), start: st.start }));
  const time = document.createElement("span");
  time.className = "schedule-time";
  time.textContent = scheduleTimeText(st);
  const when = document.createElement("span");
  when.className = "schedule-when";
  when.textContent = scheduleSceneName(st) + " · " + formatScheduleDays(st.days);
  const next = document.createElement("span");
  next.className = "schedule-next";
  next.textContent = scheduleNextText(st);
  main.append(time, when, next);
  const lr = scheduleLastRunText(st);
  if (lr.text) {
    const last = document.createElement("span");
    last.className = "schedule-last" + (lr.error ? " error" : "");
    last.textContent = lr.text;
    main.append(last);
  }
  main.addEventListener("click", () => openScheduleDialog(st));

  const hit = document.createElement("label");
  hit.className = "switch-hit";
  const toggle = document.createElement("input");
  toggle.type = "checkbox";
  toggle.setAttribute("role", "switch");
  toggle.className = "schedule-toggle";
  toggle.checked = !!st.enabled;
  toggle.setAttribute("aria-label", t("schedule.enabledAria", { scene: scheduleSceneName(st), start: st.start }));
  toggle.addEventListener("change", () => toggleSchedule(st, toggle));
  hit.append(toggle);

  li.append(main, hit);
  return li;
}

/** Oberste Zeile: naechster Start ueber alle aktiven Zeitplaene. */
function renderScheduleSummary(list) {
  if (!el.scheduleSummary) return;
  const upcoming = list
    .filter((st) => st.enabled && st.next_start)
    .sort((a, b) => new Date(a.next_start) - new Date(b.next_start))[0];
  el.scheduleSummary.hidden = !upcoming;
  if (!upcoming) return;
  const b = document.createElement("b");
  b.textContent = formatScheduleWhen(upcoming.next_start);
  el.scheduleSummary.replaceChildren(
    t("schedule.nextStart") + ": ", b, " · " + scheduleSceneName(upcoming)
  );
}

function renderSchedules(list) {
  if (!el.scheduleList) return;
  el.scheduleList.replaceChildren(...list.map(renderScheduleRow));
  renderScheduleSummary(list);
  if (list.length === 0 && !el.scheduleHint.classList.contains("error")) {
    setScheduleHint(t("schedule.empty"), false);
  } else if (!el.scheduleHint.classList.contains("error")) {
    setScheduleHint("");
  }
}

async function loadSchedules() {
  if (anyDialogOpen()) return;
  const { ok, status, body } = await api("/api/v1/schedules");
  if (status === 401) return;
  if (!ok) {
    setScheduleHint((body && body.message) || t("schedule.loadFailed"), true);
    return;
  }
  schedulesTimezone = (body && body.timezone) || schedulesTimezone;
  loadedSchedules = (body && body.schedules) || [];
  renderSchedules(loadedSchedules);
}

function startSchedulesPolling() {
  stopSchedulesPolling();
  schedulesPollTimer = setInterval(() => {
    if (document.hidden || anyDialogOpen()) return;
    loadSchedules();
  }, SCHEDULES_POLL_MS);
}

function stopSchedulesPolling() {
  if (schedulesPollTimer) {
    clearInterval(schedulesPollTimer);
    schedulesPollTimer = null;
  }
}

function schedulePayloadFrom(st, overrides = {}) {
  const p = {
    scene: st.scene,
    days: st.days || [],
    start: st.start,
    enabled: !!st.enabled,
  };
  if (st.stop) p.stop = st.stop;
  return Object.assign(p, overrides);
}

async function toggleSchedule(st, toggle) {
  toggle.disabled = true;
  setScheduleHint("");
  const { ok, status, body } = await api(
    "/api/v1/config/schedules/" + encodeURIComponent(st.id),
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(schedulePayloadFrom(st, { enabled: toggle.checked })),
    }
  );
  toggle.disabled = false;
  if (status === 401) { toggle.checked = !toggle.checked; openTokenDialog(); return; }
  if (!ok) {
    toggle.checked = !toggle.checked;
    setScheduleHint((body && body.message) || t("schedule.saveFailed"), true);
    return;
  }
  loadSchedules();
}

function setScheduleFormHint(text, isError) {
  if (!el.scheduleSaveHint) return;
  el.scheduleSaveHint.hidden = !text;
  el.scheduleSaveHint.textContent = text || "";
  el.scheduleSaveHint.classList.toggle("error", !!isError);
}

function setScheduleDays(days) {
  const set = new Set(days || []);
  for (const cb of document.querySelectorAll('input[name="scheduleDay"]')) {
    cb.checked = set.has(cb.value);
  }
}

function selectedScheduleDays() {
  return [...document.querySelectorAll('input[name="scheduleDay"]:checked')].map((cb) => cb.value);
}

function fillScheduleSceneSelect(selectedId) {
  const sel = el.scheduleScene;
  sel.replaceChildren();
  const scenes = [...loadedScenes].sort((a, b) => a.name.localeCompare(b.name, I18N.locale));
  for (const sc of scenes) {
    const opt = document.createElement("option");
    opt.value = sc.id;
    opt.textContent = sc.name;
    sel.append(opt);
  }
  if (selectedId) sel.value = selectedId;
}

function openScheduleDialog(st) {
  scheduleEditId = st ? st.id : null;
  el.scheduleDialogTitle.textContent = t(st ? "schedule.edit" : "schedule.new");
  if (el.scheduleTz) el.scheduleTz.textContent = schedulesTimezone;
  fillScheduleSceneSelect(st ? st.scene : "");
  setScheduleDays(st ? st.days : ["mon", "tue", "wed", "thu", "fri"]);
  el.scheduleStart.value = st ? st.start : "06:10";
  el.scheduleStop.value = st ? (st.stop || "") : "08:30";
  el.scheduleEnabled.checked = st ? !!st.enabled : true;
  if (el.scheduleDeleteBtn) el.scheduleDeleteBtn.hidden = !st;
  setScheduleFormHint(loadedScenes.length ? "" : t("schedule.needScene"), loadedScenes.length === 0);
  el.scheduleDialog.showModal();
}

async function saveScheduleFromDialog() {
  const days = selectedScheduleDays();
  const start = (el.scheduleStart.value || "").slice(0, 5);
  const stop = (el.scheduleStop.value || "").slice(0, 5);
  if (!el.scheduleScene.value) { setScheduleFormHint(t("schedule.chooseScene"), true); return; }
  if (days.length === 0) { setScheduleFormHint(t("schedule.needDay"), true); return; }
  if (!start) { setScheduleFormHint(t("schedule.startMissing"), true); return; }
  if (stop && stop === start) { setScheduleFormHint(t("schedule.stopEqualsStart"), true); return; }

  const sched = { scene: el.scheduleScene.value, days, start, enabled: el.scheduleEnabled.checked };
  if (stop) sched.stop = stop;

  let path = "/api/v1/config/schedules";
  let method = "POST";
  let payload = { schedule: sched };
  if (scheduleEditId) {
    path += "/" + encodeURIComponent(scheduleEditId);
    method = "PUT";
    payload = sched;
  }
  const btn = document.getElementById("scheduleSaveBtn");
  if (btn) btn.disabled = true;
  setScheduleFormHint(t("common.saving"), false);
  const { ok, status, body } = await api(path, {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (btn) btn.disabled = false;
  if (status === 401) { el.scheduleDialog.close(); openTokenDialog(); return; }
  if (!ok) {
    const msg = (body && body.message) || t("schedule.saveFailed");
    setScheduleFormHint(msg + (body && body.hint ? " (" + body.hint + ")" : ""), true);
    return;
  }
  scheduleEditId = null;
  el.scheduleDialog.close();
  setScheduleHint("");
  loadSchedules();
}

function openScheduleDeleteDialog(st) {
  pendingScheduleDelete = st;
  el.scheduleDeleteMessage.textContent =
    t("schedule.deleteConfirm", { scene: scheduleSceneName(st), days: formatScheduleDays(st.days), start: st.start });
  el.scheduleDeleteDialog.showModal();
}

async function deleteSchedule(st) {
  if (el.scheduleDialog.open) el.scheduleDialog.close();
  scheduleEditId = null;
  const { ok, status, body } = await api(
    "/api/v1/config/schedules/" + encodeURIComponent(st.id),
    { method: "DELETE" }
  );
  if (status === 401) { openTokenDialog(); return; }
  if (!ok) {
    setScheduleHint((body && body.message) || t("schedule.deleteFailed"), true);
    return;
  }
  setScheduleHint("");
  loadSchedules();
}

if (el.scheduleDeleteBtn) {
  el.scheduleDeleteBtn.addEventListener("click", () => {
    const st = loadedSchedules.find((x) => x.id === scheduleEditId);
    if (st) openScheduleDeleteDialog(st);
  });
}
if (el.scheduleForm) {
  el.scheduleForm.addEventListener("submit", (e) => {
    e.preventDefault();
    saveScheduleFromDialog();
  });
}
if (el.scheduleCancelBtn) {
  el.scheduleCancelBtn.addEventListener("click", () => {
    scheduleEditId = null;
    el.scheduleDialog.close();
  });
}
for (const btn of document.querySelectorAll(".day-preset")) {
  btn.addEventListener("click", () => setScheduleDays((btn.dataset.days || "").split(",")));
}
if (el.scheduleDeleteForm) {
  el.scheduleDeleteForm.addEventListener("submit", (e) => {
    e.preventDefault();
    const action = e.submitter && e.submitter.value;
    el.scheduleDeleteDialog.close();
    if (action === "delete" && pendingScheduleDelete) deleteSchedule(pendingScheduleDelete);
    pendingScheduleDelete = null;
  });
}

function openTokenDialog() {
  el.tokenInput.value = getToken();
  el.dialog.showModal();
  el.tokenInput.focus();
}

el.tokenForm.addEventListener("submit", (e) => {
  if (e.submitter && e.submitter.value === "save") {
    const token = el.tokenInput.value.trim();
    if (!tokenHeaderSafe(token)) {
      setHint(t("token.latin1Only"), true);
      return;
    }
    localStorage.setItem(TOKEN_KEY, token);
    setHint("");
    loadScenes();
  }
});
el.tokenBtn.addEventListener("click", openTokenDialog);

if (el.sceneName) {
  el.sceneName.addEventListener("input", () => {
    if (sceneIdManual || !el.sceneId) return;
    el.sceneId.value = slugFromName(el.sceneName.value);
  });
}
if (el.sceneId) {
  el.sceneId.addEventListener("input", () => {
    sceneIdManual = el.sceneId.value.trim().length > 0;
  });
}
for (const radio of document.querySelectorAll('input[name="sourceMode"]')) {
  radio.addEventListener("change", updateSourceModeUI);
}
if (el.sourceBoxSelect) {
  el.sourceBoxSelect.addEventListener("change", () => {
    if (el.sourceBoxSelect.value) loadStreamFromBox(el.sourceBoxSelect.value);
  });
}
if (el.sourceFavoriteSelect) {
  el.sourceFavoriteSelect.addEventListener("change", applyFavoritePreview);
}
if (el.sourceFavoriteSpeakerSelect) {
  el.sourceFavoriteSpeakerSelect.addEventListener("change", () => {
    favoriteProxySpeaker = el.sourceFavoriteSpeakerSelect.value || "";
    favoritesLoadedAt = 0;
    ensureSonosFavorites({ force: true, speaker: favoriteProxySpeaker });
  });
}
if (el.sourceFavoriteBtn) {
  el.sourceFavoriteBtn.addEventListener("click", () => {
    const fav = document.querySelector('input[name="sourceMode"][value="favorite"]');
    const radio = document.querySelector('input[name="sourceMode"][value="radio"]');
    const target = fav && fav.checked ? radio : fav;
    if (target) target.checked = true;
    updateSourceModeUI();
  });
}
if (el.radioSearchBtn) {
  el.radioSearchBtn.addEventListener("click", () => searchRadioStations());
}
if (el.radioSearchInput) {
  el.radioSearchInput.addEventListener("input", scheduleRadioSearch);
  el.radioSearchInput.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      searchRadioStations();
    }
  });
}
if (el.sceneForm) {
  el.sceneForm.addEventListener("submit", (e) => {
    e.preventDefault();
    saveSceneToConfig();
  });
}
if (el.sceneDeleteBtn) {
  el.sceneDeleteBtn.addEventListener("click", () => {
    const sc = sceneEditId && sceneById.get(sceneEditId);
    if (sc) openSceneDeleteDialog(sc);
  });
}
if (el.sceneCancelBtn) {
  el.sceneCancelBtn.addEventListener("click", () => {
    sceneEditId = null;
    setSceneIdFieldMode(false);
    if (el.sceneDialog) el.sceneDialog.close();
  });
}
function openNewSceneDialog() {
  const needsLoad = discoveredSpeakers.length === 0;
  openSceneDialog({ loadingSpeakers: needsLoad });
  loadSpeakers().then(() => refreshOpenSceneDialog());
}

if (el.addBtn) {
  el.addBtn.addEventListener("click", () => {
    if (currentTab === "zeitplaene") openScheduleDialog(null);
    else openNewSceneDialog();
  });
}
if (el.saveSpeakersConfigBtn) {
  el.saveSpeakersConfigBtn.addEventListener("click", saveSpeakersToConfig);
}
if (el.refreshSpeakersBtn) {
  el.refreshSpeakersBtn.addEventListener("click", () => {
    if (speakersLoading) return;
    refreshSpeakersAndStatus();
  });
}
if (el.discoverClose) {
  el.discoverClose.addEventListener("click", exitDiscoverMode);
}
if (el.themeToggle) {
  el.themeToggle.addEventListener("click", toggleTheme);
}

// First render only after the dictionaries are loaded (i18n.js); until then
// the page shows the English static texts from index.html.
I18N.ready.then(() => {
  initTheme();
  initRouting();

  refreshStatus();
  loadScenes();
  setInterval(refreshStatus, 15000);
  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) refreshStatus();
  });
});
