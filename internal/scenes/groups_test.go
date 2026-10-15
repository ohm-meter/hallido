package scenes

import (
	"context"
	"reflect"
	"testing"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
	"sonos-radio-agent/internal/speaker/mock"
)

func TestService_PauseResumeScene(t *testing.T) {
	ctx := context.Background()
	ctrl := mock.New("Wohnzimmer", "Kueche")
	svc := newServiceWith(ctrl)
	if _, err := svc.Play(ctx, "fm4"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Pause(ctx, "fm4"); err != nil {
		t.Fatal(err)
	}
	if st, _ := svc.SceneStatus(ctx, "fm4"); st.State != "paused" {
		t.Fatalf("nach pause: %s", st.State)
	}
	if err := svc.Resume(ctx, "fm4"); err != nil {
		t.Fatal(err)
	}
	if st, _ := svc.SceneStatus(ctx, "fm4"); st.State != "playing" {
		t.Fatalf("nach resume: %s", st.State)
	}
	if !reflect.DeepEqual(ctrl.Pauses, []string{"Wohnzimmer"}) || !reflect.DeepEqual(ctrl.Resumes, []string{"Wohnzimmer"}) {
		t.Fatalf("pauses=%v resumes=%v", ctrl.Pauses, ctrl.Resumes)
	}
}

func TestService_PauseResumeScene_Errors(t *testing.T) {
	ctx := context.Background()
	ctrl := mock.New("Wohnzimmer", "Kueche")
	svc := newServiceWith(ctrl)
	if err := svc.Pause(ctx, "gibtsnicht"); apperr.As(err).Code != apperr.CodeSceneNotFound {
		t.Fatalf("unbekannte szene: %v", err)
	}
	// Nichts geladen -> Box lehnt Resume ab.
	if err := svc.Resume(ctx, "fm4"); apperr.As(err).Code != apperr.CodePlaybackFailed {
		t.Fatalf("resume ohne quelle: %v", err)
	}
	ctrl.SetReachable("Wohnzimmer", false)
	if err := svc.Pause(ctx, "fm4"); apperr.As(err).Code != apperr.CodeSpeakerUnreachable {
		t.Fatalf("offline: %v", err)
	}
}

// Gruppe aus der Sonos-App (keine Szene): Befehl ueber ein Mitglied geht an
// den Coordinator.
func TestService_ControlGroupOf_MemberTargetsCoordinator(t *testing.T) {
	ctx := context.Background()
	ctrl := mock.New("Wohnzimmer", "Kueche", "Bad")
	ctrl.SetGroup("Bad", "Kueche")
	ctrl.SetNowPlaying("Bad", speaker.NowPlayingInfo{State: speaker.PlaybackPlaying, Title: "Spotify"})
	svc := newServiceWith(ctrl)

	for _, action := range []GroupAction{GroupPause, GroupResume, GroupStop} {
		res, err := svc.ControlGroupOf(ctx, "Kueche", action)
		if err != nil {
			t.Fatalf("%s: %v", action, err)
		}
		if !res.OK || res.Coordinator != "Bad" || !reflect.DeepEqual(res.Members, []string{"Bad", "Kueche"}) {
			t.Fatalf("%s: result %+v", action, res)
		}
	}
	if !reflect.DeepEqual(ctrl.Pauses, []string{"Bad"}) || !reflect.DeepEqual(ctrl.Resumes, []string{"Bad"}) ||
		!reflect.DeepEqual(ctrl.Stops, []string{"Bad"}) {
		t.Fatalf("pauses=%v resumes=%v stops=%v", ctrl.Pauses, ctrl.Resumes, ctrl.Stops)
	}
}

func TestService_ControlGroupOf_Errors(t *testing.T) {
	ctx := context.Background()
	ctrl := mock.New("Wohnzimmer", "Kueche")
	svc := newServiceWith(ctrl)

	if _, err := svc.ControlGroupOf(ctx, "Garage", GroupStop); apperr.As(err).Code != apperr.CodeSpeakerUnreachable {
		t.Fatalf("unbekannte box: %v", err)
	}
	// Box steht allein, nichts geladen -> Resume abgelehnt, Result trotzdem da.
	res, err := svc.ControlGroupOf(ctx, "Kueche", GroupResume)
	if apperr.As(err).Code != apperr.CodePlaybackFailed || res == nil || res.OK || res.Coordinator != "Kueche" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	ctrl.SetReachable("Wohnzimmer", false)
	ctrl.SetReachable("Kueche", false)
	if _, err := svc.ControlGroupOf(ctx, "Kueche", GroupStop); apperr.As(err).Code != apperr.CodeSpeakerUnreachable {
		t.Fatalf("topologie offline: %v", err)
	}
}

func TestService_Ungroup(t *testing.T) {
	ctx := context.Background()

	t.Run("member", func(t *testing.T) {
		ctrl := mock.New("Wohnzimmer", "Kueche", "Bad")
		ctrl.SetGroup("Wohnzimmer", "Kueche", "Bad")
		res, err := newServiceWith(ctrl).Ungroup(ctx, "Kueche")
		if err != nil || res.Dissolved || len(res.Speakers) != 1 || res.Speakers[0].Speaker != "Kueche" {
			t.Fatalf("res=%+v err=%v", res, err)
		}
		if !reflect.DeepEqual(ctrl.Ungroups, []string{"Kueche"}) {
			t.Fatalf("ungroups=%v", ctrl.Ungroups)
		}
	})

	t.Run("coordinator dissolves group", func(t *testing.T) {
		ctrl := mock.New("Wohnzimmer", "Kueche", "Bad")
		ctrl.SetGroup("Wohnzimmer", "Kueche", "Bad")
		res, err := newServiceWith(ctrl).Ungroup(ctx, "Wohnzimmer")
		if err != nil || !res.Dissolved || len(res.Speakers) != 2 {
			t.Fatalf("res=%+v err=%v", res, err)
		}
		if !reflect.DeepEqual(ctrl.Ungroups, []string{"Bad", "Kueche"}) {
			t.Fatalf("coordinator darf nicht selbst ungrouped werden: %v", ctrl.Ungroups)
		}
		groups, _ := ctrl.GetGroups(ctx)
		if len(groups) != 3 {
			t.Fatalf("erwartet 3 einzelne boxen: %+v", groups)
		}
	})

	t.Run("standalone is no-op", func(t *testing.T) {
		ctrl := mock.New("Wohnzimmer")
		res, err := newServiceWith(ctrl).Ungroup(ctx, "Wohnzimmer")
		if err != nil || !res.Standalone || len(ctrl.Ungroups) != 0 {
			t.Fatalf("res=%+v err=%v ungroups=%v", res, err, ctrl.Ungroups)
		}
	})

	t.Run("partial failure is not fatal", func(t *testing.T) {
		ctrl := mock.New("Wohnzimmer", "Kueche", "Bad")
		ctrl.SetGroup("Wohnzimmer", "Kueche", "Bad")
		ctrl.RejectUngroup("Bad")
		res, err := newServiceWith(ctrl).Ungroup(ctx, "Wohnzimmer")
		if err != nil {
			t.Fatalf("teilausfall darf kein harter fehler sein: %v", err)
		}
		byName := map[string]SpeakerStatus{}
		for _, st := range res.Speakers {
			byName[st.Speaker] = st
		}
		if !byName["Kueche"].OK || byName["Bad"].OK || byName["Bad"].Error == "" {
			t.Fatalf("pro-box-ergebnis falsch: %+v", res.Speakers)
		}
	})

	t.Run("all fail is error with result", func(t *testing.T) {
		ctrl := mock.New("Wohnzimmer", "Kueche")
		ctrl.SetGroup("Wohnzimmer", "Kueche")
		ctrl.RejectUngroup("Kueche")
		res, err := newServiceWith(ctrl).Ungroup(ctx, "Kueche")
		if apperr.As(err).Code != apperr.CodePlaybackFailed || res == nil || res.Speakers[0].OK {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
}

func TestService_Groups(t *testing.T) {
	ctx := context.Background()
	ctrl := mock.New("Wohnzimmer", "Kueche", "Bad")
	svc := newServiceWith(ctrl)
	if _, err := svc.Play(ctx, "fm4"); err != nil { // Wohnzimmer + Kueche
		t.Fatal(err)
	}
	ctrl.SetNowPlaying("Bad", speaker.NowPlayingInfo{State: speaker.PlaybackPaused, Title: "Podcast", URI: "x-sonos-spotify:abc"})

	groups, err := svc.Groups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("erwartet 2 gruppen: %+v", groups)
	}
	bad, wz := groups[0], groups[1]
	if bad.Coordinator != "Bad" || bad.State != "paused" || bad.Title != "Podcast" || bad.SceneID != "" {
		t.Fatalf("Bad: %+v", bad)
	}
	if wz.Coordinator != "Wohnzimmer" || wz.State != "playing" || wz.SceneID != "fm4" ||
		!reflect.DeepEqual(wz.Members, []string{"Wohnzimmer", "Kueche"}) {
		t.Fatalf("Wohnzimmer: %+v", wz)
	}

	ctrl.SetReachable("Wohnzimmer", false)
	ctrl.SetReachable("Kueche", false)
	ctrl.SetReachable("Bad", false)
	if _, err := svc.Groups(ctx); apperr.As(err).Code != apperr.CodeSpeakerUnreachable {
		t.Fatalf("alle offline: %v", err)
	}
}
