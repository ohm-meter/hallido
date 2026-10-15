package scenes

import (
	"context"
	"sort"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

// GroupAction ist ein Transportbefehl, der auf eine ganze Gruppe wirkt.
type GroupAction string

const (
	GroupStop   GroupAction = "stop"
	GroupPause  GroupAction = "pause"
	GroupResume GroupAction = "resume"
)

// GroupActionResult beschreibt, welche Gruppe ein Befehl ueber einen
// beliebigen Boxnamen getroffen hat.
type GroupActionResult struct {
	Speaker     string      `json:"speaker"`
	Coordinator string      `json:"coordinator"`
	Members     []string    `json:"members"`
	Action      GroupAction `json:"action"`
	OK          bool        `json:"ok"`
}

// UngroupResult beschreibt das Aufloesen einer Gruppe. Speakers listet jede
// Box, die die Gruppe verlassen sollte, mit eigenem Ergebnis.
type UngroupResult struct {
	Speaker     string          `json:"speaker"`
	Coordinator string          `json:"coordinator"`
	Dissolved   bool            `json:"dissolved"` // true: Coordinator angesprochen, ganze Gruppe aufgeloest
	Standalone  bool            `json:"standalone"`
	Speakers    []SpeakerStatus `json:"speakers"`
}

// GroupStatus ist eine aktuelle Gruppe mit Wiedergabestatus ihres
// Coordinators (best effort: Title/URI nur wenn die Box sie liefert).
type GroupStatus struct {
	Coordinator string   `json:"coordinator"`
	Members     []string `json:"members"`
	State       string   `json:"state"`
	Title       string   `json:"title,omitempty"`
	Artist      string   `json:"artist,omitempty"`
	URI         string   `json:"uri,omitempty"`
	SceneID     string   `json:"scene_id,omitempty"` // Szene, deren Quelle gerade laeuft
	Error       string   `json:"error,omitempty"`
}

// Pause pausiert eine Szene auf ihrem Coordinator; die Gruppe folgt.
func (s *Service) Pause(ctx context.Context, id string) error {
	return s.sceneTransport(ctx, id, GroupPause)
}

// Resume setzt eine pausierte (oder gestoppte) Szene auf ihrem Coordinator
// fort -- mit der dort geladenen Quelle, ohne sie neu zu setzen.
func (s *Service) Resume(ctx context.Context, id string) error {
	return s.sceneTransport(ctx, id, GroupResume)
}

func (s *Service) sceneTransport(ctx context.Context, id string, action GroupAction) error {
	scene, ok := s.Get(id)
	if !ok {
		return sceneNotFound()
	}
	coordinator := scene.Coordinator()
	if err := s.transport(ctx, coordinator, action); err != nil {
		s.log.Warn("scene "+string(action)+" failed", "scene", id, "coordinator", coordinator, "err", err)
		return err
	}
	s.log.Info("scene "+string(action), "scene", id, "coordinator", coordinator)
	return nil
}

func (s *Service) transport(ctx context.Context, coordinator string, action GroupAction) error {
	switch action {
	case GroupStop:
		return s.ctrl.Stop(ctx, coordinator)
	case GroupPause:
		return s.ctrl.Pause(ctx, coordinator)
	case GroupResume:
		return s.ctrl.Resume(ctx, coordinator)
	default:
		return apperr.New(apperr.CodeBadRequest, "unknown group action: "+string(action))
	}
}

// groupOf ermittelt die aktuelle Gruppe einer Box ueber die Topologie.
func (s *Service) groupOf(ctx context.Context, name string) (speaker.GroupInfo, error) {
	groups, err := s.ctrl.GetGroups(ctx)
	if err != nil {
		return speaker.GroupInfo{}, err
	}
	g, ok := speaker.GroupOf(groups, name)
	if !ok {
		return speaker.GroupInfo{}, apperr.New(apperr.CodeSpeakerUnreachable,
			"Speaker not in the group topology").
			WithSpeaker(name).
			WithHint("Name exactly as in the Sonos app? Speaker online? Current groups: GET /api/v1/groups.")
	}
	return g, nil
}

// ControlGroupOf stoppt, pausiert oder setzt die Gruppe fort, in der name
// gerade spielt -- egal ob sie per Szene oder in der Sonos-App entstanden
// ist. Der Befehl geht an den Coordinator; alle Mitglieder folgen.
func (s *Service) ControlGroupOf(ctx context.Context, name string, action GroupAction) (*GroupActionResult, error) {
	g, err := s.groupOf(ctx, name)
	if err != nil {
		return nil, err
	}
	res := &GroupActionResult{Speaker: name, Coordinator: g.Coordinator, Members: g.Members, Action: action}
	if err := s.transport(ctx, g.Coordinator, action); err != nil {
		s.log.Warn("group "+string(action)+" failed", "speaker", name, "coordinator", g.Coordinator, "err", err)
		return res, err
	}
	res.OK = true
	s.log.Info("group "+string(action), "speaker", name, "coordinator", g.Coordinator, "members", len(g.Members))
	return res, nil
}

// Ungroup loest name aus seiner Gruppe. Ist name der Coordinator, wird die
// ganze Gruppe aufgeloest: jedes Mitglied verlaesst sie einzeln, der
// Coordinator spielt allein weiter. Teilausfall ist kein harter Fehler --
// erst wenn keine einzige Box die Gruppe verlassen konnte, gibt es einen
// Fehler (mit Ergebnis pro Box).
func (s *Service) Ungroup(ctx context.Context, name string) (*UngroupResult, error) {
	g, err := s.groupOf(ctx, name)
	if err != nil {
		return nil, err
	}
	res := &UngroupResult{Speaker: name, Coordinator: g.Coordinator, Speakers: []SpeakerStatus{}}
	if len(g.Members) <= 1 {
		res.Standalone = true
		return res, nil
	}

	leaving := []string{name}
	if name == g.Coordinator {
		res.Dissolved = true
		leaving = leaving[:0]
		for _, m := range g.Members {
			if m != g.Coordinator {
				leaving = append(leaving, m)
			}
		}
	}

	var firstErr error
	for _, m := range leaving {
		st := SpeakerStatus{Speaker: m, OK: true}
		if err := s.ctrl.Ungroup(ctx, m); err != nil {
			st.OK = false
			st.Error = apperr.As(err).Message
			s.log.Warn("speaker could not leave group", "speaker", m, "coordinator", g.Coordinator, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
		res.Speakers = append(res.Speakers, st)
	}
	for _, st := range res.Speakers {
		if st.OK {
			s.log.Info("group dissolved", "speaker", name, "coordinator", g.Coordinator, "dissolved", res.Dissolved)
			return res, nil
		}
	}
	return res, firstErr
}

// Groups liefert alle aktuellen Gruppen mit dem Status ihres Coordinators.
// Faellt die Status-Abfrage einer Gruppe aus, steht der Fehler an der
// Gruppe; die uebrigen Gruppen kommen trotzdem.
func (s *Service) Groups(ctx context.Context) ([]GroupStatus, error) {
	groups, err := s.ctrl.GetGroups(ctx)
	if err != nil {
		return nil, err
	}
	list := s.List()
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	out := make([]GroupStatus, 0, len(groups))
	for _, g := range groups {
		gs := GroupStatus{Coordinator: g.Coordinator, Members: g.Members, State: string(speaker.PlaybackUnknown)}
		np, err := s.ctrl.GetNowPlaying(ctx, g.Coordinator)
		if err != nil {
			ae := apperr.As(err)
			if ae.Code == apperr.CodeSpeakerUnreachable {
				gs.State = "offline"
			}
			gs.Error = ae.Message
			out = append(out, gs)
			continue
		}
		gs.State = string(np.State)
		gs.Title, gs.Artist, gs.URI = np.Title, np.Artist, np.URI
		for _, sc := range list {
			if sc.Coordinator() == g.Coordinator && speaker.SourceMatchesPlayback(sc.Source, np) {
				gs.SceneID = sc.ID
				break
			}
		}
		out = append(out, gs)
	}
	return out, nil
}
