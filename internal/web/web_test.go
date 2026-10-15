package web

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerLiefertLaufzeitAssets(t *testing.T) {
	h := Handler()
	for _, p := range []string{"/", "/app.js", "/icons.js", "/style.css", "/vendor/pico.min.css"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
			t.Errorf("%s: code %d", p, rec.Code)
		}
	}
	for _, p := range []string{"/vendor/pico.css", "/vendor/lucide/x.svg"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s soll nicht ausgeliefert werden, code %d", p, rec.Code)
		}
	}
}

func TestHandlerCacheHeaders(t *testing.T) {
	h := Handler()
	for _, p := range []string{"/", "/app.js", "/style.css", "/vendor/pico.min.css"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		etag := rec.Header().Get("ETag")
		if etag == "" || rec.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("%s: ETag=%q Cache-Control=%q", p, etag, rec.Header().Get("Cache-Control"))
		}

		// Gleicher ETag -> 304 ohne Body.
		req := httptest.NewRequest(http.MethodGet, p, nil)
		req.Header.Set("If-None-Match", etag)
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
			t.Errorf("%s: If-None-Match passend -> code %d, body %d", p, rec.Code, rec.Body.Len())
		}

		// Veralteter ETag (alte Version im Cache) -> volle Datei.
		req = httptest.NewRequest(http.MethodGet, p, nil)
		req.Header.Set("If-None-Match", `"veraltet"`)
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
			t.Errorf("%s: veralteter ETag -> code %d", p, rec.Code)
		}
	}

	// Verschiedene Dateien -> verschiedene ETags.
	get := func(p string) string {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		return rec.Header().Get("ETag")
	}
	if get("/app.js") == get("/style.css") {
		t.Error("app.js und style.css haben denselben ETag")
	}
}

func TestHandlerIndexVersioniertAssets(t *testing.T) {
	h := Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	for _, name := range versionedAssets {
		a := httptest.NewRecorder()
		h.ServeHTTP(a, httptest.NewRequest(http.MethodGet, "/"+name, nil))
		v := strings.Trim(a.Header().Get("ETag"), `"`)
		if v == "" || !strings.Contains(body, `"`+name+`?v=`+v+`"`) {
			t.Errorf("index.html referenziert %s nicht mit ?v=%s", name, v)
		}
		// Asset mit Versions-Query wird normal ausgeliefert.
		q := httptest.NewRecorder()
		h.ServeHTTP(q, httptest.NewRequest(http.MethodGet, "/"+name+"?v="+v, nil))
		if q.Code != http.StatusOK || q.Body.Len() == 0 {
			t.Errorf("/%s?v=…: code %d", name, q.Code)
		}
	}
	if strings.Contains(body, `"app.js"`) || strings.Contains(body, `"style.css"`) {
		t.Error("index.html enthaelt noch unversionierte Asset-Referenzen")
	}
}

func get(t *testing.T, h http.Handler, p string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: code %d", p, rec.Code)
	}
	return rec
}

func TestManifestGueltig(t *testing.T) {
	rec := get(t, Handler(), "/manifest.webmanifest")
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/manifest+json") {
		t.Errorf("Content-Type %q", ct)
	}
	if rec.Header().Get("Cache-Control") != "no-cache" || rec.Header().Get("ETag") == "" {
		t.Error("Manifest ohne ETag/no-cache")
	}
	var m struct {
		Icons []struct{ Src, Sizes, Type, Purpose string }
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	for _, k := range []string{"name", "short_name", "display", "start_url", "scope", "theme_color", "background_color"} {
		if s, _ := raw[k].(string); s == "" {
			t.Errorf("Manifest: %s fehlt", k)
		}
	}
	if raw["display"] != "standalone" || raw["start_url"] != "/" || raw["scope"] != "/" {
		t.Errorf("display/start_url/scope: %v %v %v", raw["display"], raw["start_url"], raw["scope"])
	}
	h := Handler()
	purposes := map[string]bool{}
	for _, ic := range m.Icons {
		purposes[ic.Purpose+" "+ic.Sizes] = true
		r := get(t, h, "/"+ic.Src)
		if ct := r.Header().Get("Content-Type"); !strings.HasPrefix(ct, ic.Type) {
			t.Errorf("%s: Content-Type %q, erwartet %s", ic.Src, ct, ic.Type)
		}
	}
	for _, want := range []string{"any 192x192", "any 512x512", "maskable 512x512"} {
		if !purposes[want] {
			t.Errorf("Icon %q fehlt", want)
		}
	}
	get(t, h, "/icons/apple-touch-icon.png")
}

func TestServiceWorkerVersioniertUndOhneAPI(t *testing.T) {
	h := Handler()
	rec := get(t, h, "/sw.js")
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("Content-Type %q", ct)
	}
	if rec.Header().Get("Cache-Control") != "no-cache" || rec.Header().Get("ETag") == "" {
		t.Error("sw.js ohne ETag/no-cache")
	}
	body := rec.Body.String()
	if strings.Contains(body, "__SW_VERSION__") || strings.Contains(body, "__SW_SHELL__") {
		t.Fatal("Platzhalter in sw.js nicht ersetzt")
	}
	index := get(t, h, "/").Body.String()
	for _, name := range versionedAssets {
		a := get(t, h, "/"+name)
		ref := `"` + name + `?v=` + strings.Trim(a.Header().Get("ETag"), `"`) + `"`
		if !strings.Contains(body, ref) {
			t.Errorf("sw.js cached %s nicht in der aktuellen Version (%s)", name, ref)
		}
	}
	for _, f := range shellFiles {
		get(t, h, "/"+f) // jede Shell-Datei muss auslieferbar sein
	}
	shell := body[strings.Index(body, "const SHELL = "):]
	shell = shell[:strings.Index(shell, "\n")]
	if strings.Contains(shell, "api/") {
		t.Errorf("App-Shell enthaelt API-Pfade: %s", shell)
	}
	if strings.Contains(index, versionPlaceholder) {
		t.Error("index.html enthaelt noch den Versions-Platzhalter")
	}
	version := appVersion(contentETags(mustSub(t)))
	if !strings.Contains(index, `content="`+version+`"`) || !strings.Contains(body, `"`+version+`"`) {
		t.Errorf("index.html und sw.js tragen nicht dieselbe Version %s", version)
	}
}

func TestAppVersionAendertSichMitInhalt(t *testing.T) {
	a := appVersion(map[string]string{"app.js": `"1"`, "style.css": `"2"`})
	b := appVersion(map[string]string{"app.js": `"1"`, "style.css": `"3"`})
	if a == b || a == "" {
		t.Errorf("Version reagiert nicht auf Inhalt: %s %s", a, b)
	}
}

func mustSub(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		t.Fatal(err)
	}
	return sub
}

// TestLocalesEmbeddedAndInParity checks the web app dictionaries (ADR 0019):
// en.json and de.json are embedded, parse as flat key -> string objects, have
// identical key sets and no empty values.
func TestLocalesEmbeddedAndInParity(t *testing.T) {
	sub := mustSub(t)
	names, err := fs.Glob(sub, localeDir+"/*.json")
	if err != nil {
		t.Fatal(err)
	}
	dicts := map[string]map[string]string{}
	for _, name := range names {
		data, err := fs.ReadFile(sub, name)
		if err != nil {
			t.Fatal(err)
		}
		var d map[string]string
		if err := json.Unmarshal(data, &d); err != nil {
			t.Fatalf("%s: no flat key -> string object: %v", name, err)
		}
		for k, v := range d {
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s: empty value for %q", name, k)
			}
		}
		dicts[strings.TrimSuffix(strings.TrimPrefix(name, localeDir+"/"), ".json")] = d
	}
	en, de := dicts["en"], dicts["de"]
	if len(en) < 100 || len(de) < 100 {
		t.Fatalf("en/de not embedded or too small: en=%d de=%d (files %v)", len(en), len(de), names)
	}
	for lang, d := range dicts {
		if lang == "en" {
			continue
		}
		for k := range en {
			if _, ok := d[k]; !ok {
				t.Errorf("%s.json: key %q missing", lang, k)
			}
		}
		for k := range d {
			if _, ok := en[k]; !ok {
				t.Errorf("%s.json: key %q not in en.json", lang, k)
			}
		}
	}
}

func TestLocalesServedAndInServiceWorkerShell(t *testing.T) {
	h := Handler()
	sw := get(t, h, "/sw.js").Body.String()
	version := appVersion(contentETags(mustSub(t)))
	for _, lang := range []string{"en", "de"} {
		url := localeDir + "/" + lang + ".json?v=" + version
		rec := get(t, h, "/"+url)
		if rec.Header().Get("ETag") == "" || !strings.Contains(rec.Header().Get("Content-Type"), "json") {
			t.Errorf("%s: ETag=%q Content-Type=%q", url, rec.Header().Get("ETag"), rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(sw, `"`+url+`"`) {
			t.Errorf("sw.js app shell is missing %s", url)
		}
	}
}
