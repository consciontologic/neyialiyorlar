package model

import (
	"testing"
	"time"
)

func TestFilterByFetchedAt(t *testing.T) {
	ts1 := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 6, 23, 14, 0, 0, 0, time.UTC)
	ts3 := time.Date(2026, 6, 23, 18, 0, 0, 0, time.UTC)

	rawSet := &RawSet{
		Rows: []*RawPayload{
			{Source: "bist", FetchedAt: ts1, Confidence: ConfFresh},
			{Source: "kap", FetchedAt: ts2, Confidence: ConfFresh},
			{Source: "evds", FetchedAt: ts3, Confidence: ConfFresh},
		},
		WorstConfidence: ConfFresh,
	}

	// Filter at ts2: should include ts1 and ts2, exclude ts3.
	filtered := rawSet.FilterByFetchedAt(ts2)
	if filtered == nil {
		t.Errorf("FilterByFetchedAt: returned nil")
		return
	}

	if len(filtered.Rows) != 2 {
		t.Errorf("FilterByFetchedAt: got %d rows, want 2", len(filtered.Rows))
	}

	// Check that ts3 is excluded
	for _, row := range filtered.Rows {
		if row.FetchedAt.After(ts2) {
			t.Errorf("FilterByFetchedAt: row with FetchedAt > filter time was included")
		}
	}
}

func TestHasRequiredSources(t *testing.T) {
	rawSet := &RawSet{
		Rows: []*RawPayload{
			{Source: "bist"},
			{Source: "kap"},
			{Source: "evds"},
		},
	}

	// All required sources present
	if !rawSet.HasRequiredSources([]string{"bist", "kap", "evds"}) {
		t.Errorf("HasRequiredSources: expected true for available sources")
	}

	// Subset of required sources present
	if !rawSet.HasRequiredSources([]string{"bist", "kap"}) {
		t.Errorf("HasRequiredSources: expected true for subset")
	}

	// Missing source
	if rawSet.HasRequiredSources([]string{"bist", "kap", "evds", "mkkvap"}) {
		t.Errorf("HasRequiredSources: expected false for missing source")
	}

	// Empty raw set
	emptySet := &RawSet{Rows: []*RawPayload{}}
	if emptySet.HasRequiredSources([]string{"bist"}) {
		t.Errorf("HasRequiredSources: expected false for empty raw set")
	}
}
