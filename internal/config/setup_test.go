package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadForSetup(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		cfg, needs, err := LoadForSetup(filepath.Join(t.TempDir(), "config.json"))
		if err != nil || !needs || cfg.Listen != ":8080" || len(cfg.Scenes) != 0 {
			t.Fatalf("cfg=%+v needs=%v err=%v", cfg, needs, err)
		}
	})
	t.Run("placeholder token keeps scenes", func(t *testing.T) {
		cfg, needs, err := LoadForSetup(writeConfig(t, `{"api_token":"BITTE-AENDERN","scenes":[{"id":"a","name":"A","source":{"type":"stream","uri":"u"},"members":[{"speaker":"S","volume":1}]}]}`))
		if err != nil || !needs || len(cfg.Scenes) != 1 {
			t.Fatalf("cfg=%+v needs=%v err=%v", cfg, needs, err)
		}
	})
	t.Run("token and zero scenes", func(t *testing.T) {
		_, needs, err := LoadForSetup(writeConfig(t, `{"api_token":"x","scenes":[]}`))
		if err != nil || needs {
			t.Fatalf("needs=%v err=%v", needs, err)
		}
	})
	t.Run("env token and listen", func(t *testing.T) {
		t.Setenv("SRA_API_TOKEN", "from-env")
		t.Setenv("SRA_LISTEN", ":8790")
		cfg, needs, err := LoadForSetup(filepath.Join(t.TempDir(), "config.json"))
		if err != nil || needs || cfg.APIToken != "from-env" || cfg.Listen != ":8790" {
			t.Fatalf("cfg=%+v needs=%v err=%v", cfg, needs, err)
		}
	})
	for name, content := range map[string]string{
		"broken json":   `{nope`,
		"unknown field": `{"foo":1}`,
		"bad scene":     `{"scenes":[{"id":"A B"}]}`,
		"bad timezone":  `{"timezone":"Mars/Base"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := LoadForSetup(writeConfig(t, content)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
	t.Run("unreadable path", func(t *testing.T) {
		if _, _, err := LoadForSetup(t.TempDir()); err == nil {
			t.Fatal("directory as config path must fail")
		}
	})
}

func TestLoad_ListenFromEnv(t *testing.T) {
	t.Setenv("SRA_LISTEN", "127.0.0.1:9999")
	cfg, err := Load(writeConfig(t, validConfig))
	if err != nil || cfg.Listen != "127.0.0.1:9999" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
}

func TestSaveSetupToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	if err := SaveSetupToken(path, "", ""); err == nil {
		t.Fatal("empty token must fail")
	}
	if err := SaveSetupToken(path, "ABCD-EFGH", "Europe/Zurich"); err != nil {
		t.Fatal(err)
	}
	cfg, err := ReadFile(path)
	if err != nil || cfg.APIToken != "ABCD-EFGH" || cfg.Timezone != "Europe/Zurich" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
	if err := SaveSetupToken(path, "OTHER", ""); !errors.Is(err, ErrAlreadySetUp) {
		t.Fatalf("second save: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{nope`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SaveSetupToken(path, "OTHER", ""); err == nil {
		t.Fatal("broken config must fail")
	}
}
