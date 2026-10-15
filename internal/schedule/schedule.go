// Package schedule startet und stoppt Szenen zeitgesteuert ("Zeitplaene").
//
// Ein Zeitplan referenziert nur eine bestehende Szene (ID) und beschreibt,
// an welchen Wochentagen sie zu welcher Uhrzeit startet und optional wieder
// stoppt. Alle Uhrzeiten gelten in der Zeitzone aus der Config (Default
// Europe/Vienna). Die Zeitzonen-Datenbank ist eingebettet (time/tzdata),
// damit der Pi keine tzdata installiert haben muss.
package schedule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata" // Zeitzonen im Binary -- Pi ohne tzdata-Paket
)

// DefaultTimezone gilt, wenn die Config kein "timezone" setzt.
const DefaultTimezone = "Europe/Vienna"

// MaxSchedules begrenzt die Anzahl der Zeitplaene (DoS-Schutz).
const MaxSchedules = 64

var (
	idPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	clockPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)
)

// dayKeys bildet die Config-Schluessel auf Wochentage ab.
var dayKeys = map[string]time.Weekday{
	"mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
	"sun": time.Sunday,
}

// DayOrder ist die kanonische Reihenfolge der Wochentage (Mo zuerst).
var DayOrder = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// Schedule ist ein Zeitplan aus der Config.
//
// Stop ist optional. Liegt Stop vor Start (z. B. 23:00-01:00), endet das
// Fenster am Folgetag; Days bezieht sich immer auf den Starttag.
type Schedule struct {
	ID      string   `json:"id"`
	Scene   string   `json:"scene"`
	Days    []string `json:"days"`
	Start   string   `json:"start"`
	Stop    string   `json:"stop,omitempty"`
	Enabled bool     `json:"enabled"`
}

// UnmarshalJSON setzt enabled=true, wenn das Feld fehlt (handgeschriebene
// Config), und lehnt unbekannte Felder ab -- wie der Rest der Config.
func (s *Schedule) UnmarshalJSON(b []byte) error {
	type plain Schedule
	p := plain{Enabled: true}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return err
	}
	*s = Schedule(p)
	return nil
}

// Normalize trimmt Felder und bringt Tage in kanonische Form (klein,
// Mo-So sortiert). Doppelte oder unbekannte Tage bleiben fuer Validate stehen.
func (s *Schedule) Normalize() {
	s.ID = strings.TrimSpace(s.ID)
	s.Scene = strings.TrimSpace(s.Scene)
	s.Start = strings.TrimSpace(s.Start)
	s.Stop = strings.TrimSpace(s.Stop)
	days := make([]string, 0, len(s.Days))
	for _, d := range s.Days {
		days = append(days, strings.ToLower(strings.TrimSpace(d)))
	}
	s.Days = sortDays(days)
}

func sortDays(days []string) []string {
	rank := make(map[string]int, len(DayOrder))
	for i, d := range DayOrder {
		rank[d] = i
	}
	out := append([]string(nil), days...)
	// Insertion-Sort: max. 7 Eintraege, unbekannte ans Ende.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && dayRank(rank, out[j]) < dayRank(rank, out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func dayRank(rank map[string]int, d string) int {
	if r, ok := rank[d]; ok {
		return r
	}
	return len(rank)
}

// Validate prueft einen Zeitplan (ohne Szenen-Existenz -- das prueft config).
func (s Schedule) Validate() error {
	if !idPattern.MatchString(s.ID) {
		if s.ID == "" {
			return fmt.Errorf("schedule without id")
		}
		return fmt.Errorf("schedule id %q invalid (only a-z, 0-9, hyphen)", s.ID)
	}
	if s.Scene == "" {
		return fmt.Errorf("schedule %q: scene missing", s.ID)
	}
	if len(s.Days) == 0 {
		return fmt.Errorf("schedule %q: at least one weekday required", s.ID)
	}
	seen := make(map[string]bool, len(s.Days))
	for _, d := range s.Days {
		if _, ok := dayKeys[d]; !ok {
			return fmt.Errorf("schedule %q: unknown weekday %q (allowed: %s)",
				s.ID, d, strings.Join(DayOrder, ", "))
		}
		if seen[d] {
			return fmt.Errorf("schedule %q: weekday %q duplicated", s.ID, d)
		}
		seen[d] = true
	}
	if _, err := parseClock(s.Start); err != nil {
		return fmt.Errorf("zeitplan %q: start %v", s.ID, err)
	}
	if s.Stop != "" {
		if _, err := parseClock(s.Stop); err != nil {
			return fmt.Errorf("zeitplan %q: stop %v", s.ID, err)
		}
		if s.Stop == s.Start {
			return fmt.Errorf("schedule %q: stop must differ from start", s.ID)
		}
	}
	return nil
}

// LoadLocation loest die Config-Zeitzone auf (leer = DefaultTimezone).
func LoadLocation(name string) (*time.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = DefaultTimezone
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("time zone %q unknown (IANA name such as Europe/Vienna expected)", name)
	}
	return loc, nil
}

// parseClock wandelt "HH:MM" in Minuten seit Mitternacht.
func parseClock(v string) (int, error) {
	m := clockPattern.FindStringSubmatch(v)
	if m == nil {
		return 0, fmt.Errorf("%q is not a time in format HH:MM (00:00-23:59)", v)
	}
	h := int(m[1][0]-'0')*10 + int(m[1][1]-'0')
	mi := int(m[2][0]-'0')*10 + int(m[2][1]-'0')
	return h*60 + mi, nil
}

// window ist ein konkretes Start/Stop-Paar an einem Tag.
type window struct {
	start time.Time
	stop  time.Time // Zero, wenn der Zeitplan kein Stop hat
}

// windowOn liefert das Fenster, das am Kalendertag von day (in loc) startet,
// falls der Zeitplan an diesem Wochentag aktiv ist.
func (s Schedule) windowOn(day time.Time, loc *time.Location) (window, bool) {
	y, mo, d := day.In(loc).Date()
	date := time.Date(y, mo, d, 0, 0, 0, 0, loc)
	if !s.hasDay(date.Weekday()) {
		return window{}, false
	}
	startMin, err := parseClock(s.Start)
	if err != nil {
		return window{}, false
	}
	w := window{start: atMinute(y, mo, d, startMin, loc)}
	if s.Stop != "" {
		stopMin, err := parseClock(s.Stop)
		if err != nil {
			return window{}, false
		}
		if stopMin > startMin {
			w.stop = atMinute(y, mo, d, stopMin, loc)
		} else {
			w.stop = atMinute(y, mo, d+1, stopMin, loc)
		}
	}
	return w, true
}

// atMinute baut eine Wanduhrzeit; time.Date normalisiert DST-Luecken
// (z. B. 02:30 am Umstellungstag wird 03:30).
func atMinute(y int, mo time.Month, d, minute int, loc *time.Location) time.Time {
	return time.Date(y, mo, d, minute/60, minute%60, 0, 0, loc)
}

func (s Schedule) hasDay(wd time.Weekday) bool {
	for _, d := range s.Days {
		if dayKeys[d] == wd {
			return true
		}
	}
	return false
}

// windowsAround liefert Fenster, deren Starttag zwischen from-1 Tag und
// from+span Tagen liegt -- genug fuer "naechstes Ereignis" und Fensterpruefung.
func (s Schedule) windowsAround(from time.Time, span int, loc *time.Location) []window {
	base := from.In(loc)
	y, mo, d := base.Date()
	var out []window
	for i := -1; i <= span; i++ {
		day := time.Date(y, mo, d+i, 12, 0, 0, 0, loc)
		if w, ok := s.windowOn(day, loc); ok {
			out = append(out, w)
		}
	}
	return out
}

// activeWindow liefert das Fenster, in dem t gerade liegt (start <= t < stop).
// Zeitplaene ohne Stop haben kein Fenster.
func (s Schedule) activeWindow(t time.Time, loc *time.Location) (window, bool) {
	if s.Stop == "" {
		return window{}, false
	}
	for _, w := range s.windowsAround(t, 0, loc) {
		if !t.Before(w.start) && t.Before(w.stop) {
			return w, true
		}
	}
	return window{}, false
}

// nextWindow liefert das erste Fenster, das nach t startet.
func (s Schedule) nextWindow(t time.Time, loc *time.Location) (window, bool) {
	for _, w := range s.windowsAround(t, 8, loc) {
		if w.start.After(t) {
			return w, true
		}
	}
	return window{}, false
}

// EventKind unterscheidet Start- und Stop-Ereignisse.
type EventKind string

const (
	EventStart EventKind = "start"
	EventStop  EventKind = "stop"
)

// event ist ein konkreter Ausloesezeitpunkt.
type event struct {
	at       time.Time
	kind     EventKind
	schedule Schedule
}

// eventsBetween liefert Start/Stop-Ereignisse mit from < at <= to.
func (s Schedule) eventsBetween(from, to time.Time, loc *time.Location) []event {
	span := int(to.Sub(from).Hours()/24) + 1
	var out []event
	for _, w := range s.windowsAround(from, span, loc) {
		if w.start.After(from) && !w.start.After(to) {
			out = append(out, event{at: w.start, kind: EventStart, schedule: s})
		}
		if !w.stop.IsZero() && w.stop.After(from) && !w.stop.After(to) {
			out = append(out, event{at: w.stop, kind: EventStop, schedule: s})
		}
	}
	return out
}

// nextEventAfter liefert den naechsten Start- oder Stop-Zeitpunkt nach t.
func (s Schedule) nextEventAfter(t time.Time, loc *time.Location) (time.Time, bool) {
	var best time.Time
	for _, w := range s.windowsAround(t, 8, loc) {
		for _, at := range []time.Time{w.start, w.stop} {
			if at.IsZero() || !at.After(t) {
				continue
			}
			if best.IsZero() || at.Before(best) {
				best = at
			}
		}
	}
	return best, !best.IsZero()
}
