package schedule

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/scenes"
)

// fakeRunner simuliert den Szenen-Service.
type fakeRunner struct {
	mu      sync.Mutex
	state   map[string]string // scene -> status
	plays   []string
	stops   []string
	playErr error
	partial []string // boxen, die beim play ausfallen
}

func newFakeRunner() *fakeRunner { return &fakeRunner{state: map[string]string{}} }

func (f *fakeRunner) Play(_ context.Context, id string) (*scenes.PlayResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.plays = append(f.plays, id)
	res := &scenes.PlayResult{SceneID: id, Speakers: []scenes.SpeakerStatus{{Speaker: "Wohnzimmer", OK: true}}}
	for _, p := range f.partial {
		res.Speakers = append(res.Speakers, scenes.SpeakerStatus{Speaker: p, Error: "offline"})
	}
	if f.playErr != nil {
		return res, f.playErr
	}
	res.Playing = true
	f.state[id] = "playing"
	return res, nil
}

func (f *fakeRunner) Stop(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops = append(f.stops, id)
	f.state[id] = "stopped"
	return nil
}

func (f *fakeRunner) SceneStatus(_ context.Context, id string) (scenes.ScenePlaybackStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := f.state[id]
	if st == "" {
		st = "stopped"
	}
	return scenes.ScenePlaybackStatus{SceneID: id, State: st}, nil
}

func (f *fakeRunner) setState(id, st string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state[id] = st
}

func (f *fakeRunner) counts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.plays), len(f.stops)
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// harness startet Run mit Fake-Uhr und bietet Schritt-Helfer.
type harness struct {
	t      *testing.T
	clock  *FakeClock
	runner *fakeRunner
	sch    *Scheduler
	cancel context.CancelFunc
	done   chan struct{}
	rounds int
}

func startHarness(t *testing.T, now time.Time, list []Schedule, runner *fakeRunner) *harness {
	t.Helper()
	h := &harness{t: t, clock: NewFakeClock(now), runner: runner, done: make(chan struct{})}
	h.sch = New(list, now.Location(), runner, h.clock, quiet())
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go func() { h.sch.Run(ctx); close(h.done) }()
	h.rounds = 1
	h.clock.WaitCreated(h.rounds) // Catch-up + erste Runde fertig
	t.Cleanup(h.stop)
	return h
}

// advance stellt die Uhr in maxWait-Schritten vor und wartet jede Runde ab.
func (h *harness) advance(d time.Duration) {
	for d > 0 {
		step := maxWait
		if d < step {
			step = d
		}
		h.clock.Advance(step)
		h.rounds++
		h.clock.WaitCreated(h.rounds)
		d -= step
	}
}

func (h *harness) stop() {
	h.cancel()
	<-h.done
}

func wed(t *testing.T, hour, minute int) time.Time {
	return time.Date(2026, 10, 14, hour, minute, 0, 0, vienna(t))
}

func TestRun_StartsAndStopsInWindow(t *testing.T) {
	r := newFakeRunner()
	h := startHarness(t, wed(t, 6, 0), []Schedule{oe3Morgen()}, r)

	h.advance(9*time.Minute + 30*time.Second)
	if p, _ := r.counts(); p != 0 {
		t.Fatalf("vor 06:10 kein start, plays=%d", p)
	}
	h.advance(30 * time.Second)
	if p, _ := r.counts(); p != 1 {
		t.Fatalf("06:10 -> start erwartet, plays=%d", p)
	}
	h.advance(2*time.Hour + 20*time.Minute)
	if p, s := r.counts(); p != 1 || s != 1 {
		t.Fatalf("08:30 -> stop erwartet, plays=%d stops=%d", p, s)
	}
	st := h.sch.Statuses()[0]
	if st.LastRun == nil || st.LastRun.Action != "stop" || !st.LastRun.OK {
		t.Fatalf("last_run: %+v", st.LastRun)
	}
}

func TestRun_StopSkippedWhenPlayingOther(t *testing.T) {
	r := newFakeRunner()
	h := startHarness(t, wed(t, 8, 0), []Schedule{oe3Morgen()}, r)
	// Catch-up startet die Szene (07:00-Fall), dann schaltet jemand um.
	if p, _ := r.counts(); p != 1 {
		t.Fatalf("catch-up erwartet, plays=%d", p)
	}
	r.setState("oe3-morgen", "playing_other")
	h.advance(30 * time.Minute)
	if _, s := r.counts(); s != 0 {
		t.Fatalf("playing_other darf nicht gestoppt werden, stops=%d", s)
	}
	if lr := h.sch.Statuses()[0].LastRun; lr == nil || lr.Action != "stop_skipped" {
		t.Fatalf("last_run: %+v", lr)
	}
}

func TestRun_CatchUpOnlyInsideWindow(t *testing.T) {
	r := newFakeRunner()
	startHarness(t, wed(t, 7, 0), []Schedule{oe3Morgen()}, r).stop()
	if p, _ := r.counts(); p != 1 {
		t.Fatalf("neustart 07:00 -> catch-up, plays=%d", p)
	}

	r2 := newFakeRunner()
	startHarness(t, wed(t, 9, 0), []Schedule{oe3Morgen()}, r2).stop()
	if p, _ := r2.counts(); p != 0 {
		t.Fatalf("09:00 ausserhalb -> kein catch-up, plays=%d", p)
	}

	r3 := newFakeRunner()
	r3.setState("oe3-morgen", "playing")
	startHarness(t, wed(t, 7, 0), []Schedule{oe3Morgen()}, r3).stop()
	if p, _ := r3.counts(); p != 0 {
		t.Fatalf("spielt schon -> kein erneuter start, plays=%d", p)
	}

	r4 := newFakeRunner()
	noStop := oe3Morgen()
	noStop.Stop = ""
	startHarness(t, wed(t, 7, 0), []Schedule{noStop}, r4).stop()
	if p, _ := r4.counts(); p != 0 {
		t.Fatalf("ohne stop kein fenster -> kein catch-up, plays=%d", p)
	}
}

func TestRun_DisabledScheduleDoesNothing(t *testing.T) {
	r := newFakeRunner()
	s := oe3Morgen()
	s.Enabled = false
	h := startHarness(t, wed(t, 7, 0), []Schedule{s}, r)
	h.advance(2 * time.Hour)
	if p, st := r.counts(); p != 0 || st != 0 {
		t.Fatalf("deaktiviert: plays=%d stops=%d", p, st)
	}
	if h.sch.Statuses()[0].NextStart != nil {
		t.Fatal("deaktiviert -> kein next_start")
	}
}

func TestRun_ReloadAppliesNewSchedule(t *testing.T) {
	r := newFakeRunner()
	h := startHarness(t, wed(t, 9, 0), nil, r)
	s := oe3Morgen()
	s.Start, s.Stop = "09:05", ""
	created := h.clock.Created()
	h.sch.Reload([]Schedule{s})
	h.clock.WaitCreated(created + 1) // Loop hat neu geplant
	h.rounds = h.clock.Created()
	if p, _ := r.counts(); p != 0 {
		t.Fatal("reload darf nicht sofort starten")
	}
	h.advance(5 * time.Minute)
	if p, _ := r.counts(); p != 1 {
		t.Fatalf("09:05 nach reload -> start, plays=%d", p)
	}
}

func TestRun_PartialFailureIsRecordedNotFatal(t *testing.T) {
	r := newFakeRunner()
	r.partial = []string{"Bad"}
	h := startHarness(t, wed(t, 6, 9), []Schedule{oe3Morgen()}, r)
	h.advance(time.Minute)
	lr := h.sch.Statuses()[0].LastRun
	if lr == nil || !lr.OK || lr.Detail == "" {
		t.Fatalf("teilausfall: ok mit detail erwartet, bekam %+v", lr)
	}
}

func TestRun_PlayErrorRecorded(t *testing.T) {
	r := newFakeRunner()
	r.playErr = apperr.New(apperr.CodeSpeakerUnreachable, "Coordinator offline")
	h := startHarness(t, wed(t, 6, 9), []Schedule{oe3Morgen()}, r)
	h.advance(time.Minute)
	lr := h.sch.Statuses()[0].LastRun
	if lr == nil || lr.OK || lr.Error != "Coordinator offline" {
		t.Fatalf("fehler erwartet: %+v", lr)
	}
}

func TestRun_ClockJumpTriggersCatchUpNotReplay(t *testing.T) {
	r := newFakeRunner()
	// Pi bootet mit alter Zeit (fake-hwclock), NTP springt dann auf 07:00.
	h := startHarness(t, wed(t, 5, 0), []Schedule{oe3Morgen()}, r)
	h.clock.Advance(2 * time.Hour)
	h.rounds++
	h.clock.WaitCreated(h.rounds)
	if p, _ := r.counts(); p != 1 {
		t.Fatalf("uhrsprung ins fenster -> genau ein catch-up, plays=%d", p)
	}
	if lr := h.sch.Statuses()[0].LastRun; lr == nil || lr.Action != "catch_up" {
		t.Fatalf("last_run: %+v", lr)
	}
}

func TestRun_ShutdownViaContext(t *testing.T) {
	r := newFakeRunner()
	h := startHarness(t, wed(t, 6, 0), []Schedule{oe3Morgen()}, r)
	h.cancel()
	select {
	case <-h.done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run endet nicht nach ctx-cancel")
	}
}

func TestStatuses_NextStartAndSorting(t *testing.T) {
	loc := vienna(t)
	clock := NewFakeClock(wed(t, 7, 0))
	later := oe3Morgen()
	later.ID, later.Start, later.Stop = "abend", "18:00", ""
	off := oe3Morgen()
	off.ID, off.Enabled = "aus", false
	sch := New([]Schedule{off, later, oe3Morgen()}, loc, newFakeRunner(), clock, quiet())

	got := sch.Statuses()
	if got[0].ID != "abend" || got[1].ID != "oe3-werktags" || got[2].ID != "aus" {
		t.Fatalf("sortierung: %s %s %s", got[0].ID, got[1].ID, got[2].ID)
	}
	if !got[0].NextStart.Equal(wed(t, 18, 0)) || got[0].NextStop != nil {
		t.Fatalf("abend: %+v", got[0])
	}
	oe := got[1]
	if !oe.Active || !oe.NextStop.Equal(wed(t, 8, 30)) || !oe.NextStart.Equal(time.Date(2026, 10, 15, 6, 10, 0, 0, loc)) {
		t.Fatalf("oe3 aktiv: %+v", oe)
	}
}

// Pausiert = nicht mehr playing: zum Stop-Zeitpunkt passiert nichts.
func TestRun_StopSkippedWhenPaused(t *testing.T) {
	r := newFakeRunner()
	h := startHarness(t, wed(t, 8, 0), []Schedule{oe3Morgen()}, r)
	if p, _ := r.counts(); p != 1 {
		t.Fatalf("catch-up erwartet, plays=%d", p)
	}
	r.setState("oe3-morgen", "paused")
	h.advance(30 * time.Minute)
	if _, s := r.counts(); s != 0 {
		t.Fatalf("pausierte szene darf nicht gestoppt werden, stops=%d", s)
	}
	lr := h.sch.Statuses()[0].LastRun
	if lr == nil || lr.Action != "stop_skipped" || !lr.OK || !strings.Contains(lr.Detail, "paused") {
		t.Fatalf("last_run: %+v", lr)
	}
}

// Neustart im Fenster bei pausierter Szene: kein Catch-up, Pause bleibt.
func TestRun_CatchUpSkippedWhenPaused(t *testing.T) {
	r := newFakeRunner()
	r.setState("oe3-morgen", "paused")
	h := startHarness(t, wed(t, 7, 0), []Schedule{oe3Morgen()}, r)
	if p, _ := r.counts(); p != 0 {
		t.Fatalf("pausierte szene darf nicht neu gestartet werden, plays=%d", p)
	}
	if lr := h.sch.Statuses()[0].LastRun; lr == nil || lr.Action != "catch_up_skipped" {
		t.Fatalf("last_run: %+v", lr)
	}
}
