// Package config laedt und validiert die Laufzeit-Konfiguration.
//
// Konfiguration ist bewusst JSON (Standardbibliothek, keine Abhaengigkeit).
// Geheimnisse (API-Token) kommen aus Datei ODER Umgebungsvariable, nie aus
// dem Code. Die Env-Variable hat Vorrang, damit man das Token in
// systemd/Docker setzen kann, ohne die Config-Datei zu aendern.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/schedule"
)

// DefaultToken ist der Platzhalter aus der Beispielconfig. Laeuft der Agent
// damit, ist das ein Konfigurationsfehler (siehe Validate).
const DefaultToken = "BITTE-AENDERN"

// SpeakerOverride erlaubt, Discovery zu umgehen und eine feste IP anzugeben.
type SpeakerOverride struct {
	IP string `json:"ip,omitempty"`
}

// Config ist die gesamte Laufzeit-Konfiguration.
type Config struct {
	Listen   string                     `json:"listen"`             // z.B. ":8080"
	APIToken string                     `json:"api_token"`          // via Env SRA_API_TOKEN ueberschreibbar
	Speakers map[string]SpeakerOverride `json:"speakers,omitempty"` // optionale Fixed-IP-Overrides
	Scenes   []scenes.Scene             `json:"scenes"`

	// Timezone fuer Zeitplaene (IANA, leer = Europe/Vienna).
	Timezone  string              `json:"timezone,omitempty"`
	Schedules []schedule.Schedule `json:"schedules,omitempty"`
}

// Load liest und validiert die Config aus einer Datei.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config (%s): %w", path, err)
	}
	var cfg Config
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields() // Tippfehler in der Config frueh erkennen
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config (%s): %w", path, err)
	}

	// Env variables take precedence (token, listen address).
	applyEnv(&cfg)
	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate prueft die Konfiguration auf Vollstaendigkeit (inkl. Start).
func (c *Config) Validate() error {
	if err := c.validateCore(true, true); err != nil {
		return err
	}
	if len(c.Scenes) == 0 {
		return fmt.Errorf("no scenes configured")
	}
	return nil
}

// ValidateForSave prueft vor atomarem Schreiben; leere Szenenliste ist erlaubt
// (z. B. nach Loeschen der letzten Szene), Start via Load verlangt weiter min. eine Szene.
//
// Kommt das Token ausschliesslich aus SRA_API_TOKEN (Datei hat noch den
// Platzhalter), darf trotzdem gespeichert werden -- die Datei wird ohne
// Env-Override gelesen und geschrieben, das Token bleibt also unveraendert.
func (c *Config) ValidateForSave() error {
	return c.validateCore(false, os.Getenv("SRA_API_TOKEN") == "")
}

func (c *Config) validateCore(requireScenes, requireToken bool) error {
	if requireToken && (c.APIToken == "" || c.APIToken == DefaultToken) {
		return fmt.Errorf("api_token missing or still the placeholder -- " +
			"set your own token in the config or via env SRA_API_TOKEN")
	}
	if requireScenes && len(c.Scenes) == 0 {
		return fmt.Errorf("no scenes configured")
	}
	for name, ov := range c.Speakers {
		if ov.IP == "" {
			continue
		}
		if err := ValidateSpeakerIP(ov.IP); err != nil {
			return fmt.Errorf("speakers[%q].ip %q: %w", name, ov.IP, err)
		}
	}
	ids := make(map[string]bool, len(c.Scenes))
	for _, s := range c.Scenes {
		if err := s.Validate(); err != nil {
			return err
		}
		if ids[s.ID] {
			return fmt.Errorf("scene id %q assigned twice", s.ID)
		}
		ids[s.ID] = true
	}
	return c.validateSchedules(ids)
}

// validateSchedules prueft Zeitzone und Zeitplaene inkl. Szenen-Referenz.
func (c *Config) validateSchedules(sceneIDs map[string]bool) error {
	if _, err := schedule.LoadLocation(c.Timezone); err != nil {
		return err
	}
	if len(c.Schedules) > schedule.MaxSchedules {
		return fmt.Errorf("at most %d schedules allowed", schedule.MaxSchedules)
	}
	seen := make(map[string]bool, len(c.Schedules))
	for _, sc := range c.Schedules {
		if err := sc.Validate(); err != nil {
			return err
		}
		if seen[sc.ID] {
			return fmt.Errorf("schedule id %q assigned twice", sc.ID)
		}
		seen[sc.ID] = true
		if !sceneIDs[sc.Scene] {
			return fmt.Errorf("schedule %q: scene %q does not exist", sc.ID, sc.Scene)
		}
	}
	return nil
}

// Location liefert die Zeitzone fuer Zeitplaene (nach Validate fehlerfrei).
func (c *Config) Location() *time.Location {
	loc, err := schedule.LoadLocation(c.Timezone)
	if err != nil {
		loc, _ = schedule.LoadLocation("")
	}
	return loc
}

// FixedIPOverrides liefert Raumname -> feste IP fuer den Sonos-Treiber.
func (c *Config) FixedIPOverrides() map[string]string {
	out := make(map[string]string, len(c.Speakers))
	for name, ov := range c.Speakers {
		if ov.IP != "" {
			out[name] = ov.IP
		}
	}
	return out
}

// KnownSpeakers liefert alle in Config/Szenen referenzierten Raumnamen mit
// optionaler fester IP (leer = nur Name bekannt).
func (c *Config) KnownSpeakers() map[string]string {
	out := c.FixedIPOverrides()
	for _, sc := range c.Scenes {
		for _, m := range sc.Members {
			if _, ok := out[m.Speaker]; !ok {
				out[m.Speaker] = ""
			}
		}
	}
	return out
}
