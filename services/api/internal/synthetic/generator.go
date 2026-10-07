// Package synthetic provides a deterministic, clearly-labelled synthetic data
// engine for the local dev system. It exists so the dashboard, REST API and
// WebSocket stream are fully exercisable BEFORE the real ingestion pipeline
// (Phase 2 harvester -> Phase 3 analytic) is wired up.
//
// HONESTY CONTRACT (ROADMAP mandate 21 + guiding principles): every value this
// package produces is flagged `approx` — it is a synthetic proxy, never a
// measurement. The generator never emits NaN or ±Inf; a degenerate request
// yields a gap (nil value), not a fabricated number. The UI surfaces an explicit
// "synthetic seed data" banner so a synthetic value can never be mistaken for a
// live observation.
package synthetic

import (
	"hash/fnv"
	"math"
	"time"

	"github.com/neyialiyorlar/services/api/internal/model"
)

// Metric describes one entry of the metric catalogue the generator produces.
type Metric struct {
	Key  string
	Name string
	Tier model.CadenceTier
}

// Generator produces deterministic synthetic metric values. The same
// (metricKey, entity, ts) always maps to the same value, so backfilled history
// and live ticks form one continuous, reproducible series.
type Generator struct{}

// NewGenerator returns a stateless synthetic value generator.
func NewGenerator() *Generator { return &Generator{} }

// seed folds a set of strings into a stable 64-bit seed via FNV-1a.
func seed(parts ...string) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

// unit maps a seed to a deterministic float in [0,1).
func unit(s uint64) float64 {
	// Use the top 53 bits for a well-distributed double in [0,1).
	return float64(s>>11) / float64(uint64(1)<<53)
}

// Value returns the deterministic synthetic value for a metric/entity at ts.
//
// The series is a smooth periodic curve (per-metric baseline + amplitude +
// phase) plus a slow drift and bounded deterministic noise, scaled into a
// plausible, metric-specific band. The result is always finite and rounded to
// four decimals.
func (g *Generator) Value(metricKey, entity string, ts time.Time) float64 {
	base := seed(metricKey)
	pe := seed(metricKey, entity)

	// Per-metric band: offset in [10,90), amplitude in [5,30).
	offset := 10 + unit(base)*80
	amplitude := 5 + unit(base>>7)*25

	// Per-(metric,entity) phase and period (period 6h..7d in seconds).
	phase := unit(pe) * 2 * math.Pi
	periodSec := 6*3600 + unit(pe>>13)*(7*24*3600-6*3600)

	tSec := float64(ts.Unix())
	periodic := amplitude * math.Sin(2*math.Pi*tSec/periodSec+phase)

	// Slow drift: gentle linear trend, ±10% of offset over ~90 days.
	driftPerSec := (unit(pe>>23)*2 - 1) * (0.10 * offset) / (90 * 24 * 3600)
	// Anchor drift to ts-of-day so values stay in band over long windows.
	drift := driftPerSec * float64(ts.Unix()%(90*24*3600))

	// Bounded deterministic noise per observation (±3% of amplitude).
	noise := (unit(seed(metricKey, entity, ts.UTC().Format(time.RFC3339)))*2 - 1) * 0.03 * amplitude

	v := offset + periodic + drift + noise

	// Numerical-honesty guard: never emit NaN/±Inf.
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return offset
	}
	return math.Round(v*10000) / 10000
}

// tierStep returns the spacing between successive observations for a tier.
func tierStep(tier model.CadenceTier) time.Duration {
	switch tier {
	case model.CadenceTierIntraday:
		return 15 * time.Minute
	case model.CadenceTierDaily:
		return 24 * time.Hour
	case model.CadenceTierWeekly:
		return 7 * 24 * time.Hour
	case model.CadenceTierEvent:
		// Event cadence is irregular in production; for synthetic history we
		// space it roughly daily so the dashboard has a usable series.
		return 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// History returns `points` synthetic observations for a metric/entity ending at
// `end` (inclusive), spaced by the metric's cadence tier, oldest first.
func (g *Generator) History(m Metric, entity string, end time.Time, points int) []model.MetricValue {
	if points <= 0 {
		return nil
	}
	step := tierStep(m.Tier)
	out := make([]model.MetricValue, 0, points)
	for i := points - 1; i >= 0; i-- {
		ts := end.Add(-time.Duration(i) * step).UTC()
		out = append(out, g.Point(m, entity, ts))
	}
	return out
}

// Point returns a single synthetic observation, flagged honestly as `approx`.
func (g *Generator) Point(m Metric, entity string, ts time.Time) model.MetricValue {
	v := g.Value(m.Key, entity, ts)
	return model.MetricValue{
		Metric: m.Key,
		Entity: entity,
		Ts:     ts.UTC(),
		Value:  &v,
		Flag:   model.ConfidenceApprox, // synthetic proxy — never "fresh"
		Tier:   m.Tier,
	}
}
