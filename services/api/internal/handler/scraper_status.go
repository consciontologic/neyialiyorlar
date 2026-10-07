package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/neyialiyorlar/services/api/internal/model"
)

// GetScraperStatus handles GET /api/v1/admin/scraper.
//
// Returns the latest fund scraper run's live progress and fund coverage stats.
// Used by the monitoring panel in the Flutter PWA.
func (s *Server) GetScraperStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp := model.ScraperStatusResponse{}

	// Latest scraper run
	const runQ = `
		SELECT run_id, started_at, finished_at, status,
		       scan_start, scan_end, indices_scanned, disclosures_found,
		       funds_total, funds_done, holdings_saved, errors,
		       COALESCE(last_fund, ''), updated_at
		FROM scraper_progress
		ORDER BY started_at DESC LIMIT 1`

	var run model.ScraperRun
	var finishedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, runQ).Scan(
		&run.RunID, &run.StartedAt, &finishedAt, &run.Status,
		&run.ScanStart, &run.ScanEnd, &run.IndicesScanned, &run.DisclosuresFound,
		&run.FundsTotal, &run.FundsDone, &run.HoldingsSaved, &run.Errors,
		&run.LastFund, &run.UpdatedAt,
	)
	if err == nil {
		if finishedAt.Valid {
			run.FinishedAt = &finishedAt.Time
		}
		resp.LatestRun = &run
	}

	// Fund coverage stats — single-row multi-column query
	const covQ = `
		SELECT
			(SELECT COUNT(*) FROM entity_ref WHERE entity_type = 'security')  AS total_sec,
			(SELECT COUNT(DISTINCT stock_id) FROM fund_holding)                AS with_funds,
			(SELECT COUNT(*) FROM fund_holding)                                AS total_holdings,
			(SELECT COUNT(*) FROM fund_ref)                                    AS total_funds`

	_ = s.db.QueryRowContext(ctx, covQ).Scan(
		&resp.Coverage.TotalSecurities,
		&resp.Coverage.StocksWithFunds,
		&resp.Coverage.TotalHoldings,
		&resp.Coverage.TotalFunds,
	)

	json.NewEncoder(w).Encode(resp)
}
