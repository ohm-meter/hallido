package sonos

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer ist ein threadsicherer Log-Puffer (probeFixedIPs loggt aus Goroutinen).
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestProbeFixedIPs_OfflineWarnsOnlyOnce(t *testing.T) {
	buf := &syncBuffer{}
	log := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	// Port 1400 auf localhost ist zu -> Probe schlaegt fehl.
	c := New(map[string]string{"Sub Mini": "127.0.0.1"}, log)

	for i := 0; i < 5; i++ {
		if found := c.probeFixedIPs(context.Background()); len(found) != 0 {
			t.Fatalf("erwartet 0 boxen, bekam %d", len(found))
		}
	}
	out := buf.String()
	if n := strings.Count(out, "level=WARN"); n != 1 {
		t.Fatalf("erwartet genau 1 WARN, bekam %d:\n%s", n, out)
	}
	if n := strings.Count(out, "level=DEBUG"); n != 4 {
		t.Fatalf("erwartet 4 DEBUG, bekam %d:\n%s", n, out)
	}

	// Zustandswechsel online loggt einmalig INFO, danach wieder WARN bei erneutem Ausfall.
	c.noteProbeOK("Sub Mini", "127.0.0.1")
	c.noteProbeOK("Sub Mini", "127.0.0.1")
	c.probeFixedIPs(context.Background())
	out = buf.String()
	if n := strings.Count(out, "reachable again"); n != 1 {
		t.Fatalf("erwartet 1x 'reachable again', bekam %d:\n%s", n, out)
	}
	if n := strings.Count(out, "level=WARN"); n != 2 {
		t.Fatalf("erwartet 2 WARN nach erneutem Ausfall, bekam %d", n)
	}
}

func TestRefreshDevicesFor_OfflineFixedIPWarnsOnlyOnce(t *testing.T) {
	t.Cleanup(func() { ssdpSearchHook = nil })
	ssdpSearchHook = func(time.Duration) ([]string, error) { return nil, nil }

	buf := &syncBuffer{}
	log := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	c := New(map[string]string{"Sub Mini": "127.0.0.1"}, log)

	for i := 0; i < 3; i++ {
		_ = c.refreshDevicesFor(context.Background(), []string{"Sub Mini"})
	}
	out := buf.String()
	if n := strings.Count(out, "msg=\"fixed-ip speaker unreachable\""); n != 1 {
		t.Fatalf("erwartet genau 1 WARN, bekam %d:\n%s", n, out)
	}
}
