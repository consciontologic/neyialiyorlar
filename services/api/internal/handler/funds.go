package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

// FundHolding represents a fund that holds a given stock.
type FundHolding struct {
	FundCode    string   `json:"fund_code"`
	FundTitle   string   `json:"fund_title"`
	FundManager string   `json:"fund_manager"`
	FundType    string   `json:"fund_type"`
	WeightPct   *float64 `json:"weight_pct,omitempty"` // nil when unknown
	AsOfDate    string   `json:"as_of_date"`
	Source      string   `json:"source"`
}

// GetEntityFunds handles GET /api/v1/entities/{id}/funds
// Returns all funds (fund_holding JOIN fund_ref) that hold the given stock,
// using the most recent as_of_date per fund per source.
func (s *Server) GetEntityFunds(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("id")
	if entityID == "" {
		http.Error(w, "missing entity id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	const q = `
		WITH latest AS (
			SELECT
				fh.fund_code,
				fh.source,
				MAX(fh.as_of_date) AS max_date
			FROM fund_holding fh
			WHERE fh.stock_id = $1
			GROUP BY fh.fund_code, fh.source
		)
		SELECT
			fh.fund_code,
			fr.fund_title,
			fr.fund_manager,
			fr.fund_type,
			fh.weight_pct,
			fh.as_of_date,
			fh.source
		FROM fund_holding fh
		JOIN fund_ref fr ON fr.fund_code = fh.fund_code
		JOIN latest l
			ON l.fund_code = fh.fund_code
			AND l.source   = fh.source
			AND l.max_date = fh.as_of_date
		WHERE fh.stock_id = $1
		ORDER BY fh.weight_pct DESC NULLS LAST, fh.fund_code, fh.source
	`

	rows, err := s.db.QueryContext(ctx, q, entityID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	funds := make([]FundHolding, 0)
	for rows.Next() {
		var f FundHolding
		var asOf time.Time
		if err := rows.Scan(
			&f.FundCode,
			&f.FundTitle,
			&f.FundManager,
			&f.FundType,
			&f.WeightPct,
			&asOf,
			&f.Source,
		); err != nil {
			http.Error(w, "scan error", http.StatusInternalServerError)
			return
		}
		f.AsOfDate = asOf.Format("2006-01-02")
		funds = append(funds, f)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "rows error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"entity": entityID,
		"funds":  funds,
	})
}
