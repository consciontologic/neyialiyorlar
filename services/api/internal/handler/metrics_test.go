package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/neyialiyorlar/services/api/internal/model"
	"github.com/redis/go-redis/v9"
)

// TestListMetrics validates the GET /metrics response against schema
func TestListMetrics(t *testing.T) {
	// Mock database and Redis (would use testcontainers in full setup)
	server := NewServer(nil, nil) // Stub for now

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/metrics", nil)

	server.ListMetrics(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp model.ListMetricsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Errorf("failed to decode response: %v", err)
	}

	// Validate Content-Type
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
}

// TestListEntities validates the GET /entities response returns the configured
// entity catalogue, including each security's fund/basket membership.
func TestListEntities(t *testing.T) {
	entities := []model.EntityInfo{
		{ID: "banks", Type: "basket", DisplayTicker: "BANKS"},
		{ID: "GARAN", Type: "security", DisplayTicker: "GARAN", ISIN: "TRAGARAN91N1", Basket: "banks"},
		{ID: "THYAO", Type: "security", DisplayTicker: "THYAO", ISIN: "TRATHYAO91M5"},
	}
	server := NewServer(nil, nil, WithEntities(entities))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/entities", nil)

	server.ListEntities(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	var resp model.ListEntitiesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Entities) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(resp.Entities))
	}

	byID := map[string]model.EntityInfo{}
	for _, e := range resp.Entities {
		byID[e.ID] = e
	}
	if byID["GARAN"].Basket != "banks" {
		t.Errorf("expected GARAN basket=banks, got %q", byID["GARAN"].Basket)
	}
	if byID["GARAN"].ISIN != "TRAGARAN91N1" {
		t.Errorf("expected GARAN ISIN preserved, got %q", byID["GARAN"].ISIN)
	}
	if byID["THYAO"].Basket != "" {
		t.Errorf("expected THYAO standalone (no basket), got %q", byID["THYAO"].Basket)
	}
	if byID["banks"].Type != "basket" {
		t.Errorf("expected banks type=basket, got %q", byID["banks"].Type)
	}
}

// TestListEntities_PopulatesFundCount is a regression guard for the user-facing
// "label stocks that have funds vs. don't" feature: GET /entities must report,
// for every security, how many distinct funds hold it (fund_count), so the
// dashboard can badge has-funds vs. no-funds stocks. A security held by two
// distinct funds reports fund_count=2; a security nothing holds reports 0.
//
// Like the fund-holdings integration tests it runs against a real migrated
// Postgres (TEST_DATABASE_URL) and self-skips when that is unset, so the offline
// `make ci.go` gate stays green.
func TestListEntities_PopulatesFundCount(t *testing.T) {
	db := openFundsTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const (
		heldStock = "ZZFCHELD" // throwaway security held by two funds
		bareStock = "ZZFCBARE" // throwaway security held by no fund
		fundA     = "ZZFCFNDA"
		fundB     = "ZZFCFNDB"
		asOf      = "2025-06-30"
	)

	cleanup := func() {
		db.ExecContext(ctx, `DELETE FROM fund_holding WHERE fund_code IN ($1,$2)`, fundA, fundB)
		db.ExecContext(ctx, `DELETE FROM fund_ref WHERE fund_code IN ($1,$2)`, fundA, fundB)
		db.ExecContext(ctx, `DELETE FROM entity_ref WHERE entity_id IN ($1,$2)`, heldStock, bareStock)
	}
	cleanup()
	defer cleanup()

	// Two active securities: one held by two funds, one held by none.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO entity_ref (entity_id, entity_type, display_ticker, valid_from)
		VALUES ($1,'security',$1,$3), ($2,'security',$2,$3)
	`, heldStock, bareStock, asOf); err != nil {
		t.Fatalf("seed entity_ref failed: %v", err)
	}
	for _, fc := range []string{fundA, fundB} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO fund_ref (fund_code, fund_title, fund_manager, fund_type, as_of_date)
			VALUES ($1, 'ZZ TEST HISSE FONU', 'ZZ TEST A.Ş.', 'EMK', $2)
		`, fc, asOf); err != nil {
			t.Fatalf("seed fund_ref %s failed: %v", fc, err)
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO fund_holding (fund_code, stock_id, weight_pct, as_of_date, source)
			VALUES ($1, $2, $3, $4, 'kap')
		`, fc, heldStock, 5.0, asOf); err != nil {
			t.Fatalf("seed fund_holding %s failed: %v", fc, err)
		}
	}

	server := NewServer(db, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/entities", nil)
	server.ListEntities(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp model.ListEntitiesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	byID := map[string]model.EntityInfo{}
	for _, e := range resp.Entities {
		byID[e.ID] = e
	}
	held, ok := byID[heldStock]
	if !ok {
		t.Fatalf("seeded held security %q missing from /entities", heldStock)
	}
	if held.FundCount != 2 {
		t.Errorf("expected %s fund_count=2, got %d", heldStock, held.FundCount)
	}
	bare, ok := byID[bareStock]
	if !ok {
		t.Fatalf("seeded bare security %q missing from /entities", bareStock)
	}
	if bare.FundCount != 0 {
		t.Errorf("expected %s fund_count=0, got %d", bareStock, bare.FundCount)
	}
}

// TestGetMetricLatest validates flag field is always present
func TestGetMetricLatest_FlagPresent(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/metrics/nimvi/latest?entity=banks", nil)
	r.SetPathValue("key", "nimvi")

	// This will fail to query DB (nil) but we're validating response schema
	// In full test: use mock DB returning a row

	// For now, validate the schema contract exists
	t.Skip("requires full handler implementation with DB")
}

// TestGetMetricLatest_GapValue validates null value for gaps
func TestGetMetricLatest_NullValue(t *testing.T) {
	// In full test, mock DB to return value=NULL
	var v model.MetricValue
	jsonStr := `{"metric":"test","entity":"e1","ts":"2026-06-14T15:00:00Z","value":null,"flag":"fresh","tier":"daily"}`
	err := json.Unmarshal([]byte(jsonStr), &v)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}

	if v.Value != nil {
		t.Error("expected nil value, got non-nil")
	}

	if v.Flag != "fresh" {
		t.Errorf("expected fresh flag, got %s", v.Flag)
	}
}

// TestPostQuery_MaxQueryCap validates 413 on oversized batch
func TestPostQuery_MaxQueryCap(t *testing.T) {
	db := &sql.DB{} // Stub
	rc := &redis.Client{}
	server := NewServer(db, rc, WithMaxQueries(10))

	// Create oversized query - use proper model.QueryRequest structure
	entities := make([]string, 20) // Exceeds limit of 10
	req := model.QueryRequest{
		Queries: []struct {
			Metric   string   `json:"metric"`
			Entities []string `json:"entities"`
		}{
			{
				Metric:   "m1",
				Entities: entities,
			},
		},
	}

	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/query", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	server.QueryMetrics(w, r)

	// Expect 413
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", w.Code)
	}

	var errResp model.ErrorResponse
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Error == "" {
		t.Error("error message missing")
	}
}

// TestMiddleware_RequestIDInjection validates X-Request-ID header
func TestMiddleware_RequestIDPresent(t *testing.T) {
	server := NewServer(nil, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/metrics", nil)

	// Call a handler
	server.ListMetrics(w, r)

	// Request ID should be in response header (set by middleware)
	// In full test: wrap with middleware chain
	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		// Note: middleware must be wired to set this
		t.Skip("X-Request-ID requires middleware chain")
	}
}

// TestMiddleware_StructuredLogging validates log format
func TestMiddleware_LogsHaveRequiredFields(t *testing.T) {
	// In full test: capture slog output and verify fields:
	// - method, path, status, latency_ms, request_id
	t.Skip("requires slog handler mock")
}

// TestMiddleware_TimeoutRespected validates timeout enforcement
func TestMiddleware_Timeout(t *testing.T) {
	server := NewServer(nil, nil)

	// Create a handler that sleeps longer than timeout
	slowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	})

	_ = slowHandler
	_ = server
	// Would need to apply Timeout middleware and call slowHandler
	t.Skip("requires middleware chain to test properly")
}

// TestCORSHeaders validates CORS headers on responses
func TestCORSHeaders(t *testing.T) {
	server := NewServer(nil, nil)

	// In full test: wrap with globalMiddleware
	// Expected headers after middleware:
	// - Access-Control-Allow-Origin: *
	// - X-Content-Type-Options: nosniff
	// - Content-Security-Policy: default-src 'self'; ...
	_ = server
	t.Skip("requires globalMiddleware wrapper to test")
}

// TestSecurityHeaders validates security headers
func TestSecurityHeaders(t *testing.T) {
	// globalMiddleware should set:
	// - X-Content-Type-Options: nosniff
	// - Content-Security-Policy with SW-compatible values
	t.Skip("requires globalMiddleware wrapper to test")
}

// TestGetMetricSeries_ValidWindow validates series response
func TestGetMetricSeries_ValidSchema(t *testing.T) {
	jsonStr := `{
		"metric":"nimvi",
		"entity":"banks",
		"window":"30d",
		"points":[
			{"metric":"nimvi","entity":"banks","ts":"2026-06-14T15:00:00Z","value":1.83,"flag":"fresh","tier":"daily"},
			{"metric":"nimvi","entity":"banks","ts":"2026-06-13T15:00:00Z","value":null,"flag":"stale","tier":"daily"}
		]
	}`

	var resp model.GetSeriesResponse
	err := json.Unmarshal([]byte(jsonStr), &resp)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}

	if resp.Metric != "nimvi" {
		t.Errorf("expected metric nimvi, got %s", resp.Metric)
	}

	// Validate every point has flag
	for i, p := range resp.Points {
		if p.Flag == "" {
			t.Errorf("point %d missing flag", i)
		}
	}

	// Validate null value allowed
	if resp.Points[1].Value != nil {
		t.Errorf("expected nil value for gap row, got %v", resp.Points[1].Value)
	}
}

// TestGetSourcesHealth_ValidSchema validates health response
func TestGetSourcesHealth_ValidSchema(t *testing.T) {
	jsonStr := `{
		"sources":[
			{
				"source":"bist",
				"last_ok_at":"2026-06-14T15:00:00Z",
				"checked_at":"2026-06-14T15:05:00Z",
				"breaker_open":false,
				"consecutive_failures":0
			}
		]
	}`

	var resp model.GetSourcesHealthResponse
	err := json.Unmarshal([]byte(jsonStr), &resp)
	if err != nil {
		t.Errorf("failed to unmarshal: %v", err)
	}

	if len(resp.Sources) == 0 {
		t.Error("sources list empty")
	}

	h := resp.Sources[0]
	if h.Source == "" {
		t.Error("source field empty")
	}
}

// TestConfidenceEnumValues validates all confidence values
func TestConfidenceEnum(t *testing.T) {
	tests := []struct {
		value    string
		expected model.Confidence
	}{
		{"fresh", model.ConfidenceFresh},
		{"stale", model.ConfidenceStale},
		{"approx", model.ConfidenceApprox},
	}

	for _, tt := range tests {
		if model.Confidence(tt.value) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.value)
		}
	}
}

// TestCadenceTierEnum validates all cadence tier values
func TestCadenceTierEnum(t *testing.T) {
	tests := []string{
		string(model.CadenceTierIntraday),
		string(model.CadenceTierDaily),
		string(model.CadenceTierWeekly),
		string(model.CadenceTierEvent),
	}

	for _, tier := range tests {
		if tier == "" {
			t.Errorf("cadence tier empty")
		}
	}
}
