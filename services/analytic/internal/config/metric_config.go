package config

import (
	"crypto/md5"
	"fmt"
)

// MetricConfig holds per-metric parameters from the config file.
// These parameters are folded into inputs_hash to make derived rows idempotent
// across parameter changes (mandate per bullet 13).
type MetricConfig struct {
	Key            string                 `yaml:"key"`            // metric identifier
	Tier           string                 `yaml:"tier"`           // intraday|daily|weekly|event
	Window         int                    `yaml:"window"`         // size of rolling window
	Lookback       int                    `yaml:"lookback"`       // zscore lookback
	MinSample      int                    `yaml:"min_sample"`     // minimum sample size
	OverlapCap     int                    `yaml:"overlap_cap"`    // max fund fan-out before degradation
	RequiredSources []string              `yaml:"required_sources"`
	Extra          map[string]interface{} `yaml:"-"` // catch-all for future extensions
}

// ParameterHash returns a hash of the parameter set (for inputs_hash composition).
// This ensures that changing a window or lookback produces a new derived row.
func (mc *MetricConfig) ParameterHash() string {
	h := md5.New()
	// Hash the core parameters that affect computation.
	fmt.Fprintf(h, "window:%d|lookback:%d|min_sample:%d|overlap_cap:%d",
		mc.Window, mc.Lookback, mc.MinSample, mc.OverlapCap)
	for _, s := range mc.RequiredSources {
		fmt.Fprintf(h, "|source:%s", s)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// DefaultMetricConfig returns a config with reasonable defaults.
func DefaultMetricConfig(metricKey string) *MetricConfig {
	return &MetricConfig{
		Key:        metricKey,
		Window:     30,
		Lookback:   60,
		MinSample:  5,
		OverlapCap: 100,
		Extra:      make(map[string]interface{}),
	}
}
