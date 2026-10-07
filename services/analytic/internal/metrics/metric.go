package metrics

import (
	"context"

	"github.com/neyialiyorlar/services/analytic/internal/model"
)

// Metric defines the interface for a single metric computation.
// Each metric reads a RawSet and produces a Derived result.
type Metric interface {
	// Key returns the canonical metric key (e.g., "m01_velocity_accumulation").
	Key() string

	// Description returns a human-readable description of the metric.
	Description() string

	// Cadence returns when this metric recomputes (intraday/daily/weekly/event).
	Cadence() model.Cadence

	// Compute takes a context and a RawSet and returns the computed Derived metric value.
	// It must be a total function: never returning NaN/Inf, always returning a valid
	// value or a gap (Derived with Value = nil).
	Compute(ctx context.Context, rawSet *model.RawSet) (*model.Derived, error)

	// RequiredSources returns the sources this metric depends on (e.g., ["kap", "bist"]).
	// An unavailable source should trigger a gap row.
	RequiredSources() []string
}
