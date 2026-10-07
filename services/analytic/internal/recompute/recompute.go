package recompute

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/neyialiyorlar/services/analytic/internal/model"
)

// RecomputeResult represents the outcome of a recompute operation.
type RecomputeResult struct {
	MetricKey   string
	Entity      string
	Timestamp   time.Time
	NewRow      bool // true if a new row was inserted; false if it was a no-op (idempotent)
	Error       error
}

// Recomputer handles idempotent upsert of derived metrics into storage.
type Recomputer struct {
	// In a real implementation, this would connect to Postgres.
	// For now, this is a placeholder that tracks inserted rows for testing.
	storage map[string]*model.Derived // keyed by (metric_key, entity, ts, inputs_hash)
	logger  *slog.Logger              // structured logging (mandate 12)
}

// NewRecomputer creates a new recompute engine.
func NewRecomputer() *Recomputer {
	return &Recomputer{
		storage: make(map[string]*model.Derived),
		logger:  slog.Default(), // use default logger; can be overridden for testing
	}
}

// NewRecomputerWithLogger creates a recompute engine with a custom logger.
func NewRecomputerWithLogger(logger *slog.Logger) *Recomputer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Recomputer{
		storage: make(map[string]*model.Derived),
		logger:  logger,
	}
}

// storageKey creates a composite key for idempotent deduplication.
func storageKey(d *model.Derived) string {
	return fmt.Sprintf("%s|%s|%d|%s", d.MetricKey, d.Entity, d.Timestamp.Unix(), d.InputsHash)
}

// UpsertDerived inserts or updates a derived metric row idempotently.
// Returns (newRow=true, error=nil) if a new row was inserted.
// Returns (newRow=false, error=nil) if a row with the same (metric_key, entity, ts, inputs_hash) already exists.
// This implements the ADR-0002 idempotency contract: same inputs → no-op on second call.
// Logs structured events per mandate 12: metric_key, entity, flag, latency_ms, inputs_hash_prefix.
func (r *Recomputer) UpsertDerived(ctx context.Context, d *model.Derived) (*RecomputeResult, error) {
	startTime := time.Now()

	if d == nil {
		r.logger.Error("upsert derived: nil metric")
		return &RecomputeResult{Error: fmt.Errorf("derived metric is nil")}, nil
	}

	// Validate inputs.
	if d.MetricKey == "" {
		r.logger.Error("upsert derived: empty metric_key")
		return &RecomputeResult{Error: fmt.Errorf("metric_key is empty")}, nil
	}
	if d.Entity == "" {
		r.logger.Error("upsert derived: empty entity", "metric_key", d.MetricKey)
		return &RecomputeResult{Error: fmt.Errorf("entity is empty")}, nil
	}
	if d.InputsHash == "" {
		r.logger.Error("upsert derived: empty inputs_hash", "metric_key", d.MetricKey, "entity", d.Entity)
		return &RecomputeResult{Error: fmt.Errorf("inputs_hash is empty")}, nil
	}

	key := storageKey(d)

	// Compute latency and inputs_hash_prefix for logging.
	latencyMs := float64(time.Since(startTime).Microseconds()) / 1000.0
	hashPrefix := d.InputsHash
	if len(hashPrefix) > 8 {
		hashPrefix = hashPrefix[:8]
	}

	// Check if the row already exists (idempotency).
	if _, ok := r.storage[key]; ok {
		// Row already exists: this is a no-op.
		r.logger.Info("recompute (idempotent no-op)",
			"metric_key", d.MetricKey,
			"entity", d.Entity,
			"flag", d.Flag.String(),
			"latency_ms", latencyMs,
			"inputs_hash_prefix", hashPrefix,
		)
		return &RecomputeResult{
			MetricKey:   d.MetricKey,
			Entity:      d.Entity,
			Timestamp:   d.Timestamp,
			NewRow:      false,
		}, nil
	}

	// Insert the new row.
	r.storage[key] = d

	// Log the new row event. If it's a gap row, log at WARN with gap_reason.
	if IsGapRow(d) {
		r.logger.Warn("recompute (gap row)",
			"metric_key", d.MetricKey,
			"entity", d.Entity,
			"flag", d.Flag.String(),
			"latency_ms", latencyMs,
			"inputs_hash_prefix", hashPrefix,
		)
	} else {
		r.logger.Info("recompute (new row)",
			"metric_key", d.MetricKey,
			"entity", d.Entity,
			"flag", d.Flag.String(),
			"latency_ms", latencyMs,
			"inputs_hash_prefix", hashPrefix,
		)
	}

	return &RecomputeResult{
		MetricKey:   d.MetricKey,
		Entity:      d.Entity,
		Timestamp:   d.Timestamp,
		NewRow:      true,
	}, nil
}

// GetDerived retrieves a stored derived metric row.
func (r *Recomputer) GetDerived(metricKey, entity string, ts time.Time, inputsHash string) *model.Derived {
	key := fmt.Sprintf("%s|%s|%d|%s", metricKey, entity, ts.Unix(), inputsHash)
	return r.storage[key]
}

// RowCount returns the number of stored rows (for testing).
func (r *Recomputer) RowCount() int {
	return len(r.storage)
}

// IsGapRow returns true if the derived metric is a gap row (value is nil).
func IsGapRow(d *model.Derived) bool {
	return d.Value == nil
}

// CreateGapRow creates a gap row for an unavailable metric.
func CreateGapRow(metricKey, entity string, ts time.Time, reason string, flag model.Confidence) *model.Derived {
	return &model.Derived{
		MetricKey:   metricKey,
		Entity:      entity,
		Timestamp:   ts,
		Value:       nil, // nil indicates a gap
		Flag:        flag,
		ComputedAt:  time.Now(),
		InputsHash:  "", // gap rows don't require an inputs hash
	}
}

// IsIdempotent checks whether two derived rows are "the same" for idempotency purposes.
// Two rows are idempotent if they have the same (metric_key, entity, ts, inputs_hash).
func IsIdempotent(d1, d2 *model.Derived) bool {
	if d1 == nil || d2 == nil {
		return d1 == d2
	}
	return d1.MetricKey == d2.MetricKey &&
		d1.Entity == d2.Entity &&
		d1.Timestamp.Equal(d2.Timestamp) &&
		d1.InputsHash == d2.InputsHash
}
