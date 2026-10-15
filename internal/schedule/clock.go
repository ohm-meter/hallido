package schedule

import (
	"sort"
	"sync"
	"time"
)

// Clock abstrahiert die Uhr, damit Tests ohne echte Sleeps laufen.
type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) Timer
}

// Timer ist ein stoppbarer Einmal-Timer.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

// RealClock nutzt time.Now und time.NewTimer.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

func (RealClock) NewTimer(d time.Duration) Timer { return realTimer{time.NewTimer(d)} }

type realTimer struct{ t *time.Timer }

func (r realTimer) C() <-chan time.Time { return r.t.C }
func (r realTimer) Stop() bool          { return r.t.Stop() }

// FakeClock ist eine manuell vorgestellte Uhr fuer Tests.
type FakeClock struct {
	mu      sync.Mutex
	cond    *sync.Cond
	now     time.Time
	timers  []*fakeTimer
	created int
}

// NewFakeClock erstellt eine Test-Uhr mit Startzeit now.
func NewFakeClock(now time.Time) *FakeClock {
	f := &FakeClock{now: now}
	f.cond = sync.NewCond(&f.mu)
	return f
}

func (f *FakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *FakeClock) NewTimer(d time.Duration) Timer {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := &fakeTimer{clock: f, at: f.now.Add(d), ch: make(chan time.Time, 1)}
	if d <= 0 {
		t.ch <- f.now
	} else {
		f.timers = append(f.timers, t)
	}
	f.created++
	f.cond.Broadcast()
	return t
}

// Advance stellt die Uhr vor und loest faellige Timer aus.
func (f *FakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
	sort.Slice(f.timers, func(i, j int) bool { return f.timers[i].at.Before(f.timers[j].at) })
	keep := f.timers[:0]
	for _, t := range f.timers {
		if !t.at.After(f.now) {
			t.ch <- f.now
			continue
		}
		keep = append(keep, t)
	}
	f.timers = keep
}

// WaitCreated blockiert, bis insgesamt mindestens n Timer erzeugt wurden.
// So wissen Tests, dass der Scheduler eine Runde abgeschlossen hat.
func (f *FakeClock) WaitCreated(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for f.created < n {
		f.cond.Wait()
	}
}

// Created liefert die Anzahl bisher erzeugter Timer.
func (f *FakeClock) Created() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.created
}

type fakeTimer struct {
	clock *FakeClock
	at    time.Time
	ch    chan time.Time
}

func (t *fakeTimer) C() <-chan time.Time { return t.ch }

func (t *fakeTimer) Stop() bool {
	f := t.clock
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, other := range f.timers {
		if other == t {
			f.timers = append(f.timers[:i], f.timers[i+1:]...)
			return true
		}
	}
	return false
}
