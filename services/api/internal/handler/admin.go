package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/neyialiyorlar/services/api/internal/model"
)

// AdminTick handles POST /api/v1/admin/tick.
//
// It triggers one synthetic recompute round (dev "tweak/test" control): a fresh
// value is generated for every metric/entity pair, persisted, cached, and pushed
// to live WebSocket subscribers. Returns the number of values emitted.
func (s *Server) AdminTick(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	if s.ticker == nil {
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(model.ErrorResponse{
			Error:     "synthetic engine not enabled",
			Timestamp: time.Now().UTC(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	now := time.Now().UTC()
	emitted, err := s.ticker.Tick(ctx, now)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(model.ErrorResponse{
			Error:     "tick failed: " + err.Error(),
			Timestamp: time.Now().UTC(),
		})
		return
	}

	json.NewEncoder(w).Encode(model.AdminTickResponse{
		Emitted: emitted,
		Ts:      now,
	})
}

// AdminStatus handles GET /api/v1/admin/status.
//
// It reports a compact operator view of the system: whether the synthetic
// engine is active and live table row counts — the "investigate my system"
// endpoint.
func (s *Server) AdminStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	counts := map[string]int64{}
	if s.counter != nil {
		if c, err := s.counter.Counts(ctx); err == nil {
			counts = c
		}
	}

	json.NewEncoder(w).Encode(model.AdminStatusResponse{
		Service:          "api",
		Time:             time.Now().UTC(),
		SyntheticEnabled: s.ticker != nil,
		MetricsCatalog:   len(s.catalog),
		Counts:           counts,
	})
}
