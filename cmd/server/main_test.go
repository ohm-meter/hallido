package main

import (
	"io"
	"log/slog"
	"sort"
	"testing"

	"sonos-radio-agent/internal/config"
	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/speaker"
)

func testCfg() *config.Config {
	return &config.Config{
		Speakers: map[string]config.SpeakerOverride{"A": {IP: "10.0.0.1"}, "B": {}},
		Scenes: []scenes.Scene{{ID: "s", Name: "S",
			Source:  speaker.Source{Type: speaker.SourceStream, URI: "https://x.example/a"},
			Members: []scenes.Member{{Speaker: "C", Volume: 1}, {Speaker: "A", Volume: 1}}}},
	}
}

func TestBuildController(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, d := range []string{"mock", "sonos"} {
		if c, err := buildController(d, testCfg(), log); err != nil || c == nil {
			t.Errorf("%s: %v", d, err)
		}
	}
	if _, err := buildController("unsinn", testCfg(), log); err == nil {
		t.Error("unbekannter Treiber muss scheitern")
	}
}

func TestKnownSpeakersUndReferenced(t *testing.T) {
	known := testCfg().KnownSpeakers()
	if known["A"] != "10.0.0.1" || known["C"] != "" || len(known) != 2 {
		t.Errorf("known = %v", known)
	}
	ref := referencedSpeakers(testCfg())
	sort.Strings(ref)
	if len(ref) != 3 || ref[0] != "A" || ref[2] != "C" {
		t.Errorf("ref = %v", ref)
	}
}

func TestEnvOr(t *testing.T) {
	t.Setenv("SRA_TEST_X", "")
	if envOr("SRA_TEST_X", "fb") != "fb" {
		t.Error("Fallback")
	}
	t.Setenv("SRA_TEST_X", "v")
	if envOr("SRA_TEST_X", "fb") != "v" {
		t.Error("Env")
	}
}
