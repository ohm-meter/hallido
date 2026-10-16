// Package web liefert die statische Bedienoberflaeche aus. Die Dateien sind
// per go:embed ins Binary eingebettet -- ein einziges Artefakt fuer den Pi,
// keine separaten Asset-Pfade im Betrieb.
package web

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Nur Laufzeit-Assets einbetten: vendor/pico.css (unminifiziert) und
// vendor/lucide/*.svg sind Referenz und blaehen das Binary nur auf.
//
//go:embed static/index.html static/app.js static/setup.js static/i18n.js static/icons.js static/pwa.js static/style.css static/vendor/pico.min.css
//go:embed static/manifest.webmanifest static/sw.js static/icons
//go:embed static/locales/*.json
var staticFS embed.FS

// versionedAssets werden in index.html mit ?v=<Inhalts-Hash> referenziert.
var versionedAssets = []string{"vendor/pico.min.css", "style.css", "icons.js", "i18n.js", "setup.js", "app.js", "pwa.js"}

// shellFiles haelt der Service Worker zusaetzlich zur Seite offline vor
// (unversioniert, daher "Netzwerk zuerst").
var shellFiles = []string{
	"manifest.webmanifest",
	"icons/icon.svg",
	"icons/icon-192.png",
	"icons/icon-512.png",
	"icons/apple-touch-icon.png",
}

// localeDir holds the web app dictionaries (locales/<lang>.json, ADR 0019).
// i18n.js requests them as locales/<lang>.json?v=<app version> (read from
// <meta name="app-version">); the service worker keeps every file offline.
const localeDir = "locales"

// versionPlaceholder steht in index.html und wird durch die App-Version ersetzt.
const versionPlaceholder = "__APP_VERSION__"

// Handler liefert die GUI. "/" -> index.html, "/app.js"/"/style.css" -> Assets.
//
// Caching: Eingebettete Dateien haben keine Aenderungszeit, ohne eigene Header
// setzen Proxys (Cloudflare: 4 h) oder Browser beliebige Cache-Dauern -- nach
// einem Update mischt sich dann neues HTML mit altem CSS/JS. Daher:
//   - je Datei ein ETag aus dem Inhalt plus "no-cache" (Client fragt nach, 304)
//   - index.html verweist auf Assets mit ?v=<Hash>; selbst wenn ein Proxy die
//     Browser-Cache-Dauer erzwingt, laedt eine neue Version neue URLs.
func Handler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err) // kann nur bei Build-Fehler passieren
	}
	etags := contentETags(sub)
	version := appVersion(etags)
	index, err := versionedIndex(sub, etags, version)
	if err != nil {
		panic(err) // index.html fehlt: Build-Fehler
	}
	sw, err := serviceWorker(sub, etags, version, localeURLs(sub, version))
	if err != nil {
		panic(err) // sw.js fehlt: Build-Fehler
	}
	generated := map[string][]byte{"/": index, "/sw.js": sw}
	generatedTags := map[string]string{"/": etagOf(index), "/sw.js": etagOf(sw)}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := generated[r.URL.Path]; ok {
			name := "index.html"
			if r.URL.Path == "/sw.js" {
				name = "sw.js"
			}
			w.Header().Set("ETag", generatedTags[r.URL.Path])
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if tag, ok := etags[path]; ok {
			w.Header().Set("ETag", tag)
			w.Header().Set("Cache-Control", "no-cache")
		}
		if strings.HasSuffix(path, ".webmanifest") {
			// Go kennt die Endung nicht; Browser erwarten diesen Typ.
			w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
		}
		// http.FileServer beantwortet If-None-Match anhand des gesetzten ETag (304).
		files.ServeHTTP(w, r)
	})
}

// versionedIndex haengt an jede Asset-Referenz in index.html ?v=<Hash> an und
// setzt die App-Version ein (Vergleich mit dem aktiven Service Worker).
func versionedIndex(fsys fs.FS, etags map[string]string, version string) ([]byte, error) {
	html, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return nil, err
	}
	out := strings.ReplaceAll(string(html), versionPlaceholder, version)
	for _, name := range versionedAssets {
		out = strings.ReplaceAll(out, `"`+name+`"`, `"`+versionedURL(name, etags)+`"`)
	}
	return []byte(out), nil
}

// serviceWorker setzt Version und App-Shell-Liste in sw.js ein. Jede Aenderung
// an einer eingebetteten Datei aendert damit auch sw.js -- der Browser erkennt
// beim naechsten Aufruf den neuen Service Worker.
func serviceWorker(fsys fs.FS, etags map[string]string, version string, extra []string) ([]byte, error) {
	src, err := fs.ReadFile(fsys, "sw.js")
	if err != nil {
		return nil, err
	}
	shell := []string{"/"}
	for _, name := range versionedAssets {
		shell = append(shell, versionedURL(name, etags))
	}
	shell = append(shell, extra...)
	shell = append(shell, shellFiles...)
	list, err := json.Marshal(shell)
	if err != nil {
		return nil, err
	}
	out := strings.Replace(string(src), "__SW_VERSION__", version, 1)
	out = strings.Replace(out, "__SW_SHELL__", string(list), 1)
	return []byte(out), nil
}

// localeURLs lists the dictionary URLs exactly as i18n.js requests them.
// The app version covers all embedded files, so a changed dictionary gets a
// new URL (cache busting, like versionedAssets).
func localeURLs(fsys fs.FS, version string) []string {
	names, err := fs.Glob(fsys, localeDir+"/*.json")
	if err != nil {
		return nil
	}
	sort.Strings(names)
	urls := make([]string, 0, len(names))
	for _, name := range names {
		urls = append(urls, name+"?v="+version)
	}
	return urls
}

func versionedURL(name string, etags map[string]string) string {
	return name + "?v=" + strings.Trim(etags[name], `"`)
}

// appVersion fasst die Inhalte aller eingebetteten Dateien zu einem Hash zusammen.
func appVersion(etags map[string]string) string {
	names := make([]string, 0, len(etags))
	for name := range etags {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name + "=" + etags[name] + "\n")
	}
	return strings.Trim(etagOf([]byte(b.String())), `"`)
}

// contentETags berechnet je eingebetteter Datei ein starkes ETag (SHA-256, gekuerzt).
func contentETags(fsys fs.FS) map[string]string {
	tags := map[string]string{}
	_ = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		tags[path] = etagOf(data)
		return nil
	})
	return tags
}

func etagOf(data []byte) string {
	sum := sha256.Sum256(data)
	return `"` + hex.EncodeToString(sum[:8]) + `"`
}
