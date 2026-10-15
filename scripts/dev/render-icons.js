// Rendert die App-Icons (PNG) aus den SVG-Quellen unter internal/web/static/icons/.
// Nur bei geaendertem Icon noetig; die PNGs sind eingecheckt (kein Build-Schritt).
// Aufruf im Repo-Root: node scripts/dev/render-icons.js
const fs = require("fs");
const path = require("path");
const { chromium } = require("@playwright/test");

const DIR = path.join(__dirname, "../../internal/web/static/icons");
const JOBS = [
  ["icon.svg", "icon-192.png", 192],
  ["icon.svg", "icon-512.png", 512],
  ["icon-maskable.svg", "icon-maskable-192.png", 192],
  ["icon-maskable.svg", "icon-maskable-512.png", 512],
  // iOS rundet selbst ab und kennt keine Transparenz -> randlose Variante
  ["icon-maskable.svg", "apple-touch-icon.png", 180],
];

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage();
  for (const [src, out, size] of JOBS) {
    const svg = fs.readFileSync(path.join(DIR, src), "utf8");
    await page.setViewportSize({ width: size, height: size });
    await page.setContent(
      `<html><body style="margin:0;background:transparent">` +
        svg.replace("<svg ", `<svg width="${size}" height="${size}" style="display:block" `) +
        `</body></html>`
    );
    await page.screenshot({ path: path.join(DIR, out), omitBackground: true });
    console.log(out, size);
  }
  await browser.close();
})();
