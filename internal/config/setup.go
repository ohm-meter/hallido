package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/schedule"
)

// First-run setup (ADR 0022): with setup enabled (container image), the agent
// also starts without config.json or without an API token. It then serves the
// web app's setup wizard, which creates the token on the server. Without setup
// enabled (systemd install, Pi) Load stays strict.

// LoadForSetup loads the config like Load, but tolerates the first-run state:
//   - config file missing  -> empty default config, needsSetup = true
//   - no token (file + env) -> config as read,       needsSetup = true
//
// Zero scenes are allowed (a fresh installation has none yet). Everything else
// (unknown fields, invalid scenes/schedules/IPs) is still an error.
func LoadForSetup(path string) (cfg *Config, needsSetup bool, err error) {
	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		cfg = &Config{}
	case err != nil:
		return nil, false, fmt.Errorf("read config (%s): %w", path, err)
	default:
		cfg, err = parseRaw(raw)
		if err != nil {
			return nil, false, fmt.Errorf("%s: %w", path, err)
		}
	}
	applyEnv(cfg)
	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}
	needsSetup = !HasUsableToken(cfg.APIToken)
	if err := cfg.validateCore(false, false); err != nil {
		return nil, false, err
	}
	return cfg, needsSetup, nil
}

// HasUsableToken reports whether token can protect the API (not empty, not
// the placeholder from the example config).
func HasUsableToken(token string) bool {
	token = strings.TrimSpace(token)
	return token != "" && token != DefaultToken
}

// applyEnv applies the environment overrides (token, listen address).
func applyEnv(cfg *Config) {
	if env := os.Getenv("SRA_API_TOKEN"); env != "" {
		cfg.APIToken = env
	}
	if env := os.Getenv("SRA_LISTEN"); env != "" {
		cfg.Listen = env
	}
}

// SaveSetupToken writes the token created by the setup wizard into the config
// file. An existing file keeps its scenes, schedules and speakers; a missing
// file is created. timezone (IANA, optional) is only set if the file has none
// yet. The file is written atomically with mode 0600 (SaveAtomic).
func SaveSetupToken(path, token, timezone string) error {
	if !HasUsableToken(token) {
		return fmt.Errorf("setup token is empty")
	}
	cfg, err := ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		cfg = &Config{Listen: ":8080"}
	} else if err != nil {
		return err
	}
	if HasUsableToken(cfg.APIToken) {
		return ErrAlreadySetUp
	}
	cfg.APIToken = token
	if cfg.Scenes == nil {
		cfg.Scenes = []scenes.Scene{}
	}
	if cfg.Timezone == "" && timezone != "" {
		if _, err := schedule.LoadLocation(timezone); err == nil {
			cfg.Timezone = timezone
		}
	}
	return SaveAtomic(path, cfg)
}

// ErrAlreadySetUp: the config file already has a token (setup must not
// overwrite it).
var ErrAlreadySetUp = errors.New("config already has an API token")
