package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/neyialiyorlar/services/api/internal/model"
)

// GetAdminEvents handles GET /api/v1/admin/events.
//
// It returns a time-ordered feed of REAL backend activity, derived purely from
// persisted writes — there are no synthetic or fabricated entries:
//
//   - harvester → batches of price_close rows (one event per write-second)
//   - analytic  → batches of derived metric_value rows
//   - scraper   → fund-scraper runs (scraper_progress)
//
// The monitoring panel (İzleme Paneli) polls this endpoint so an operator can
// watch the ingestion + analytic pipelines actually doing work. Each source is
// best-effort: a failing query for one source never blanks the whole feed.
func (s *Server) GetAdminEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	const perSource = 20
	events := make([]model.ActivityEvent, 0, perSource*3)
	events = append(events, s.harvesterEvents(ctx, perSource)...)
	events = append(events, s.analyticEvents(ctx, perSource)...)
	events = append(events, s.scraperEvents(ctx, perSource)...)

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Ts.After(events[j].Ts)
	})

	const maxEvents = 40
	if len(events) > maxEvents {
		events = events[:maxEvents]
	}

	_ = json.NewEncoder(w).Encode(model.AdminEventsResponse{
		Events:    events,
		Generated: time.Now().UTC(),
	})
}

// harvesterEvents groups recent price_close writes by the second they were
// computed — one event per harvester batch ("N hisse fiyatı çekildi").
func (s *Server) harvesterEvents(ctx context.Context, limit int) []model.ActivityEvent {
	const q = `
		SELECT date_trunc('second', computed_at) AS ts,
		       COUNT(*)                           AS n,
		       COUNT(DISTINCT entity)             AS ents
		FROM metric_value
		WHERE metric_key = 'price_close'
		GROUP BY 1
		ORDER BY 1 DESC
		LIMIT $1`
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []model.ActivityEvent
	for rows.Next() {
		var ts time.Time
		var n, ents int
		if err := rows.Scan(&ts, &n, &ents); err != nil {
			continue
		}
		out = append(out, model.ActivityEvent{
			Ts:     ts.UTC(),
			Source: "harvester",
			Title:  "Fiyat güncellendi",
			Detail: fmt.Sprintf("%d hisse için kapanış fiyatı çekildi", ents),
			Count:  n,
		})
	}
	return out
}

// analyticEvents groups recent derived metric writes by computed-second — one
// event per analytic recompute batch.
func (s *Server) analyticEvents(ctx context.Context, limit int) []model.ActivityEvent {
	const q = `
		SELECT date_trunc('second', computed_at) AS ts,
		       COUNT(*)                           AS n,
		       COUNT(DISTINCT metric_key)         AS metrics,
		       COUNT(DISTINCT entity)             AS ents
		FROM metric_value
		WHERE metric_key <> 'price_close'
		GROUP BY 1
		ORDER BY 1 DESC
		LIMIT $1`
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []model.ActivityEvent
	for rows.Next() {
		var ts time.Time
		var n, metrics, ents int
		if err := rows.Scan(&ts, &n, &metrics, &ents); err != nil {
			continue
		}
		out = append(out, model.ActivityEvent{
			Ts:     ts.UTC(),
			Source: "analytic",
			Title:  "Metrikler hesaplandı",
			Detail: fmt.Sprintf("%d metrik türü × %d hisse", metrics, ents),
			Count:  n,
		})
	}
	return out
}

// scraperEvents emits one event per recent fund-scraper run.
func (s *Server) scraperEvents(ctx context.Context, limit int) []model.ActivityEvent {
	const q = `
		SELECT COALESCE(updated_at, started_at) AS ts,
		       status,
		       funds_done,
		       holdings_saved,
		       COALESCE(last_fund, '')
		FROM scraper_progress
		ORDER BY ts DESC
		LIMIT $1`
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []model.ActivityEvent
	for rows.Next() {
		var ts time.Time
		var status, lastFund string
		var fundsDone, holdings int
		if err := rows.Scan(&ts, &status, &fundsDone, &holdings, &lastFund); err != nil {
			continue
		}
		title := "Fon taraması"
		switch status {
		case "done":
			title = "Fon taraması tamamlandı"
		case "running":
			title = "Fon taraması sürüyor"
		case "error":
			title = "Fon taraması hatası"
		}
		detail := fmt.Sprintf("%d fon, %d holding kaydedildi", fundsDone, holdings)
		if lastFund != "" {
			detail += " · son: " + lastFund
		}
		out = append(out, model.ActivityEvent{
			Ts:     ts.UTC(),
			Source: "scraper",
			Title:  title,
			Detail: detail,
			Count:  holdings,
		})
	}
	return out
}
