package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/speaker"
)

// MaxSceneMembers begrenzt die Groesse einer Szene (DoS-Schutz).
const MaxSceneMembers = 32

var sceneIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// ReadFile laedt die Config aus der Datei ohne Env-Override (fuer sicheres Schreiben).
func ReadFile(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config (%s): %w", path, err)
	}
	return parseRaw(raw)
}

func parseRaw(raw []byte) (*Config, error) {
	var cfg Config
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}
	return &cfg, nil
}

// SaveAtomic schreibt die Config atomar (temp + rename) und validiert vorher.
func SaveAtomic(path string, cfg *Config) error {
	if err := cfg.ValidateForSave(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize config: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("config directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("config permissions: %w", err)
	}
	// Stromausfall (Pi): erst auf Platte, dann umbenennen.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("config sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	// Vorherigen Stand sichern (ADR 0008): eine .bak-Generation neben der Config.
	if prev, err := os.ReadFile(path); err == nil {
		if err := os.WriteFile(path+".bak", prev, 0o600); err != nil {
			return fmt.Errorf("config backup: %w", err)
		}
		// WriteFile setzt die Rechte nur beim Anlegen; eine aeltere .bak
		// (enthaelt das Token) ebenfalls auf 0600 bringen.
		if err := os.Chmod(path+".bak", 0o600); err != nil {
			return fmt.Errorf("config backup permissions: %w", err)
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync() // Verzeichniseintrag sichern (best effort)
		_ = d.Close()
	}
	return nil
}

// PublicView liefert einen token-freien Ueberblick fuer die GUI.
func PublicView(cfg *Config) map[string]any {
	sceneIDs := make([]string, 0, len(cfg.Scenes))
	for _, sc := range cfg.Scenes {
		sceneIDs = append(sceneIDs, sc.ID)
	}
	speakerKeys := make([]string, 0, len(cfg.Speakers))
	for name := range cfg.Speakers {
		speakerKeys = append(speakerKeys, name)
	}
	return map[string]any{
		"listen":         cfg.Listen,
		"scene_ids":      sceneIDs,
		"speaker_names":  speakerKeys,
		"scenes_count":   len(cfg.Scenes),
		"speakers_count": len(cfg.Speakers),
	}
}

// ValidateSceneID prueft die Szenen-ID als URL-Slug.
func ValidateSceneID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("scene id missing")
	}
	if !sceneIDPattern.MatchString(id) {
		return fmt.Errorf("scene id %q invalid (only a-z, 0-9, hyphen)", id)
	}
	return nil
}

// ValidateStreamURI prueft Stream-URIs fuer Szenen (http/https, kein file://).
func ValidateStreamURI(uri string) error {
	u := strings.TrimSpace(uri)
	if u == "" {
		return fmt.Errorf("stream uri missing")
	}
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "file:") {
		return fmt.Errorf("file:// uris are not allowed")
	}
	if strings.Contains(u, "..") {
		return fmt.Errorf("uri contains invalid path segments")
	}
	// Direkt http(s) oder Sonos-Huellen (aac://, x-rincon-mp3radio://, …).
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		parsed, err := url.Parse(u)
		if err != nil || parsed.Host == "" {
			return fmt.Errorf("stream uri not parseable")
		}
		return nil
	}
	allowed := []string{"aac://", "x-rincon-mp3radio://", "x-sonos-http://", "x-sonos-htastream://"}
	for _, p := range allowed {
		if strings.HasPrefix(lower, p) {
			return nil
		}
	}
	return fmt.Errorf("stream uri must be http(s) or a supported sonos format")
}

func validateSceneForConfig(scene scenes.Scene) error {
	if err := ValidateSceneID(scene.ID); err != nil {
		return err
	}
	if err := scene.Validate(); err != nil {
		return err
	}
	if len(scene.Members) > MaxSceneMembers {
		return fmt.Errorf("scene %q: at most %d speakers", scene.ID, MaxSceneMembers)
	}
	switch scene.Source.Type {
	case speaker.SourceStream:
		return ValidateStreamURI(scene.Source.URI)
	case speaker.SourceFavorite:
		return ValidateFavoriteSource(scene.Source)
	}
	return fmt.Errorf("source type %q is not supported (stream or favorite)", scene.Source.Type)
}

// MaxFavoriteNameLen begrenzt den Favoriten-Titel einer Szene.
const MaxFavoriteNameLen = 200

// ValidateFavoriteSource prueft eine Favoriten-Quelle (ADR 0018): Titel wie in
// der Sonos-App, optional die Favoriten-URI (nur x-sonosapi-stream, fuer den
// Status-Abgleich).
func ValidateFavoriteSource(src speaker.Source) error {
	name := strings.TrimSpace(src.Name)
	if name == "" {
		return fmt.Errorf("favorite name missing")
	}
	if len(name) > MaxFavoriteNameLen {
		return fmt.Errorf("favoriten-name laenger als %d zeichen", MaxFavoriteNameLen)
	}
	if strings.ContainsFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return fmt.Errorf("favorite name contains control characters")
	}
	if uri := strings.TrimSpace(src.URI); uri != "" && speaker.ServiceStreamKey(uri) == "" {
		return fmt.Errorf("favorite URI must be x-sonosapi-stream:… (or empty)")
	}
	return nil
}

// ErrSceneNotFound markiert "Szene existiert nicht" (errors.Is).
var ErrSceneNotFound = errors.New("scene not found")

type sceneNotFoundError struct{ id string }

func (e sceneNotFoundError) Error() string        { return fmt.Sprintf("scene %q not found", e.id) }
func (e sceneNotFoundError) Is(target error) bool { return target == ErrSceneNotFound }

// UpsertScene fuegt eine Szene ein oder ersetzt sie bei gleicher ID.
func UpsertScene(cfg *Config, scene scenes.Scene) error {
	if err := validateSceneForConfig(scene); err != nil {
		return err
	}
	for i, sc := range cfg.Scenes {
		if sc.ID == scene.ID {
			cfg.Scenes[i] = scene
			return nil
		}
	}
	cfg.Scenes = append(cfg.Scenes, scene)
	return nil
}

// UpdateScene ersetzt eine bestehende Szene; ID kommt aus dem Pfad.
func UpdateScene(cfg *Config, id string, scene scenes.Scene) error {
	if err := ValidateSceneID(id); err != nil {
		return err
	}
	scene.ID = id
	if err := validateSceneForConfig(scene); err != nil {
		return err
	}
	for i, sc := range cfg.Scenes {
		if sc.ID == id {
			cfg.Scenes[i] = scene
			return nil
		}
	}
	return sceneNotFoundError{id: id}
}

// DeleteScene entfernt eine Szene aus der Config anhand der ID.
func DeleteScene(cfg *Config, id string) error {
	if err := ValidateSceneID(id); err != nil {
		return err
	}
	for _, sc := range cfg.Schedules {
		if sc.Scene == id {
			return fmt.Errorf("scene %q is used by schedule %q -- delete or change the schedule first", id, sc.ID)
		}
	}
	for i, sc := range cfg.Scenes {
		if sc.ID == id {
			cfg.Scenes = append(cfg.Scenes[:i], cfg.Scenes[i+1:]...)
			return nil
		}
	}
	return sceneNotFoundError{id: id}
}

// ValidateSpeakerIP erlaubt nur literale IPv4-Adressen (Sonos-Boxen im LAN);
// verhindert, dass Hostnamen oder URL-Bestandteile in die Basis-URL gelangen.
func ValidateSpeakerIP(ip string) error {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil || !addr.Is4() || addr.IsUnspecified() || addr.IsMulticast() {
		return fmt.Errorf("not a valid IPv4 address")
	}
	return nil
}

// SpeakerSyncEntry ist ein Discovery-Eintrag fuer den Speakers-Sync.
type SpeakerSyncEntry struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// MergeSpeakersFromDiscovery uebernimmt name+ip aus Discovery in cfg.Speakers.
// Bestehende Eintraege bleiben; fehlende IPs werden gesetzt, neue Namen angelegt.
func MergeSpeakersFromDiscovery(cfg *Config, entries []SpeakerSyncEntry) (int, error) {
	if len(entries) == 0 {
		return 0, fmt.Errorf("no speakers to save")
	}
	if cfg.Speakers == nil {
		cfg.Speakers = map[string]SpeakerOverride{}
	}
	updated := 0
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		name := strings.TrimSpace(e.Name)
		if name == "" {
			return updated, fmt.Errorf("speaker without name")
		}
		if seen[name] {
			return updated, fmt.Errorf("speaker %q duplicated in request", name)
		}
		seen[name] = true
		ip := strings.TrimSpace(e.IP)
		if ip != "" {
			if err := ValidateSpeakerIP(ip); err != nil {
				return updated, fmt.Errorf("invalid ip for %q: %w", name, err)
			}
		}
		prev, ok := cfg.Speakers[name]
		if !ok || prev.IP != ip {
			cfg.Speakers[name] = SpeakerOverride{IP: ip}
			updated++
		}
	}
	return updated, nil
}
