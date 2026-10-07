package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/neyialiyorlar/services/api/internal/model"
)

// fakeTicker is a stub synthetic engine for the admin handler tests.
type fakeTicker struct {
	emitted int
	err     error
	calls   int
}

func (f *fakeTicker) Tick(_ context.Context, _ time.Time) (int, error) {
	f.calls++
	return f.emitted, f.err
}

// fakeCounter is a stub row-count source.
type fakeCounter struct {
	counts map[string]int64
}

func (f *fakeCounter) Counts(_ context.Context) (map[string]int64, error) {
	return f.counts, nil
}

// TestAdminTick_Emits verifies a successful tick returns the emitted count.
func TestAdminTick_Emits(t *testing.T) {
	ticker := &fakeTicker{emitted: 20}
	server := NewServer(nil, nil, WithTicker(ticker))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tick", nil)
	server.AdminTick(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp model.AdminTickResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Emitted != 20 {
		t.Errorf("expected emitted=20, got %d", resp.Emitted)
	}
	if ticker.calls != 1 {
		t.Errorf("expected ticker called once, got %d", ticker.calls)
	}
}

// TestAdminTick_DisabledReturns501 verifies the endpoint reports honestly when
// the synthetic engine is not wired.
func TestAdminTick_DisabledReturns501(t *testing.T) {
	server := NewServer(nil, nil) // no ticker

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tick", nil)
	server.AdminTick(w, r)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}
}

// TestAdminStatus_ReportsState verifies status reflects synthetic state, catalog
// size and row counts.
func TestAdminStatus_ReportsState(t *testing.T) {
	catalog := []model.MetricInfo{
		{Key: "nimvi", Name: "Nimvi", Flag: model.ConfidenceApprox, Tier: model.CadenceTierDaily},
		{Key: "basis_spread", Name: "Basis Spread", Flag: model.ConfidenceApprox, Tier: model.CadenceTierDaily},
	}
	counter := &fakeCounter{counts: map[string]int64{"metric_value": 540, "entity_ref": 5}}
	server := NewServer(nil, nil,
		WithCatalog(catalog),
		WithCounter(counter),
		WithTicker(&fakeTicker{}),
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/status", nil)
	server.AdminStatus(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp model.AdminStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.SyntheticEnabled {
		t.Error("expected synthetic_enabled=true when ticker wired")
	}
	if resp.MetricsCatalog != 2 {
		t.Errorf("expected metrics_catalog=2, got %d", resp.MetricsCatalog)
	}
	if resp.Counts["metric_value"] != 540 {
		t.Errorf("expected metric_value count 540, got %d", resp.Counts["metric_value"])
	}
	if resp.Service != "api" {
		t.Errorf("expected service=api, got %s", resp.Service)
	}
}

// TestAdminStatus_NoTickerDisabled verifies synthetic_enabled is false without a
// ticker (and counts default to empty rather than erroring).
func TestAdminStatus_NoTickerDisabled(t *testing.T) {
	server := NewServer(nil, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/status", nil)
	server.AdminStatus(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp model.AdminStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.SyntheticEnabled {
		t.Error("expected synthetic_enabled=false without ticker")
	}
}
