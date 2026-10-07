package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// mkPoint builds a PresencePoint for a date string "2006-01-02", computing the
// presence percentage from funds/total exactly as the handler does.
func mkPoint(t *testing.T, date string, funds, total int) PresencePoint {
	t.Helper()
	at, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatalf("bad date %q: %v", date, err)
	}
	return newPresencePoint(at, funds, total)
}

func TestPresenceWindowDays(t *testing.T) {
	cases := map[string]int{
		"1m": 30, "30d": 30,
		"3m": 90, "6m": 180,
		"1y": 365, "12m": 365, "365d": 365,
		"2y": 730,
		"all": 0, "max": 0,
		"45d":     45, // explicit day count
		"":        365,
		"garbage": 365, // unknown falls back to 1y
		" 1Y ":    365, // trimmed + case-insensitive
	}
	for in, want := range cases {
		if got := presenceWindowDays(in); got != want {
			t.Errorf("presenceWindowDays(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestPresenceCutoff(t *testing.T) {
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)

	if got := presenceCutoff(30, now); !got.Equal(now.AddDate(0, 0, -30)) {
		t.Errorf("presenceCutoff(30) = %v, want %v", got, now.AddDate(0, 0, -30))
	}
	// all-history sentinel is far in the past
	if got := presenceCutoff(0, now); !got.Before(now.AddDate(-50, 0, 0)) {
		t.Errorf("presenceCutoff(0) = %v, want far past", got)
	}
}

func TestNewPresencePoint(t *testing.T) {
	p := mkPoint(t, "2026-06-26", 1, 3)
	if math.Abs(p.Pct-33.33) > 0.001 {
		t.Errorf("Pct = %.4f, want 33.33", p.Pct)
	}
	if p.Date != "2026-06-26" || p.Funds != 1 || p.Total != 3 {
		t.Errorf("unexpected point %+v", p)
	}
	// total == 0 must not divide by zero
	z := mkPoint(t, "2026-06-26", 0, 0)
	if z.Pct != 0 {
		t.Errorf("Pct with zero total = %.4f, want 0", z.Pct)
	}
}

func TestPctChangeOverDays(t *testing.T) {
	// Sparse snapshots: an old 20% reading and a recent 30% reading.
	pts := []PresencePoint{
		mkPoint(t, "2025-06-26", 20, 100), // 1 year before latest
		mkPoint(t, "2026-05-27", 25, 100), // ~30 days before latest
		mkPoint(t, "2026-06-26", 30, 100), // latest
	}

	// Monthly: latest 30% vs ~30d-ago 25% -> +20%.
	if got := pctChangeOverDays(pts, 30); got == nil || math.Abs(*got-20) > 0.01 {
		t.Errorf("monthly change = %v, want +20", got)
	}
	// Yearly: latest 30% vs 1y-ago 20% -> +50%.
	if got := pctChangeOverDays(pts, 365); got == nil || math.Abs(*got-50) > 0.01 {
		t.Errorf("yearly change = %v, want +50", got)
	}
	// Weekly: nearest snapshot at/older than 7d cutoff is the 25% point -> +20%.
	if got := pctChangeOverDays(pts, 7); got == nil || math.Abs(*got-20) > 0.01 {
		t.Errorf("weekly change = %v, want +20 (nearest older snapshot)", got)
	}

	// No baseline old enough -> nil.
	recent := []PresencePoint{
		mkPoint(t, "2026-06-25", 28, 100),
		mkPoint(t, "2026-06-26", 30, 100),
	}
	if got := pctChangeOverDays(recent, 365); got != nil {
		t.Errorf("yearly change with no old baseline = %v, want nil", got)
	}

	// Single point -> nil (no comparison possible).
	if got := pctChangeOverDays(pts[:1], 1); got != nil {
		t.Errorf("change with one point = %v, want nil", got)
	}

	// Zero baseline -> nil (relative change against zero is undefined).
	zeroBase := []PresencePoint{
		mkPoint(t, "2026-05-01", 0, 100), // 0% presence baseline
		mkPoint(t, "2026-06-26", 10, 100),
	}
	if got := pctChangeOverDays(zeroBase, 30); got != nil {
		t.Errorf("change against zero baseline = %v, want nil", got)
	}
}

func TestComputePresenceChanges(t *testing.T) {
	pts := []PresencePoint{
		mkPoint(t, "2025-06-26", 10, 100), // 1y ago: 10%
		mkPoint(t, "2026-05-27", 18, 100), // ~30d ago: 18%
		mkPoint(t, "2026-06-19", 19, 100), // ~7d ago: 19%
		mkPoint(t, "2026-06-25", 20, 100), // ~1d ago: 20%
		mkPoint(t, "2026-06-26", 22, 100), // latest: 22%
	}
	ch := computePresenceChanges(pts)

	if ch.Daily == nil || math.Abs(*ch.Daily-10) > 0.01 { // 22 vs 20 -> +10%
		t.Errorf("daily = %v, want +10", ch.Daily)
	}
	if ch.Weekly == nil || math.Abs(*ch.Weekly-(15.79)) > 0.05 { // 22 vs 19
		t.Errorf("weekly = %v, want ~+15.79", ch.Weekly)
	}
	if ch.Monthly == nil || math.Abs(*ch.Monthly-(22.22)) > 0.05 { // 22 vs 18
		t.Errorf("monthly = %v, want ~+22.22", ch.Monthly)
	}
	if ch.Yearly == nil || math.Abs(*ch.Yearly-120) > 0.01 { // 22 vs 10 -> +120%
		t.Errorf("yearly = %v, want +120", ch.Yearly)
	}
}

func TestTrimPointsToDays(t *testing.T) {
	pts := []PresencePoint{
		mkPoint(t, "2025-06-26", 10, 100),
		mkPoint(t, "2026-05-27", 18, 100),
		mkPoint(t, "2026-06-26", 22, 100),
	}
	// 30-day window keeps only the last two points.
	got := trimPointsToDays(pts, 30)
	if len(got) != 2 || got[0].Date != "2026-05-27" {
		t.Errorf("trim 30d = %d points (first %s), want 2 starting 2026-05-27", len(got), got[0].Date)
	}
	// 0 keeps everything.
	if got := trimPointsToDays(pts, 0); len(got) != 3 {
		t.Errorf("trim 0 = %d points, want 3", len(got))
	}
	// empty input is safe.
	if got := trimPointsToDays(nil, 30); len(got) != 0 {
		t.Errorf("trim nil = %d, want 0", len(got))
	}
}

// ─── DB-gated integration test ────────────────────────────────────────────────

// openPresenceTestDB connects to a real Postgres for the presence integration
// test, mirroring openFundsTestDB. It skips when TEST_DATABASE_URL is unset or
// the database is unreachable so the offline `make ci.go` gate stays green.
func openPresenceTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping presence DB integration test")
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

// TestGetEntityPresence_ComputesSeries seeds funds + holdings across two dates
// and asserts the endpoint returns the presence series, latest value, and the
// monthly change, with the yearly change nil (no ~1y-old baseline seeded).
func TestGetEntityPresence_ComputesSeries(t *testing.T) {
	db := openPresenceTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	const (
		stockID = "ZZPRESSTK"
		other   = "ZZPRESOTH"
	)
	funds := []string{"ZZP1", "ZZP2", "ZZP3"}

	now := time.Now().UTC()
	dOld := now.AddDate(0, 0, -200).Format("2006-01-02")
	dNew := now.Format("2006-01-02")

	cleanup := func() {
		for _, f := range funds {
			db.ExecContext(ctx, `DELETE FROM fund_holding WHERE fund_code = $1`, f)
			db.ExecContext(ctx, `DELETE FROM fund_ref WHERE fund_code = $1`, f)
		}
	}
	cleanup()
	defer cleanup()

	for _, f := range funds {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO fund_ref (fund_code, fund_title, fund_manager, fund_type, as_of_date)
			VALUES ($1, $2, 'ZZ TEST A.Ş.', 'EMK', $3)`,
			f, "ZZ TEST FONU "+f, dNew); err != nil {
			t.Fatalf("seed fund_ref %s failed (is migration 0002 applied?): %v", f, err)
		}
	}

	// Holdings. dOld: ZZP1+ZZP2 hold the stock, ZZP3 reports something else ->
	// 2/3 = 66.67%. dNew: only ZZP1 holds the stock -> 1/3 = 33.33%.
	type h struct {
		fund, stock, date string
	}
	holdings := []h{
		{"ZZP1", stockID, dOld}, {"ZZP2", stockID, dOld}, {"ZZP3", other, dOld},
		{"ZZP1", stockID, dNew}, {"ZZP2", other, dNew}, {"ZZP3", other, dNew},
	}
	for _, hh := range holdings {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO fund_holding (fund_code, stock_id, weight_pct, as_of_date, source)
			VALUES ($1, $2, NULL, $3, 'tefas')`, hh.fund, hh.stock, hh.date); err != nil {
			t.Fatalf("seed fund_holding %+v failed: %v", hh, err)
		}
	}

	// Exercise the full HTTP path so routing + JSON are covered too.
	s := &Server{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/entities/{id}/presence", s.GetEntityPresence)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/entities/"+stockID+"/presence?win=all", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp PresenceResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Entity != stockID {
		t.Errorf("entity = %q, want %q", resp.Entity, stockID)
	}
	if len(resp.Points) != 2 {
		t.Fatalf("points = %d, want 2: %+v", len(resp.Points), resp.Points)
	}
	if math.Abs(resp.PresencePct-33.33) > 0.1 {
		t.Errorf("latest presence_pct = %.2f, want ~33.33", resp.PresencePct)
	}
	if resp.Funds != 1 || resp.Total != 3 {
		t.Errorf("latest funds/total = %d/%d, want 1/3", resp.Funds, resp.Total)
	}
	if resp.AsOf != dNew {
		t.Errorf("as_of = %q, want %q", resp.AsOf, dNew)
	}
	// Monthly: 33.33% vs the 200d-old 66.67% baseline -> ~-50%.
	if resp.Changes.Monthly == nil || math.Abs(*resp.Changes.Monthly-(-50)) > 0.5 {
		t.Errorf("monthly change = %v, want ~-50", resp.Changes.Monthly)
	}
	// Yearly: no baseline ~1y old was seeded -> nil.
	if resp.Changes.Yearly != nil {
		t.Errorf("yearly change = %v, want nil", resp.Changes.Yearly)
	}
}
