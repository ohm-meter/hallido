package mock

import (
	"context"
	"reflect"
	"testing"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

func TestGetGroups_FollowsGroupCalls(t *testing.T) {
	ctx := context.Background()
	c := New("Wohnzimmer", "Kueche", "Bad", "Buero")
	_ = c.Group(ctx, "Wohnzimmer", []string{"Kueche"})
	_ = c.Group(ctx, "Wohnzimmer", []string{"Bad"})

	groups, err := c.GetGroups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []speaker.GroupInfo{
		{Coordinator: "Buero", Members: []string{"Buero"}},
		{Coordinator: "Wohnzimmer", Members: []string{"Wohnzimmer", "Bad", "Kueche"}},
	}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups = %+v, want %+v", groups, want)
	}
	if g, ok := speaker.GroupOf(groups, "Bad"); !ok || g.Coordinator != "Wohnzimmer" {
		t.Fatalf("GroupOf(Bad) = %+v, %v", g, ok)
	}
	if _, ok := speaker.GroupOf(groups, "Garage"); ok {
		t.Fatal("unbekannte Box darf keine Gruppe haben")
	}
}

func TestGetGroups_AllOffline(t *testing.T) {
	c := New("Wohnzimmer")
	c.SetReachable("Wohnzimmer", false)
	_, err := c.GetGroups(context.Background())
	if apperr.As(err).Code != apperr.CodeSpeakerUnreachable {
		t.Fatalf("err = %v", err)
	}
}

func TestResume_AfterPauseAndStop(t *testing.T) {
	ctx := context.Background()
	c := New("Wohnzimmer")
	if err := c.Resume(ctx, "Wohnzimmer"); apperr.As(err).Code != apperr.CodePlaybackFailed {
		t.Fatalf("ohne Quelle: err = %v", err)
	}
	_ = c.Play(ctx, "Wohnzimmer", speaker.Source{Type: speaker.SourceStream, URI: "https://x/fm4"})
	_ = c.Pause(ctx, "Wohnzimmer")
	if err := c.Resume(ctx, "Wohnzimmer"); err != nil {
		t.Fatal(err)
	}
	if st, _ := c.GetTransportState(ctx, "Wohnzimmer"); st != speaker.PlaybackPlaying {
		t.Fatalf("nach resume: %s", st)
	}
	_ = c.Stop(ctx, "Wohnzimmer")
	if err := c.Resume(ctx, "Wohnzimmer"); err != nil {
		t.Fatalf("resume nach stop: %v", err)
	}
	np, _ := c.GetNowPlaying(ctx, "Wohnzimmer")
	if np.URI != "x-rincon-mp3radio://x/fm4" || np.State != speaker.PlaybackPlaying {
		t.Fatalf("now playing: %+v", np)
	}
	c.SetReachable("Wohnzimmer", false)
	if err := c.Resume(ctx, "Wohnzimmer"); apperr.As(err).Code != apperr.CodeSpeakerUnreachable {
		t.Fatalf("offline: err = %v", err)
	}
}

func TestUngroup_MemberAndCoordinator(t *testing.T) {
	ctx := context.Background()
	c := New("Wohnzimmer", "Kueche", "Bad")
	c.SetGroup("Wohnzimmer", "Kueche", "Bad")
	_ = c.Play(ctx, "Wohnzimmer", speaker.Source{Type: speaker.SourceStream, URI: "https://x/fm4"})

	// Mitglied verlaesst die Gruppe.
	if err := c.Ungroup(ctx, "Bad"); err != nil {
		t.Fatal(err)
	}
	groups, _ := c.GetGroups(ctx)
	if g, _ := speaker.GroupOf(groups, "Bad"); g.Coordinator != "Bad" || len(g.Members) != 1 {
		t.Fatalf("Bad sollte allein stehen: %+v", groups)
	}

	// Coordinator verlaesst die Gruppe -> Kueche uebernimmt und spielt weiter.
	if err := c.Ungroup(ctx, "Wohnzimmer"); err != nil {
		t.Fatal(err)
	}
	if st, _ := c.GetTransportState(ctx, "Kueche"); st != speaker.PlaybackPlaying {
		t.Fatalf("Kueche sollte als neuer Coordinator spielen: %s", st)
	}
	if st, _ := c.GetTransportState(ctx, "Wohnzimmer"); st != speaker.PlaybackStopped {
		t.Fatalf("Wohnzimmer sollte gestoppt sein: %s", st)
	}

	// Allein stehende Box: kein Fehler.
	if err := c.Ungroup(ctx, "Wohnzimmer"); err != nil {
		t.Fatal(err)
	}
	c.SetReachable("Bad", false)
	if err := c.Ungroup(ctx, "Bad"); apperr.As(err).Code != apperr.CodeSpeakerUnreachable {
		t.Fatalf("offline: err = %v", err)
	}
}
