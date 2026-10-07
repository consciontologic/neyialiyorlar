package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/neyialiyorlar/services/shared/config"
	sharedlog "github.com/neyialiyorlar/services/shared/logger"

	// Phase 3 components
	"github.com/neyialiyorlar/services/analytic/internal/ingestwatch"
	"github.com/neyialiyorlar/services/analytic/internal/metrics"
	"github.com/neyialiyorlar/services/analytic/internal/recompute"
	"github.com/neyialiyorlar/services/analytic/internal/scheduler"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "run healthcheck and exit")
	flag.Parse()

	// Load config
	cfg, err := config.Load("/etc/neyi/config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup structured logging
	logger := sharedlog.New("analytic", cfg.Runtime.LogLevel)

	// If healthcheck flag is set, just verify config and exit
	if *healthcheck {
		logger.Info("healthcheck passed", "config_version", cfg.Version)
		os.Exit(0)
	}

	logger.Info("Analytic service starting",
		"log_level", cfg.Runtime.LogLevel,
		"database_host", cfg.Database.Host,
		"redis_addr", cfg.Redis.Addr,
	)

	// Setup graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// Initialize Phase 3 components: metric computation pipeline.
	registry := metrics.NewRegistry()
	logger.Info("initialized metric registry", "count", registry.Count())

	recomputer := recompute.NewRecomputer()
	logger.Info("initialized recompute engine (in-memory; Postgres in Phase 4)")

	sch := scheduler.NewScheduler(logger)
	logger.Info("initialized scheduler")

	watcher := ingestwatch.NewWatcher(logger)
	logger.Info("initialized ingestwatch")

	// Wire hybrid recompute model: event-driven (Redis) as primary, cadence (scheduler) as fallback.
	listener := NewHybridListener(registry, recomputer, logger)
	watcher.Register(listener)

	// Start scheduler in a goroutine.
	go func() {
		logger.Info("starting scheduler")
		if err := sch.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("scheduler error", "err", err)
		}
	}()

	// Start watcher in a goroutine.
	go func() {
		logger.Info("starting watcher")
		if err := watcher.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("watcher error", "err", err)
		}
	}()

	// Main loop: process scheduler ticks.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case tick := <-sch.TickChannel():
				if tick != nil {
					listener.OnCadenceTick(ctx, tick)
				}
			}
		}
	}()

	logger.Info("analytic core initialized with hybrid recompute (event + cadence)")

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("shutting down gracefully")
}

func getLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// HybridListener combines event-driven and cadence-tick recomputation paths.
type HybridListener struct {
	registry   *metrics.Registry
	recomputer *recompute.Recomputer
	logger     *slog.Logger
}

// NewHybridListener creates a listener that bridges event-driven and cadence-based paths.
func NewHybridListener(
	registry *metrics.Registry,
	recomputer *recompute.Recomputer,
	logger *slog.Logger,
) *HybridListener {
	return &HybridListener{
		registry:   registry,
		recomputer: recomputer,
		logger:     logger,
	}
}

// OnRawIngested handles raw ingestion events (event-driven primary path).
// It triggers event-cadence metrics for the given source.
func (h *HybridListener) OnRawIngested(ctx context.Context, event *ingestwatch.RawEvent) error {
	h.logger.Info("raw ingested event", "source", event.Source, "natural_key", event.NaturalKey)
	// TODO Phase 3: implement event-driven recompute trigger.
	return nil
}

// OnCadenceTick handles periodic recomputation (fallback path).
// It triggers intraday/daily/weekly-cadence metrics.
func (h *HybridListener) OnCadenceTick(ctx context.Context, tick *scheduler.CadenceTick) error {
	h.logger.Info("cadence tick", "cadence", tick.Cadence, "ts", tick.Timestamp)
	// TODO Phase 3: implement cadence-based recompute trigger.
	return nil
}
