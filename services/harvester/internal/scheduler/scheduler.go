package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Cadence represents a tick frequency for ingestion.
// Rationale (mandate 15): multiple observation frequencies require
// separate scheduling logic so intraday polling doesn't starve daily jobs.
type Cadence string

const (
	Event    Cadence = "event"      // Real-time events (push or rapid polling)
	Intraday Cadence = "intraday"   // Multiple times per trading day
	Daily    Cadence = "daily"      // Once per trading day
	Weekly   Cadence = "weekly"     // Once per week
)

// IsValid checks if a cadence is known.
func (c Cadence) IsValid() bool {
	switch c {
	case Event, Intraday, Daily, Weekly:
		return true
	default:
		return false
	}
}

// TickFunc is the callback invoked on each tick.
type TickFunc func(ctx context.Context, cadence Cadence) error

// Scheduler manages multi-cadence ticking for data ingestion.
// Rationale: sources have different update patterns; BIST intraday data
// updates every 5 minutes during market hours, daily closes once per day,
// weekly data once per week. A single scheduler loop can't honor all
// constraints efficiently. Instead, we spawn a goroutine per cadence
// and let them tick independently, coordinating via a context.
type Scheduler struct {
	mu            sync.RWMutex
	cadences      map[Cadence]*CadenceState
	tickFunc      TickFunc
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	stopped       bool
}

// CadenceState tracks the state of a single cadence ticker.
type CadenceState struct {
	Cadence        Cadence
	Interval       time.Duration
	NextTick       time.Time
	LastTickTime   time.Time
	LastTickError  error
	TickCount      int64
	ErrorCount     int64
	IsRunning      bool
}

// NewScheduler creates a scheduler with the given tick callback.
func NewScheduler(tickFunc TickFunc) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		cadences: make(map[Cadence]*CadenceState),
		tickFunc: tickFunc,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// RegisterCadence registers a cadence with a tick interval.
// Must be called before Start().
func (s *Scheduler) RegisterCadence(cadence Cadence, interval time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !cadence.IsValid() {
		return fmt.Errorf("invalid cadence: %s", cadence)
	}

	if interval <= 0 {
		return fmt.Errorf("interval must be positive, got %v", interval)
	}

	if _, exists := s.cadences[cadence]; exists {
		return fmt.Errorf("cadence already registered: %s", cadence)
	}

	s.cadences[cadence] = &CadenceState{
		Cadence:  cadence,
		Interval: interval,
		NextTick: time.Now(),
	}

	return nil
}

// Start begins the scheduler. Must be called once; subsequent calls are no-ops.
func (s *Scheduler) Start() error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already stopped; cannot restart")
	}

	cadences := make([]*CadenceState, 0, len(s.cadences))
	for _, state := range s.cadences {
		cadences = append(cadences, state)
	}
	s.mu.Unlock()

	if len(cadences) == 0 {
		return fmt.Errorf("no cadences registered; nothing to schedule")
	}

	// Spawn a goroutine per cadence
	for _, state := range cadences {
		s.wg.Add(1)
		go s.runCadence(state)
	}

	return nil
}

// runCadence runs the tick loop for a single cadence.
func (s *Scheduler) runCadence(state *CadenceState) {
	defer s.wg.Done()

	s.mu.Lock()
	state.IsRunning = true
	s.mu.Unlock()

	for {
		s.mu.RLock()
		now := time.Now()
		shouldTick := now.After(state.NextTick) || now.Equal(state.NextTick)
		s.mu.RUnlock()

		if shouldTick {
			// Perform the tick
			tickCtx, cancel := context.WithTimeout(s.ctx, state.Interval)
			err := s.tickFunc(tickCtx, state.Cadence)
			cancel()

			// Record the tick
			s.mu.Lock()
			state.LastTickTime = time.Now()
			state.LastTickError = err
			state.TickCount++
			if err != nil {
				state.ErrorCount++
			}
			state.NextTick = time.Now().Add(state.Interval)
			s.mu.Unlock()
		}

		// Sleep briefly to avoid tight-looping
		select {
		case <-s.ctx.Done():
			s.mu.Lock()
			state.IsRunning = false
			s.mu.Unlock()
			return
		case <-time.After(100 * time.Millisecond):
			// Continue
		}
	}
}

// Stop gracefully stops the scheduler and waits for all cadences to finish.
// Rationale (mandate 16): graceful shutdown ensures in-flight ticks complete.
func (s *Scheduler) Stop(timeout time.Duration) error {
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()

	s.cancel() // Signal all cadence goroutines to exit

	done := make(chan error)
	go func() {
		s.wg.Wait()
		done <- nil
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("scheduler stop timeout after %v", timeout)
	}
}

// GetState returns a snapshot of the current state of all cadences.
func (s *Scheduler) GetState() map[Cadence]CadenceState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[Cadence]CadenceState)
	for cadence, state := range s.cadences {
		result[cadence] = *state
	}
	return result
}

// GetCadenceState returns the state of a specific cadence, or nil if not registered.
func (s *Scheduler) GetCadenceState(cadence Cadence) *CadenceState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.cadences[cadence]
	if !ok {
		return nil
	}

	// Return a copy to avoid races
	copy := *state
	return &copy
}
