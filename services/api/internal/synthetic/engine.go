package synthetic

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"time"

	"github.com/neyialiyorlar/services/api/internal/model"
)

// Entity is a seeded reference entity (security / fund / index / basket).
type Entity struct {
	ID            string // stable entity id (ISIN / MKK code / aggregate key)
	Type          string // security | fund | index | basket
	DisplayTicker string
	ISIN          string
}

// MetricRow is one derived row destined for the metric_value table.
type MetricRow struct {
	MetricKey  string
	Entity     string
	Ts         time.Time
	Value      *float64
	Flag       model.Confidence
	Tier       model.CadenceTier
	InputsHash []byte
}

// Store persists synthetic reference + derived data. The Postgres implementation
// lives in postgres_store.go; tests use an in-memory fake so the engine is
// exercisable under offline `make ci`.
type Store interface {
	EnsureEntities(ctx context.Context, entities []Entity) error
	EnsureSources(ctx context.Context, sources []string, at time.Time) error
	UpsertMetric(ctx context.Context, row MetricRow) error
	HasMetricValues(ctx context.Context) (bool, error)
	Counts(ctx context.Context) (map[string]int64, error)
}

// Broadcaster pushes a freshly-generated value to live WebSocket subscribers.
type Broadcaster interface {
	Broadcast(v model.MetricValue)
}

// Cache writes the hot "latest" entry so REST reads hit Redis, mirroring the
// production read path. It is optional (may be nil).
type Cache interface {
	SetLatest(ctx context.Context, v model.MetricValue) error
}

// Config controls the synthetic engine.
type Config struct {
	Enabled       bool
	TickInterval  time.Duration
	HistoryPoints int
	Entities      []Entity
	Sources       []string
	Catalog       []Metric
}

// Engine backfills synthetic history on startup and emits live ticks. It is the
// dev-mode stand-in for the harvester→analytic pipeline; Phase 2/3 replace it
// with real ingestion and computation.
type Engine struct {
	cfg    Config
	gen    *Generator
	store  Store
	bc     Broadcaster
	cache  Cache
	logger *slog.Logger
}

// NewEngine constructs a synthetic engine. store must be non-nil; bc and cache
// may be nil (broadcast / caching are then skipped).
func NewEngine(cfg Config, store Store, bc Broadcaster, cache Cache, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{
		cfg:    cfg,
		gen:    NewGenerator(),
		store:  store,
		bc:     bc,
		cache:  cache,
		logger: logger,
	}
}

// inputsHash derives a deterministic, non-empty idempotency hash for a synthetic
// row. The "synthetic" prefix marks provenance so these rows are never confused
// with hashes of real harvested inputs.
func inputsHash(metricKey, entity string, ts time.Time) []byte {
	h := sha256.Sum256([]byte("synthetic|" + metricKey + "|" + entity + "|" + ts.UTC().Format(time.RFC3339)))
	return h[:]
}

// pairs returns every (metric, entity) combination in the catalogue.
func (e *Engine) pairs() []struct {
	m      Metric
	entity string
} {
	out := make([]struct {
		m      Metric
		entity string
	}, 0, len(e.cfg.Catalog)*len(e.cfg.Entities))
	for _, m := range e.cfg.Catalog {
		for _, ent := range e.cfg.Entities {
			out = append(out, struct {
				m      Metric
				entity string
			}{m, ent.ID})
		}
	}
	return out
}

// Backfill seeds reference data and, if metric_value is empty, generates the
// historical synthetic series. It is idempotent: with data already present the
// history generation is skipped.
func (e *Engine) Backfill(ctx context.Context) error {
	now := time.Now().UTC()

	if err := e.store.EnsureEntities(ctx, e.cfg.Entities); err != nil {
		return err
	}
	if err := e.store.EnsureSources(ctx, e.cfg.Sources, now); err != nil {
		return err
	}

	has, err := e.store.HasMetricValues(ctx)
	if err != nil {
		return err
	}
	if has {
		e.logger.Info("synthetic backfill skipped: metric_value already populated")
		return nil
	}

	points := e.cfg.HistoryPoints
	if points <= 0 {
		points = 90
	}

	rows := 0
	for _, m := range e.cfg.Catalog {
		for _, ent := range e.cfg.Entities {
			for _, p := range e.gen.History(m, ent.ID, now, points) {
				if err := e.store.UpsertMetric(ctx, e.rowFrom(p)); err != nil {
					return err
				}
				rows++
			}
		}
	}
	e.logger.Info("synthetic backfill complete",
		"rows", rows,
		"metrics", len(e.cfg.Catalog),
		"entities", len(e.cfg.Entities),
		"points_each", points,
	)
	return nil
}

// Tick generates one fresh observation per (metric, entity) at `at`, persists
// it, refreshes the hot cache, and broadcasts it to WebSocket subscribers.
// Returns the number of values emitted.
func (e *Engine) Tick(ctx context.Context, at time.Time) (int, error) {
	at = at.UTC()
	n := 0
	for _, p := range e.pairs() {
		mv := e.gen.Point(p.m, p.entity, at)
		if err := e.store.UpsertMetric(ctx, e.rowFrom(mv)); err != nil {
			return n, err
		}
		if e.cache != nil {
			if err := e.cache.SetLatest(ctx, mv); err != nil {
				e.logger.Warn("synthetic cache set failed", "err", err, "metric", mv.Metric, "entity", mv.Entity)
			}
		}
		if e.bc != nil {
			e.bc.Broadcast(mv)
		}
		n++
	}
	e.logger.Info("synthetic tick emitted", "values", n, "ts", at)
	return n, nil
}

// rowFrom converts a generated value into a persistable MetricRow.
func (e *Engine) rowFrom(mv model.MetricValue) MetricRow {
	return MetricRow{
		MetricKey:  mv.Metric,
		Entity:     mv.Entity,
		Ts:         mv.Ts,
		Value:      mv.Value,
		Flag:       mv.Flag,
		Tier:       mv.Tier,
		InputsHash: inputsHash(mv.Metric, mv.Entity, mv.Ts),
	}
}

// Run drives the live tick loop until ctx is cancelled. It is meant to run in a
// goroutine after Backfill.
func (e *Engine) Run(ctx context.Context) {
	interval := e.cfg.TickInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	e.logger.Info("synthetic engine running", "tick_interval", interval)
	for {
		select {
		case <-ctx.Done():
			e.logger.Info("synthetic engine stopping")
			return
		case t := <-ticker.C:
			if _, err := e.Tick(ctx, t); err != nil {
				e.logger.Error("synthetic tick failed", "err", err)
			}
		}
	}
}
