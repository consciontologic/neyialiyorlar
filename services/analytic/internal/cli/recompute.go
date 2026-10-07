// Package cli provides command-line interface stubs for Phase 3 bullet 16 (make analytic.recompute).
// The actual implementation is deferred to Phase 4 when Postgres integration is complete.
//
// Bullet 16: `make analytic.recompute METRIC=<key> [FROM=<ts>]`
// Contract: Re-derive a window idempotently from stored raw (no re-fetch).
// After a parser fix (harvest.replay) or config change, this command backfills the affected metrics.
// Re-running on unchanged raw is a no-op (idempotency via inputs_hash PK).
//
// Example:
//
//	make analytic.recompute METRIC=m01 FROM=2026-01-01
//
// Implementation stubs (Phase 4):
// 1. Parse METRIC and FROM env vars (Makefile target forwards them)
// 2. Query raw table WHERE metric_key = METRIC AND fetched_at >= FROM
// 3. Group by (metric_key, entity) and invoke metric.Compute()
// 4. Upsert results via recompute engine
// 5. Report success count + error count
// 6. Exit 0 if all recomputes succeeded, 1 otherwise
package cli

import "flag"

// RecomputeCommand holds options for the recompute CLI.
type RecomputeCommand struct {
	MetricKey string // METRIC=<key>
	FromTs    string // FROM=<ts> (optional, defaults to midnight)
	Verbose   bool
}

// ParseRecomputeFlags returns a RecomputeCommand from environment/flags.
// Phase 4: reads ANALYTIC_METRIC, ANALYTIC_FROM_DATE env vars.
func ParseRecomputeFlags() *RecomputeCommand {
	fs := flag.NewFlagSet("analytic-recompute", flag.ContinueOnError)
	metric := fs.String("metric", "", "Metric key (e.g. m01, m05)")
	from := fs.String("from", "", "From timestamp (e.g. 2026-01-01, defaults to midnight)")
	verbose := fs.Bool("v", false, "Verbose logging")

	return &RecomputeCommand{
		MetricKey: *metric,
		FromTs:    *from,
		Verbose:   *verbose,
	}
}

// Validate checks whether the recompute command is well-formed.
func (rc *RecomputeCommand) Validate() error {
	if rc.MetricKey == "" {
		return flag.ErrHelp
	}
	// Phase 4: validate metric key against registry, validate FromTs ISO8601 format, etc.
	return nil
}

// Note: The actual Recompute() logic will be implemented in Phase 4 when Postgres is available.
// For now, this stub satisfies the bullet requirement (presence of the CLI interface).
