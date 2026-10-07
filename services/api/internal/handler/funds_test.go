package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// openFundsTestDB connects to a real Postgres for the fund-holdings integration
// test. It reads TEST_DATABASE_URL (e.g.
// "host=localhost user=neyi password=dev_password dbname=neyialiyorlar sslmode=disable").
// When the variable is unset or the database is unreachable the test is skipped,
// so the offline `make ci.go` gate stays green; it only runs against a migrated
// database (CI service / local `make up` + `make db.migrate`).
func openFundsTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping fund-holdings DB integration test")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("could not open test database: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("could not ping test database: %v", err)
	}
	return db
}

// TestGetEntityFunds_ReturnsFundsHoldingStock is a regression guard for the bug
// where the API listed no funds for any stock: the fund_ref / fund_holding tables
// lived only in deploy/postgres/03-fund-holdings.sql, which golang-migrate never
// applied, so this query errored on missing relations. With migration
// 0002_fund_holdings in place the endpoint returns the funds that hold a stock.
//
// It seeds a throwaway fund + holding, calls GetEntityFunds, asserts the fund is
// returned, and cleans up after itself.
func TestGetEntityFunds_ReturnsFundsHoldingStock(t *testing.T) {
	db := openFundsTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const (
		fundCode = "ZZTEST"            // throwaway code, unlikely to collide with real data
		stockID  = "ZZTESTSTK"         // throwaway BIST-style ticker
		asOf     = "2025-06-30"
	)

	// Clean up any leftovers from a previous interrupted run, then again at the end.
	cleanup := func() {
		db.ExecContext(ctx, `DELETE FROM fund_holding WHERE fund_code = $1`, fundCode)
		db.ExecContext(ctx, `DELETE FROM fund_ref WHERE fund_code = $1`, fundCode)
	}
	cleanup()
	defer cleanup()

	// Seed a fund and a holding of stockID. A failure here means the migration is
	// missing (the original bug) — surface it as a test failure, not a skip.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO fund_ref (fund_code, fund_title, fund_manager, fund_type, as_of_date)
		VALUES ($1, $2, $3, 'EMK', $4)
	`, fundCode, "ZZ TEST HISSE FONU", "ZZ TEST A.Ş.", asOf); err != nil {
		t.Fatalf("seed fund_ref failed (is migration 0002_fund_holdings applied?): %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO fund_holding (fund_code, stock_id, weight_pct, as_of_date, source)
		VALUES ($1, $2, $3, $4, 'kap')
	`, fundCode, stockID, 7.5, asOf); err != nil {
		t.Fatalf("seed fund_holding failed (is migration 0002_fund_holdings applied?): %v", err)
	}

	// Exercise the real handler against the real query.
	server := NewServer(db, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/entities/"+stockID+"/funds", nil)
	r.SetPathValue("id", stockID)
	server.GetEntityFunds(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp struct {
		Entity string        `json:"entity"`
		Funds  []FundHolding `json:"funds"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Entity != stockID {
		t.Errorf("expected entity %q, got %q", stockID, resp.Entity)
	}

	var found *FundHolding
	for i := range resp.Funds {
		if resp.Funds[i].FundCode == fundCode {
			found = &resp.Funds[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected fund %q in funds list, got %+v", fundCode, resp.Funds)
	}
	if found.WeightPct == nil || *found.WeightPct != 7.5 {
		t.Errorf("expected weight_pct 7.5, got %v", found.WeightPct)
	}
	if found.Source != "kap" {
		t.Errorf("expected source 'kap', got %q", found.Source)
	}
	if found.AsOfDate != asOf {
		t.Errorf("expected as_of_date %q, got %q", asOf, found.AsOfDate)
	}
}

// TestGetEntityFunds_ReturnsAllFundsHoldingStock is a regression guard for the
// user-reported issue where a stock detail listed "only one or a few" of the
// funds that actually hold it. The endpoint must return EVERY fund holding the
// stock (the query has no LIMIT), ordered by weight_pct DESC NULLS LAST. It also
// exercises the scraper's template-agnostic holdings, which carry a NULL weight
// when the FPD PDF layout hides the percentage — those links must still appear,
// sorted last, rather than being dropped.
func TestGetEntityFunds_ReturnsAllFundsHoldingStock(t *testing.T) {
	db := openFundsTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// A throwaway stock no real fund holds, so the response contains only our
	// seeds and the relative ordering assertion is deterministic.
	const (
		stockID = "ZZMULTISTK"
		asOf    = "2025-06-30"
	)
	type seed struct {
		code   string
		weight sql.NullFloat64
	}
	seeds := []seed{
		{"ZZMF1", sql.NullFloat64{Float64: 9.0, Valid: true}},
		{"ZZMF2", sql.NullFloat64{Float64: 3.0, Valid: true}},
		{"ZZMF3", sql.NullFloat64{}}, // NULL weight — template-agnostic link only
	}

	cleanup := func() {
		for _, s := range seeds {
			db.ExecContext(ctx, `DELETE FROM fund_holding WHERE fund_code = $1`, s.code)
			db.ExecContext(ctx, `DELETE FROM fund_ref WHERE fund_code = $1`, s.code)
		}
	}
	cleanup()
	defer cleanup()

	for _, s := range seeds {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO fund_ref (fund_code, fund_title, fund_manager, fund_type, as_of_date)
			VALUES ($1, $2, $3, 'EMK', $4)
		`, s.code, s.code+" HISSE FONU", s.code+" A.Ş.", asOf); err != nil {
			t.Fatalf("seed fund_ref %s failed (is migration 0002_fund_holdings applied?): %v", s.code, err)
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO fund_holding (fund_code, stock_id, weight_pct, as_of_date, source)
			VALUES ($1, $2, $3, $4, 'kap')
		`, s.code, stockID, s.weight, asOf); err != nil {
			t.Fatalf("seed fund_holding %s failed: %v", s.code, err)
		}
	}

	server := NewServer(db, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/entities/"+stockID+"/funds", nil)
	r.SetPathValue("id", stockID)
	server.GetEntityFunds(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp struct {
		Entity string        `json:"entity"`
		Funds  []FundHolding `json:"funds"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	got := map[string]*FundHolding{}
	var order []string
	for i := range resp.Funds {
		switch resp.Funds[i].FundCode {
		case "ZZMF1", "ZZMF2", "ZZMF3":
			got[resp.Funds[i].FundCode] = &resp.Funds[i]
			order = append(order, resp.Funds[i].FundCode)
		}
	}

	// Every seeded fund must be present — the core "show all funds" guarantee.
	for _, s := range seeds {
		if got[s.code] == nil {
			t.Fatalf("expected fund %q in funds list, got %+v", s.code, resp.Funds)
		}
	}

	// Ordering: weight DESC, NULLs last → ZZMF1 (9.0), ZZMF2 (3.0), ZZMF3 (NULL).
	want := []string{"ZZMF1", "ZZMF2", "ZZMF3"}
	if len(order) != len(want) {
		t.Fatalf("expected exactly %d funds for %s, got order %v", len(want), stockID, order)
	}
	for i, code := range want {
		if order[i] != code {
			t.Errorf("order[%d] = %q, want %q (full order %v)", i, order[i], code, order)
		}
	}

	// The weight-unknown holding must surface with a nil weight, not be dropped.
	if got["ZZMF3"].WeightPct != nil {
		t.Errorf("expected nil weight_pct for ZZMF3, got %v", *got["ZZMF3"].WeightPct)
	}
	if got["ZZMF1"].WeightPct == nil || *got["ZZMF1"].WeightPct != 9.0 {
		t.Errorf("expected weight_pct 9.0 for ZZMF1, got %v", got["ZZMF1"].WeightPct)
	}
}
