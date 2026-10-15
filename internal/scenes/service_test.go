package scenes

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
	"sonos-radio-agent/internal/speaker/mock"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newServiceWith(ctrl *mock.Controller) *Service {
	return NewService([]Scene{validStreamScene()}, ctrl, quietLogger())
}

func TestService_Play_AllReachable(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	svc := newServiceWith(ctrl)

	res, err := svc.Play(context.Background(), "fm4")
	if err != nil {
		t.Fatalf("unerwarteter fehler: %v", err)
	}
	if !res.Playing {
		t.Fatal("erwarte playing=true")
	}
	for _, s := range res.Speakers {
		if !s.OK {
			t.Fatalf("speaker %s sollte ok sein: %s", s.Speaker, s.Error)
		}
	}
	if ctrl.Volumes["Wohnzimmer"] != 30 || ctrl.Volumes["Kueche"] != 20 {
		t.Fatalf("lautstaerken falsch gesetzt: %+v", ctrl.Volumes)
	}
	if len(ctrl.Plays) != 1 || ctrl.Plays[0].Coordinator != "Wohnzimmer" {
		t.Fatalf("erwarte 1 play auf Wohnzimmer, bekam: %+v", ctrl.Plays)
	}
}

func TestService_Play_SecondarySpeakerDown_PartialSuccess(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	ctrl.SetReachable("Kueche", false)
	svc := newServiceWith(ctrl)

	res, err := svc.Play(context.Background(), "fm4")
	if err != nil {
		t.Fatalf("teilerfolg sollte keinen harten fehler liefern: %v", err)
	}
	if !res.Playing {
		t.Fatal("coordinator erreichbar -> sollte spielen")
	}
	got := map[string]bool{}
	for _, s := range res.Speakers {
		got[s.Speaker] = s.OK
	}
	if !got["Wohnzimmer"] {
		t.Fatal("Wohnzimmer sollte ok sein")
	}
	if got["Kueche"] {
		t.Fatal("Kueche sollte als fehlgeschlagen markiert sein")
	}
}

func TestService_Play_CoordinatorDown_HardError(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	ctrl.SetReachable("Wohnzimmer", false)
	svc := newServiceWith(ctrl)

	res, err := svc.Play(context.Background(), "fm4")
	if err == nil {
		t.Fatal("coordinator offline muss harten fehler liefern")
	}
	ae := apperr.As(err)
	if ae.Code != apperr.CodeSpeakerUnreachable {
		t.Fatalf("code = %q, erwartet %q", ae.Code, apperr.CodeSpeakerUnreachable)
	}
	if res == nil || res.Playing {
		t.Fatal("ohne coordinator darf nichts spielen")
	}
}

func TestService_Play_Favorite_NotYetImplemented(t *testing.T) {
	ctrl := mock.New("Kueche")
	fav := validStreamScene()
	fav.ID = "oe1"
	fav.Name = "Ö1"
	fav.Source = speaker.Source{Type: speaker.SourceFavorite, Name: "Ö1"}
	fav.Members = []Member{{Speaker: "Kueche", Volume: 22}}
	svc := NewService([]Scene{fav}, ctrl, quietLogger())

	res, err := svc.Play(context.Background(), "oe1")
	if err == nil {
		t.Fatal("favoriten-szene muss fehlschlagen")
	}
	if apperr.As(err).Code != apperr.CodePlaybackFailed {
		t.Fatalf("code = %q, erwartet %q", apperr.As(err).Code, apperr.CodePlaybackFailed)
	}
	if res == nil || res.Playing {
		t.Fatal("ohne erfolgreiche wiedergabe darf playing nicht true sein")
	}
}

func TestService_Play_SceneNotFound(t *testing.T) {
	svc := newServiceWith(mock.New("Wohnzimmer", "Kueche"))
	_, err := svc.Play(context.Background(), "gibtsnicht")
	if apperr.As(err).Code != apperr.CodeSceneNotFound {
		t.Fatalf("erwarte scene_not_found, bekam: %v", err)
	}
}

func TestService_Play_UnknownMember(t *testing.T) {
	ctrl := mock.New("Wohnzimmer")
	ctrl.SetDiscovery([]speaker.SpeakerInfo{
		{Name: "Wohnzimmer", IP: "192.168.1.10", UID: "RINCON_W", Reachable: true},
	})
	svc := newServiceWith(ctrl)
	_, err := svc.Play(context.Background(), "fm4")
	if err == nil {
		t.Fatal("unbekannter member muss fehlschlagen")
	}
	if apperr.As(err).Code != apperr.CodeSceneMisconfigured {
		t.Fatalf("code = %q, erwartet %q", apperr.As(err).Code, apperr.CodeSceneMisconfigured)
	}
}

func TestService_PlaybackStatus(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	ctrl.SetTransportState("Wohnzimmer", speaker.PlaybackPlaying)
	ctrl.SetNowPlaying("Wohnzimmer", speaker.NowPlayingInfo{
		State: speaker.PlaybackPlaying,
		URI:   "x-rincon-mp3radio://example/stream",
		Title: "FM4",
	})
	svc := newServiceWith(ctrl)

	statuses := svc.PlaybackStatus(context.Background())
	if len(statuses) != 1 {
		t.Fatalf("erwarte 1 status, bekam %d", len(statuses))
	}
	if statuses[0].State != "playing" {
		t.Fatalf("state = %q, want playing", statuses[0].State)
	}

	ctrl.SetNowPlaying("Wohnzimmer", speaker.NowPlayingInfo{
		State: speaker.PlaybackPlaying,
		URI:   "x-rincon-mp3radio://other.example/stream",
		Title: "Anderer Sender",
	})
	statuses = svc.PlaybackStatus(context.Background())
	if statuses[0].State != "playing_other" {
		t.Fatalf("state = %q, want playing_other", statuses[0].State)
	}
	if statuses[0].NowPlayingTitle != "Anderer Sender" {
		t.Fatalf("now_playing_title = %q", statuses[0].NowPlayingTitle)
	}

	ctrl.SetReachable("Wohnzimmer", false)
	statuses = svc.PlaybackStatus(context.Background())
	if statuses[0].State != "offline" {
		t.Fatalf("state = %q, want offline", statuses[0].State)
	}
}

func TestService_ListAndGet(t *testing.T) {
	svc := newServiceWith(mock.New("Wohnzimmer", "Kueche"))
	if len(svc.List()) != 1 {
		t.Fatalf("erwarte 1 szene, bekam %d", len(svc.List()))
	}
	if _, ok := svc.Get("fm4"); !ok {
		t.Fatal("fm4 sollte existieren")
	}
	if _, ok := svc.Get("nope"); ok {
		t.Fatal("nope sollte nicht existieren")
	}
}

func TestService_Stop(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	svc := newServiceWith(ctrl)
	ctx := context.Background()
	if _, err := svc.Play(ctx, "fm4"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Stop(ctx, "fm4"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if len(ctrl.Stops) != 1 || ctrl.Stops[0] != "Wohnzimmer" {
		t.Fatalf("stop auf coordinator erwartet, bekam %v", ctrl.Stops)
	}
	st, err := svc.SceneStatus(ctx, "fm4")
	if err != nil || st.State != string(speaker.PlaybackStopped) {
		t.Fatalf("status nach stop = %+v, err=%v", st, err)
	}
}

func TestService_Stop_Errors(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	svc := newServiceWith(ctrl)
	if got := apperr.As(svc.Stop(context.Background(), "gibtsnicht")).Code; got != apperr.CodeSceneNotFound {
		t.Fatalf("code = %q", got)
	}
	ctrl.SetReachable("Wohnzimmer", false)
	if got := apperr.As(svc.Stop(context.Background(), "fm4")).Code; got != apperr.CodeSpeakerUnreachable {
		t.Fatalf("code = %q", got)
	}
}

func TestService_SceneStatus(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	svc := newServiceWith(ctrl)
	ctx := context.Background()
	if _, err := svc.SceneStatus(ctx, "gibtsnicht"); apperr.As(err).Code != apperr.CodeSceneNotFound {
		t.Fatalf("erwarte scene_not_found, bekam %v", err)
	}
	if _, err := svc.Play(ctx, "fm4"); err != nil {
		t.Fatal(err)
	}
	st, _ := svc.SceneStatus(ctx, "fm4")
	if st.State != string(speaker.PlaybackPlaying) {
		t.Fatalf("state = %q, want playing", st.State)
	}
	ctrl.SetNowPlaying("Wohnzimmer", speaker.NowPlayingInfo{State: speaker.PlaybackPlaying, URI: "x-sonos-spotify:abc"})
	st, _ = svc.SceneStatus(ctx, "fm4")
	if st.State != "playing_other" {
		t.Fatalf("state = %q, want playing_other", st.State)
	}
	if ids := svc.IDs(); len(ids) != 1 || ids[0] != "fm4" {
		t.Fatalf("ids = %v", ids)
	}
}

func TestReloadParallelZuPlayUndList(t *testing.T) {
	m := mock.New("A")
	sc := Scene{ID: "a", Name: "a", Source: speaker.Source{Type: speaker.SourceStream, URI: "http://x/y"}, Members: []Member{{Speaker: "A", Volume: 5}}}
	svc := NewService([]Scene{sc}, m, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = svc.Reload([]Scene{sc})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_, _ = svc.Play(context.Background(), "a")
			svc.List()
		}
	}()
	wg.Wait()
}

// Befund 2026-10-15: Der Szenen-Coordinator war in der Sonos-App Mitglied
// einer anderen Gruppe -> jeder Beitritt scheiterte (UPnP 800), nur der
// Coordinator spielte. Play loest ihn jetzt vorher aus der fremden Gruppe.
func TestService_Play_CoordinatorInFremderGruppe(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche", "Bad")
	ctrl.SetGroup("Bad", "Bad", "Wohnzimmer")
	svc := newServiceWith(ctrl)

	res, err := svc.Play(context.Background(), "fm4")
	if err != nil {
		t.Fatalf("play: %v", err)
	}
	for _, sp := range res.Speakers {
		if !sp.OK {
			t.Fatalf("%s sollte mitspielen: %s (%s)", sp.Speaker, sp.Error, sp.Hint)
		}
	}
	if len(ctrl.Ungroups) != 1 || ctrl.Ungroups[0] != "Wohnzimmer" {
		t.Fatalf("Coordinator muss vorher die fremde Gruppe verlassen, Ungroups=%v", ctrl.Ungroups)
	}
	groups, _ := ctrl.GetGroups(context.Background())
	g, _ := speaker.GroupOf(groups, "Kueche")
	if g.Coordinator != "Wohnzimmer" {
		t.Fatalf("Kueche soll Wohnzimmer folgen, Gruppe=%+v", g)
	}
}

func TestService_Play_CoordinatorSchonEigenstaendig_KeinUngroup(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche")
	ctrl.SetGroup("Wohnzimmer", "Wohnzimmer", "Kueche")
	svc := newServiceWith(ctrl)
	if _, err := svc.Play(context.Background(), "fm4"); err != nil {
		t.Fatal(err)
	}
	if len(ctrl.Ungroups) != 0 {
		t.Fatalf("Coordinator fuehrt schon die Gruppe -> kein Ungroup, bekam %v", ctrl.Ungroups)
	}
}

func TestService_Play_UngroupAbgelehnt_Teilausfall(t *testing.T) {
	ctrl := mock.New("Wohnzimmer", "Kueche", "Bad")
	ctrl.SetGroup("Bad", "Bad", "Wohnzimmer")
	ctrl.RejectUngroup("Wohnzimmer")
	svc := newServiceWith(ctrl)

	res, err := svc.Play(context.Background(), "fm4")
	if err != nil {
		t.Fatalf("Teilausfall ist kein harter Fehler: %v", err)
	}
	if !res.Playing {
		t.Fatal("Coordinator spielt trotzdem")
	}
	for _, sp := range res.Speakers {
		if sp.Speaker == "Kueche" && (sp.OK || !strings.Contains(sp.Hint, "800")) {
			t.Fatalf("Kueche: grouping_failed mit UPnP 800 erwartet, bekam %+v", sp)
		}
	}
}
