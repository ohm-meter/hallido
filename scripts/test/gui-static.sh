#!/usr/bin/env bash
# Static web app checks without a browser (HTML/JS/CSS consistency).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
STATIC="${ROOT}/internal/web/static"

_PASS=0
_FAIL=0

red() { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }

assert_file_contains() {
  local name="$1" file="$2" pattern="$3"
  if grep -qE -- "$pattern" "$file"; then
    green "PASS  ${name}"
    _PASS=$((_PASS + 1))
  else
    red "FAIL  ${name} (pattern missing in ${file})"
    _FAIL=$((_FAIL + 1))
  fi
}

assert_file_exists() {
  local name="$1" file="$2"
  if [[ -f "$file" ]]; then
    green "PASS  ${name}"
    _PASS=$((_PASS + 1))
  else
    red "FAIL  ${name} (file missing: ${file})"
    _FAIL=$((_FAIL + 1))
  fi
}

echo "=== GUI-Static ==="
echo ""

assert_file_exists "index.html vorhanden" "${STATIC}/index.html"
assert_file_exists "app.js vorhanden" "${STATIC}/app.js"
assert_file_exists "style.css vorhanden" "${STATIC}/style.css"

assert_file_contains "index.html laedt style.css" "${STATIC}/index.html" 'href="style\.css"'
assert_file_contains "index.html laedt app.js" "${STATIC}/index.html" 'src="app\.js"'
assert_file_contains "index.html laedt icons.js" "${STATIC}/index.html" 'src="icons\.js"'

# i18n (ADR 0019): English default, German via navigator.language
assert_file_exists "i18n.js vorhanden" "${STATIC}/i18n.js"
assert_file_contains "index.html laedt i18n.js" "${STATIC}/index.html" 'src="i18n\.js"'
assert_file_contains "index.html lang en" "${STATIC}/index.html" '<html lang="en">'
assert_file_exists "locales/en.json vorhanden" "${STATIC}/locales/en.json"
assert_file_exists "locales/de.json vorhanden" "${STATIC}/locales/de.json"
assert_file_contains "i18n.js laedt locales/<lang>.json" "${STATIC}/i18n.js" '"locales/" \+ code \+ "\.json"'
assert_file_contains "i18n.js languages en+de" "${STATIC}/i18n.js" 'const LANGUAGES = \["en", "de"\]'
assert_file_contains "i18n.js I18N.ready" "${STATIC}/i18n.js" 'ready'
assert_file_contains "app.js wartet auf I18N.ready" "${STATIC}/app.js" 'I18N\.ready\.then'
assert_file_contains "web.go embeds locales" "${ROOT}/internal/web/web.go" 'go:embed .*static/locales/\*\.json'
assert_file_contains "i18n.js t()" "${STATIC}/i18n.js" 'function t\(key, params\)'
assert_file_contains "index.html data-i18n" "${STATIC}/index.html" 'data-i18n="tab\.scenes"'
assert_file_contains "web.go embeds i18n.js" "${ROOT}/internal/web/web.go" 'go:embed .*static/i18n\.js'
assert_file_contains "web.go versions i18n.js" "${ROOT}/internal/web/web.go" 'versionedAssets = .*"i18n\.js"'
if awk '/src="i18n\.js"/{i=NR} /src="app\.js"/{a=NR} END{exit !(i && a && i < a)}' "${STATIC}/index.html"; then
  green "PASS  index.html laedt i18n.js vor app.js"
  _PASS=$((_PASS + 1))
else
  red "FAIL  index.html laedt i18n.js vor app.js"
  _FAIL=$((_FAIL + 1))
fi
assert_file_exists "icons.js vorhanden" "${STATIC}/icons.js"
assert_file_exists "lucide pencil.svg" "${STATIC}/vendor/lucide/pencil.svg"
assert_file_contains "icons.js createLucideIcon" "${STATIC}/icons.js" 'function createLucideIcon'
assert_file_contains "style.css lucide-icon" "${STATIC}/style.css" '\.lucide-icon'
assert_file_contains "style.css icon tokens" "${STATIC}/style.css" 'icon-theme: 20px'
assert_file_contains "style.css icon scene token" "${STATIC}/style.css" 'icon-scene-action: 16px'

for id in statusDot grid tokenDialog refreshSpeakersBtn tokenBtn statusText speakerList discoverPanel discoverClose addBtn viewTitle themeToggle saveSpeakersConfigBtn sceneDialog sceneDialogTitle sceneForm sceneName sceneId sceneCancelBtn sceneDeleteDialog sceneDeleteForm sceneDeleteMessage; do
  assert_file_contains "index.html #${id}" "${STATIC}/index.html" "id=\"${id}\""
done

assert_file_contains "app.js STATUS_POLL_MS" "${STATIC}/app.js" 'STATUS_POLL_MS'
assert_file_contains "app.js tokenHeaderSafe" "${STATIC}/app.js" 'function tokenHeaderSafe'
assert_file_contains "app.js pollSceneStatus" "${STATIC}/app.js" 'pollSceneStatus'
assert_file_contains "app.js refreshSpeakersBtn" "${STATIC}/app.js" 'refreshSpeakersBtn'
assert_file_contains "app.js hash routing" "${STATIC}/app.js" 'hashchange'
assert_file_contains "app.js tabs" "${STATIC}/app.js" 'function showTab'
assert_file_contains "index.html tabbar" "${STATIC}/index.html" 'class="tabbar"'
assert_file_contains "index.html tab szenen" "${STATIC}/index.html" 'href="#szenen"'
assert_file_contains "index.html tab zeitplaene" "${STATIC}/index.html" 'href="#zeitplaene"'
assert_file_contains "index.html tab boxen" "${STATIC}/index.html" 'href="#boxen"'
assert_file_contains "style.css tap tokens" "${STATIC}/style.css" '--tap-min: 48px'
assert_file_contains "style.css primary tap token" "${STATIC}/style.css" '--tap-primary: 56px'
assert_file_contains "style.css tap gap token" "${STATIC}/style.css" '--tap-gap: 8px'
assert_file_contains "style.css Handy-Spalte" "${STATIC}/style.css" '--col-w: 460px'
assert_file_contains "index.html Powered" "${STATIC}/index.html" '<p class="brand-text">Hallido <em>Powered by siraris</em></p>'
assert_file_contains "index.html title Hallido" "${STATIC}/index.html" '<title>Hallido — Powered by siraris</title>'
assert_file_contains "manifest name Hallido" "${STATIC}/manifest.webmanifest" '"name": "Hallido"'
# old product names (pattern written so the release scan does not flag this line)
if grep -rqE "O[l]i\b|O[l]i's|SONOS Agent|Sonos Agent" "${STATIC}" --include='*.html' --include='*.js' --include='*.json' --include='*.webmanifest' --include='*.svg' --include='*.css' --exclude-dir=vendor; then
  red "FAIL  old product name in the web app (use Hallido)"
  _FAIL=$((_FAIL + 1))
else
  green "PASS  kein alter Produktname in der Web-App"
  _PASS=$((_PASS + 1))
fi
assert_file_contains "app.js discover mode" "${STATIC}/app.js" 'enterDiscoverMode'
assert_file_contains "app.js scene builder" "${STATIC}/app.js" 'saveSceneToConfig'
assert_file_contains "app.js stream filter" "${STATIC}/app.js" 'isIntegratableStreamURI'
assert_file_contains "app.js stream-sources API" "${STATIC}/app.js" 'speakers/stream-sources'
assert_file_contains "app.js sonos favorites API" "${STATIC}/app.js" 'sonos/favorites'
assert_file_contains "app.js radio search API" "${STATIC}/app.js" 'radio/search'
assert_file_contains "index.html radio search" "${STATIC}/index.html" 'Search radio'
assert_file_contains "app.js from-favorite API" "${STATIC}/app.js" 'scenes/from-favorite'
assert_file_contains "index.html favorite source" "${STATIC}/index.html" 'value="favorite"'
assert_file_contains "app.js syncSpeakersDOM" "${STATIC}/app.js" 'function syncSpeakersDOM'
assert_file_contains "app.js runStatusPollTick" "${STATIC}/app.js" 'runStatusPollTick'
assert_file_contains "index.html source segment" "${STATIC}/index.html" 'class="seg"'
assert_file_contains "index.html radio default" "${STATIC}/index.html" 'value="radio" checked'
assert_file_contains "index.html slug advanced" "${STATIC}/index.html" 'class="advanced"'
assert_file_contains "index.html save and play" "${STATIC}/index.html" 'Save &amp; play'
assert_file_contains "app.js volume slider" "${STATIC}/app.js" 'function createVolumeControl'
assert_file_contains "app.js speakers sync" "${STATIC}/app.js" 'saveSpeakersToConfig'
assert_file_contains "index.html views" "${STATIC}/index.html" 'class="views"'
assert_file_contains "app.js scene delete" "${STATIC}/app.js" 'config/scenes/'
assert_file_contains "app.js scene edit" "${STATIC}/app.js" 'openSceneEditDialog'
assert_file_contains "app.js scene put" "${STATIC}/app.js" 'method = "PUT"'
assert_file_contains "style.css scene row" "${STATIC}/style.css" '\.scene-row'
assert_file_contains "style.css play button" "${STATIC}/style.css" '\.scene-play'
assert_file_contains "app.js scene stop API" "${STATIC}/app.js" '"/stop"'
assert_file_contains "index.html scene menu sheet" "${STATIC}/index.html" 'id="sceneMenuSheet"'
assert_file_contains "index.html delete in editor" "${STATIC}/index.html" 'id="sceneDeleteBtn"'
assert_file_contains "index.html now playing" "${STATIC}/index.html" 'id="nowPlaying"'
assert_file_contains "app.js now playing bar" "${STATIC}/app.js" 'function updateNowPlayingBar'
assert_file_contains "style.css bottom sheet" "${STATIC}/style.css" 'dialog\.sheet'
assert_file_contains "index.html schedule sheet" "${STATIC}/index.html" 'id="scheduleDialog" class="sheet'
assert_file_contains "index.html schedule delete in editor" "${STATIC}/index.html" 'id="scheduleDeleteBtn"'
assert_file_contains "style.css switch hit area" "${STATIC}/style.css" '\.switch-hit'
assert_file_contains "index.html volumes in scene menu" "${STATIC}/index.html" 'id="sceneMenuVolumes"'
assert_file_contains "app.js volume API" "${STATIC}/app.js" '/volume"'
assert_file_contains "app.js delete dialog" "${STATIC}/app.js" 'sceneDeleteDialog'
assert_file_contains "index.html save speakers label" "${STATIC}/index.html" 'Save speakers to config'
assert_file_contains "index.html refresh speakers label" "${STATIC}/index.html" '>Refresh<'
assert_file_contains "index.html save speakers aria" "${STATIC}/index.html" 'aria-label="Save found speakers to config.json"'
assert_file_exists "manifest vorhanden" "${STATIC}/manifest.webmanifest"
assert_file_exists "Service Worker vorhanden" "${STATIC}/sw.js"
assert_file_exists "pwa.js vorhanden" "${STATIC}/pwa.js"
for icon in icon.svg icon-maskable.svg icon-192.png icon-512.png icon-maskable-192.png icon-maskable-512.png apple-touch-icon.png; do
  assert_file_exists "Icon ${icon}" "${STATIC}/icons/${icon}"
done
assert_file_contains "index.html manifest mit Credentials" "${STATIC}/index.html" 'rel="manifest" href="manifest\.webmanifest" crossorigin="use-credentials"'
assert_file_contains "index.html laedt pwa.js" "${STATIC}/index.html" 'src="pwa\.js"'
assert_file_contains "index.html apple-touch-icon" "${STATIC}/index.html" 'rel="apple-touch-icon"'
assert_file_contains "index.html theme-color dunkel" "${STATIC}/index.html" 'name="theme-color" content="#0d0c0a" media="\(prefers-color-scheme: dark\)"'
assert_file_contains "sw.js laesst /api/ aus" "${STATIC}/sw.js" 'startsWith\("/api/"\)\) return'
assert_file_contains "style.css responsive bar" "${STATIC}/style.css" '@media \(max-width: 479px\)'

# i18n: locale format/parity and no hard-coded visible text in scripts
if node "${ROOT}/scripts/test/i18n-lint.js"; then
  green "PASS  i18n-lint (locales, keine Literal-Texte in app.js)"
  _PASS=$((_PASS + 1))
else
  red "FAIL  i18n-lint"
  _FAIL=$((_FAIL + 1))
fi

echo ""
echo "Result: ${_PASS} passed, ${_FAIL} failed"
if (( _FAIL > 0 )); then
  exit 1
fi
