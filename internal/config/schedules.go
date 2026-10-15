package config

import (
	"errors"
	"fmt"
	"strings"

	"sonos-radio-agent/internal/schedule"
)

// ErrScheduleNotFound: Zeitplan-ID existiert nicht (API -> 404).
var ErrScheduleNotFound = errors.New("schedule not found")

// ErrScheduleInvalid markiert Validierungsfehler (API -> 400).
var ErrScheduleInvalid = errors.New("schedule invalid")

func invalid(err error) error { return fmt.Errorf("%w: %v", ErrScheduleInvalid, err) }

// validateScheduleForConfig prueft einen Zeitplan gegen die bestehenden Szenen.
func validateScheduleForConfig(cfg *Config, s schedule.Schedule) error {
	if err := s.Validate(); err != nil {
		return invalid(err)
	}
	for _, sc := range cfg.Scenes {
		if sc.ID == s.Scene {
			return nil
		}
	}
	return invalid(fmt.Errorf("scene %q does not exist", s.Scene))
}

// UpsertSchedule fuegt einen Zeitplan ein oder ersetzt ihn bei gleicher ID.
// Ohne ID wird eine aus Szene und Startzeit erzeugt (z. B. morning-radio-0610).
// Liefert die (ggf. erzeugte) ID.
func UpsertSchedule(cfg *Config, s schedule.Schedule) (string, error) {
	s.Normalize()
	if s.ID == "" {
		s.ID = generateScheduleID(cfg, s)
	}
	if err := validateScheduleForConfig(cfg, s); err != nil {
		return "", err
	}
	for i, existing := range cfg.Schedules {
		if existing.ID == s.ID {
			cfg.Schedules[i] = s
			return s.ID, nil
		}
	}
	if len(cfg.Schedules) >= schedule.MaxSchedules {
		return "", invalid(fmt.Errorf("at most %d schedules allowed", schedule.MaxSchedules))
	}
	cfg.Schedules = append(cfg.Schedules, s)
	return s.ID, nil
}

// UpdateSchedule ersetzt einen bestehenden Zeitplan; die ID kommt aus dem Pfad.
func UpdateSchedule(cfg *Config, id string, s schedule.Schedule) error {
	s.ID = strings.TrimSpace(id)
	s.Normalize()
	if err := validateScheduleForConfig(cfg, s); err != nil {
		return err
	}
	for i, existing := range cfg.Schedules {
		if existing.ID == s.ID {
			cfg.Schedules[i] = s
			return nil
		}
	}
	return fmt.Errorf("%w: %q", ErrScheduleNotFound, s.ID)
}

// DeleteSchedule entfernt einen Zeitplan.
func DeleteSchedule(cfg *Config, id string) error {
	id = strings.TrimSpace(id)
	for i, existing := range cfg.Schedules {
		if existing.ID == id {
			cfg.Schedules = append(cfg.Schedules[:i], cfg.Schedules[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("%w: %q", ErrScheduleNotFound, id)
}

func generateScheduleID(cfg *Config, s schedule.Schedule) string {
	base := s.Scene + "-" + strings.ReplaceAll(s.Start, ":", "")
	if len(base) > 58 {
		base = strings.TrimRight(base[:58], "-")
	}
	taken := make(map[string]bool, len(cfg.Schedules))
	for _, existing := range cfg.Schedules {
		taken[existing.ID] = true
	}
	id := base
	for n := 2; taken[id]; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}
