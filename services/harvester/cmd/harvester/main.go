package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	harvlog "github.com/neyialiyorlar/services/harvester/internal/log"
	"github.com/neyialiyorlar/services/harvester/internal/parse"
	"github.com/neyialiyorlar/services/harvester/internal/source"
	"github.com/neyialiyorlar/services/harvester/internal/store"
	"github.com/neyialiyorlar/services/shared/config"
	sharedlog "github.com/neyialiyorlar/services/shared/logger"
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
	logger := sharedlog.New("harvester", cfg.Runtime.LogLevel)

	// If healthcheck flag is set, just verify config and exit
	if *healthcheck {
		logger.Info("healthcheck passed", "config_version", cfg.Version)
		os.Exit(0)
	}

	logger.Info("Harvester service starting",
		"log_level", cfg.Runtime.LogLevel,
		"database_host", cfg.Database.Host,
		"redis_addr", cfg.Redis.Addr,
	)

	// Connect to Postgres — build DSN from shared config fields
	password := os.Getenv("DB_PASSWORD")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.Database.User, password, cfg.Database.Host,
		cfg.Database.Port, cfg.Database.Name)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		logger.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	// Setup graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// Ping DB (retry up to 30s for docker-compose startup ordering)
	for i := 0; i < 30; i++ {
		if err := db.PingContext(ctx); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			os.Exit(0)
		case <-time.After(1 * time.Second):
		}
	}
	logger.Info("harvester initialized")

	// Yahoo Finance price fetcher: backfill 90 days on startup, then daily.
	fetcher := source.NewYahooPriceFetcher(db, logger)

	// Schema-drift detection: persist BREAKING structural changes in the Yahoo
	// chart JSON to drift_alert so they surface on the İzleme Paneli and survive
	// restarts. Each alert fans out to the operator log (WARN) and the database
	// store. Gated on config so it can be disabled without a rebuild.
	if cfg.Drift.Enabled {
		driftStore := store.NewDriftStore(db, logger)
		detector := parse.NewDriftDetector(nil)
		detector.SetLogger(parse.MultiLogger{harvlog.NewLogger(logger), driftStore})
		fetcher.SetDriftDetector(detector)
		logger.Info("schema-drift detection enabled", slog.String("sources", "yahoo"))
	}

	go runPriceFetcher(ctx, fetcher, logger)

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("shutting down gracefully")
}

// runPriceFetcher runs an initial backfill then a daily refresh.
func runPriceFetcher(ctx context.Context, fetcher *source.YahooPriceFetcher, logger *slog.Logger) {
	// Initial backfill: fetch 90 days of history
	logger.Info("starting price backfill (90 days)")
	if err := fetcher.FetchAndStore(ctx, 90); err != nil {
		logger.Warn("price backfill error", slog.String("error", err.Error()))
	}

	// Daily refresh: run once per day at 19:00 Istanbul time (after BIST close)
	for {
		nextRun := nextDailyRun(19, 0, "Europe/Istanbul")
		logger.Info("next price refresh scheduled", slog.Time("at", nextRun))
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(nextRun)):
		}
		logger.Info("running daily price refresh")
		if err := fetcher.FetchAndStore(ctx, 5); err != nil {
			logger.Warn("daily price refresh error", slog.String("error", err.Error()))
		}
	}
}

// nextDailyRun returns the next wall-clock time for hour:min in loc.
func nextDailyRun(hour, min int, locName string) time.Time {
	loc, err := time.LoadLocation(locName)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, loc)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
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
