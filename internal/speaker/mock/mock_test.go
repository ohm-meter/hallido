package mock

import (
	"context"
	"testing"

	"sonos-radio-agent/internal/apperr"
	"sonos-radio-agent/internal/speaker"
)

func TestPlay_Favorite_Integratable(t *testing.T) {
	c := New("Kueche")
	err := c.Play(context.Background(), "Kueche", speaker.Source{
		Type: speaker.SourceFavorite,
		Name: "Test Radio",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Plays) != 1 || c.Plays[0].Source.Type != speaker.SourceStream {
		t.Fatalf("plays=%+v", c.Plays)
	}
}

func TestPlay_Favorite_NotYetImplemented(t *testing.T) {
	c := New("Kueche")
	err := c.Play(context.Background(), "Kueche", speaker.Source{
		Type: speaker.SourceFavorite,
		Name: "Ö1",
	})
	if err == nil {
		t.Fatal("erwartet fehler fuer favoriten-quelle")
	}
	if got := apperr.As(err).Code; got != apperr.CodePlaybackFailed {
		t.Errorf("erwartet code %q, bekam %q", apperr.CodePlaybackFailed, got)
	}
}

func TestListSpeakers_ReturnsStructuredInfo(t *testing.T) {
	c := New("Wohnzimmer", "Kueche")
	list, err := c.ListSpeakers(context.Background())
	if err != nil {
		t.Fatalf("ListSpeakers: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("erwarte 2, bekam %d", len(list))
	}
	if list[0].IP == "" || list[0].UID == "" || !list[0].Reachable {
		t.Fatalf("unvollstaendige info: %+v", list[0])
	}
}

func TestListSpeakers_IndependentOfReachable(t *testing.T) {
	c := New("Wohnzimmer")
	c.SetDiscovery([]speaker.SpeakerInfo{
		{Name: "Wohnzimmer", IP: "192.168.1.10", UID: "RINCON_W", Reachable: true},
		{Name: "Kueche", IP: "192.168.1.11", UID: "RINCON_K", Reachable: true},
	})
	c.SetReachable("Kueche", false)

	list, err := c.ListSpeakers(context.Background())
	if err != nil {
		t.Fatalf("ListSpeakers: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("discovery soll 2 liefern, bekam %d", len(list))
	}
	for _, sp := range list {
		if sp.Name == "Kueche" && sp.Reachable {
			t.Fatal("Kueche soll unreachable sein")
		}
	}
}

func TestStopAndPause_ChangeTransportState(t *testing.T) {
	c := New("Kueche")
	ctx := context.Background()
	if err := c.Play(ctx, "Kueche", speaker.Source{Type: speaker.SourceStream, URI: "https://example.org/s"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Pause(ctx, "Kueche"); err != nil {
		t.Fatal(err)
	}
	if st, _ := c.GetTransportState(ctx, "Kueche"); st != speaker.PlaybackPaused {
		t.Fatalf("nach pause: %q", st)
	}
	if err := c.Stop(ctx, "Kueche"); err != nil {
		t.Fatal(err)
	}
	if st, _ := c.GetTransportState(ctx, "Kueche"); st != speaker.PlaybackStopped {
		t.Fatalf("nach stop: %q", st)
	}
	if len(c.Stops) != 1 || len(c.Pauses) != 1 {
		t.Fatalf("stops=%v pauses=%v", c.Stops, c.Pauses)
	}
}

func TestStop_Unreachable(t *testing.T) {
	c := New("Kueche")
	c.SetReachable("Kueche", false)
	if got := apperr.As(c.Stop(context.Background(), "Kueche")).Code; got != apperr.CodeSpeakerUnreachable {
		t.Fatalf("code = %q", got)
	}
	if got := apperr.As(c.Pause(context.Background(), "Kueche")).Code; got != apperr.CodeSpeakerUnreachable {
		t.Fatalf("code = %q", got)
	}
}

func TestPlay_StreamMatchesSceneSource(t *testing.T) {
	c := New("Kueche")
	src := speaker.Source{Type: speaker.SourceStream, URI: "https://orf-live.ors-shoutcast.at/oe3-q2a"}
	if err := c.Play(context.Background(), "Kueche", src); err != nil {
		t.Fatal(err)
	}
	np, _ := c.GetNowPlaying(context.Background(), "Kueche")
	if np.URI != "x-rincon-mp3radio://orf-live.ors-shoutcast.at/oe3-q2a" {
		t.Fatalf("uri = %q", np.URI)
	}
	if !speaker.SourceMatchesPlayback(src, np) {
		t.Fatal("mock-wiedergabe sollte zur szenen-quelle passen")
	}
}
