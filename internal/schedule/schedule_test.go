package schedule

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func vienna(t *testing.T) *time.Location {
	t.Helper()
	loc, err := LoadLocation("")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func oe3Morgen() Schedule {
	return Schedule{
		ID: "oe3-werktags", Scene: "oe3-morgen",
		Days:  []string{"mon", "tue", "wed", "thu", "fri"},
		Start: "06:10", Stop: "08:30", Enabled: true,
	}
}

func TestValidate(t *testing.T) {
	if err := oe3Morgen().Validate(); err != nil {
		t.Fatalf("gueltiger zeitplan abgelehnt: %v", err)
	}
	cases := map[string]func(*Schedule){
		"id fehlt":          func(s *Schedule) { s.ID = "" },
		"id ungueltig":      func(s *Schedule) { s.ID = "Oe3 Morgen" },
		"szene fehlt":       func(s *Schedule) { s.Scene = "" },
		"keine tage":        func(s *Schedule) { s.Days = nil },
		"unbekannter tag":   func(s *Schedule) { s.Days = []string{"montag"} },
		"tag doppelt":       func(s *Schedule) { s.Days = []string{"mon", "mon"} },
		"start ungueltig":   func(s *Schedule) { s.Start = "6:10" },
		"start 24 uhr":      func(s *Schedule) { s.Start = "24:00" },
		"stop ungueltig":    func(s *Schedule) { s.Stop = "08:60" },
		"stop gleich start": func(s *Schedule) { s.Stop = s.Start },
	}
	for name, mut := range cases {
		s := oe3Morgen()
		mut(&s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s: fehler erwartet", name)
		}
	}
	s := oe3Morgen()
	s.Stop = ""
	if err := s.Validate(); err != nil {
		t.Errorf("stop ist optional: %v", err)
	}
}

func TestUnmarshal_EnabledDefaultsTrueAndStrict(t *testing.T) {
	var s Schedule
	if err := json.Unmarshal([]byte(`{"id":"a","scene":"b","days":["mon"],"start":"06:00"}`), &s); err != nil {
		t.Fatal(err)
	}
	if !s.Enabled {
		t.Fatal("enabled fehlt -> true erwartet")
	}
	if err := json.Unmarshal([]byte(`{"id":"a","scene":"b","days":["mon"],"start":"06:00","enabled":false}`), &s); err != nil || s.Enabled {
		t.Fatalf("enabled=false muss bleiben: %+v %v", s, err)
	}
	err := json.Unmarshal([]byte(`{"id":"a","scene":"b","days":["mon"],"strat":"06:00"}`), &s)
	if err == nil || !strings.Contains(err.Error(), "strat") {
		t.Fatalf("tippfehler im feld muss auffallen: %v", err)
	}
}

func TestNormalize_SortsDays(t *testing.T) {
	s := Schedule{ID: " a ", Days: []string{"SUN", "fri", " mon"}, Start: " 06:00 "}
	s.Normalize()
	if s.ID != "a" || s.Start != "06:00" || strings.Join(s.Days, ",") != "mon,fri,sun" {
		t.Fatalf("normalize: %+v", s)
	}
}

func TestLoadLocation(t *testing.T) {
	if loc, err := LoadLocation(""); err != nil || loc.String() != "Europe/Vienna" {
		t.Fatalf("default: %v %v", loc, err)
	}
	if _, err := LoadLocation("Mars/Olympus"); err == nil {
		t.Fatal("unbekannte zeitzone muss fehler liefern")
	}
}

func TestNextWindow_SkipsWeekend(t *testing.T) {
	loc := vienna(t)
	// Freitag 2026-10-16 09:00 -> naechster Start Montag 06:10.
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, loc)
	w, ok := oe3Morgen().nextWindow(now, loc)
	if !ok {
		t.Fatal("kein fenster")
	}
	want := time.Date(2026, 10, 19, 6, 10, 0, 0, loc)
	if !w.start.Equal(want) || !w.stop.Equal(time.Date(2026, 10, 19, 8, 30, 0, 0, loc)) {
		t.Fatalf("start=%v stop=%v", w.start, w.stop)
	}
}

func TestActiveWindow(t *testing.T) {
	loc := vienna(t)
	s := oe3Morgen()
	in := time.Date(2026, 10, 14, 7, 0, 0, 0, loc) // Mittwoch
	if _, ok := s.activeWindow(in, loc); !ok {
		t.Fatal("07:00 am mittwoch liegt im fenster")
	}
	for _, out := range []time.Time{
		time.Date(2026, 10, 14, 6, 9, 0, 0, loc),
		time.Date(2026, 10, 14, 8, 30, 0, 0, loc), // stop ist exklusiv
		time.Date(2026, 10, 17, 7, 0, 0, 0, loc),  // Samstag
	} {
		if _, ok := s.activeWindow(out, loc); ok {
			t.Errorf("%v sollte ausserhalb liegen", out)
		}
	}
	s.Stop = ""
	if _, ok := s.activeWindow(in, loc); ok {
		t.Fatal("ohne stop gibt es kein fenster")
	}
}

func TestOvernightWindow(t *testing.T) {
	loc := vienna(t)
	s := Schedule{ID: "nacht", Scene: "x", Days: []string{"fri"}, Start: "23:00", Stop: "01:00", Enabled: true}
	sat0030 := time.Date(2026, 10, 17, 0, 30, 0, 0, loc)
	w, ok := s.activeWindow(sat0030, loc)
	if !ok || w.start.Day() != 16 || w.stop.Day() != 17 {
		t.Fatalf("fenster ueber mitternacht: %v %v", w, ok)
	}
	events := s.eventsBetween(time.Date(2026, 10, 16, 22, 0, 0, 0, loc), time.Date(2026, 10, 17, 2, 0, 0, 0, loc), loc)
	if len(events) != 2 || events[0].kind != EventStart || events[1].kind != EventStop {
		t.Fatalf("events: %+v", events)
	}
}

func TestDST_SpringForwardGap(t *testing.T) {
	loc := vienna(t)
	// 2027-03-28: 02:00 -> 03:00 in Wien. 02:30 existiert nicht.
	s := Schedule{ID: "dst", Scene: "x", Days: []string{"sun"}, Start: "02:30", Enabled: true}
	w, ok := s.windowOn(time.Date(2027, 3, 28, 12, 0, 0, 0, loc), loc)
	if !ok || w.start.Hour() != 3 || w.start.Minute() != 30 {
		t.Fatalf("dst-luecke: %v", w.start)
	}
	// Normaler Start am Umstellungstag bleibt Wanduhrzeit.
	m := oe3Morgen()
	m.Days = []string{"sun"}
	w, _ = m.windowOn(time.Date(2027, 3, 28, 12, 0, 0, 0, loc), loc)
	if w.start.Hour() != 6 || w.start.Minute() != 10 {
		t.Fatalf("06:10 bleibt 06:10: %v", w.start)
	}
}

func TestEventsBetween_ExclusiveFromInclusiveTo(t *testing.T) {
	loc := vienna(t)
	start := time.Date(2026, 10, 14, 6, 10, 0, 0, loc)
	s := oe3Morgen()
	if ev := s.eventsBetween(start.Add(-time.Second), start, loc); len(ev) != 1 {
		t.Fatalf("start im intervall erwartet: %+v", ev)
	}
	if ev := s.eventsBetween(start, start.Add(time.Minute), loc); len(ev) != 0 {
		t.Fatalf("from ist exklusiv: %+v", ev)
	}
}
