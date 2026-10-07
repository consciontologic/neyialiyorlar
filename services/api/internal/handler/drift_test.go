package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/lib/pq"

	"github.com/neyialiyorlar/services/api/internal/model"
)

// TestSortDriftAlerts_BreakingFirstThenRecent verifies the panel ordering:
// breaking contract breaks float to the top, and within a severity the most
// recently-seen alert wins. Pure — no database required.
func TestSortDriftAlerts_BreakingFirstThenRecent(t *testing.T) {
	now := time.Now().UTC()
	alerts := []model.DriftAlert{
		{ID: 1, Severity: "benign", LastSeen: now},
		{ID: 2, Severity: "breaking", LastSeen: now.Add(-time.Hour)},
		{ID: 3, Severity: "breaking", LastSeen: now},
		{ID: 4, Severity: "benign", LastSeen: now.Add(-2 * time.Hour)},
	}

	sortDriftAlerts(alerts)

	gotOrder := []int64{alerts[0].ID, alerts[1].ID, alerts[2].ID, alerts[3].ID}
	want := []int64{3, 2, 1, 4} // breaking(newest), breaking(older), benign(newest), benign(older)
	for i := range want {
		if gotOrder[i] != want[i] {
			t.Fatalf("unexpected order: got %v, want %v", gotOrder, want)
		}
	}
}

// TestGetAndResolveAdminDrift exercises the full open→resolve lifecycle against a
// real database: a breaking and a benign alert are seeded open, GET returns both
// (breaking first), the breaking alert is resolved via the POST handler, and a
// second GET confirms it is gone while the still-open benign one remains. Skips
// when TEST_DATABASE_URL is unset, like the other DB integration tests.
func TestGetAndResolveAdminDrift(t *testing.T) {
	db := openFundsTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const source = "zz-drift-test"

	cleanup := func() {
		db.ExecContext(ctx, `DELETE FROM drift_alert WHERE source = $1`, source)
	}
	cleanup()
	defer cleanup()

	now := time.Now().UTC()

	// Breaking alert: a required field removed.
	var breakingID int64
	if err := db.QueryRowContext(ctx, `
		INSERT INTO drift_alert
			(source, severity, signature, summary, added, removed, retyped,
			 status, occurrences, first_seen_at, last_seen_at)
		VALUES ($1, 'breaking', 'sig-breaking', 'removed: chart.result[].indicators',
			$2, $3, $4, 'open', 2, $5, $5)
		RETURNING id
	`, source, pq.Array([]string{}), pq.Array([]string{"chart.result[].indicators"}),
		pq.Array([]string{}), now).Scan(&breakingID); err != nil {
		t.Fatalf("seed breaking alert failed: %v", err)
	}

	// Benign alert: an additive field, seen earlier.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO drift_alert
			(source, severity, signature, summary, added, removed, retyped,
			 status, occurrences, first_seen_at, last_seen_at)
		VALUES ($1, 'benign', 'sig-benign', 'added: chart.meta.newField',
			$2, $3, $4, 'open', 1, $5, $5)
	`, source, pq.Array([]string{"chart.meta.newField"}), pq.Array([]string{}),
		pq.Array([]string{}), now.Add(-time.Hour)); err != nil {
		t.Fatalf("seed benign alert failed: %v", err)
	}

	server := NewServer(db, nil)

	// First GET: both alerts present, breaking first, with field-level diffs.
	get := func() model.DriftAlertsResponse {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/drift", nil)
		server.GetAdminDrift(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("GET drift: expected 200, got %d (body: %s)", w.Code, w.Body.String())
		}
		var resp model.DriftAlertsResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode drift response: %v", err)
		}
		return resp
	}

	resp := get()
	mine := filterBySource(resp.Alerts, source)
	if len(mine) != 2 {
		t.Fatalf("expected 2 open alerts for source %q, got %d (%+v)", source, len(mine), mine)
	}
	if mine[0].Severity != "breaking" {
		t.Errorf("expected breaking alert first, got %q", mine[0].Severity)
	}
	if len(mine[0].Removed) != 1 || mine[0].Removed[0] != "chart.result[].indicators" {
		t.Errorf("expected field-level removed diff on breaking alert, got %v", mine[0].Removed)
	}

	// Resolve the breaking alert.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/drift/"+strconv.FormatInt(breakingID, 10)+"/resolve", nil)
	r.SetPathValue("id", strconv.FormatInt(breakingID, 10))
	server.ResolveAdminDrift(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("resolve: expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Second GET: the breaking alert is gone; the benign one remains open.
	resp = get()
	mine = filterBySource(resp.Alerts, source)
	if len(mine) != 1 {
		t.Fatalf("expected 1 open alert after resolve, got %d (%+v)", len(mine), mine)
	}
	if mine[0].Severity != "benign" {
		t.Errorf("expected the remaining open alert to be benign, got %q", mine[0].Severity)
	}

	// Resolving again is a 404 (nothing open with that id).
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/api/v1/admin/drift/"+strconv.FormatInt(breakingID, 10)+"/resolve", nil)
	r.SetPathValue("id", strconv.FormatInt(breakingID, 10))
	server.ResolveAdminDrift(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("re-resolve: expected 404, got %d", w.Code)
	}
}

// TestResolveAdminDrift_InvalidID rejects a non-numeric id with 400 without
// touching the database.
func TestResolveAdminDrift_InvalidID(t *testing.T) {
	server := NewServer(nil, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/drift/abc/resolve", nil)
	r.SetPathValue("id", "abc")
	server.ResolveAdminDrift(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-numeric id, got %d", w.Code)
	}
}

func filterBySource(alerts []model.DriftAlert, source string) []model.DriftAlert {
	out := make([]model.DriftAlert, 0, len(alerts))
	for _, a := range alerts {
		if a.Source == source {
			out = append(out, a)
		}
	}
	return out
}
