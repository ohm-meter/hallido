package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/speaker"
)

const (
	// maxWait begrenzt jede Wartezeit. Der Scheduler wacht spaetestens so oft
	// auf und vergleicht die Wanduhr -- so fallen Uhrspruenge (NTP-Sync nach
	// dem Booten eines Pi ohne RTC, Suspend) auf.
	maxWait = 30 * time.Second
	// jumpThreshold: liegt seit dem letzten Aufwachen mehr als das zwischen
	// den Pruefungen, gilt das als Uhrsprung -> Catch-up statt alte
	// Ereignisse nachzufeuern.
	jumpThreshold = maxWait + 2*time.Minute
	// actionTimeout entspricht dem Play-Timeout der API.
	actionTimeout = 45 * time.Second
)

// Runner ist der Ausschnitt des Szenen-Service, den der Scheduler braucht.
type Runner interface {
	Play(ctx context.Context, id string) (*scenes.PlayResult, error)
	Stop(ctx context.Context, id string) error
	SceneStatus(ctx context.Context, id string) (scenes.ScenePlaybackStatus, error)
}

// RunInfo beschreibt die letzte Ausfuehrung eines Zeitplans.
//
// action: start | stop | catch_up | stop_skipped | catch_up_skipped
type RunInfo struct {
	At     time.Time `json:"at"`
	Action string    `json:"action"`
	OK     bool      `json:"ok"`
	Detail string    `json:"detail,omitempty"`
	Error  string    `json:"error,omitempty"`
}

// Status ist der Laufzeit-Blick auf einen Zeitplan (GET /api/v1/schedules).
type Status struct {
	ID        string     `json:"id"`
	Scene     string     `json:"scene"`
	Days      []string   `json:"days"`
	Start     string     `json:"start"`
	Stop      string     `json:"stop,omitempty"`
	Enabled   bool       `json:"enabled"`
	Active    bool       `json:"active"`
	NextStart *time.Time `json:"next_start,omitempty"`
	NextStop  *time.Time `json:"next_stop,omitempty"`
	LastRun   *RunInfo   `json:"last_run,omitempty"`
}

// Scheduler fuehrt Zeitplaene aus. Nebenlaeufig sicher.
type Scheduler struct {
	runner Runner
	clock  Clock
	loc    atomic.Pointer[time.Location] // replaceable (first-run setup sets the time zone)
	log    *slog.Logger

	mu     sync.Mutex
	list   []Schedule
	last   map[string]RunInfo
	reload chan struct{}
}

// New erstellt einen Scheduler. clock nil = echte Uhr, loc nil = DefaultTimezone.
func New(list []Schedule, loc *time.Location, runner Runner, clock Clock, log *slog.Logger) *Scheduler {
	if clock == nil {
		clock = RealClock{}
	}
	if loc == nil {
		loc, _ = LoadLocation("")
	}
	s := &Scheduler{
		runner: runner,
		clock:  clock,
		log:    log,
		list:   cloneList(list),
		last:   map[string]RunInfo{},
		reload: make(chan struct{}, 1),
	}
	s.loc.Store(loc)
	return s
}

func cloneList(in []Schedule) []Schedule {
	out := make([]Schedule, len(in))
	for i, s := range in {
		s.Days = append([]string(nil), s.Days...)
		out[i] = s
	}
	return out
}

// Location liefert die Zeitzone der Zeitplaene.
func (s *Scheduler) Location() *time.Location { return s.loc.Load() }

// SetLocation changes the time zone of all schedules (first-run setup).
func (s *Scheduler) SetLocation(loc *time.Location) {
	if loc == nil {
		return
	}
	s.loc.Store(loc)
	select {
	case s.reload <- struct{}{}:
	default:
	}
}

// Now liefert die aktuelle Zeit der Scheduler-Uhr in dessen Zeitzone.
func (s *Scheduler) Now() time.Time { return s.clock.Now().In(s.Location()) }

// Reload ersetzt die Zeitplaene (Hot-Reload nach Config-Schreiben). Laufende
// Fenster werden nicht neu gestartet; ein Catch-up gibt es nur beim Start.
func (s *Scheduler) Reload(list []Schedule) {
	s.mu.Lock()
	s.list = cloneList(list)
	ids := make(map[string]bool, len(list))
	for _, sc := range list {
		ids[sc.ID] = true
	}
	for id := range s.last {
		if !ids[id] {
			delete(s.last, id)
		}
	}
	s.mu.Unlock()
	select {
	case s.reload <- struct{}{}:
	default:
	}
}

func (s *Scheduler) snapshot() []Schedule {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneList(s.list)
}

// Statuses liefert alle Zeitplaene mit naechstem Ausloesezeitpunkt,
// sortiert nach naechstem Start (deaktivierte zuletzt).
func (s *Scheduler) Statuses() []Status {
	now := s.Now()
	list := s.snapshot()
	s.mu.Lock()
	last := make(map[string]RunInfo, len(s.last))
	for k, v := range s.last {
		last[k] = v
	}
	s.mu.Unlock()

	out := make([]Status, 0, len(list))
	for _, sc := range list {
		st := Status{
			ID: sc.ID, Scene: sc.Scene, Days: sc.Days,
			Start: sc.Start, Stop: sc.Stop, Enabled: sc.Enabled,
		}
		if ri, ok := last[sc.ID]; ok {
			ri := ri
			st.LastRun = &ri
		}
		if sc.Enabled {
			if w, ok := sc.activeWindow(now, s.Location()); ok {
				st.Active = true
				stop := w.stop.In(s.Location())
				st.NextStop = &stop
			}
			if w, ok := sc.nextWindow(now, s.Location()); ok {
				start := w.start.In(s.Location())
				st.NextStart = &start
				if st.NextStop == nil && !w.stop.IsZero() {
					stop := w.stop.In(s.Location())
					st.NextStop = &stop
				}
			}
		}
		out = append(out, st)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].NextStart, out[j].NextStart
		switch {
		case a == nil && b == nil:
			return out[i].ID < out[j].ID
		case a == nil:
			return false
		case b == nil:
			return true
		default:
			return a.Before(*b)
		}
	})
	return out
}

// Run fuehrt die Zeitplaene aus, bis ctx endet. Beim Start werden Szenen,
// deren Fenster gerade laeuft, nachgeholt (Catch-up).
func (s *Scheduler) Run(ctx context.Context) {
	s.log.Info("scheduler started", "timezone", s.Location().String(), "schedules", len(s.snapshot()))
	last := s.clock.Now()
	s.catchUp(ctx, last)

	for {
		now := s.clock.Now()
		switch gap := now.Sub(last); {
		case gap < 0:
			s.log.Warn("clock moved backwards -- no events replayed", "gap", gap)
		case gap > jumpThreshold:
			s.log.Warn("clock jump detected -- catch-up instead of replay", "gap", gap.Round(time.Second))
			s.catchUp(ctx, now)
		default:
			s.fire(ctx, last, now)
		}
		last = now
		if ctx.Err() != nil {
			s.log.Info("scheduler stopped")
			return
		}

		wait := maxWait
		if next, ok := s.nextEvent(now); ok && next.Sub(now) < wait {
			wait = next.Sub(now)
		}
		t := s.clock.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			s.log.Info("scheduler stopped")
			return
		case <-s.reload:
			t.Stop()
		case <-t.C():
		}
	}
}

func (s *Scheduler) nextEvent(after time.Time) (time.Time, bool) {
	var best time.Time
	for _, sc := range s.snapshot() {
		if !sc.Enabled {
			continue
		}
		if at, ok := sc.nextEventAfter(after, s.Location()); ok && (best.IsZero() || at.Before(best)) {
			best = at
		}
	}
	return best, !best.IsZero()
}

// fire fuehrt alle Ereignisse mit from < at <= to der Reihe nach aus.
func (s *Scheduler) fire(ctx context.Context, from, to time.Time) {
	var events []event
	for _, sc := range s.snapshot() {
		if sc.Enabled {
			events = append(events, sc.eventsBetween(from, to, s.Location())...)
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].at.Before(events[j].at) })
	for _, ev := range events {
		if ctx.Err() != nil {
			return
		}
		switch ev.kind {
		case EventStart:
			s.start(ctx, ev.schedule, "start")
		case EventStop:
			s.stop(ctx, ev.schedule)
		}
	}
}

// catchUp startet Szenen, deren Fenster zum Zeitpunkt now laeuft. Spielt die
// Szene schon (Neustart des Agents), laeuft manuell etwas anderes oder hat
// jemand pausiert, bleibt alles wie es ist (eine Pause ist eine bewusste
// Entscheidung, die ein Agent-Neustart nicht ueberstimmen soll).
// Zeitplaene ohne Stop haben kein Fenster -> kein Catch-up.
func (s *Scheduler) catchUp(ctx context.Context, now time.Time) {
	for _, sc := range s.snapshot() {
		if !sc.Enabled {
			continue
		}
		if _, ok := sc.activeWindow(now, s.Location()); !ok {
			continue
		}
		st, err := s.status(ctx, sc.Scene)
		if err == nil && skipCatchUp(st.State) {
			s.record(sc.ID, RunInfo{Action: "catch_up_skipped", OK: true,
				Detail: "Scene speaker already playing or paused (" + st.State + ")"})
			s.log.Info("schedule: catch-up skipped", "schedule", sc.ID, "scene", sc.Scene, "state", st.State)
			continue
		}
		s.start(ctx, sc, "catch_up")
	}
}

func skipCatchUp(state string) bool {
	switch state {
	case string(speaker.PlaybackPlaying), "playing_other", string(speaker.PlaybackPaused):
		return true
	}
	return false
}

func (s *Scheduler) start(ctx context.Context, sc Schedule, action string) {
	actx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()
	res, err := s.runner.Play(actx, sc.Scene)
	info := RunInfo{Action: action, OK: err == nil}
	if err != nil {
		info.Error = apperr.As(err).Message
		s.log.Warn("schedule: scene could not start", "schedule", sc.ID, "scene", sc.Scene,
			"action", action, "err", err)
	}
	if res != nil {
		var failed []string
		for _, sp := range res.Speakers {
			if !sp.OK {
				failed = append(failed, sp.Speaker)
				s.log.Warn("schedule: speaker down (partial failure)", "schedule", sc.ID,
					"speaker", sp.Speaker, "err", sp.Error)
			}
		}
		if len(failed) > 0 && err == nil {
			info.Detail = fmt.Sprintf("%d of %d speakers unreachable: %v",
				len(failed), len(res.Speakers), failed)
		}
	}
	if err == nil {
		s.log.Info("schedule: scene started", "schedule", sc.ID, "scene", sc.Scene, "action", action)
	}
	s.record(sc.ID, info)
}

// stop stoppt die Szene nur, wenn sie laut Status noch selbst spielt. Wurde
// manuell auf etwas anderes umgeschaltet (playing_other), bleibt es laufen;
// ist sie pausiert (paused), gibt es nichts zu stoppen -- die Pause bleibt,
// ein spaeteres Resume setzt also auch nach dem Fensterende fort.
func (s *Scheduler) stop(ctx context.Context, sc Schedule) {
	actx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()
	st, err := s.status(actx, sc.Scene)
	if err != nil {
		s.record(sc.ID, RunInfo{Action: "stop", Error: apperr.As(err).Message})
		s.log.Warn("schedule: status before stop failed", "schedule", sc.ID, "scene", sc.Scene, "err", err)
		return
	}
	if st.State != string(speaker.PlaybackPlaying) {
		s.record(sc.ID, RunInfo{Action: "stop_skipped", OK: true,
			Detail: "Scene no longer playing (" + st.State + ")"})
		s.log.Info("schedule: stop skipped", "schedule", sc.ID, "scene", sc.Scene, "state", st.State)
		return
	}
	if err := s.runner.Stop(actx, sc.Scene); err != nil {
		s.record(sc.ID, RunInfo{Action: "stop", Error: apperr.As(err).Message})
		s.log.Warn("schedule: scene could not stop", "schedule", sc.ID, "scene", sc.Scene, "err", err)
		return
	}
	s.log.Info("schedule: scene stopped", "schedule", sc.ID, "scene", sc.Scene)
	s.record(sc.ID, RunInfo{Action: "stop", OK: true})
}

func (s *Scheduler) status(ctx context.Context, sceneID string) (scenes.ScenePlaybackStatus, error) {
	actx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()
	return s.runner.SceneStatus(actx, sceneID)
}

func (s *Scheduler) record(id string, info RunInfo) {
	info.At = s.Now()
	s.mu.Lock()
	s.last[id] = info
	s.mu.Unlock()
}
