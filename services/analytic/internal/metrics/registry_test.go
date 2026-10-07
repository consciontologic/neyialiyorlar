package metrics

import (
	"context"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Errorf("NewRegistry() returned nil")
	}
	if r.Count() != 10 {
		t.Errorf("NewRegistry() registered %d metrics, want 10", r.Count())
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()

	m := r.Get("m01_velocity_accumulation")
	if m == nil {
		t.Errorf("Registry.Get(m01_velocity_accumulation) = nil, want Metric")
	}
	if m.Key() != "m01_velocity_accumulation" {
		t.Errorf("Registry.Get(m01).Key() = %s, want m01_velocity_accumulation", m.Key())
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()

	m := r.Get("nonexistent_metric")
	if m != nil {
		t.Errorf("Registry.Get(nonexistent) = %v, want nil", m)
	}
}

func TestRegistryAll(t *testing.T) {
	r := NewRegistry()
	all := r.All()
	if len(all) != 10 {
		t.Errorf("Registry.All() returned %d metrics, want 10", len(all))
	}
}

func TestRegistryKeys(t *testing.T) {
	r := NewRegistry()
	keys := r.Keys()
	if len(keys) != 10 {
		t.Errorf("Registry.Keys() returned %d keys, want 10", len(keys))
	}

	// Check that all expected keys are present.
	expected := map[string]bool{
		"m01_velocity_accumulation":      true,
		"m02_holdings_overlap":           true,
		"m03_category_concentration":     true,
		"m04_rebalance_frequency":        true,
		"m05_diversification_ratio":      true,
		"m06_transaction_diversity":      true,
		"m07_estimated_annual_return":    true,
		"m08_rolling_return_volatility":  true,
		"m09_burst_detection_score":      true,
		"m10_real_yield":                 true,
	}

	found := make(map[string]bool)
	for _, k := range keys {
		found[k] = true
	}

	for exp := range expected {
		if !found[exp] {
			t.Errorf("Registry.Keys() missing %s", exp)
		}
	}
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()
	initial := r.Count()

	// Attempt to register a duplicate (should overwrite).
	r.Register(NewM01VelocityAccumulation())
	if r.Count() != initial {
		t.Errorf("Registry.Register() duplicate changed count: %d, want %d", r.Count(), initial)
	}
}

func TestAllMetricsHaveCadence(t *testing.T) {
	r := NewRegistry()
	for _, m := range r.All() {
		cadence := m.Cadence()
		if cadence == "" {
			t.Errorf("Metric %s has empty cadence", m.Key())
		}
	}
}

func TestAllMetricsHaveRequiredSources(t *testing.T) {
	r := NewRegistry()
	for _, m := range r.All() {
		sources := m.RequiredSources()
		if len(sources) == 0 {
			t.Errorf("Metric %s has no required sources", m.Key())
		}
	}
}

func TestAllMetricsCanCompute(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()

	for _, m := range r.All() {
		// Try to compute with nil rawSet (should produce a gap).
		result, err := m.Compute(ctx, nil)
		if err != nil {
			t.Errorf("Metric %s Compute(nil) returned error: %v", m.Key(), err)
		}
		if result == nil {
			t.Errorf("Metric %s Compute(nil) returned nil", m.Key())
		}
		if result.Value != nil {
			t.Errorf("Metric %s Compute(nil) should return a gap (nil value), got %v", m.Key(), *result.Value)
		}
	}
}
