package model

import (
	"testing"
)

func TestResolveEntity(t *testing.T) {
	// Phase 3 stub: returns input as-is
	entity := ResolveEntity("disclosure_12345")
	if entity != "disclosure_12345" {
		t.Errorf("ResolveEntity: got %s, want disclosure_12345", entity)
	}

	// ISIN input
	entity2 := ResolveEntity("TR0005000C93")
	if entity2 != "TR0005000C93" {
		t.Errorf("ResolveEntity: got %s, want TR0005000C93", entity2)
	}
}

func TestMonotonicCacheGuard(t *testing.T) {
	guard := &MonotonicCacheGuard{
		ComputedAtKey: "computed_at",
		ValueKey:      "value",
		FlagKey:       "flag",
	}

	if guard.ComputedAtKey != "computed_at" {
		t.Errorf("MonotonicCacheGuard: unexpected ComputedAtKey")
	}
}

func TestOverlapFanoutConfig(t *testing.T) {
	cfg := &OverlapFanoutConfig{
		CapPerCategory: 100,
		ApplySampling:  true,
	}

	if cfg.CapPerCategory != 100 {
		t.Errorf("OverlapFanoutConfig: CapPerCategory = %d, want 100", cfg.CapPerCategory)
	}

	if !cfg.ApplySampling {
		t.Errorf("OverlapFanoutConfig: ApplySampling = false, want true")
	}
}

func TestEntityResolutionConfig(t *testing.T) {
	cfg := &EntityResolutionConfig{
		ISINCode: "TR0005000C93",
		MKKCode:  "GARAN",
		Ticker:   "GARAN",
	}

	if cfg.ISINCode != "TR0005000C93" {
		t.Errorf("EntityResolutionConfig: ISINCode mismatch")
	}
}
