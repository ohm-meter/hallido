#!/usr/bin/env node
// Static i18n check for the web app (ADR 0019, docs/styleguide.md
// "Internationalization"). Called by scripts/test/gui-static.sh.
//
// 1. locales/*.json: flat key -> non-empty string, same keys as en.json,
//    keys sorted, 2-space indent.
// 2. app.js: no visible text as a literal — user-facing texts come from t()/tn().
//    - no string literal with German umlauts/ß,
//    - no literal words assigned to textContent / innerText / title /
//      placeholder / ariaLabel, or set via setAttribute("aria-label" | "title" |
//      "placeholder", "…").
//    Pragmatic: comments are stripped, literals without letters ("—", "…",
//    "", "+") pass; ALLOWED_LITERALS lists the few intended exceptions.
"use strict";

const fs = require("fs");
const path = require("path");

const STATIC = path.join(__dirname, "..", "..", "internal", "web", "static");
const SCRIPTS = ["app.js", "setup.js", "pwa.js", "icons.js"];
// Literal values that may be assigned although they contain letters
// (brand names, technical tokens). Keep this list short.
const ALLOWED_LITERALS = new Set(["Hallido", "URL", "API"]);

const errors = [];

// ---- 1. Locale files ----
const localeDir = path.join(STATIC, "locales");
const files = fs.readdirSync(localeDir).filter((f) => f.endsWith(".json")).sort();
const dicts = {};
for (const file of files) {
  const raw = fs.readFileSync(path.join(localeDir, file), "utf8");
  let d;
  try {
    d = JSON.parse(raw);
  } catch (err) {
    errors.push(`locales/${file}: invalid JSON (${err.message})`);
    continue;
  }
  if (!d || typeof d !== "object" || Array.isArray(d)) {
    errors.push(`locales/${file}: must be a flat object`);
    continue;
  }
  for (const [k, v] of Object.entries(d)) {
    if (typeof v !== "string" || v.trim() === "") errors.push(`locales/${file}: "${k}" must be a non-empty string`);
  }
  const canonical = JSON.stringify(Object.fromEntries(Object.keys(d).sort().map((k) => [k, d[k]])), null, 2) + "\n";
  if (raw !== canonical) errors.push(`locales/${file}: keys not sorted or not 2-space indented (format: JSON.stringify(sorted, null, 2) + newline)`);
  dicts[file.replace(/\.json$/, "")] = d;
}
if (!dicts.en) errors.push("locales/en.json missing");
for (const [lang, d] of Object.entries(dicts)) {
  if (lang === "en" || !dicts.en) continue;
  for (const k of Object.keys(dicts.en)) if (!(k in d)) errors.push(`locales/${lang}.json: key "${k}" missing`);
  for (const k of Object.keys(d)) if (!(k in dicts.en)) errors.push(`locales/${lang}.json: key "${k}" not in en.json`);
  for (const [k, v] of Object.entries(dicts.en)) {
    const want = (v.match(/\{\w+\}/g) || []).sort().join(",");
    const got = (String(d[k] || "").match(/\{\w+\}/g) || []).sort().join(",");
    if (k in d && want !== got) errors.push(`locales/${lang}.json: "${k}" placeholders ${got || "-"} differ from en ${want || "-"}`);
  }
}

// ---- 2. Scripts ----

/**
 * Source with comments removed (strings and regex literals kept); string
 * literals found on the way are pushed to `strings` as { text, index }.
 */
function stripComments(src, strings = []) {
  let out = "";
  let i = 0;
  let lastSig = ""; // last significant (non-space) character outside comments
  while (i < src.length) {
    const c = src[i];
    const n = src[i + 1];
    if (c === "/" && n === "/") {
      while (i < src.length && src[i] !== "\n") i++;
      continue;
    }
    if (c === "/" && n === "*") {
      const end = src.indexOf("*/", i + 2);
      const stop = end < 0 ? src.length : end + 2;
      out += src.slice(i, stop).replace(/[^\n]/g, " ");
      i = stop;
      continue;
    }
    if (c === '"' || c === "'" || c === "`") {
      let j = i + 1;
      while (j < src.length && src[j] !== c) j += src[j] === "\\" ? 2 : 1;
      strings.push({ text: src.slice(i, j + 1), index: out.length });
      out += src.slice(i, j + 1);
      i = j + 1;
      lastSig = c;
      continue;
    }
    if (c === "/" && (lastSig === "" || "(,=:[!&|?{};+".includes(lastSig))) {
      // regex literal
      let j = i + 1;
      let inClass = false;
      while (j < src.length && (inClass || src[j] !== "/") && src[j] !== "\n") {
        if (src[j] === "\\") j++;
        else if (src[j] === "[") inClass = true;
        else if (src[j] === "]") inClass = false;
        j++;
      }
      out += src.slice(i, j + 1);
      i = j + 1;
      lastSig = "/";
      continue;
    }
    out += c;
    if (!/\s/.test(c)) lastSig = c;
    i++;
  }
  return out;
}

/** String literal regex; g = number of its quote group in the full pattern. */
const literal = (g) => "([\"'`])((?:\\\\.|(?!\\" + g + ")[^\\\\])*)\\" + g;
const hasWords = (s) => /[A-Za-zÄÖÜäöüß]{2,}/.test(s.replace(/\$\{[^}]*\}/g, ""));
const lineOf = (src, idx) => src.slice(0, idx).split("\n").length;

for (const name of SCRIPTS) {
  const file = path.join(STATIC, name);
  if (!fs.existsSync(file)) continue;
  const strings = [];
  const code = stripComments(fs.readFileSync(file, "utf8"), strings);

  for (const lit of strings) {
    if (/[äöüÄÖÜß]/.test(lit.text)) errors.push(`${name}:${lineOf(code, lit.index)}: German text in string literal ${lit.text} — use t()`);
  }
  const assign = new RegExp(String.raw`\.(textContent|innerText|title|placeholder|ariaLabel)\s*=\s*` + literal(2), "g");
  for (const m of code.matchAll(assign)) {
    if (hasWords(m[3]) && !ALLOWED_LITERALS.has(m[3])) {
      errors.push(`${name}:${lineOf(code, m.index)}: literal text assigned to .${m[1]}: ${m[2]}${m[3]}${m[2]} — use t()`);
    }
  }
  const setAttr = new RegExp(String.raw`setAttribute\(\s*["'](aria-label|title|placeholder)["']\s*,\s*` + literal(2), "g");
  for (const m of code.matchAll(setAttr)) {
    if (hasWords(m[3]) && !ALLOWED_LITERALS.has(m[3])) {
      errors.push(`${name}:${lineOf(code, m.index)}: literal text in setAttribute("${m[1]}"): ${m[2]}${m[3]}${m[2]} — use t()`);
    }
  }
}

if (errors.length) {
  for (const e of errors) console.error("FAIL  " + e);
  process.exit(1);
}
console.log(`i18n-lint: ${files.length} locale files, ${SCRIPTS.length} scripts ok`);
