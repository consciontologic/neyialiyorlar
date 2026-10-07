package model

import "time"

// Confidence flag values: fresh, stale, approx
type Confidence string

const (
	ConfidenceFresh  Confidence = "fresh"
	ConfidenceStale  Confidence = "stale"
	ConfidenceApprox Confidence = "approx"
)

// CadenceTier represents the metric cadence
type CadenceTier string

const (
	CadenceTierIntraday CadenceTier = "intraday"
	CadenceTierDaily    CadenceTier = "daily"
	CadenceTierWeekly   CadenceTier = "weekly"
	CadenceTierEvent    CadenceTier = "event"
)

// MetricValue represents a single metric data point
type MetricValue struct {
	Metric string      `json:"metric"`
	Entity string      `json:"entity"`
	Ts     time.Time   `json:"ts"`
	Value  *float64    `json:"value"` // null for gaps
	Flag   Confidence  `json:"flag"`
	Tier   CadenceTier `json:"tier"`
}

// MetricInfo represents metric catalog info
type MetricInfo struct {
	Key  string      `json:"key"`
	Name string      `json:"name"`
	Flag Confidence  `json:"flag"`
	Tier CadenceTier `json:"tier"`
}

// EntityInfo represents a monitored entity (security / basket / fund / index).
// It mirrors the entities the system tracks so the dashboard can list every
// stock under analysis and link a security to the fund/basket it belongs to.
type EntityInfo struct {
	ID            string   `json:"id"`             // stable id (ISIN / MKK code / basket key)
	Type          string   `json:"type"`           // security | basket | fund | index
	DisplayTicker string   `json:"display_ticker"` // human-facing ticker
	ISIN          string   `json:"isin,omitempty"`
	Basket        string   `json:"basket,omitempty"`  // primary basket (back-compat)
	Baskets       []string `json:"baskets,omitempty"` // all baskets this entity belongs to
	FundCount     int      `json:"fund_count"`        // number of funds holding this security (0 = none)
}

// SourceHealth represents source health status
type SourceHealth struct {
	Source              string     `json:"source"`
	LastOkAt            *time.Time `json:"last_ok_at"`
	CheckedAt           time.Time  `json:"checked_at"`
	BreakerOpen         bool       `json:"breaker_open"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
}

// ListMetricsResponse is the response to GET /api/v1/metrics
type ListMetricsResponse struct {
	Metrics []MetricInfo `json:"metrics"`
}

// ListEntitiesResponse is the response to GET /api/v1/entities
type ListEntitiesResponse struct {
	Entities []EntityInfo `json:"entities"`
}

// GetSeriesResponse is the response to GET /api/v1/metrics/{key}/series
type GetSeriesResponse struct {
	Metric string        `json:"metric"`
	Entity string        `json:"entity"`
	Window string        `json:"window"`
	Points []MetricValue `json:"points"`
}

// GetSourcesHealthResponse is the response to GET /api/v1/sources/health
type GetSourcesHealthResponse struct {
	Sources []SourceHealth `json:"sources"`
}

// QueryRequest is the request body for POST /api/v1/query
type QueryRequest struct {
	Queries []struct {
		Metric   string   `json:"metric"`
		Entities []string `json:"entities"`
	} `json:"queries"`
}

// QueryResponse is the response to POST /api/v1/query
type QueryResponse struct {
	Result []MetricValue `json:"result"`
}

// ErrorResponse is a standard error response
type ErrorResponse struct {
	Error     string    `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// AdminTickResponse is the response to POST /api/v1/admin/tick
type AdminTickResponse struct {
	Emitted int       `json:"emitted"`
	Ts      time.Time `json:"ts"`
}

// AdminStatusResponse is the response to GET /api/v1/admin/status
type AdminStatusResponse struct {
	Service          string           `json:"service"`
	Time             time.Time        `json:"time"`
	SyntheticEnabled bool             `json:"synthetic_enabled"`
	MetricsCatalog   int              `json:"metrics_catalog"`
	Counts           map[string]int64 `json:"counts"`
}

// ScraperRun represents one fund scraper run's live progress.
type ScraperRun struct {
	RunID            string     `json:"run_id"`
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	Status           string     `json:"status"` // running | done | error
	ScanStart        int        `json:"scan_start"`
	ScanEnd          int        `json:"scan_end"`
	IndicesScanned   int        `json:"indices_scanned"`
	DisclosuresFound int        `json:"disclosures_found"`
	FundsTotal       int        `json:"funds_total"`
	FundsDone        int        `json:"funds_done"`
	HoldingsSaved    int        `json:"holdings_saved"`
	Errors           int        `json:"errors"`
	LastFund         string     `json:"last_fund,omitempty"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// FundCoverage summarises how many stocks have fund holding data.
type FundCoverage struct {
	TotalSecurities int `json:"total_securities"`
	StocksWithFunds int `json:"stocks_with_funds"`
	TotalHoldings   int `json:"total_holdings"`
	TotalFunds      int `json:"total_funds"`
}

// ScraperStatusResponse is returned by GET /api/v1/admin/scraper.
type ScraperStatusResponse struct {
	LatestRun *ScraperRun  `json:"latest_run,omitempty"`
	Coverage  FundCoverage `json:"coverage"`
}

// ActivityEvent is one real backend activity, derived from persisted writes
// (harvester price fetches, analytic recomputes, scraper runs). No synthetic or
// fabricated entries — every event reflects a row actually written to the DB.
// Powers the live "Canlı Olaylar" feed in the monitoring panel.
type ActivityEvent struct {
	Ts     time.Time `json:"ts"`
	Source string    `json:"source"` // harvester | analytic | scraper
	Title  string    `json:"title"`  // short label
	Detail string    `json:"detail,omitempty"`
	Count  int       `json:"count,omitempty"` // rows affected, when relevant
}

// AdminEventsResponse is returned by GET /api/v1/admin/events.
type AdminEventsResponse struct {
	Events    []ActivityEvent `json:"events"`
	Generated time.Time       `json:"generated_at"`
}

// DriftAlert is one persisted schema/DOM drift alert (drift_alert table),
// surfaced in the dedicated drift section of the İzleme Paneli. A `breaking`
// alert means an upstream source changed its structural contract (a required
// field removed or a field retyped); it stays `open` — with field-level diffs
// — until the underlying drift is fixed and the alert is resolved, so the
// operator always sees outstanding contract breaks.
type DriftAlert struct {
	ID          int64      `json:"id"`
	Source      string     `json:"source"`
	Severity    string     `json:"severity"` // benign | breaking
	Summary     string     `json:"summary"`
	Added       []string   `json:"added"`   // field paths added
	Removed     []string   `json:"removed"` // field paths removed
	Retyped     []string   `json:"retyped"` // field paths whose kind changed
	Status      string     `json:"status"`  // open | resolved
	Occurrences int        `json:"occurrences"`
	FirstSeen   time.Time  `json:"first_seen_at"`
	LastSeen    time.Time  `json:"last_seen_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// DriftAlertsResponse is returned by GET /api/v1/admin/drift.
type DriftAlertsResponse struct {
	Alerts    []DriftAlert `json:"alerts"`
	Generated time.Time    `json:"generated_at"`
}

// Signal represents a buy/sell/hold/insufficient_data directional signal.
type Signal string

const (
	SignalBuy          Signal = "BUY"
	SignalSell         Signal = "SELL"
	SignalHold         Signal = "HOLD"
	SignalInsufficient Signal = "INSUFFICIENT_DATA"
)

// SignalResponse is the response to GET /api/v1/entities/{id}/signal.
type SignalResponse struct {
	EntityID   string     `json:"entity_id"`
	Signal     Signal     `json:"signal"`
	Confidence Confidence `json:"confidence"`
	Score      float64    `json:"score"` // % deviation of MA5 from MA20
	MA5        float64    `json:"ma5"`
	MA20       float64    `json:"ma20"`
	Price      float64    `json:"price"`
	Reason     string     `json:"reason"`
	ComputedAt time.Time  `json:"computed_at"`
}
