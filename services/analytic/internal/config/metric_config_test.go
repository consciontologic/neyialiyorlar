package config

import (
	"testing"
)

func TestParameterHash(t *testing.T) {
	cfg1 := &MetricConfig{
		Key:        "m01",
		Window:     30,
		Lookback:   60,
		MinSample:  5,
		OverlapCap: 100,
	}

	cfg2 := &MetricConfig{
		Key:        "m01",
		Window:     30,
		Lookback:   60,
		MinSample:  5,
		OverlapCap: 100,
	}

	// Same config → same hash
	hash1 := cfg1.ParameterHash()
	hash2 := cfg2.ParameterHash()
	if hash1 != hash2 {
		t.Errorf("ParameterHash: identical configs produce different hashes: %s != %s", hash1, hash2)
	}

	// Different window → different hash
	cfg3 := &MetricConfig{
		Key:        "m01",
		Window:     35, // changed
		Lookback:   60,
		MinSample:  5,
		OverlapCap: 100,
	}
	hash3 := cfg3.ParameterHash()
	if hash1 == hash3 {
		t.Errorf("ParameterHash: different windows produce same hash: %s", hash1)
	}
}

func TestDefaultMetricConfig(t *testing.T) {
	cfg := DefaultMetricConfig("m05")
	if cfg.Key != "m05" {
		t.Errorf("DefaultMetricConfig: Key = %s, want m05", cfg.Key)
	}
	if cfg.Window <= 0 {
		t.Errorf("DefaultMetricConfig: Window = %d, want > 0", cfg.Window)
	}
	if cfg.MinSample <= 0 {
		t.Errorf("DefaultMetricConfig: MinSample = %d, want > 0", cfg.MinSample)
	}
}
