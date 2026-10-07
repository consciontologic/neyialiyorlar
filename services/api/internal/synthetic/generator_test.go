package synthetic

import (
	"math"
	"testing"
	"time"

	"github.com/neyialiyorlar/services/api/internal/model"
)

func testMetric() Metric {
	return Metric{Key: "velocity_accumulation", Name: "Velocity Accumulation", Tier: model.CadenceTierDaily}
}

// TestValueDeterministic verifies the generator is a pure function of its inputs:
// the same (metric, entity, ts) always maps to the same value. This is what lets
// backfilled history and live ticks form one continuous, reproducible series.
func TestValueDeterministic(t *testing.T) {
	g1 := NewGenerator()
	g2 := NewGenerator()
	ts := time.Date(2025, 3, 1, 12, 0, 0, 0, time.UTC)

	for _, entity := range []string{"banks", "GARAN", "THYAO"} {
		v1 := g1.Value("nimvi", entity, ts)
		v2 := g2.Value("nimvi", entity, ts)
		if v1 != v2 {
			t.Fatalf("non-deterministic value for %s: %v != %v", entity, v1, v2)
		}
	}
}

// TestValueAlwaysFinite enforces the numerical-honesty guard: a synthetic value
// is never NaN or ±Inf, across a wide spread of metrics, entities and times.
func TestValueAlwaysFinite(t *testing.T) {
	g := NewGenerator()
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	metrics := []string{"velocity_accumulation", "nimvi", "basis_spread", "real_yield_divergence"}
	entities := []string{"banks", "GARAN", "AKBNK", "ISCTR", "THYAO"}

	for _, m := range metrics {
		for _, e := range entities {
			for i := 0; i < 500; i++ {
				ts := base.Add(time.Duration(i) * 7 * time.Hour)
				v := g.Value(m, e, ts)
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("non-finite value for %s/%s at %s: %v", m, e, ts, v)
				}
			}
		}
	}
}

// TestPointIsApprox verifies every synthetic point is honestly flagged `approx`
// (never "fresh"), carries a non-nil value, the requested tier, and a UTC ts.
func TestPointIsApprox(t *testing.T) {
	g := NewGenerator()
	m := testMetric()
	ts := time.Date(2025, 6, 23, 9, 30, 0, 0, time.FixedZone("IST", 3*3600))

	p := g.Point(m, "banks", ts)
	if p.Flag != model.ConfidenceApprox {
		t.Errorf("expected approx flag, got %s", p.Flag)
	}
	if p.Value == nil {
		t.Error("expected non-nil value")
	}
	if p.Tier != m.Tier {
		t.Errorf("expected tier %s, got %s", m.Tier, p.Tier)
	}
	if p.Ts.Location() != time.UTC {
		t.Errorf("expected UTC ts, got %s", p.Ts.Location())
	}
	if p.Metric != m.Key || p.Entity != "banks" {
		t.Errorf("unexpected identity: %s/%s", p.Metric, p.Entity)
	}
}

// TestHistoryLengthAndOrdering verifies History returns the requested number of
// points, oldest-first, ending exactly at `end`, spaced by the tier step.
func TestHistoryLengthAndOrdering(t *testing.T) {
	g := NewGenerator()
	m := testMetric()
	end := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)

	const points = 30
	hist := g.History(m, "banks", end, points)
	if len(hist) != points {
		t.Fatalf("expected %d points, got %d", points, len(hist))
	}
	if !hist[len(hist)-1].Ts.Equal(end) {
		t.Errorf("expected last ts %s, got %s", end, hist[len(hist)-1].Ts)
	}
	for i := 1; i < len(hist); i++ {
		if !hist[i].Ts.After(hist[i-1].Ts) {
			t.Fatalf("history not strictly ascending at %d: %s !> %s", i, hist[i].Ts, hist[i-1].Ts)
		}
	}
	// Daily tier => 24h spacing.
	gap := hist[1].Ts.Sub(hist[0].Ts)
	if gap != 24*time.Hour {
		t.Errorf("expected 24h spacing for daily tier, got %s", gap)
	}
}

// TestHistoryNonPositivePoints verifies a degenerate request yields no points
// (a gap), never a fabricated series.
func TestHistoryNonPositivePoints(t *testing.T) {
	g := NewGenerator()
	if got := g.History(testMetric(), "banks", time.Now(), 0); got != nil {
		t.Errorf("expected nil for 0 points, got %v", got)
	}
	if got := g.History(testMetric(), "banks", time.Now(), -5); got != nil {
		t.Errorf("expected nil for negative points, got %v", got)
	}
}

// TestValueVariesByEntity guards against a degenerate constant generator: two
// distinct entities should differ on at least one timestamp.
func TestValueVariesByEntity(t *testing.T) {
	g := NewGenerator()
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	differs := false
	for i := 0; i < 10; i++ {
		ts := base.Add(time.Duration(i) * 24 * time.Hour)
		if g.Value("nimvi", "banks", ts) != g.Value("nimvi", "GARAN", ts) {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("expected distinct series for distinct entities")
	}
}
