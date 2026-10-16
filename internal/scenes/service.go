package scenes

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

// SpeakerStatus berichtet pro Lautsprecher, ob die Aktion geklappt hat.
type SpeakerStatus struct {
	Speaker string `json:"speaker"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

// PlayResult fasst das Ergebnis eines Szenen-Starts zusammen.
type PlayResult struct {
	SceneID  string          `json:"scene_id"`
	Playing  bool            `json:"playing"`
	Speakers []SpeakerStatus `json:"speakers"`
}

// ScenePlaybackStatus beschreibt den aktuellen Wiedergabestatus einer Szene.
// state: playing | playing_other | stopped | paused | unknown | offline
type ScenePlaybackStatus struct {
	SceneID         string `json:"scene_id"`
	Coordinator     string `json:"coordinator"`
	State           string `json:"state"`
	NowPlayingTitle string `json:"now_playing_title,omitempty"`
	Error           string `json:"error,omitempty"`
}

// Service haelt die Szenen und steuert Lautsprecher ueber den Controller.
// Nebenlaeufig sicher: API-Handler und Scheduler greifen parallel zu.
type Service struct {
	mu     sync.RWMutex // schuetzt scenes (Reload laeuft parallel zu Play/List)
	boxMu  sync.Mutex   // schuetzt boxLocks
	boxes  map[string]*sync.Mutex
	scenes map[string]Scene
	ctrl   speaker.SpeakerController
	log    *slog.Logger
}

// NewService erstellt den Service aus einer Szenen-Liste.
func NewService(list []Scene, ctrl speaker.SpeakerController, log *slog.Logger) *Service {
	m := make(map[string]Scene, len(list))
	for _, s := range list {
		m[s.ID] = s
	}
	return &Service{scenes: m, ctrl: ctrl, log: log, boxes: map[string]*sync.Mutex{}}
}

// Reload ersetzt die geladenen Szenen (Hot-Reload nach Config-Schreiben).
func (s *Service) Reload(list []Scene) error {
	m := make(map[string]Scene, len(list))
	for _, sc := range list {
		if err := sc.Validate(); err != nil {
			return err
		}
		m[sc.ID] = sc
	}
	s.mu.Lock()
	s.scenes = m
	s.mu.Unlock()
	return nil
}

// List liefert alle Szenen (Reihenfolge nicht garantiert).
func (s *Service) List() []Scene {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Scene, 0, len(s.scenes))
	for _, sc := range s.scenes {
		out = append(out, sc)
	}
	return out
}

// PlaybackStatus liefert den AVTransport-Status je konfigurierter Szene.
func (s *Service) PlaybackStatus(ctx context.Context) []ScenePlaybackStatus {
	list := s.List()
	out := make([]ScenePlaybackStatus, 0, len(list))
	for _, sc := range list {
		out = append(out, s.sceneStatus(ctx, sc))
	}
	return out
}

// SceneStatus liefert den Wiedergabestatus genau einer Szene.
func (s *Service) SceneStatus(ctx context.Context, id string) (ScenePlaybackStatus, error) {
	sc, ok := s.Get(id)
	if !ok {
		return ScenePlaybackStatus{}, sceneNotFound()
	}
	return s.sceneStatus(ctx, sc), nil
}

func (s *Service) sceneStatus(ctx context.Context, sc Scene) ScenePlaybackStatus {
	coordinator := sc.Coordinator()
	st := ScenePlaybackStatus{
		SceneID:     sc.ID,
		Coordinator: coordinator,
		State:       string(speaker.PlaybackUnknown),
	}
	if coordinator == "" {
		st.Error = "Scene without coordinator"
		return st
	}
	transport, err := s.ctrl.GetTransportState(ctx, coordinator)
	if err != nil {
		ae := apperr.As(err)
		switch ae.Code {
		case apperr.CodeSpeakerUnreachable:
			st.State = "offline"
			st.Error = ae.Message
		default:
			st.State = string(speaker.PlaybackUnknown)
		}
		return st
	}
	if transport != speaker.PlaybackPlaying {
		st.State = string(transport)
		return st
	}

	np, npErr := s.ctrl.GetNowPlaying(ctx, coordinator)
	if npErr != nil {
		st.State = string(speaker.PlaybackPlaying)
		st.Error = apperr.As(npErr).Message
		return st
	}
	st.NowPlayingTitle = np.Title
	if speaker.SourceMatchesPlayback(sc.Source, np) {
		st.State = string(speaker.PlaybackPlaying)
	} else {
		st.State = "playing_other"
	}
	return st
}

// Get liefert eine Szene per ID.
func (s *Service) Get(id string) (Scene, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sc, ok := s.scenes[id]
	return sc, ok
}

// IDs liefert alle Szenen-IDs sortiert.
func (s *Service) IDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.scenes))
	for id := range s.scenes {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Stop beendet die Wiedergabe einer Szene auf ihrem Coordinator; die
// gruppierten Boxen folgen. Der aktuelle Inhalt wird NICHT geprueft -- wer
// nur die eigene Szene stoppen will (Scheduler), fragt vorher SceneStatus.
func (s *Service) Stop(ctx context.Context, id string) error {
	scene, ok := s.Get(id)
	if !ok {
		return sceneNotFound()
	}
	coordinator := scene.Coordinator()
	if err := s.ctrl.Stop(ctx, coordinator); err != nil {
		s.log.Warn("scene stop failed", "scene", id, "coordinator", coordinator, "err", err)
		return err
	}
	s.log.Info("scene stopped", "scene", id, "coordinator", coordinator)
	return nil
}

func sceneNotFound() *apperr.Error {
	return apperr.New(apperr.CodeSceneNotFound,
		"Scene not found").
		WithHint("Check available scenes via GET /api/v1/scenes.")
}

// Play spielt eine Szene ab.
//
// Ablauf: Coordinator aus einer fremden Gruppe loesen, Sekundaer-Boxen unter
// den Coordinator gruppieren, Lautstaerke je Box setzen, dann die Quelle auf
// dem Coordinator starten.
//
// Fehlertoleranz: Faellt der Coordinator aus, scheitert die Szene komplett
// (ohne ihn gibt es keine Wiedergabe). Faellt eine Sekundaer-Box aus, laeuft
// die Szene mit den uebrigen weiter und der Fehler wird pro Box berichtet.
func (s *Service) Play(ctx context.Context, id string) (*PlayResult, error) {
	scene, ok := s.Get(id)
	if !ok {
		return nil, sceneNotFound()
	}

	// Szenen mit gemeinsamen Boxen nicht verschraenken: je Box ein Lock,
	// in fester Reihenfolge (kein Deadlock). Szenen auf getrennten Boxen
	// laufen weiter parallel.
	defer s.lockBoxes(scene)()

	coordinator := scene.Coordinator()
	start := time.Now()
	if err := s.validateMembers(ctx, scene); err != nil {
		return nil, err
	}

	result := &PlayResult{SceneID: id}
	statusByName := make(map[string]*SpeakerStatus, len(scene.Members))
	for _, m := range scene.Members {
		st := &SpeakerStatus{Speaker: m.Speaker, OK: true}
		statusByName[m.Speaker] = st
		result.Speakers = append(result.Speakers, *st)
	}

	// 0. Ist der Coordinator gerade nur Mitglied einer anderen Gruppe (z. B.
	//    in der Sonos-App gruppiert), lehnt Sonos jeden Beitritt mit UPnP 800
	//    ab -- dann spielte nur der Coordinator allein.
	grouped := s.ensureStandaloneCoordinator(ctx, id, coordinator)
	tPrepare := time.Since(start)

	// 1. Sekundaer-Boxen gruppieren (jede einzeln, damit ein Ausfall die
	//    anderen nicht mitreisst). Boxen, die schon in der Gruppe des
	//    Coordinators sind, bleiben unberuehrt (spart je einen SOAP-Aufruf).
	t0 := time.Now()
	skipped := 0
	for _, m := range scene.Members {
		if m.Speaker == coordinator {
			continue
		}
		if grouped[m.Speaker] {
			skipped++
			continue
		}
		if err := s.ctrl.Group(ctx, coordinator, []string{m.Speaker}); err != nil {
			s.markFailed(statusByName, m.Speaker, err)
		}
	}
	tGroup := time.Since(t0)

	// 2. Lautstaerke je Box setzen — parallel, die Boxen sind unabhaengig.
	t0 = time.Now()
	var wg sync.WaitGroup
	var failMu sync.Mutex
	for _, m := range scene.Members {
		if !statusByName[m.Speaker].OK {
			continue // schon als ausgefallen markiert
		}
		wg.Add(1)
		go func(m Member) {
			defer wg.Done()
			if err := s.ctrl.SetVolume(ctx, m.Speaker, m.Volume); err != nil {
				failMu.Lock()
				s.markFailed(statusByName, m.Speaker, err)
				failMu.Unlock()
			}
		}(m)
	}
	wg.Wait()
	tVolume := time.Since(t0)

	// 3. Wiedergabe auf dem Coordinator starten.
	t0 = time.Now()
	var coordPlayErr error
	if statusByName[coordinator].OK {
		if err := s.ctrl.Play(ctx, coordinator, scene.Source); err != nil {
			s.markFailed(statusByName, coordinator, err)
			coordPlayErr = err
		}
	}
	tPlay := time.Since(t0)
	timing := []any{
		"took", ms(time.Since(start)), "prepare", ms(tPrepare), "group", ms(tGroup),
		"already_grouped", skipped, "volume", ms(tVolume), "play", ms(tPlay),
	}

	// Coordinator ausgefallen -> keine Wiedergabe moeglich.
	if !statusByName[coordinator].OK {
		s.syncStatuses(result, statusByName)
		var ae *apperr.Error
		if coordPlayErr != nil {
			ae = apperr.As(coordPlayErr)
			s.log.Warn("scene failed: playback on coordinator",
				append([]any{"scene", id, "coordinator", coordinator, "err", coordPlayErr}, timing...)...)
		} else {
			ae = apperr.As(coordinatorErr(statusByName[coordinator]))
			s.log.Warn("scene failed: coordinator unreachable",
				"scene", id, "coordinator", coordinator)
		}
		return result, ae
	}

	result.Playing = true
	s.syncStatuses(result, statusByName)
	s.log.Info("scene started", append([]any{"scene", id, "coordinator", coordinator}, timing...)...)
	return result, nil
}

// ensureStandaloneCoordinator loest coordinator aus seiner Gruppe, wenn er
// dort nicht selbst Coordinator ist. Best effort: scheitert die Topologie-
// Abfrage oder das Loesen, laeuft Play normal weiter (die Gruppierung meldet
// dann pro Box, was schiefging). Fuehrt coordinator bereits eine Gruppe,
// liefert es deren Mitglieder (ohne ihn) — die muessen nicht neu gruppiert
// werden.
func (s *Service) ensureStandaloneCoordinator(ctx context.Context, sceneID, coordinator string) map[string]bool {
	groups, err := s.ctrl.GetGroups(ctx)
	if err != nil {
		s.log.Debug("group topology not readable before play", "scene", sceneID, "err", err)
		return nil
	}
	g, ok := speaker.GroupOf(groups, coordinator)
	if !ok {
		return nil
	}
	if g.Coordinator == coordinator {
		members := make(map[string]bool, len(g.Members))
		for _, m := range g.Members {
			if m != coordinator {
				members[m] = true
			}
		}
		return members
	}
	if err := s.ctrl.Ungroup(ctx, coordinator); err != nil {
		s.log.Warn("coordinator could not leave foreign group", "scene", sceneID,
			"coordinator", coordinator, "group_coordinator", g.Coordinator, "err", err)
		return nil
	}
	s.log.Info("coordinator left foreign group", "scene", sceneID,
		"coordinator", coordinator, "group_coordinator", g.Coordinator)
	return nil
}

func (s *Service) markFailed(byName map[string]*SpeakerStatus, name string, err error) {
	st := byName[name]
	st.OK = false
	ae := apperr.As(err)
	st.Error = ae.Message
	st.Hint = ae.Hint
	s.log.Warn("speaker action failed", "speaker", name, "err", err, "hint", ae.Hint)
}

func (s *Service) syncStatuses(result *PlayResult, byName map[string]*SpeakerStatus) {
	for i := range result.Speakers {
		if st, ok := byName[result.Speakers[i].Speaker]; ok {
			result.Speakers[i] = *st
		}
	}
}

func coordinatorErr(st *SpeakerStatus) error {
	return apperr.New(apperr.CodeSpeakerUnreachable,
		"Coordinator speaker unreachable -- scene could not start").
		WithSpeaker(st.Speaker).
		WithHint("The first speaker of the scene leads the group and must be reachable.")
}

// validateMembers prueft, ob alle Szenen-Member erreichbar registriert sind.
func (s *Service) validateMembers(ctx context.Context, scene Scene) error {
	names := make([]string, len(scene.Members))
	for i, m := range scene.Members {
		names[i] = m.Speaker
	}
	return s.ctrl.EnsureSpeakers(ctx, names)
}

// lockBoxes sperrt alle Boxen der Szene (sortiert) und liefert die Freigabe.
func (s *Service) lockBoxes(scene Scene) func() {
	names := make([]string, 0, len(scene.Members))
	for _, m := range scene.Members {
		names = append(names, m.Speaker)
	}
	sort.Strings(names)
	locks := make([]*sync.Mutex, 0, len(names))
	s.boxMu.Lock()
	for _, n := range names {
		l, ok := s.boxes[n]
		if !ok {
			l = &sync.Mutex{}
			s.boxes[n] = l
		}
		locks = append(locks, l)
	}
	s.boxMu.Unlock()
	for _, l := range locks {
		l.Lock()
	}
	return func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].Unlock()
		}
	}
}

// ms rundet eine Dauer fuers Log auf Millisekunden.
func ms(d time.Duration) string { return d.Round(time.Millisecond).String() }
