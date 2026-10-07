package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/neyialiyorlar/services/api/internal/model"
)

// GetEntitySignal handles GET /api/v1/entities/{id}/signal.
//
// Computes a simple momentum signal from the last 30 daily closing prices
// (metric_key='price_close') stored in metric_value:
//
//   - 5-day simple moving average (MA5) vs 20-day SMA (MA20)
//   - BUY  when MA5 crosses above MA20 (golden cross) and current price > MA20
//   - SELL when MA5 crosses below MA20 (death cross) and current price < MA20
//   - HOLD otherwise
//
// Returns INSUFFICIENT_DATA when fewer than 20 closing prices are available.
// Confidence is 'fresh' when the latest price is ≤1 trading day old, else 'stale'.
func (s *Server) GetEntitySignal(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("id")
	if entityID == "" {
		http.Error(w, `{"error":"entity id required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	sig, err := s.computeSignal(ctx, entityID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(model.ErrorResponse{
			Error:     "signal computation failed: " + err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=300")
	json.NewEncoder(w).Encode(sig)
}

// computeSignal fetches price_close series from metric_value and returns a signal.
func (s *Server) computeSignal(ctx context.Context, entityID string) (model.SignalResponse, error) {
	insufficient := model.SignalResponse{
		EntityID:   entityID,
		Signal:     model.SignalInsufficient,
		Confidence: model.ConfidenceStale,
		Reason:     "fewer than 20 daily price observations available",
		ComputedAt: time.Now().UTC(),
	}

	if s.db == nil {
		return insufficient, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT ts, value, flag
		FROM metric_value
		WHERE metric_key = 'price_close'
		  AND entity = $1
		  AND value IS NOT NULL
		ORDER BY ts DESC
		LIMIT 30`, entityID)
	if err != nil {
		return insufficient, err
	}
	defer rows.Close()

	type obs struct {
		ts    time.Time
		value float64
		flag  string
	}
	var data []obs
	for rows.Next() {
		var o obs
		if err := rows.Scan(&o.ts, &o.value, &o.flag); err != nil {
			return insufficient, err
		}
		data = append(data, o)
	}
	if err := rows.Err(); err != nil {
		return insufficient, err
	}

	if len(data) < 20 {
		return insufficient, nil
	}

	// data[0] is most recent; reverse for chronological order
	prices := make([]float64, len(data))
	for i, d := range data {
		prices[len(data)-1-i] = d.value
	}

	n := len(prices)
	current := prices[n-1]

	ma5 := sma(prices[n-5 : n])
	ma20 := sma(prices[n-20 : n])

	// Previous-period MAs (shift back one) to detect crossover
	var ma5prev, ma20prev float64
	if n >= 21 {
		ma5prev = sma(prices[n-6 : n-1])
		ma20prev = sma(prices[n-21 : n-1])
	} else {
		ma5prev = ma5
		ma20prev = ma20
	}

	var signal model.Signal
	var reason string

	switch {
	case ma5 > ma20 && ma5prev <= ma20prev:
		// Golden cross: 5-day just crossed above 20-day
		signal = model.SignalBuy
		reason = fmt.Sprintf("altın çapraz: MA5 (%.2f) > MA20 (%.2f), fiyat: %.2f", ma5, ma20, current)
	case ma5 < ma20 && ma5prev >= ma20prev:
		// Death cross: 5-day just crossed below 20-day
		signal = model.SignalSell
		reason = fmt.Sprintf("ölüm çaprazı: MA5 (%.2f) < MA20 (%.2f), fiyat: %.2f", ma5, ma20, current)
	case ma5 > ma20 && current > ma20:
		signal = model.SignalBuy
		reason = fmt.Sprintf("yükseliş trendi: MA5 (%.2f) > MA20 (%.2f), fiyat: %.2f", ma5, ma20, current)
	case ma5 < ma20 && current < ma20:
		signal = model.SignalSell
		reason = fmt.Sprintf("düşüş trendi: MA5 (%.2f) < MA20 (%.2f), fiyat: %.2f", ma5, ma20, current)
	default:
		signal = model.SignalHold
		reason = fmt.Sprintf("nötr: MA5 (%.2f), MA20 (%.2f), fiyat: %.2f", ma5, ma20, current)
	}

	// Confidence: fresh if latest price is ≤2 calendar days old
	confidence := model.ConfidenceStale
	if time.Since(data[0].ts) <= 48*time.Hour {
		confidence = model.ConfidenceFresh
	}

	score := (ma5 - ma20) / ma20 * 100 // % deviation of MA5 from MA20

	return model.SignalResponse{
		EntityID:   entityID,
		Signal:     signal,
		Confidence: confidence,
		Score:      roundAt(score, 4),
		MA5:        roundAt(ma5, 2),
		MA20:       roundAt(ma20, 2),
		Price:      roundAt(current, 2),
		Reason:     reason,
		ComputedAt: time.Now().UTC(),
	}, nil
}

// sma computes simple moving average of a slice.
func sma(prices []float64) float64 {
	if len(prices) == 0 {
		return 0
	}
	sum := 0.0
	for _, p := range prices {
		sum += p
	}
	return sum / float64(len(prices))
}

func roundAt(v float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	return math.Round(v*p) / p
}
