// Command server startet den Sonos Radio Agent.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sonos-radio-agent/internal/api"
	"sonos-radio-agent/internal/config"
	"sonos-radio-agent/internal/scenes"
	"sonos-radio-agent/internal/schedule"
	"sonos-radio-agent/internal/speaker"
	"sonos-radio-agent/internal/speaker/mock"
	"sonos-radio-agent/internal/speaker/sonos"
	"sonos-radio-agent/internal/web"
)

func main() {
	cfgPath := flag.String("config", "config.json", "Path to the configuration file")
	driver := flag.String("driver", envOr("SRA_DRIVER", "mock"),
		"Speaker driver: mock | sonos")
	debug := flag.Bool("debug", false, "verbose logging")
	flag.Parse()

	log := newLogger(*debug)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	ctrl, err := buildController(*driver, cfg, log)
	if err != nil {
		log.Error("driver could not be initialized", "driver", *driver, "err", err)
		os.Exit(1)
	}

	svc := scenes.NewService(cfg.Scenes, ctrl, log)
	srv := api.New(svc, ctrl, cfg.APIToken, *cfgPath, cfg.KnownSpeakers(), web.Handler(), log)

	// Zeitplaene laufen im selben Prozess; ctx endet bei SIGINT/SIGTERM.
	runCtx, stopRun := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopRun()
	sched := schedule.New(cfg.Schedules, cfg.Location(), svc, schedule.RealClock{}, log)
	srv.SetScheduler(sched)
	schedDone := make(chan struct{})
	go func() {
		defer close(schedDone)
		sched.Run(runCtx)
	}()

	httpSrv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      api.MaxRequestDuration + 10*time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}

	go func() {
		log.Info("agent started", "listen", cfg.Listen, "driver", *driver,
			"scenes", len(cfg.Scenes), "schedules", len(cfg.Schedules))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server stopped", "err", err)
			os.Exit(1)
		}
	}()

	// Sauberes Herunterfahren bei SIGINT/SIGTERM (systemd).
	<-runCtx.Done()
	log.Info("shutting down…")
	ctx, cancel := context.WithTimeout(context.Background(), api.MaxRequestDuration+5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	select {
	case <-schedDone:
	case <-ctx.Done():
		log.Warn("scheduler did not stop in time")
	}
}

// buildController waehlt den Lautsprecher-Treiber.
//   - mock:  in-memory, keine Hardware (Tests, lokale Entwicklung)
//   - sonos: echte Geraete via UPnP/SOAP im LAN
func buildController(driver string, cfg *config.Config, log *slog.Logger) (speaker.SpeakerController, error) {
	switch driver {
	case "mock":
		log.Warn("MOCK DRIVER active -- NO real hardware is controlled")
		return mock.New(referencedSpeakers(cfg)...), nil
	case "sonos":
		overrides := cfg.FixedIPOverrides()
		log.Info("sonos driver active (UPnP/SOAP on the LAN)", "fixed_ips", len(overrides))
		return sonos.New(overrides, log), nil
	default:
		return nil, errUnknownDriver
	}
}

// referencedSpeakers sammelt alle in Szenen/Config genannten Lautsprecher,
// damit der Mock sie als erreichbar meldet.
func referencedSpeakers(cfg *config.Config) []string {
	set := map[string]bool{}
	for name := range cfg.Speakers {
		set[name] = true
	}
	for _, sc := range cfg.Scenes {
		for _, m := range sc.Members {
			set[m.Speaker] = true
		}
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	return out
}

func newLogger(debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var errUnknownDriver = sentinel("unknown driver (allowed: mock, sonos)")

type sentinel string

func (s sentinel) Error() string { return string(s) }
