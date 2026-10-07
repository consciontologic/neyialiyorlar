package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neyialiyorlar/services/analytic/internal/model"
)

// CadenceTick represents a scheduled tick event.
type CadenceTick struct {
	Cadence   model.Cadence
	Timestamp time.Time
}

// Scheduler manages periodic metric recomputation per tier.
// Cadence tiers: intraday, daily, weekly, event.
type Scheduler struct {
	mu                sync.Mutex
	onceStop          sync.Once
	logger            *slog.Logger
	tickers           map[model.Cadence]*time.Ticker
	cadenceIntervals  map[model.Cadence]time.Duration
	tickChan          chan *CadenceTick
	done              chan struct{}
	stopped           atomic.Bool
	started           bool
}

// NewScheduler creates a new scheduler with default cadence intervals.
func NewScheduler(logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		logger: logger,
		tickers: make(map[model.Cadence]*time.Ticker),
		cadenceIntervals: map[model.Cadence]time.Duration{
			model.CadenceIntraday: 5 * time.Minute,   // intraday tick every 5 minutes
			model.CadenceDaily:    24 * time.Hour,    // daily tick every day
			model.CadenceWeekly:   7 * 24 * time.Hour, // weekly tick every 7 days
			model.CadenceEvent:    0,                  // event cadence is not timer-driven
		},
		tickChan: make(chan *CadenceTick, 100),
		done:     make(chan struct{}),
		started:  false,
	}
}

// SetCadenceInterval overrides the interval for a cadence tier.
func (s *Scheduler) SetCadenceInterval(cadence model.Cadence, interval time.Duration) {
	s.cadenceIntervals[cadence] = interval
}

// Start begins the scheduler; it emits ticks on the channel.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil // idempotent
	}
	s.started = true

	// Create tickers for non-event cadences (while locked).
	for cadence, interval := range s.cadenceIntervals {
		if cadence == model.CadenceEvent {
			continue // event cadence is not timer-driven
		}
		if interval > 0 {
			ticker := time.NewTicker(interval)
			s.tickers[cadence] = ticker
		}
	}

	// Capture tickers to iterate (we'll release the lock before spawning goroutines).
	tickersToStart := make([]struct {
		cadence model.Cadence
		ticker  *time.Ticker
	}, 0, len(s.tickers))
	for cad, ticker := range s.tickers {
		tickersToStart = append(tickersToStart, struct {
			cadence model.Cadence
			ticker  *time.Ticker
		}{cad, ticker})
	}

	s.mu.Unlock()

	// Spawn goroutines (outside the lock).
	for _, item := range tickersToStart {
		go func(cad model.Cadence, t *time.Ticker) {
			defer t.Stop()
			for {
				// Check if stopped via atomic flag (race-safe)
				if s.stopped.Load() {
					return
				}
				select {
				case <-ctx.Done():
					return
				case tick := <-t.C:
					if s.stopped.Load() {
						return
					}
					select {
					case s.tickChan <- &CadenceTick{
						Cadence:   cad,
						Timestamp: tick,
					}:
					case <-ctx.Done():
						return
					}
				}
			}
		}(item.cadence, item.ticker)
	}

	// Main scheduler loop: wait for context cancellation or stop signal.
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop() {
	s.onceStop.Do(func() {
		s.stopped.Store(true)
		s.mu.Lock()
		defer s.mu.Unlock()
		if !s.started {
			return
		}
		s.started = false
		// Stop all tickers (safe to call multiple times per ticker.Stop docs)
		for _, ticker := range s.tickers {
			ticker.Stop()
		}
		// Give goroutines a moment to exit via the stopped flag check
		// Don't close done or tickChan here to avoid race conditions with senders
	})
}

// TickChannel returns the channel on which ticks are emitted.
// Callers should read from this channel to receive cadence tick events.
func (s *Scheduler) TickChannel() <-chan *CadenceTick {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tickChan
}

// EmitManualTick allows manual emission of a tick (for testing or forced recomputation).
func (s *Scheduler) EmitManualTick(cadence model.Cadence) {
	s.mu.Lock()
	if s.stopped.Load() {
		s.mu.Unlock()
		return
	}
	tickChan := s.tickChan
	s.mu.Unlock()

	select {
	case tickChan <- &CadenceTick{
		Cadence:   cadence,
		Timestamp: time.Now(),
	}:
	default:
		// Channel full or scheduler stopped
	}
}

// AffectedMetrics returns which metric cadences match or exceed the given tier.
// For example, if a daily-tier metric exists and intraday tick fires, only intraday metrics recompute.
// A weekly metric does NOT recompute on a daily tick (bullet requirement).
func AffectedMetrics(tickCadence model.Cadence, metricCadence model.Cadence) bool {
	// Exact match: always affected.
	if tickCadence == metricCadence {
		return true
	}
	// No upsampling: a metric is only affected by ticks of its own cadence or lower granularity.
	// Intraday < Daily < Weekly < Event (by recomputation frequency)
	// A weekly metric should NOT recompute on a daily tick.
	return false
}
