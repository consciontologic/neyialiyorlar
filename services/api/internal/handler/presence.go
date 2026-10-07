package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/neyialiyorlar/services/api/internal/model"
)

// PresencePoint is one snapshot of a stock's fund presence: on a given
// as-of date, `Funds` distinct funds held the stock out of `Total` distinct
// funds that reported any holding that day, giving `Pct` = Funds/Total*100.
type PresencePoint struct {
	Date  string  `json:"date"`  // YYYY-MM-DD
	Pct   float64 `json:"pct"`   // presence percentage, 0..100
	Funds int     `json:"funds"` // distinct funds holding the stock
	Total int     `json:"total"` // distinct funds reporting that day
	at    time.Time
}

// PresenceChanges holds the relative percentage change of the presence
// percentage over each lookback window. A field is nil when there is no
// baseline snapshot at or before the lookback cutoff, or when that baseline
// was zero (a relative change against zero is undefined).
type PresenceChanges struct {
	Daily   *float64 `json:"daily"`
	Weekly  *float64 `json:"weekly"`
	Monthly *float64 `json:"monthly"`
	Yearly  *float64 `json:"yearly"`
}

// PresenceResponse is the payload of GET /api/v1/entities/{id}/presence.
type PresenceResponse struct {
	Entity      string          `json:"entity"`
	AsOf        string          `json:"as_of"`        // latest snapshot date, "" when no data
	PresencePct float64         `json:"presence_pct"` // latest presence percentage
	Funds       int             `json:"funds"`        // latest distinct funds holding
	Total       int             `json:"total"`        // latest distinct funds reporting
	Window      string          `json:"window"`       // echoed chart window
	Changes     PresenceChanges `json:"changes"`
	Points      []PresencePoint `json:"points"`
}

// minPresenceFetchDays is the minimum history fetched regardless of the chart
// window, so the yearly change always has a ~1y baseline to compare against
// even when the user is viewing a shorter window.
const minPresenceFetchDays = 370

// GetEntityPresence handles GET /api/v1/entities/{id}/presence?win=1y.
//
// It reports how widely a BIST stock is held across all scraped funds
// (TEFAS + KAP) — its "presence" = the share of reporting funds that hold it —
// as a time series plus daily/weekly/monthly/yearly relative changes. The
// series is the breadth signal behind buy/sell reads: rising fund presence is
// accumulation (more funds buying in), falling presence is distribution.
func (s *Server) GetEntityPresence(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("id")
	if entityID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(model.ErrorResponse{
			Error:     "entity id required",
			Timestamp: time.Now().UTC(),
		})
		return
	}

	win := r.URL.Query().Get("win")
	if win == "" {
		win = "1y"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := s.computePresence(ctx, entityID, win)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(model.ErrorResponse{
			Error:     "presence computation failed: " + err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=60")
	json.NewEncoder(w).Encode(resp)
}

// computePresence builds the presence series and change summary for an entity.
func (s *Server) computePresence(ctx context.Context, entityID, win string) (PresenceResponse, error) {
	resp := PresenceResponse{
		Entity:  entityID,
		Window:  win,
		Points:  []PresencePoint{},
		Changes: PresenceChanges{},
	}

	if s.db == nil {
		return resp, nil
	}

	windowDays := presenceWindowDays(win)
	fetchDays := windowDays
	if fetchDays != 0 && fetchDays < minPresenceFetchDays {
		fetchDays = minPresenceFetchDays
	}
	cutoff := presenceCutoff(fetchDays, time.Now().UTC())

	// Per as-of date: how many distinct funds hold this stock, and how many
	// distinct funds reported any holding that day (the denominator). Counting
	// distinct fund_code (not fund_code+source) avoids double-counting a fund
	// that appears in both the TEFAS and KAP feeds.
	const q = `
		SELECT
			as_of_date,
			COUNT(DISTINCT fund_code) FILTER (WHERE stock_id = $1) AS funds_holding,
			COUNT(DISTINCT fund_code)                              AS total_funds
		FROM fund_holding
		WHERE as_of_date >= $2
		GROUP BY as_of_date
		HAVING COUNT(DISTINCT fund_code) > 0
		ORDER BY as_of_date ASC`

	rows, err := s.db.QueryContext(ctx, q, entityID, cutoff)
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	full := make([]PresencePoint, 0)
	for rows.Next() {
		var asOf time.Time
		var funds, total int
		if err := rows.Scan(&asOf, &funds, &total); err != nil {
			return resp, err
		}
		full = append(full, newPresencePoint(asOf, funds, total))
	}
	if err := rows.Err(); err != nil {
		return resp, err
	}

	if len(full) == 0 {
		return resp, nil
	}

	// Changes are computed from the full fetched history so the yearly figure
	// can reach a ~1y-old baseline even when the chart window is shorter; the
	// returned points are trimmed to the requested window for display.
	resp.Changes = computePresenceChanges(full)
	resp.Points = trimPointsToDays(full, windowDays)

	latest := full[len(full)-1]
	resp.AsOf = latest.Date
	resp.PresencePct = latest.Pct
	resp.Funds = latest.Funds
	resp.Total = latest.Total

	return resp, nil
}

// newPresencePoint builds a point, computing the rounded presence percentage.
func newPresencePoint(asOf time.Time, funds, total int) PresencePoint {
	pct := 0.0
	if total > 0 {
		pct = roundAt(float64(funds)/float64(total)*100, 2)
	}
	return PresencePoint{
		Date:  asOf.Format("2006-01-02"),
		Pct:   pct,
		Funds: funds,
		Total: total,
		at:    asOf,
	}
}

// presenceWindowDays maps a window token to a day count. 0 means "all history".
// Unknown tokens fall back to one year so the d/w/m/y changes stay meaningful.
func presenceWindowDays(win string) int {
	switch strings.ToLower(strings.TrimSpace(win)) {
	case "1m", "30d":
		return 30
	case "3m", "90d":
		return 90
	case "6m", "180d":
		return 180
	case "1y", "12m", "365d":
		return 365
	case "2y", "730d":
		return 730
	case "all", "max":
		return 0
	}
	if s, ok := strings.CutSuffix(strings.ToLower(strings.TrimSpace(win)), "d"); ok {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 365
}

// presenceCutoff returns the earliest as-of date to fetch. days<=0 fetches all
// available history (bounded by a far-past sentinel).
func presenceCutoff(days int, now time.Time) time.Time {
	if days <= 0 {
		return now.AddDate(-100, 0, 0)
	}
	return now.AddDate(0, 0, -days)
}

// computePresenceChanges returns the relative percentage change of presence
// over the standard lookback windows.
func computePresenceChanges(pts []PresencePoint) PresenceChanges {
	return PresenceChanges{
		Daily:   pctChangeOverDays(pts, 1),
		Weekly:  pctChangeOverDays(pts, 7),
		Monthly: pctChangeOverDays(pts, 30),
		Yearly:  pctChangeOverDays(pts, 365),
	}
}

// pctChangeOverDays returns the relative percentage change between the latest
// point and the nearest snapshot at or before `days` before it. It returns nil
// when no such baseline exists or the baseline presence was zero. Snapshots are
// sparse (funds report periodically, not daily), so "nearest at or older than
// the cutoff" is the robust comparison rather than requiring an exact date.
func pctChangeOverDays(pts []PresencePoint, days int) *float64 {
	if len(pts) < 2 {
		return nil
	}
	latest := pts[len(pts)-1]
	cutoff := latest.at.AddDate(0, 0, -days)

	for i := len(pts) - 1; i >= 0; i-- {
		if pts[i].at.After(cutoff) {
			continue
		}
		baseline := pts[i]
		if baseline.Pct == 0 {
			return nil
		}
		chg := roundAt((latest.Pct-baseline.Pct)/baseline.Pct*100, 2)
		return &chg
	}
	return nil
}

// trimPointsToDays keeps only points within `days` of the latest point.
// days<=0 keeps all points. Trimming is relative to the latest snapshot (not
// wall-clock now) so a stale feed still yields a full-looking chart window.
func trimPointsToDays(pts []PresencePoint, days int) []PresencePoint {
	if days <= 0 || len(pts) == 0 {
		return pts
	}
	cutoff := pts[len(pts)-1].at.AddDate(0, 0, -days)
	out := make([]PresencePoint, 0, len(pts))
	for _, p := range pts {
		if !p.at.Before(cutoff) {
			out = append(out, p)
		}
	}
	return out
}
