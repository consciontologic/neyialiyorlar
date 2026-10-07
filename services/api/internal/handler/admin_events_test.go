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

// TestGetAdminEvents_DerivesFeedFromRealWrites verifies the live activity feed
// is built from ACTUAL persisted writes — harvester price_close rows, analytic
// derived-metric rows, and scraper_progress runs — and never from synthetic
// data. It seeds one row per source, calls GetAdminEvents, and asserts an event
// surfaces for each source, newest-first.
//
// Like the other DB integration tests it reuses openFundsTestDB, so it skips
// cleanly when TEST_DATABASE_URL is unset (keeping the offline `make ci.go`
// gate green) and only runs against a migrated database.
func TestGetAdminEvents_DerivesFeedFromRealWrites(t *testing.T) {
	db := openFundsTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const (
		entity = "ZZEVTSTK" // throwaway ticker, unlikely to collide with real data
		runID  = "zz-events-test-run"
	)

	cleanup := func() {
		db.ExecContext(ctx, `DELETE FROM metric_value WHERE entity = $1`, entity)
		db.ExecContext(ctx, `DELETE FROM scraper_progress WHERE run_id = $1`, runID)
	}
	cleanup()
	defer cleanup()

	now := time.Now().UTC()

	// Harvester write: a price_close row.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO metric_value (metric_key, entity, ts, value, flag, tier, inputs_hash, computed_at)
		VALUES ('price_close', $1, $2, 123.45, 'fresh', 'daily', decode('00','hex'), $2)
	`, entity, now); err != nil {
		t.Fatalf("seed price_close (harvester) failed: %v", err)
	}

	// Analytic write: a derived metric row.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO metric_value (metric_key, entity, ts, value, flag, tier, inputs_hash, computed_at)
		VALUES ('nimvi', $1, $2, 0.42, 'fresh', 'daily', decode('01','hex'), $2)
	`, entity, now); err != nil {
		t.Fatalf("seed derived metric (analytic) failed: %v", err)
	}

	// Scraper run.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO scraper_progress (run_id, status, funds_done, holdings_saved, last_fund, updated_at)
		VALUES ($1, 'done', 3, 15, 'AEH', $2)
	`, runID, now); err != nil {
		t.Fatalf("seed scraper_progress failed: %v", err)
	}

	server := NewServer(db, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/events", nil)
	server.GetAdminEvents(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp model.AdminEventsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	sources := map[string]bool{}
	for _, e := range resp.Events {
		sources[e.Source] = true
	}
	for _, want := range []string{"harvester", "analytic", "scraper"} {
		if !sources[want] {
			t.Errorf("expected an event from source %q in the feed; got events: %+v", want, resp.Events)
		}
	}

	// Feed must be ordered newest-first.
	for i := 1; i < len(resp.Events); i++ {
		if resp.Events[i-1].Ts.Before(resp.Events[i].Ts) {
			t.Errorf("events not sorted desc by ts at index %d: %v before %v",
				i, resp.Events[i-1].Ts, resp.Events[i].Ts)
		}
	}
}
