package confidence

import (
	"github.com/neyialiyorlar/services/analytic/internal/model"
)

// Worst returns the maximum (most conservative) confidence flag.
// Ordering: approx > stale > fresh.
// This is the same as model.Worst but kept here for convenience in the confidence package.
func Worst(flags ...model.Confidence) model.Confidence {
	return model.Worst(flags...)
}

// ProxyMetrics is the set of metrics that are inherently proxies (no direct measurement).
// These metrics are seeded with Approx confidence regardless of input freshness.
var ProxyMetrics = map[string]bool{
	"m01_velocity_accumulation": true, // proxy: derived from holdings
	"m02_holdings_overlap":       true, // proxy: derived from holdings
	"m03_category_concentration": true, // proxy: derived from holdings
	"m04_rebalance_frequency":    true, // proxy: derived from transaction timing
	"m07_estimated_annual_return": true, // proxy: forward-looking estimate
}

// IsProxyMetric checks if a metric is inherently a proxy.
func IsProxyMetric(metricKey string) bool {
	return ProxyMetrics[metricKey]
}

// SeedConfidence seeds the confidence flag for a metric based on its input set.
// If the metric is a proxy, it is seeded as Approx.
// Otherwise, it uses the worst confidence of the input set.
func SeedConfidence(metricKey string, inputs *model.RawSet) model.Confidence {
	if IsProxyMetric(metricKey) {
		return model.ConfApprox
	}
	if inputs != nil {
		return inputs.WorstConfidence
	}
	return model.ConfFresh
}

// PropagateConfidence merges a base confidence with output confidence flags.
// The result is the worst of the two.
func PropagateConfidence(base, output model.Confidence) model.Confidence {
	return Worst(base, output)
}

// Test helper: returns all proxy metrics as a slice (for unit test coverage).
func ProxyMetricsList() []string {
	metrics := make([]string, 0, len(ProxyMetrics))
	for m := range ProxyMetrics {
		metrics = append(metrics, m)
	}
	return metrics
}
