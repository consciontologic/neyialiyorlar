package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/lib/pq"

	"github.com/neyialiyorlar/services/api/internal/model"
)

// GetAdminDrift handles GET /api/v1/admin/drift.
//
// It returns the OPEN schema/DOM drift alerts (drift_alert table) for the
// dedicated drift section of the İzleme Paneli. These are BREAKING structural
// changes the harvester detected in an upstream payload — a required field
// removed or a field retyped — persisted with field-level diffs so they survive
// restarts and remain visible until the underlying drift is fixed and the alert
// is resolved. Breaking alerts sort first, then most-recently-seen.
//
// Best-effort: a query failure yields an empty (non-null) list rather than a 500,
// so the panel degrades gracefully.
func (s *Server) GetAdminDrift(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_ = json.NewEncoder(w).Encode(model.DriftAlertsResponse{
		Alerts:    s.openDriftAlerts(ctx),
		Generated: time.Now().UTC(),
	})
}

// openDriftAlerts loads every open drift alert, newest-contract-break first.
// It always returns a non-nil slice so the JSON `alerts` field is `[]`, never
// `null`, when there is nothing outstanding.
func (s *Server) openDriftAlerts(ctx context.Context) []model.DriftAlert {
	out := make([]model.DriftAlert, 0, 8)

	const q = `
		SELECT id, source, severity, summary,
		       added, removed, retyped,
		       status, occurrences,
		       first_seen_at, last_seen_at, resolved_at
		FROM drift_alert
		WHERE status = 'open'
		ORDER BY last_seen_at DESC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return out
	}
	defer rows.Close()

	for rows.Next() {
		var a model.DriftAlert
		var resolved sql.NullTime
		if err := rows.Scan(
			&a.ID, &a.Source, &a.Severity, &a.Summary,
			pq.Array(&a.Added), pq.Array(&a.Removed), pq.Array(&a.Retyped),
			&a.Status, &a.Occurrences,
			&a.FirstSeen, &a.LastSeen, &resolved,
		); err != nil {
			continue
		}
		if resolved.Valid {
			t := resolved.Time.UTC()
			a.ResolvedAt = &t
		}
		a.FirstSeen = a.FirstSeen.UTC()
		a.LastSeen = a.LastSeen.UTC()
		out = append(out, a)
	}

	sortDriftAlerts(out)
	return out
}

// sortDriftAlerts orders alerts breaking-first, then most-recently-seen, so the
// most urgent outstanding contract break sits at the top of the panel. Pure and
// independently testable.
func sortDriftAlerts(alerts []model.DriftAlert) {
	sort.SliceStable(alerts, func(i, j int) bool {
		bi := alerts[i].Severity == "breaking"
		bj := alerts[j].Severity == "breaking"
		if bi != bj {
			return bi // breaking sorts before non-breaking
		}
		return alerts[i].LastSeen.After(alerts[j].LastSeen)
	})
}

// ResolveAdminDrift handles POST /api/v1/admin/drift/{id}/resolve.
//
// It marks a single open alert resolved (status='resolved', resolved_at=now()),
// which removes it from the panel. This is the write side of the "keep it until
// the fix is applied" contract: an operator — or an agent that just fixed the
// upstream drift — calls this once the schema change is handled. Resolving an
// unknown or already-resolved alert is a 404 (nothing open to resolve).
func (s *Server) ResolveAdminDrift(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, `{"error":"invalid drift alert id"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	const q = `
		UPDATE drift_alert
		SET status = 'resolved', resolved_at = now()
		WHERE id = $1 AND status = 'open'`
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		http.Error(w, `{"error":"failed to resolve drift alert"}`, http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, `{"error":"drift alert not found or already resolved"}`, http.StatusNotFound)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"resolved": true, "id": id})
}
