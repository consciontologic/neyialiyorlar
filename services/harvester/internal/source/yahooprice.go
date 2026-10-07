package source

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/neyialiyorlar/services/harvester/internal/parse"
)

// yahooRequiredFields are the structural backbone paths the price extractor
// depends on, expressed in the fingerprint vocabulary of parse.FingerprintJSON
// (object keys joined by '.', array elements as '[]'). If Yahoo drops any of
// them the drift is BREAKING (a WARN alert), not a benign additive change.
// Kept conservative so only a genuine contract break — never an illiquid
// ticker's sparse-but-valid payload — raises an alert.
var yahooRequiredFields = []string{
	"chart.result",
	"chart.result[].timestamp",
	"chart.result[].indicators",
}

// YahooPriceFetcher fetches daily closing prices for BIST-listed securities
// from Yahoo Finance (tickers suffixed with ".IS") and writes them to the
// metric_value table as metric_key='price_close', flag='fresh', tier='daily'.
//
// Design: no auth required; Yahoo Finance v8/finance/chart is a public
// unauthenticated JSON endpoint. Rate-limit: 1 req/sec, ≤2 concurrent.
type YahooPriceFetcher struct {
	db         *sql.DB
	httpClient *http.Client
	logger     *slog.Logger
	// drift, when set, fingerprints each healthy Yahoo chart response and
	// raises a (persisted) breaking-drift alert if Yahoo changes the JSON's
	// structural backbone. Nil disables it; price ingestion is unaffected
	// either way.
	drift *parse.DriftDetector
}

// NewYahooPriceFetcher constructs a fetcher that writes to db.
func NewYahooPriceFetcher(db *sql.DB, logger *slog.Logger) *YahooPriceFetcher {
	return &YahooPriceFetcher{
		db:     db,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// SetDriftDetector enables structural drift detection on the Yahoo chart
// response. The detector self-seeds on its first observation, so no explicit
// contract seeding is required; only a later structural change (a required
// backbone field removed, or any field retyped) raises a breaking alert.
func (f *YahooPriceFetcher) SetDriftDetector(d *parse.DriftDetector) {
	f.drift = d
}

// yfChart is the minimal shape of the Yahoo Finance v8 chart response.
type yfChart struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Symbol              string  `json:"symbol"`
				RegularMarketPrice  float64 `json:"regularMarketPrice"`
				ExchangeTimezoneName string `json:"exchangeTimezoneName"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close  []*float64 `json:"close"`
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Volume []*float64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

// FetchAndStore fetches the last lookbackDays of closing prices for all
// securities in entity_ref and upserts them into metric_value.
func (f *YahooPriceFetcher) FetchAndStore(ctx context.Context, lookbackDays int) error {
	entities, err := f.loadSecurities(ctx)
	if err != nil {
		return fmt.Errorf("load securities: %w", err)
	}

	f.logger.Info("yahoo price fetch started", slog.Int("entities", len(entities)))

	ok, skip, fail := 0, 0, 0
	for _, ticker := range entities {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		points, err := f.fetchPrices(ctx, ticker, lookbackDays)
		if err != nil {
			f.logger.Warn("yahoo fetch failed",
				slog.String("ticker", ticker), slog.String("error", err.Error()))
			fail++
			// Throttle: 1 req/s regardless of error
			time.Sleep(1 * time.Second)
			continue
		}
		if len(points) == 0 {
			skip++
			time.Sleep(300 * time.Millisecond)
			continue
		}

		if err := f.storePrices(ctx, ticker, points); err != nil {
			f.logger.Warn("store prices failed",
				slog.String("ticker", ticker), slog.String("error", err.Error()))
			fail++
		} else {
			if err := f.computeAndStoreDerivedMetrics(ctx, ticker); err != nil {
				f.logger.Warn("compute derived metrics failed",
					slog.String("ticker", ticker), slog.String("error", err.Error()))
			}
			ok++
		}
		// Polite rate-limit: 1 req/s average
		time.Sleep(1 * time.Second)
	}

	f.logger.Info("yahoo price fetch complete",
		slog.Int("ok", ok), slog.Int("skip", skip), slog.Int("fail", fail))
	return nil
}

// pricePoint holds one daily price observation.
type pricePoint struct {
	ts    time.Time
	close float64
}

// fetchPrices calls the Yahoo Finance chart API and returns close prices.
func (f *YahooPriceFetcher) fetchPrices(ctx context.Context, ticker string, lookbackDays int) ([]pricePoint, error) {
	yfsym := ticker + ".IS"
	url := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=%dd",
		yfsym, lookbackDays,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "neyialiyorlar-research/0.1 (+local)")
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		// Ticker not on Yahoo Finance (delisted or wrong ticker)
		return nil, nil //nolint:nilnil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB cap
	if err != nil {
		return nil, err
	}

	var chart yfChart
	if err := json.Unmarshal(body, &chart); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if chart.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo error: %s", chart.Chart.Error.Description)
	}
	if len(chart.Chart.Result) == 0 {
		return nil, nil //nolint:nilnil
	}

	result := chart.Chart.Result[0]
	if len(result.Indicators.Quote) == 0 {
		return nil, nil //nolint:nilnil
	}

	// Best-effort structural drift detection on the canonical healthy Yahoo
	// shape. Keyed by a single "yahoo" source so every ticker shares one
	// baseline; the detector self-seeds on first sight and only a genuine
	// structural change raises an alert (which the wired logger persists).
	// Errors are swallowed so price ingestion is never affected.
	if f.drift != nil {
		_, _ = f.drift.ObserveJSON("yahoo", body, yahooRequiredFields)
	}

	closes := result.Indicators.Quote[0].Close
	timestamps := result.Timestamp

	var out []pricePoint
	for i, ts := range timestamps {
		if i >= len(closes) || closes[i] == nil {
			continue
		}
		v := *closes[i]
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			continue
		}
		out = append(out, pricePoint{
			ts:    time.Unix(ts, 0).UTC().Truncate(24 * time.Hour),
			close: v,
		})
	}
	return out, nil
}

// storePrices upserts price_close observations into metric_value.
// Uses INSERT ... ON CONFLICT DO NOTHING so re-runs are idempotent.
// The inputs_hash (bytea) is computed in Go to avoid pq type-deduction
// problems when the same parameter appears in both a typed column position
// and inside a SQL expression.
func (f *YahooPriceFetcher) storePrices(ctx context.Context, ticker string, points []pricePoint) error {
	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO metric_value
			(metric_key, entity, ts, value, flag, tier, inputs_hash, computed_at)
		VALUES
			('price_close', $1, $2, $3, 'fresh'::confidence, 'daily'::cadence_tier, $4, NOW())
		ON CONFLICT (metric_key, entity, ts, inputs_hash) DO NOTHING`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range points {
		// Compute hash in Go so the same parameter isn't used in different type
		// contexts (which confuses the pq extended-query type-deduction protocol).
		raw := fmt.Sprintf("%s|%d|%.6f", ticker, p.ts.Unix(), p.close)
		h := sha256.Sum256([]byte(raw))
		if _, err := stmt.ExecContext(ctx, ticker, p.ts, p.close, h[:]); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// loadSecurities returns all active security entity_ids from entity_ref.
func (f *YahooPriceFetcher) loadSecurities(ctx context.Context) ([]string, error) {
	rows, err := f.db.QueryContext(ctx, `
		SELECT entity_id FROM entity_ref
		WHERE entity_type = 'security' AND valid_to IS NULL
		ORDER BY entity_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// priceMetric holds a computed derived metric value.
type priceMetric struct {
	key   string
	value float64
}

// pmean returns the arithmetic mean of a float64 slice.
func pmean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// loadPriceHistory returns the last n price_close values for ticker, oldest first.
func (f *YahooPriceFetcher) loadPriceHistory(ctx context.Context, ticker string, n int) ([]float64, error) {
	rows, err := f.db.QueryContext(ctx, `
		SELECT value FROM metric_value
		WHERE metric_key = 'price_close' AND entity = $1 AND value IS NOT NULL
		ORDER BY ts DESC
		LIMIT $2`, ticker, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prices []float64
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		prices = append(prices, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Reverse to oldest-first order.
	for i, j := 0, len(prices)-1; i < j; i, j = i+1, j-1 {
		prices[i], prices[j] = prices[j], prices[i]
	}
	return prices, nil
}

// computeRSI computes a simple RSI from a price slice (len >= 2).
func computeRSI(prices []float64) float64 {
	n := len(prices)
	if n < 2 {
		return 50
	}
	gains, losses := 0.0, 0.0
	for i := 1; i < n; i++ {
		d := prices[i] - prices[i-1]
		if d > 0 {
			gains += d
		} else {
			losses -= d
		}
	}
	avgGain := gains / float64(n-1)
	avgLoss := losses / float64(n-1)
	if avgLoss == 0 {
		return 100
	}
	return 100 - 100/(1+avgGain/avgLoss)
}

// computeAndStoreDerivedMetrics reads recent price history for ticker and
// persists proxy values for the 10 analytic metrics into metric_value (flag=approx).
// These are price-based approximations used until real data sources are wired.
func (f *YahooPriceFetcher) computeAndStoreDerivedMetrics(ctx context.Context, ticker string) error {
	prices, err := f.loadPriceHistory(ctx, ticker, 60)
	if err != nil {
		return fmt.Errorf("load price history: %w", err)
	}
	if len(prices) < 2 {
		return nil
	}
	n := len(prices)
	latest := prices[n-1]

	var derived []priceMetric

	// velocity_accumulation: 5-day momentum normalized 0–100.
	// High = strong positive momentum; 50 = neutral.
	if n >= 6 {
		ret5 := (latest - prices[n-6]) / prices[n-6] * 100
		norm := math.Max(0, math.Min(100, ret5/20*50+50))
		derived = append(derived, priceMetric{"velocity_accumulation", norm})
	}

	// herding_index: RSI(14) as crowd-direction proxy.
	if n >= 15 {
		rsi := computeRSI(prices[n-15:])
		derived = append(derived, priceMetric{"herding_index", rsi})
	}

	// basis_spread: absolute % deviation from MA20 (spread between spot and avg).
	if n >= 20 {
		ma20 := pmean(prices[n-20:])
		spread := (latest - ma20) / ma20 * 100
		derived = append(derived, priceMetric{"basis_spread", math.Abs(spread)})
	}

	// float_demand_ratio: 21-day return mapped to a demand ratio around 1.0.
	// 0% return → 1.0; +10% → 1.5; -10% → 0.5.
	if n >= 22 {
		ret21 := (latest - prices[n-22]) / prices[n-22] * 100
		ratio := math.Max(0, 1.0+ret21/20)
		derived = append(derived, priceMetric{"float_demand_ratio", ratio})
	}

	// property_equity_ratio: signed % from MA50 (long-term valuation spread).
	if n >= 50 {
		ma50 := pmean(prices[n-50:])
		derived = append(derived, priceMetric{"property_equity_ratio", (latest - ma50) / ma50 * 100})
	}

	// foreign_accum_velocity: 21-day linear-regression slope as daily % (trend strength).
	if n >= 21 {
		w21 := prices[n-21:]
		xm := 10.0
		ym := pmean(w21)
		sxy, sxx := 0.0, 0.0
		for i, p := range w21 {
			xi := float64(i) - xm
			sxy += xi * (p - ym)
			sxx += xi * xi
		}
		derived = append(derived, priceMetric{"foreign_accum_velocity", sxy / sxx / latest * 100})
	}

	// mandate_expansion: 21-day drawup from trough (upside extension).
	if n >= 21 {
		low21 := prices[n-21]
		for _, p := range prices[n-21:] {
			if p < low21 {
				low21 = p
			}
		}
		derived = append(derived, priceMetric{"mandate_expansion", (latest - low21) / low21 * 100})
	}

	// nimvi: composite 0–100 index (40% momentum, 30% RSI, 30% trend).
	ret5 := 0.0
	if n >= 6 {
		ret5 = (latest - prices[n-6]) / prices[n-6] * 100
	}
	rsi14 := 50.0
	if n >= 15 {
		rsi14 = computeRSI(prices[n-15:])
	}
	var slopeComp float64
	if n >= 21 {
		w21 := prices[n-21:]
		xm, ym := 10.0, pmean(w21)
		sxy, sxx := 0.0, 0.0
		for i, p := range w21 {
			xi := float64(i) - xm
			sxy += xi * (p - ym)
			sxx += xi * xi
		}
		slopeComp = sxy / sxx / latest * 100
	}
	nimvi := math.Max(0, math.Min(100,
		0.4*(ret5/20*50+50)+0.3*rsi14+0.3*(slopeComp/2*50+50)))
	derived = append(derived, priceMetric{"nimvi", nimvi})

	// kap_notification: abs(1-day return) scaled as event intensity 0–100.
	if n >= 2 {
		absRet := math.Abs((latest - prices[n-2]) / prices[n-2] * 100)
		derived = append(derived, priceMetric{"kap_notification", math.Min(100, absRet/5*100)})
	}

	// real_yield_divergence: z-score of current price vs 20-day distribution.
	if n >= 20 {
		ma := pmean(prices[n-20:])
		variance := 0.0
		for _, p := range prices[n-20:] {
			d := p - ma
			variance += d * d
		}
		if stddev := math.Sqrt(variance / 20); stddev > 0 {
			derived = append(derived, priceMetric{"real_yield_divergence", (latest - ma) / stddev})
		}
	}

	return f.storeDerivedBatch(ctx, ticker, derived)
}

// storeDerivedBatch upserts a batch of derived metric values for ticker.
func (f *YahooPriceFetcher) storeDerivedBatch(ctx context.Context, ticker string, metrics []priceMetric) error {
	if len(metrics) == 0 {
		return nil
	}
	ts := time.Now().UTC().Truncate(24 * time.Hour)

	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO metric_value
			(metric_key, entity, ts, value, flag, tier, inputs_hash, computed_at)
		VALUES
			($1, $2, $3, $4, 'approx'::confidence, 'daily'::cadence_tier, $5, NOW())
		ON CONFLICT (metric_key, entity, ts, inputs_hash) DO NOTHING`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range metrics {
		raw := fmt.Sprintf("%s|%s|%d|%.6f", m.key, ticker, ts.Unix(), m.value)
		h := sha256.Sum256([]byte(raw))
		if _, err := stmt.ExecContext(ctx, m.key, ticker, ts, m.value, h[:]); err != nil {
			return err
		}
	}
	return tx.Commit()
}
