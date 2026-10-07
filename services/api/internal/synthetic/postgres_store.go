package synthetic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/neyialiyorlar/services/api/internal/model"
)

// PostgresStore is the production Store backed by the metric_value / entity_ref /
// source_health tables. All writes are idempotent upserts.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore wraps a *sql.DB as a synthetic Store.
func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// EnsureEntities inserts reference entities, leaving existing rows untouched.
func (s *PostgresStore) EnsureEntities(ctx context.Context, entities []Entity) error {
	const q = `
		INSERT INTO entity_ref (entity_id, entity_type, display_ticker, valid_from, isin)
		VALUES ($1, $2, $3, DATE '2020-01-01', $4)
		ON CONFLICT (entity_id) DO NOTHING`
	for _, e := range entities {
		var isin any
		if e.ISIN != "" {
			isin = e.ISIN
		}
		if _, err := s.db.ExecContext(ctx, q, e.ID, e.Type, e.DisplayTicker, isin); err != nil {
			return fmt.Errorf("ensure entity %q: %w", e.ID, err)
		}
	}
	return nil
}

// EnsureSources upserts source_health rows as healthy at `at`.
func (s *PostgresStore) EnsureSources(ctx context.Context, sources []string, at time.Time) error {
	const q = `
		INSERT INTO source_health (source, last_ok_at, checked_at, breaker_open, consecutive_failures)
		VALUES ($1, $2, $2, false, 0)
		ON CONFLICT (source) DO UPDATE
		SET last_ok_at = EXCLUDED.last_ok_at,
		    checked_at = EXCLUDED.checked_at,
		    breaker_open = false,
		    consecutive_failures = 0`
	for _, src := range sources {
		if _, err := s.db.ExecContext(ctx, q, src, at.UTC()); err != nil {
			return fmt.Errorf("ensure source %q: %w", src, err)
		}
	}
	return nil
}

// UpsertMetric writes a single derived row idempotently on
// (metric_key, entity, ts, inputs_hash).
func (s *PostgresStore) UpsertMetric(ctx context.Context, row MetricRow) error {
	const q = `
		INSERT INTO metric_value (metric_key, entity, ts, value, flag, tier, inputs_hash, computed_at)
		VALUES ($1, $2, $3, $4, $5::confidence, $6::cadence_tier, $7, now())
		ON CONFLICT (metric_key, entity, ts, inputs_hash) DO UPDATE
		SET value = EXCLUDED.value,
		    flag = EXCLUDED.flag,
		    tier = EXCLUDED.tier,
		    computed_at = now()`
	_, err := s.db.ExecContext(ctx, q,
		row.MetricKey, row.Entity, row.Ts.UTC(), row.Value,
		string(row.Flag), string(row.Tier), row.InputsHash,
	)
	if err != nil {
		return fmt.Errorf("upsert metric %s/%s: %w", row.MetricKey, row.Entity, err)
	}
	return nil
}

// HasMetricValues reports whether any derived rows already exist.
func (s *PostgresStore) HasMetricValues(ctx context.Context) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM metric_value LIMIT 1)`).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check metric_value: %w", err)
	}
	return exists, nil
}

// Counts returns row counts for the operator "investigate" view.
func (s *PostgresStore) Counts(ctx context.Context) (map[string]int64, error) {
	tables := []string{"metric_value", "entity_ref", "source_health", "raw_payload"}
	out := make(map[string]int64, len(tables))
	for _, t := range tables {
		var n int64
		// Table names are from a fixed allow-list above — not user input.
		if err := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT count(*) FROM %s", t)).Scan(&n); err != nil {
			return nil, fmt.Errorf("count %s: %w", t, err)
		}
		out[t] = n
	}
	return out, nil
}

var _ Store = (*PostgresStore)(nil)

// RedisCache adapts a latest-value setter onto the Cache interface. It mirrors
// the GetMetricLatest read path's cache key ("m:<key>:<entity>:latest").
type RedisCache struct {
	set func(ctx context.Context, key, value string, ttl time.Duration) error
	ttl time.Duration
}

// NewRedisCache builds a Cache from a raw set function so the synthetic package
// stays decoupled from the concrete redis client type.
func NewRedisCache(set func(ctx context.Context, key, value string, ttl time.Duration) error, ttl time.Duration) *RedisCache {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &RedisCache{set: set, ttl: ttl}
}

// SetLatest serialises the value and writes it to the hot cache key.
func (c *RedisCache) SetLatest(ctx context.Context, v model.MetricValue) error {
	if c == nil || c.set == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.set(ctx, "m:"+v.Metric+":"+v.Entity+":latest", string(data), c.ttl)
}

var _ Cache = (*RedisCache)(nil)
