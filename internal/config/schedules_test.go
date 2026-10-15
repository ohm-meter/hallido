package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/schedule"
	"sonos-radio-agent/internal/speaker"
)

func cfgWithScene() *Config {
	return &Config{
		Listen:   ":8080",
		APIToken: "secret-token",
		Scenes: []scenes.Scene{{
			ID:      "oe3-morgen",
			Name:    "Ö3 Morgen",
			Source:  speaker.Source{Type: speaker.SourceStream, URI: "https://orf-live.ors-shoutcast.at/oe3-q2a"},
			Members: []scenes.Member{{Speaker: "Wohnzimmer", Volume: 20}},
		}},
	}
}

func werktags() schedule.Schedule {
	return schedule.Schedule{
		Scene: "oe3-morgen", Days: []string{"fri", "mon", "tue", "wed", "thu"},
		Start: "06:10", Stop: "08:30", Enabled: true,
	}
}

func TestLoad_WithSchedulesAndTimezone(t *testing.T) {
	p := writeConfig(t, `{
  "api_token": "geheim123",
  "timezone": "Europe/Vienna",
  "scenes": [{"id":"oe3","name":"Ö3","source":{"type":"stream","uri":"https://example/oe3"},
              "members":[{"speaker":"Wohnzimmer","volume":20}]}],
  "schedules": [{"id":"morgen","scene":"oe3","days":["mon","fri"],"start":"06:10","stop":"08:30"}]
}`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Schedules) != 1 || !cfg.Schedules[0].Enabled {
		t.Fatalf("schedules: %+v", cfg.Schedules)
	}
	if cfg.Location().String() != "Europe/Vienna" {
		t.Fatalf("location: %v", cfg.Location())
	}
}

func TestLoad_ScheduleErrors(t *testing.T) {
	base := `{"api_token":"geheim123",%s"scenes":[{"id":"oe3","name":"Ö3","source":{"type":"stream","uri":"https://example/oe3"},"members":[{"speaker":"W","volume":20}]}],"schedules":[%s]}`
	cases := map[string][2]string{
		"zeitzone":         {`"timezone":"Mars/Base",`, `{"id":"a","scene":"oe3","days":["mon"],"start":"06:00"}`},
		"unbekannte szene": {``, `{"id":"a","scene":"fm4","days":["mon"],"start":"06:00"}`},
		"doppelte id":      {``, `{"id":"a","scene":"oe3","days":["mon"],"start":"06:00"},{"id":"a","scene":"oe3","days":["tue"],"start":"06:00"}`},
		"falsche zeit":     {``, `{"id":"a","scene":"oe3","days":["mon"],"start":"6 Uhr"}`},
		"tippfehler feld":  {``, `{"id":"a","scene":"oe3","dayz":["mon"],"start":"06:00"}`},
	}
	for name, c := range cases {
		p := writeConfig(t, strings.Replace(strings.Replace(base, "%s", c[0], 1), "%s", c[1], 1))
		if _, err := Load(p); err == nil {
			t.Errorf("%s: fehler erwartet", name)
		}
	}
}

func TestUpsertSchedule_GeneratesIDAndNormalizes(t *testing.T) {
	cfg := cfgWithScene()
	id, err := UpsertSchedule(cfg, werktags())
	if err != nil {
		t.Fatal(err)
	}
	if id != "oe3-morgen-0610" {
		t.Fatalf("id = %q", id)
	}
	if got := strings.Join(cfg.Schedules[0].Days, ","); got != "mon,tue,wed,thu,fri" {
		t.Fatalf("tage nicht sortiert: %s", got)
	}
	id2, err := UpsertSchedule(cfg, werktags())
	if err != nil || id2 != "oe3-morgen-0610-2" {
		t.Fatalf("zweite id = %q, err=%v", id2, err)
	}
	// Upsert mit bestehender ID ersetzt.
	s := werktags()
	s.ID, s.Start = id, "06:30"
	if _, err := UpsertSchedule(cfg, s); err != nil || len(cfg.Schedules) != 2 || cfg.Schedules[0].Start != "06:30" {
		t.Fatalf("upsert ersetzt nicht: %+v %v", cfg.Schedules, err)
	}
}

func TestUpsertSchedule_UnknownSceneInvalid(t *testing.T) {
	s := werktags()
	s.Scene = "gibtsnicht"
	_, err := UpsertSchedule(cfgWithScene(), s)
	if !errors.Is(err, ErrScheduleInvalid) {
		t.Fatalf("ErrScheduleInvalid erwartet: %v", err)
	}
}

func TestUpdateAndDeleteSchedule(t *testing.T) {
	cfg := cfgWithScene()
	id, _ := UpsertSchedule(cfg, werktags())
	s := werktags()
	s.Enabled = false
	if err := UpdateSchedule(cfg, id, s); err != nil || cfg.Schedules[0].Enabled {
		t.Fatalf("update: %v %+v", err, cfg.Schedules)
	}
	if err := UpdateSchedule(cfg, "fehlt", s); !errors.Is(err, ErrScheduleNotFound) {
		t.Fatalf("update fehlt: %v", err)
	}
	if err := DeleteScene(cfg, "oe3-morgen"); err == nil || !strings.Contains(err.Error(), "schedule") {
		t.Fatalf("szene mit zeitplan darf nicht geloescht werden: %v", err)
	}
	if err := DeleteSchedule(cfg, id); err != nil || len(cfg.Schedules) != 0 {
		t.Fatalf("delete: %v", err)
	}
	if err := DeleteSchedule(cfg, id); !errors.Is(err, ErrScheduleNotFound) {
		t.Fatalf("delete fehlt: %v", err)
	}
}

func TestSaveAtomic_SchedulesRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := cfgWithScene()
	s := werktags()
	s.Enabled = false
	if _, err := UpsertSchedule(cfg, s); err != nil {
		t.Fatal(err)
	}
	if err := SaveAtomic(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Schedules) != 1 || loaded.Schedules[0].Enabled {
		t.Fatalf("enabled=false muss den roundtrip ueberleben: %+v", loaded.Schedules)
	}
}

func TestExampleConfig_IsValid(t *testing.T) {
	t.Setenv("SRA_API_TOKEN", "beispiel-token")
	cfg, err := Load(filepath.Join("..", "..", "config.example.json"))
	if err != nil {
		t.Fatalf("config.example.json ungueltig: %v", err)
	}
	if len(cfg.Schedules) == 0 || cfg.Schedules[0].Scene != "morning-radio" {
		t.Fatalf("beispiel-zeitplan fehlt: %+v", cfg.Schedules)
	}
}
