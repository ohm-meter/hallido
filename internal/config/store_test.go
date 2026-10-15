package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/speaker"
)

func TestSaveAtomic_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := &Config{
		Listen:   ":8080",
		APIToken: "secret-token",
		Scenes: []scenes.Scene{{
			ID:      "test",
			Name:    "Test",
			Source:  speaker.Source{Type: speaker.SourceStream, URI: "https://example/radio"},
			Members: []scenes.Member{{Speaker: "Wohnzimmer", Volume: 20}},
		}},
	}
	if err := SaveAtomic(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIToken != "secret-token" || len(loaded.Scenes) != 1 {
		t.Fatalf("roundtrip: %+v", loaded)
	}
	raw, _ := os.ReadFile(path)
	if len(raw) == 0 {
		t.Fatal("datei leer")
	}
}

func TestValidateSceneID(t *testing.T) {
	if ValidateSceneID("fm4-wohnzimmer") != nil {
		t.Fatal("gueltige id")
	}
	if ValidateSceneID("FM4") == nil {
		t.Fatal("grossbuchstaben ungueltig")
	}
}

func TestUpdateScene(t *testing.T) {
	cfg := &Config{
		APIToken: "x",
		Scenes:   validConfigScenes(),
	}
	updated := scenes.Scene{
		ID:     "a",
		Name:   "A neu",
		Source: speaker.Source{Type: speaker.SourceStream, URI: "https://x/z"},
		Members: []scenes.Member{
			{Speaker: "S", Volume: 50},
			{Speaker: "T", Volume: 30},
		},
	}
	if err := UpdateScene(cfg, "a", updated); err != nil {
		t.Fatal(err)
	}
	if cfg.Scenes[0].Name != "A neu" || len(cfg.Scenes[0].Members) != 2 {
		t.Fatalf("update: %+v", cfg.Scenes[0])
	}
	if err := UpdateScene(cfg, "missing", updated); err == nil {
		t.Fatal("fehlende szene erwartet fehler")
	}
}

func TestDeleteScene(t *testing.T) {
	cfg := &Config{
		APIToken: "x",
		Scenes:   validConfigScenes(),
	}
	if err := DeleteScene(cfg, "a"); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Scenes) != 0 {
		t.Fatalf("szene soll weg sein: %+v", cfg.Scenes)
	}
	if err := DeleteScene(cfg, "missing"); err == nil {
		t.Fatal("fehlende szene erwartet fehler")
	}
}

func TestMergeSpeakersFromDiscovery(t *testing.T) {
	cfg := &Config{APIToken: "x", Scenes: validConfigScenes()}
	cfg.Speakers = map[string]SpeakerOverride{"Alt": {IP: "1.2.3.4"}}
	n, err := MergeSpeakersFromDiscovery(cfg, []SpeakerSyncEntry{
		{Name: "Wohnzimmer", IP: "10.0.0.1"},
		{Name: "Alt", IP: "1.2.3.4"},
	})
	if err != nil || n != 1 {
		t.Fatalf("merge: n=%d err=%v", n, err)
	}
	if cfg.Speakers["Wohnzimmer"].IP != "10.0.0.1" {
		t.Fatalf("wohnzimmer: %+v", cfg.Speakers)
	}
	if cfg.Speakers["Alt"].IP != "1.2.3.4" {
		t.Fatal("alt sollte bleiben")
	}
}

func validConfigScenes() []scenes.Scene {
	return []scenes.Scene{{
		ID:      "a",
		Name:    "A",
		Source:  speaker.Source{Type: speaker.SourceStream, URI: "https://x/y"},
		Members: []scenes.Member{{Speaker: "S", Volume: 10}},
	}}
}

func TestValidateSpeakerIP(t *testing.T) {
	for _, ok := range []string{"192.168.1.10", " 10.0.0.2 "} {
		if err := ValidateSpeakerIP(ok); err != nil {
			t.Errorf("%q sollte gueltig sein: %v", ok, err)
		}
	}
	for _, bad := range []string{"evil.example/", "10.0.0.1@evil.example", "10.0.0.1:80", "::1", "0.0.0.0", "224.0.0.1", "1.2.3", "a b"} {
		if err := ValidateSpeakerIP(bad); err == nil {
			t.Errorf("%q sollte abgelehnt werden", bad)
		}
	}
}

func TestMergeSpeakersRejectsBadIP(t *testing.T) {
	cfg := &Config{}
	_, err := MergeSpeakersFromDiscovery(cfg, []SpeakerSyncEntry{{Name: "A", IP: "evil.example/x"}})
	if err == nil {
		t.Fatal("ungueltige IP muss abgelehnt werden")
	}
}

func TestValidateRejectsBadSpeakerIPInConfig(t *testing.T) {
	cfg := &Config{APIToken: "tok", Speakers: map[string]SpeakerOverride{"A": {IP: "host.example"}}}
	if err := cfg.ValidateForSave(); err == nil {
		t.Fatal("Config mit Hostname als IP muss scheitern")
	}
}

func TestValidateForSave_PlatzhalterTokenNurMitEnv(t *testing.T) {
	cfg := &Config{APIToken: DefaultToken}
	t.Setenv("SRA_API_TOKEN", "")
	if err := cfg.ValidateForSave(); err == nil {
		t.Fatal("Platzhalter ohne Env muss abgelehnt werden")
	}
	t.Setenv("SRA_API_TOKEN", "aus-der-umgebung")
	if err := cfg.ValidateForSave(); err != nil {
		t.Fatalf("mit Env-Token muss Speichern gehen: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Start-Validierung (ohne Szenen/Token) bleibt strikt")
	}
}

func TestSaveAtomic_LegtBackupAn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := &Config{APIToken: "tok-1234567890abcd"}
	if err := SaveAtomic(path, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Fatal("erstes Speichern hat keinen Vorgaenger")
	}
	first, _ := os.ReadFile(path)
	cfg.Listen = ":9999"
	if err := SaveAtomic(path, cfg); err != nil {
		t.Fatal(err)
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil || string(bak) != string(first) {
		t.Fatalf("Backup muss den vorherigen Stand enthalten (err=%v)", err)
	}
}

func TestSaveAtomic_BackupRechteAuchBeiAlterBak(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := &Config{APIToken: "tok-1234567890abcd"}
	if err := SaveAtomic(path, cfg); err != nil {
		t.Fatal(err)
	}
	// Aeltere .bak mit zu offenen Rechten (z. B. aus frueherer Version).
	if err := os.WriteFile(path+".bak", []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path+".bak", 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveAtomic(path, cfg); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Fatalf("Backup-Rechte: erwartet 0600, bekam %o", perm)
	}
}

func TestValidateStreamURI_Faelle(t *testing.T) {
	ok := []string{"https://a.example/x", "http://a.example", "aac://a.example/x", "x-rincon-mp3radio://a.example/x"}
	bad := []string{"", "file:///etc/passwd", "FILE:///x", "https://a.example/../x", "ftp://a.example", "https://", "javascript:alert(1)"}
	for _, u := range ok {
		if err := ValidateStreamURI(u); err != nil {
			t.Errorf("%q: %v", u, err)
		}
	}
	for _, u := range bad {
		if err := ValidateStreamURI(u); err == nil {
			t.Errorf("%q muss abgelehnt werden", u)
		}
	}
}

func TestUpsertScene_NeuUndErsetzenUndFehler(t *testing.T) {
	cfg := &Config{}
	sc := scenes.Scene{ID: "a", Name: "A",
		Source:  speaker.Source{Type: speaker.SourceStream, URI: "https://a.example/s"},
		Members: []scenes.Member{{Speaker: "X", Volume: 10}}}
	if err := UpsertScene(cfg, sc); err != nil || len(cfg.Scenes) != 1 {
		t.Fatalf("neu: %v", err)
	}
	sc.Name = "B"
	if err := UpsertScene(cfg, sc); err != nil || len(cfg.Scenes) != 1 || cfg.Scenes[0].Name != "B" {
		t.Fatalf("ersetzen: %v %+v", err, cfg.Scenes)
	}
	sc.ID = "Ungueltig ID"
	if err := UpsertScene(cfg, sc); err == nil {
		t.Fatal("ungueltige ID muss scheitern")
	}
	sc.ID = "c"
	sc.Source = speaker.Source{Type: speaker.SourceFavorite, Name: "Radio Superfly",
		URI: "x-sonosapi-stream:tunein%3a17315?sid=303&flags=8232&sn=9"}
	if err := UpsertScene(cfg, sc); err != nil {
		t.Fatalf("TuneIn-Favorit muss erlaubt sein: %v", err)
	}
	sc.Source = speaker.Source{Type: "spotify", URI: "x"}
	if err := UpsertScene(cfg, sc); err == nil {
		t.Fatal("unbekannter Quelltyp muss scheitern")
	}
}

func TestValidateFavoriteSource_Faelle(t *testing.T) {
	ok := []speaker.Source{
		{Type: speaker.SourceFavorite, Name: "Radio Superfly"},
		{Type: speaker.SourceFavorite, Name: "Radio Superfly", URI: "x-sonosapi-stream:tunein%3a17315?sid=303"},
	}
	for _, src := range ok {
		if err := ValidateFavoriteSource(src); err != nil {
			t.Errorf("%+v: %v", src, err)
		}
	}
	bad := []speaker.Source{
		{Type: speaker.SourceFavorite, Name: "  "},
		{Type: speaker.SourceFavorite, Name: strings.Repeat("x", MaxFavoriteNameLen+1)},
		{Type: speaker.SourceFavorite, Name: "a\nb"},
		{Type: speaker.SourceFavorite, Name: "Jazz", URI: "x-sonosapi-radio:catalog?sid=201"},
		{Type: speaker.SourceFavorite, Name: "Jazz", URI: "https://a.example/s"},
	}
	for _, src := range bad {
		if err := ValidateFavoriteSource(src); err == nil {
			t.Errorf("%+v: erwartet Fehler", src)
		}
	}
}

func TestFixedIPOverridesUndPublicView(t *testing.T) {
	cfg := &Config{Listen: ":1", Speakers: map[string]SpeakerOverride{"A": {IP: "10.0.0.1"}, "B": {}}}
	ov := cfg.FixedIPOverrides()
	if len(ov) != 1 || ov["A"] != "10.0.0.1" {
		t.Fatalf("%v", ov)
	}
	v := PublicView(cfg)
	if _, has := v["api_token"]; has || v["speakers_count"] != 2 {
		t.Fatalf("%v", v)
	}
}

func TestSaveAtomic_NichtBeschreibbaresVerzeichnis(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "datei")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Elternpfad ist eine Datei -> MkdirAll scheitert.
	if err := SaveAtomic(filepath.Join(blocker, "sub", "config.json"), &Config{APIToken: "tok-1234567890abcd"}); err == nil {
		t.Fatal("Fehler erwartet")
	}
}
