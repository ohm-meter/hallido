package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const validConfig = `{
  "listen": ":9090",
  "api_token": "geheim123",
  "scenes": [
    {
      "id": "fm4",
      "name": "FM4",
      "source": { "type": "stream", "uri": "https://example/fm4" },
      "members": [ { "speaker": "Wohnzimmer", "volume": 30 } ]
    }
  ]
}`

func TestLoad_Valid(t *testing.T) {
	cfg, err := Load(writeConfig(t, validConfig))
	if err != nil {
		t.Fatalf("unerwarteter fehler: %v", err)
	}
	if cfg.Listen != ":9090" || cfg.APIToken != "geheim123" || len(cfg.Scenes) != 1 {
		t.Fatalf("config falsch geladen: %+v", cfg)
	}
}

func TestLoad_EnvTokenOverride(t *testing.T) {
	t.Setenv("SRA_API_TOKEN", "aus-env")
	cfg, err := Load(writeConfig(t, validConfig))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIToken != "aus-env" {
		t.Fatalf("env sollte token ueberschreiben, bekam %q", cfg.APIToken)
	}
}

func TestLoad_DefaultListen(t *testing.T) {
	c := `{"api_token":"x","scenes":[{"id":"a","name":"A","source":{"type":"stream","uri":"u"},"members":[{"speaker":"S","volume":10}]}]}`
	cfg, err := Load(writeConfig(t, c))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != ":8080" {
		t.Fatalf("default listen erwartet :8080, bekam %q", cfg.Listen)
	}
}

func TestLoad_Errors(t *testing.T) {
	cases := map[string]string{
		"placeholder token": `{"api_token":"BITTE-AENDERN","scenes":[{"id":"a","name":"A","source":{"type":"stream","uri":"u"},"members":[{"speaker":"S","volume":1}]}]}`,
		"kein token":        `{"scenes":[{"id":"a","name":"A","source":{"type":"stream","uri":"u"},"members":[{"speaker":"S","volume":1}]}]}`,
		"keine szenen":      `{"api_token":"x","scenes":[]}`,
		"doppelte id":       `{"api_token":"x","scenes":[{"id":"a","name":"A","source":{"type":"stream","uri":"u"},"members":[{"speaker":"S","volume":1}]},{"id":"a","name":"B","source":{"type":"stream","uri":"u"},"members":[{"speaker":"S","volume":1}]}]}`,
		"unbekanntes feld":  `{"api_token":"x","foo":true,"scenes":[]}`,
		"kaputtes json":     `{nope`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, content)); err == nil {
				t.Fatalf("erwarte fehler fuer %q", name)
			}
		})
	}
}

func TestLoad_FileMissing(t *testing.T) {
	if _, err := Load("/nonexistent/config.json"); err == nil {
		t.Fatal("fehlende datei sollte fehler liefern")
	}
}
