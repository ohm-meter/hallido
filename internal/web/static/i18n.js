"use strict";

// Web app texts (ADR 0019): loads the dictionaries from locales/<lang>.json.
// English is the default and the fallback; terms follow
// docs/reference/glossary.md. Loaded before app.js; app.js waits for
// I18N.ready before its first render.
//
// Dictionary format: flat JSON object "key" -> "text", keys sorted. Plurals
// are two keys "key.one" / "key.other" (tn("key", n) picks one and fills {n}).
// Placeholders: {name} in the text, filled by t("key", { name: "…" }).
//
// Static markup: data-i18n (textContent), data-i18n-aria (aria-label),
// data-i18n-title (title), data-i18n-placeholder (placeholder).
// How to add a language: docs/how-to/add-a-language.md.
(function () {
  // Registered languages: the code is the file name locales/<code>.json and
  // matches the start of navigator.language ("de-AT" -> "de").
  const LANGUAGES = ["en", "de"];
  const FALLBACK = "en";
  // Locale for Intl (dates, sorting) per language; English keeps the browser's
  // regional variant (en-GB, en-US, …).
  const LOCALES = { de: "de-AT" };

  const browserLang = String(navigator.language || "").toLowerCase();
  const lang = LANGUAGES.find((code) => code !== FALLBACK && browserLang.startsWith(code)) || FALLBACK;
  const locale = LOCALES[lang] || (browserLang.startsWith("en") ? navigator.language : "en-US");

  // Same cache-busting version as the other assets (set by internal/web/web.go);
  // the service worker keeps exactly these URLs offline.
  const versionMeta = document.querySelector('meta[name="app-version"]');
  const version = versionMeta ? versionMeta.content : "";

  const dict = {};
  let strings = {};
  let fallback = {};
  const warned = new Set();
  const has = (obj, key) => Object.prototype.hasOwnProperty.call(obj, key);

  function localeURL(code) {
    return "locales/" + code + ".json" + (version ? "?v=" + encodeURIComponent(version) : "");
  }

  async function load(code) {
    try {
      const res = await fetch(localeURL(code), { credentials: "same-origin" });
      if (!res.ok) throw new Error("HTTP " + res.status);
      dict[code] = await res.json();
    } catch (err) {
      console.warn("i18n: locales/" + code + ".json could not be loaded:", err);
      dict[code] = {};
    }
    return dict[code];
  }

  /** Text for key with {placeholder} replacement; falls back to English, then the key. */
  function t(key, params) {
    let s = has(strings, key) ? strings[key] : has(fallback, key) ? fallback[key] : null;
    if (s == null) {
      if (!warned.has(key)) {
        warned.add(key);
        console.warn("i18n: missing key " + key);
      }
      return key;
    }
    if (params) {
      s = s.replace(/\{(\w+)\}/g, (m, name) => (has(params, name) ? String(params[name]) : m));
    }
    return s;
  }

  /** Plural: key.one for n === 1, else key.other; {n} is filled in. */
  function tn(key, n, params) {
    return t(key + (n === 1 ? ".one" : ".other"), Object.assign({ n }, params));
  }

  /** Translate static markup below root (data-i18n* attributes). */
  function apply(root) {
    const scope = root || document;
    for (const node of scope.querySelectorAll("[data-i18n]")) node.textContent = t(node.dataset.i18n);
    for (const node of scope.querySelectorAll("[data-i18n-aria]")) node.setAttribute("aria-label", t(node.dataset.i18nAria));
    for (const node of scope.querySelectorAll("[data-i18n-title]")) node.setAttribute("title", t(node.dataset.i18nTitle));
    for (const node of scope.querySelectorAll("[data-i18n-placeholder]")) node.setAttribute("placeholder", t(node.dataset.i18nPlaceholder));
  }

  // Never rejects: a missing file leaves the English static texts / keys visible.
  const ready = Promise.all([load(lang), lang === FALLBACK ? null : load(FALLBACK)]).then(() => {
    strings = dict[lang];
    fallback = dict[FALLBACK];
    apply(document);
  });

  document.documentElement.lang = lang;
  window.I18N = { lang, locale, languages: LANGUAGES, t, tn, apply, dict, ready };
  window.t = t;
  window.tn = tn;
})();
