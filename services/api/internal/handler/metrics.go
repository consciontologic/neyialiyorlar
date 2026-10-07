package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/neyialiyorlar/services/api/internal/model"
	"github.com/redis/go-redis/v9"
)

// Ticker triggers one synthetic recompute round (dev-mode data engine).
// Implemented by *synthetic.Engine; wired in main so the handler package stays
// decoupled from the synthetic package.
type Ticker interface {
	Tick(ctx context.Context, at time.Time) (int, error)
}

// Counter returns table row counts for the operator "investigate" view.
type Counter interface {
	Counts(ctx context.Context) (map[string]int64, error)
}

// Server holds API dependencies
type Server struct {
	db          *sql.DB
	redis       *redis.Client
	maxBodySize int64              // Max request body size
	maxQueries  int                // Max metrics per query
	wsHub       *WSHub             // WebSocket hub for real-time updates
	catalog     []model.MetricInfo // metric catalogue (from config)
	entities    []model.EntityInfo // entity catalogue (securities + baskets, from config)
	ticker      Ticker             // optional dev synthetic engine
	counter     Counter            // optional row-count source for status
}

// NewServer creates a new API server
func NewServer(db *sql.DB, client *redis.Client, opts ...Option) *Server {
	s := &Server{
		db:          db,
		redis:       client,
		maxBodySize: 10 * 1024 * 1024, // 10MB default
		maxQueries:  100,              // Default max queries per batch
	}

	for _, opt := range opts {
		opt(s)
	}

	// Initialize WebSocket hub
	s.wsHub = NewWSHub(client)

	return s
}

// Hub exposes the shared WebSocket hub so the synthetic engine can broadcast.
func (s *Server) Hub() *WSHub { return s.wsHub }

// SetTicker wires the dev synthetic engine after construction (the engine needs
// the server's WebSocket hub, so it cannot be supplied as a constructor option).
func (s *Server) SetTicker(t Ticker) { s.ticker = t }

// CloseWS gracefully closes all WebSocket connections
func (s *Server) CloseWS(timeout time.Duration) {
	if s.wsHub == nil {
		return
	}
	s.wsHub.Close(timeout)
}

// Option is a functional option for Server
type Option func(*Server)

// WithMaxBodySize sets the max request body size
func WithMaxBodySize(size int64) Option {
	return func(s *Server) {
		s.maxBodySize = size
	}
}

// WithMaxQueries sets the max queries per batch
func WithMaxQueries(max int) Option {
	return func(s *Server) {
		s.maxQueries = max
	}
}

// WithCatalog sets the metric catalogue returned by ListMetrics.
func WithCatalog(catalog []model.MetricInfo) Option {
	return func(s *Server) {
		s.catalog = catalog
	}
}

// WithEntities sets the entity catalogue (securities + baskets) returned by
// ListEntities.
func WithEntities(entities []model.EntityInfo) Option {
	return func(s *Server) {
		s.entities = entities
	}
}

// WithTicker wires the dev synthetic engine used by POST /api/v1/admin/tick.
func WithTicker(t Ticker) Option {
	return func(s *Server) {
		s.ticker = t
	}
}

// WithCounter wires the row-count source used by GET /api/v1/admin/status.
func WithCounter(c Counter) Option {
	return func(s *Server) {
		s.counter = c
	}
}

// ListMetrics handles GET /api/v1/metrics.
//
// It returns the configured metric catalogue so the dashboard always has the
// full set to render. When a database is wired, each metric's flag is overlaid
// with the latest observed confidence flag from metric_value so the badge
// reflects real freshness.
func (s *Server) ListMetrics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	metrics := make([]model.MetricInfo, len(s.catalog))
	copy(metrics, s.catalog)

	if s.db != nil && len(metrics) > 0 {
		if rows, err := s.db.QueryContext(ctx, `
			SELECT DISTINCT ON (metric_key) metric_key, flag
			FROM metric_value
			ORDER BY metric_key, ts DESC`); err == nil {
			defer rows.Close()
			latest := make(map[string]string)
			for rows.Next() {
				var key, flag string
				if err := rows.Scan(&key, &flag); err == nil {
					latest[key] = flag
				}
			}
			for i := range metrics {
				if flag, ok := latest[metrics[i].Key]; ok {
					metrics[i].Flag = model.Confidence(flag)
				}
			}
		}
	}

	response := model.ListMetricsResponse{Metrics: metrics}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(response)
}

// ListEntities handles GET /api/v1/entities.
//
// Returns the full entity catalogue from the entity_ref table so the dashboard
// renders every BIST-listed stock under analysis. Falls back to the in-memory
// config entities when the DB is unavailable (e.g., cold start race).
func (s *Server) ListEntities(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	entities, err := s.loadEntitiesFromDB(ctx)
	if err != nil || len(entities) == 0 {
		// Fallback: config-seeded in-memory catalogue
		entities = make([]model.EntityInfo, len(s.entities))
		copy(entities, s.entities)
	}

	response := model.ListEntitiesResponse{Entities: entities}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=60")
	json.NewEncoder(w).Encode(response)
}

// GetEntityMetrics handles GET /api/v1/entities/{id}/metrics.
// Returns the most-recent value for each distinct metric_key available for
// the entity, so the Flutter client can populate the metric grid without
// waiting for the live WebSocket stream.
func (s *Server) GetEntityMetrics(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("id")
	if entityID == "" {
		http.Error(w, `{"error":"entity id required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	metrics, err := s.latestEntityMetrics(ctx, entityID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error":%q}`, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=60")
	json.NewEncoder(w).Encode(metrics)
}

// metricRow is the JSON shape returned by GetEntityMetrics.
// Both "key" and "metric_key" are included so the Flutter client can
// parse whichever field name its MetricInfo.fromJson expects.
type metricRow struct {
	Key       string    `json:"key"`        // short alias used by Flutter MetricInfo
	MetricKey string    `json:"metric_key"` // DB column name alias
	Entity    string    `json:"entity"`
	TS        time.Time `json:"ts"`
	Value     *float64  `json:"value"`
	Flag      string    `json:"flag"`
	Tier      string    `json:"tier"`
}

// latestEntityMetrics returns the most-recent metric_value row per metric_key
// for the given entity.  Returns an empty (non-nil) slice when there is no data.
func (s *Server) latestEntityMetrics(ctx context.Context, entityID string) ([]metricRow, error) {
	out := []metricRow{} // never nil so JSON encodes as []
	if s.db == nil {
		return out, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT ON (metric_key)
			metric_key, entity, ts, value, flag::text, tier::text
		FROM metric_value
		WHERE entity = $1
		ORDER BY metric_key, ts DESC`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var mr metricRow
		if err := rows.Scan(&mr.MetricKey, &mr.Entity, &mr.TS, &mr.Value, &mr.Flag, &mr.Tier); err != nil {
			return nil, err
		}
		mr.Key = mr.MetricKey // populate alias
		out = append(out, mr)
	}
	return out, rows.Err()
}

// GetMetricTop handles GET /api/v1/metrics/{key}/top
// Returns the top N security entities by their latest value for the given metric.
// Query params: n (default 10, max 50), order ("asc"/"desc", default "desc").
func (s *Server) GetMetricTop(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"metric key required"}`)
		return
	}

	n := 10
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 && parsed <= 50 {
			n = parsed
		}
	}

	orderDir := "DESC"
	if strings.EqualFold(r.URL.Query().Get("order"), "asc") {
		orderDir = "ASC"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	type topMover struct {
		Entity string   `json:"entity"`
		Value  *float64 `json:"value"`
		Flag   string   `json:"flag"`
	}

	movers := []topMover{}
	if s.db != nil {
		q := fmt.Sprintf(`
			WITH latest AS (
				SELECT DISTINCT ON (mv.entity)
					mv.entity, mv.value, mv.flag::text
				FROM metric_value mv
				JOIN entity_ref er
					ON er.entity_id = mv.entity
					AND er.valid_to IS NULL
					AND er.entity_type = 'security'
				WHERE mv.metric_key = $1 AND mv.value IS NOT NULL
				ORDER BY mv.entity, mv.ts DESC
			)
			SELECT entity, value, flag FROM latest
			ORDER BY value %s
			LIMIT $2`, orderDir)
		rows, err := s.db.QueryContext(ctx, q, key, n)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var m topMover
				if err := rows.Scan(&m.Entity, &m.Value, &m.Flag); err == nil {
					movers = append(movers, m)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=60")
	json.NewEncoder(w).Encode(map[string]any{"metric": key, "movers": movers})
}

// loadEntitiesFromDB reads all active entities from entity_ref joined with
// basket_member (aggregated with array_agg) to populate Basket + Baskets.
func (s *Server) loadEntitiesFromDB(ctx context.Context) ([]model.EntityInfo, error) {
	if s.db == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			e.entity_id,
			e.entity_type,
			COALESCE(e.display_ticker, e.entity_id) AS display_ticker,
			COALESCE(e.isin, '')                     AS isin,
			COALESCE(
				array_to_string(array_agg(bm.basket_id ORDER BY bm.basket_id) FILTER (WHERE bm.basket_id IS NOT NULL), ','),
				''
			) AS basket_ids,
			COALESCE(fc.fund_count, 0) AS fund_count
		FROM entity_ref e
		LEFT JOIN basket_member bm
			ON bm.entity_id = e.entity_id AND bm.valid_to IS NULL
		LEFT JOIN (
			SELECT stock_id, COUNT(DISTINCT fund_code) AS fund_count
			FROM fund_holding
			GROUP BY stock_id
		) fc ON fc.stock_id = e.entity_id
		WHERE e.valid_to IS NULL
		GROUP BY e.entity_id, e.entity_type, e.display_ticker, e.isin, fc.fund_count
		ORDER BY e.entity_type DESC, e.display_ticker ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.EntityInfo
	for rows.Next() {
		var ei model.EntityInfo
		var isin, basketIDs string
		if err := rows.Scan(&ei.ID, &ei.Type, &ei.DisplayTicker, &isin, &basketIDs, &ei.FundCount); err != nil {
			return nil, err
		}
		if isin != "" {
			ei.ISIN = isin
		}
		if basketIDs != "" {
			ids := strings.Split(basketIDs, ",")
			ei.Baskets = ids
			ei.Basket = ids[0] // back-compat: first basket
		}
		out = append(out, ei)
	}
	return out, rows.Err()
}

// GetMetricLatest handles GET /api/v1/metrics/{key}/latest?entity=
func (s *Server) GetMetricLatest(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	key := r.PathValue("key")
	entity := r.URL.Query().Get("entity")

	if key == "" || entity == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(model.ErrorResponse{
			Error:     "missing key or entity parameter",
			Timestamp: time.Now().UTC(),
		})
		return
	}

	// Try Redis first (hot cache)
	cacheKey := "m:" + key + ":" + entity + ":latest"
	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache hit: parse and return
		var value model.MetricValue
		if err := json.Unmarshal([]byte(cached), &value); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			json.NewEncoder(w).Encode(value)
			return
		}
	}

	// Cache miss: fall through to Postgres
	// Use mv_hot covering index for fast lookup
	var value *float64
	var flag string
	var ts time.Time
	var tier string

	row := s.db.QueryRowContext(ctx, `
		SELECT value, flag, ts, tier FROM metric_value
		WHERE metric_key = $1 AND entity = $2
		ORDER BY ts DESC LIMIT 1
	`, key, entity)

	if err := row.Scan(&value, &flag, &ts, &tier); err == sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(model.ErrorResponse{
			Error:     "metric or entity not found",
			Timestamp: time.Now().UTC(),
		})
		return
	} else if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(model.ErrorResponse{
			Error:     "database error",
			Timestamp: time.Now().UTC(),
		})
		return
	}

	result := model.MetricValue{
		Metric: key,
		Entity: entity,
		Ts:     ts,
		Value:  value,
		Flag:   model.Confidence(flag),
		Tier:   model.CadenceTier(tier),
	}

	// Repopulate Redis cache
	if data, err := json.Marshal(result); err == nil {
		s.redis.Set(ctx, cacheKey, string(data), 1*time.Hour) // TTL from config
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(result)
}

// GetMetricSeries handles GET /api/v1/metrics/{key}/series?entity=&win=
func (s *Server) GetMetricSeries(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	key := r.PathValue("key")
	entity := r.URL.Query().Get("entity")
	window := r.URL.Query().Get("win")
	if window == "" {
		window = "30d"
	}

	if key == "" || entity == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(model.ErrorResponse{
			Error:     "missing key or entity parameter",
			Timestamp: time.Now().UTC(),
		})
		return
	}

	// Query recent points from metric_value
	// ORDER BY ts DESC to get most recent first
	rows, err := s.db.QueryContext(ctx, `
		SELECT value, flag, ts, tier FROM metric_value
		WHERE metric_key = $1 AND entity = $2
		AND ts > NOW() - INTERVAL '90 days'
		ORDER BY ts DESC LIMIT 500
	`, key, entity)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(model.ErrorResponse{
			Error:     "database error",
			Timestamp: time.Now().UTC(),
		})
		return
	}
	defer rows.Close()

	points := []model.MetricValue{}
	for rows.Next() {
		var value *float64
		var flag string
		var ts time.Time
		var tier string

		if err := rows.Scan(&value, &flag, &ts, &tier); err != nil {
			continue
		}

		points = append(points, model.MetricValue{
			Metric: key,
			Entity: entity,
			Ts:     ts,
			Value:  value,
			Flag:   model.Confidence(flag),
			Tier:   model.CadenceTier(tier),
		})
	}

	response := model.GetSeriesResponse{
		Metric: key,
		Entity: entity,
		Window: window,
		Points: points,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(response)
}

// GetSourcesHealth handles GET /api/v1/sources/health
func (s *Server) GetSourcesHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, `
		SELECT source, last_ok_at, checked_at, breaker_open, consecutive_failures
		FROM source_health
		ORDER BY source
	`)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(model.ErrorResponse{
			Error:     "database error",
			Timestamp: time.Now().UTC(),
		})
		return
	}
	defer rows.Close()

	sources := []model.SourceHealth{}
	for rows.Next() {
		var source string
		var lastOkAt sql.NullTime
		var checkedAt time.Time
		var breakerOpen bool
		var failures int

		if err := rows.Scan(&source, &lastOkAt, &checkedAt, &breakerOpen, &failures); err != nil {
			continue
		}

		health := model.SourceHealth{
			Source:              source,
			CheckedAt:           checkedAt,
			BreakerOpen:         breakerOpen,
			ConsecutiveFailures: failures,
		}

		if lastOkAt.Valid {
			health.LastOkAt = &lastOkAt.Time
		}

		sources = append(sources, health)
	}

	response := model.GetSourcesHealthResponse{
		Sources: sources,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(response)
}

// QueryMetrics handles POST /api/v1/query
func (s *Server) QueryMetrics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Enforce max body size
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBodySize)

	var req model.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(model.ErrorResponse{
			Error:     "invalid request body",
			Timestamp: time.Now().UTC(),
		})
		return
	}

	// Count total queries
	totalQueries := 0
	for _, q := range req.Queries {
		totalQueries += len(q.Entities)
	}

	if totalQueries > s.maxQueries {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestEntityTooLarge) // 413
		json.NewEncoder(w).Encode(model.ErrorResponse{
			Error:     "query exceeds maximum metrics per batch",
			Timestamp: time.Now().UTC(),
		})
		return
	}

	// Execute all queries
	result := []model.MetricValue{}

	for _, q := range req.Queries {
		for _, entity := range q.Entities {
			var value *float64
			var flag string
			var ts time.Time
			var tier string

			row := s.db.QueryRowContext(ctx, `
				SELECT value, flag, ts, tier FROM metric_value
				WHERE metric_key = $1 AND entity = $2
				ORDER BY ts DESC LIMIT 1
			`, q.Metric, entity)

			if err := row.Scan(&value, &flag, &ts, &tier); err == nil {
				result = append(result, model.MetricValue{
					Metric: q.Metric,
					Entity: entity,
					Ts:     ts,
					Value:  value,
					Flag:   model.Confidence(flag),
					Tier:   model.CadenceTier(tier),
				})
			}
		}
	}

	response := model.QueryResponse{
		Result: result,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(response)
}
