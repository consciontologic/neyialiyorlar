package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestCadenceValid tests cadence validation.
func TestCadenceValid(t *testing.T) {
	tests := []struct {
		cadence Cadence
		valid   bool
	}{
		{Event, true},
		{Intraday, true},
		{Daily, true},
		{Weekly, true},
		{Cadence("unknown"), false},
		{Cadence(""), false},
	}

	for _, tt := range tests {
		if got := tt.cadence.IsValid(); got != tt.valid {
			t.Errorf("cadence %q: expected valid=%v, got %v", tt.cadence, tt.valid, got)
		}
	}
}

// TestSchedulerRegisterCadence tests cadence registration.
func TestSchedulerRegisterCadence(t *testing.T) {
	s := NewScheduler(func(ctx context.Context, cadence Cadence) error {
		return nil
	})

	// Register valid cadence
	if err := s.RegisterCadence(Daily, 24*time.Hour); err != nil {
		t.Fatalf("register daily: %v", err)
	}

	// Register another cadence
	if err := s.RegisterCadence(Intraday, 5*time.Minute); err != nil {
		t.Fatalf("register intraday: %v", err)
	}

	// Try to register invalid cadence
	if err := s.RegisterCadence(Cadence("invalid"), time.Hour); err == nil {
		t.Errorf("expected error for invalid cadence, got none")
	}

	// Try to register duplicate
	if err := s.RegisterCadence(Daily, 24*time.Hour); err == nil {
		t.Errorf("expected error for duplicate cadence, got none")
	}

	// Try to register with invalid interval
	if err := s.RegisterCadence(Weekly, 0); err == nil {
		t.Errorf("expected error for zero interval, got none")
	}
}

// TestSchedulerTicking tests that the scheduler ticks on the correct intervals.
func TestSchedulerTicking(t *testing.T) {
	// Track how many times each cadence was ticked
	var tickMutex sync.Mutex
	tickTimes := make(map[Cadence][]time.Time)

	tickFunc := func(ctx context.Context, cadence Cadence) error {
		tickMutex.Lock()
		tickTimes[cadence] = append(tickTimes[cadence], time.Now())
		tickMutex.Unlock()
		return nil
	}

	s := NewScheduler(tickFunc)

	// Register fast cadences for testing
	if err := s.RegisterCadence(Intraday, 100*time.Millisecond); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.RegisterCadence(Daily, 200*time.Millisecond); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Start scheduler
	if err := s.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Wait for ticks
	time.Sleep(500 * time.Millisecond)

	// Stop scheduler
	if err := s.Stop(2 * time.Second); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Verify ticks occurred
	tickMutex.Lock()
	intradayTicks := len(tickTimes[Intraday])
	dailyTicks := len(tickTimes[Daily])
	tickMutex.Unlock()

	if intradayTicks < 3 {
		t.Errorf("intraday: expected at least 3 ticks, got %d", intradayTicks)
	}
	if dailyTicks < 2 {
		t.Errorf("daily: expected at least 2 ticks, got %d", dailyTicks)
	}

	// Intraday should tick more often than daily
	if intradayTicks <= dailyTicks {
		t.Errorf("intraday should tick more often: intraday=%d, daily=%d", intradayTicks, dailyTicks)
	}
}

// TestSchedulerGetState tests state reporting.
func TestSchedulerGetState(t *testing.T) {
	tickCount := atomic.Int32{}
	tickFunc := func(ctx context.Context, cadence Cadence) error {
		tickCount.Add(1)
		return nil
	}

	s := NewScheduler(tickFunc)
	if err := s.RegisterCadence(Daily, 100*time.Millisecond); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Before start, state should be registered but not running
	state := s.GetCadenceState(Daily)
	if state == nil {
		t.Fatalf("expected state, got nil")
	}
	if state.IsRunning {
		t.Errorf("before start: IsRunning should be false")
	}

	if err := s.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Let it tick a few times
	time.Sleep(350 * time.Millisecond)

	// Check state
	state = s.GetCadenceState(Daily)
	if state == nil {
		t.Fatalf("expected state, got nil")
	}
	if !state.IsRunning {
		t.Errorf("after start: IsRunning should be true")
	}
	if state.TickCount < 2 {
		t.Errorf("expected at least 2 ticks, got %d", state.TickCount)
	}

	if err := s.Stop(2 * time.Second); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// After stop, IsRunning should be false
	state = s.GetCadenceState(Daily)
	if state.IsRunning {
		t.Errorf("after stop: IsRunning should be false")
	}
}

// TestSchedulerErrorTracking tests that errors are tracked.
func TestSchedulerErrorTracking(t *testing.T) {
	tickCount := atomic.Int32{}
	tickFunc := func(ctx context.Context, cadence Cadence) error {
		tickCount.Add(1)
		// Fail on even ticks
		if tickCount.Load()%2 == 0 {
			return fmt.Errorf("simulated error")
		}
		return nil
	}

	s := NewScheduler(tickFunc)
	if err := s.RegisterCadence(Intraday, 50*time.Millisecond); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := s.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Wait long enough to guarantee at least one even tick occurs and is the last tick
	// Scheduler sleeps 100ms between tick checks, so wait until we're sure we have an even tick at the end
	// With 50ms interval and 100ms sleep, ticks occur at ~0, ~100, ~200, ~300, ~400ms
	// Waiting 450ms ensures we get at least 4 ticks, with tick 4 being even (and the last)
	time.Sleep(450 * time.Millisecond)

	if err := s.Stop(2 * time.Second); err != nil {
		t.Fatalf("stop: %v", err)
	}

	state := s.GetCadenceState(Intraday)
	if state.TickCount < 3 {
		t.Fatalf("expected at least 3 ticks, got %d", state.TickCount)
	}
	if state.ErrorCount == 0 {
		t.Errorf("expected error count > 0, got %d; tickCount=%d", state.ErrorCount, state.TickCount)
	}
	if state.LastTickError == nil && state.TickCount%2 == 0 {
		// If TickCount is even, LastTickError should be set
		t.Errorf("expected LastTickError to be set (TickCount=%d), got nil", state.TickCount)
	}
}

// TestSchedulerStopDoesntRestart tests that stopped scheduler can't be restarted.
func TestSchedulerStopDoesntRestart(t *testing.T) {
	s := NewScheduler(func(ctx context.Context, cadence Cadence) error {
		return nil
	})

	if err := s.RegisterCadence(Daily, time.Hour); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := s.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := s.Stop(2 * time.Second); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Try to start again
	if err := s.Start(); err == nil {
		t.Errorf("expected error when restarting stopped scheduler")
	}
}
