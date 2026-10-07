package recompute

import (
	"context"
	"testing"
	"time"

	"github.com/neyialiyorlar/services/analytic/internal/model"
)

func TestNewRecomputer(t *testing.T) {
	r := NewRecomputer()
	if r == nil {
		t.Errorf("NewRecomputer() returned nil")
	}
	if r.RowCount() != 0 {
		t.Errorf("NewRecomputer() initial row count = %d, want 0", r.RowCount())
	}
}

func TestUpsertDerivedNewRow(t *testing.T) {
	r := NewRecomputer()
	ctx := context.Background()

	now := time.Now()
	v := 42.0
	d := &model.Derived{
		MetricKey:   "m01_velocity_accumulation",
		Entity:      "ISIN_TR123",
		Timestamp:   now,
		Value:       &v,
		Flag:        model.ConfFresh,
		ComputedAt:  now,
		InputsHash:  "abc123",
	}

	result, err := r.UpsertDerived(ctx, d)
	if err != nil {
		t.Errorf("UpsertDerived() returned error: %v", err)
	}
	if !result.NewRow {
		t.Errorf("UpsertDerived() NewRow = false, want true (first insert)")
	}
	if r.RowCount() != 1 {
		t.Errorf("UpsertDerived() row count = %d, want 1", r.RowCount())
	}
}

func TestUpsertDerivedIdempotent(t *testing.T) {
	r := NewRecomputer()
	ctx := context.Background()

	now := time.Now()
	v := 42.0
	d := &model.Derived{
		MetricKey:   "m01_velocity_accumulation",
		Entity:      "ISIN_TR123",
		Timestamp:   now,
		Value:       &v,
		Flag:        model.ConfFresh,
		ComputedAt:  now,
		InputsHash:  "abc123",
	}

	// First insert.
	result1, err1 := r.UpsertDerived(ctx, d)
	if err1 != nil || !result1.NewRow {
		t.Errorf("First UpsertDerived() failed: NewRow=%v, err=%v", result1.NewRow, err1)
	}

	// Second insert with same key (should be a no-op).
	result2, _ := r.UpsertDerived(ctx, d)
	if result2.NewRow {
		t.Errorf("Second UpsertDerived() should be no-op: NewRow=%v", result2.NewRow)
	}

	// Row count should still be 1 (idempotent).
	if r.RowCount() != 1 {
		t.Errorf("After second insert, row count = %d, want 1 (idempotent)", r.RowCount())
	}
}

func TestUpsertDerivedNilInput(t *testing.T) {
	r := NewRecomputer()
	ctx := context.Background()

	result, _ := r.UpsertDerived(ctx, nil)
	if result.Error == nil {
		t.Errorf("UpsertDerived(nil) should return error")
	}
}

func TestUpsertDerivedMissingMetricKey(t *testing.T) {
	r := NewRecomputer()
	ctx := context.Background()

	now := time.Now()
	v := 42.0
	d := &model.Derived{
		MetricKey:   "", // missing
		Entity:      "ISIN_TR123",
		Timestamp:   now,
		Value:       &v,
		Flag:        model.ConfFresh,
		ComputedAt:  now,
		InputsHash:  "abc123",
	}

	result, _ := r.UpsertDerived(ctx, d)
	if result.Error == nil {
		t.Errorf("UpsertDerived() with empty metric_key should return error")
	}
}

func TestGetDerived(t *testing.T) {
	r := NewRecomputer()
	ctx := context.Background()

	now := time.Now()
	v := 42.0
	d := &model.Derived{
		MetricKey:   "m01_velocity_accumulation",
		Entity:      "ISIN_TR123",
		Timestamp:   now,
		Value:       &v,
		Flag:        model.ConfFresh,
		ComputedAt:  now,
		InputsHash:  "abc123",
	}

	r.UpsertDerived(ctx, d)

	// Retrieve the row.
	retrieved := r.GetDerived("m01_velocity_accumulation", "ISIN_TR123", now, "abc123")
	if retrieved == nil {
		t.Errorf("GetDerived() returned nil")
	}
	if *retrieved.Value != 42.0 {
		t.Errorf("GetDerived() value = %f, want 42.0", *retrieved.Value)
	}
}

func TestGetDerivedNotFound(t *testing.T) {
	r := NewRecomputer()

	retrieved := r.GetDerived("nonexistent", "ISIN_TR123", time.Now(), "xyz")
	if retrieved != nil {
		t.Errorf("GetDerived() for nonexistent row returned %v, want nil", retrieved)
	}
}

func TestIsGapRow(t *testing.T) {
	// Gap row (value is nil).
	gap := &model.Derived{
		Value: nil,
	}
	if !IsGapRow(gap) {
		t.Errorf("IsGapRow(nil value) = false, want true")
	}

	// Non-gap row (value is not nil).
	v := 42.0
	notGap := &model.Derived{
		Value: &v,
	}
	if IsGapRow(notGap) {
		t.Errorf("IsGapRow(non-nil value) = true, want false")
	}
}

func TestCreateGapRow(t *testing.T) {
	now := time.Now()
	gap := CreateGapRow("m01_velocity_accumulation", "ISIN_TR123", now, "no data", model.ConfStale)

	if gap.Value != nil {
		t.Errorf("CreateGapRow() value = %v, want nil", gap.Value)
	}
	if gap.Flag != model.ConfStale {
		t.Errorf("CreateGapRow() flag = %s, want %s", gap.Flag, model.ConfStale)
	}
	if !IsGapRow(gap) {
		t.Errorf("CreateGapRow() should be a gap row")
	}
}

func TestIsIdempotentSame(t *testing.T) {
	now := time.Now()
	v := 42.0
	d1 := &model.Derived{
		MetricKey:  "m01_velocity_accumulation",
		Entity:     "ISIN_TR123",
		Timestamp:  now,
		Value:      &v,
		InputsHash: "abc123",
	}
	d2 := &model.Derived{
		MetricKey:  "m01_velocity_accumulation",
		Entity:     "ISIN_TR123",
		Timestamp:  now,
		Value:      &v, // can be different value
		InputsHash: "abc123",
	}

	if !IsIdempotent(d1, d2) {
		t.Errorf("IsIdempotent() should return true for same (metric, entity, ts, inputs_hash)")
	}
}

func TestIsIdempotentDifferentMetric(t *testing.T) {
	now := time.Now()
	v := 42.0
	d1 := &model.Derived{
		MetricKey:  "m01_velocity_accumulation",
		Entity:     "ISIN_TR123",
		Timestamp:  now,
		Value:      &v,
		InputsHash: "abc123",
	}
	d2 := &model.Derived{
		MetricKey:  "m02_holdings_overlap", // different
		Entity:     "ISIN_TR123",
		Timestamp:  now,
		Value:      &v,
		InputsHash: "abc123",
	}

	if IsIdempotent(d1, d2) {
		t.Errorf("IsIdempotent() should return false for different metrics")
	}
}

func TestIsIdempotentDifferentEntity(t *testing.T) {
	now := time.Now()
	v := 42.0
	d1 := &model.Derived{
		MetricKey:  "m01_velocity_accumulation",
		Entity:     "ISIN_TR123",
		Timestamp:  now,
		Value:      &v,
		InputsHash: "abc123",
	}
	d2 := &model.Derived{
		MetricKey:  "m01_velocity_accumulation",
		Entity:     "ISIN_TR456", // different
		Timestamp:  now,
		Value:      &v,
		InputsHash: "abc123",
	}

	if IsIdempotent(d1, d2) {
		t.Errorf("IsIdempotent() should return false for different entities")
	}
}

func TestIsIdempotentDifferentInputsHash(t *testing.T) {
	now := time.Now()
	v := 42.0
	d1 := &model.Derived{
		MetricKey:  "m01_velocity_accumulation",
		Entity:     "ISIN_TR123",
		Timestamp:  now,
		Value:      &v,
		InputsHash: "abc123",
	}
	d2 := &model.Derived{
		MetricKey:  "m01_velocity_accumulation",
		Entity:     "ISIN_TR123",
		Timestamp:  now,
		Value:      &v,
		InputsHash: "xyz789", // different inputs
	}

	if IsIdempotent(d1, d2) {
		t.Errorf("IsIdempotent() should return false for different inputs_hash")
	}
}
