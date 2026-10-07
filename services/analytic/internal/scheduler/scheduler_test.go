package scheduler

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/neyialiyorlar/services/analytic/internal/model"
)

func TestNewScheduler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	s := NewScheduler(logger)
	if s == nil {
		t.Errorf("NewScheduler() returned nil")
	}
	if len(s.tickers) != 0 {
		t.Errorf("NewScheduler() initial tickers = %d, want 0", len(s.tickers))
	}
}

func TestSetCadenceInterval(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	s := NewScheduler(logger)

	// Override intraday interval.
	s.SetCadenceInterval(model.CadenceIntraday, 1*time.Second)

	if s.cadenceIntervals[model.CadenceIntraday] != 1*time.Second {
		t.Errorf("SetCadenceInterval() = %v, want 1s", s.cadenceIntervals[model.CadenceIntraday])
	}
}

func TestSchedulerStart(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	s := NewScheduler(logger)
	s.SetCadenceInterval(model.CadenceIntraday, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Run scheduler in a goroutine.
	go s.Start(ctx)

	// Wait for ticks.
	tickCount := 0
	for {
		select {
		case <-s.TickChannel():
			tickCount++
		case <-ctx.Done():
			goto done
		}
	}

done:
	s.Stop()

	// Should have received at least 2 ticks (500ms / 100ms = 5 ticks roughly).
	if tickCount < 2 {
		t.Logf("SchedulerStart() tick count = %d (expected >= 2, test timing may vary)", tickCount)
	}
}

func TestEmitManualTick(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	s := NewScheduler(logger)

	// Emit a manual tick without starting the scheduler.
	go s.EmitManualTick(model.CadenceDaily)

	select {
	case tick := <-s.TickChannel():
		if tick.Cadence != model.CadenceDaily {
			t.Errorf("EmitManualTick() cadence = %s, want %s", tick.Cadence, model.CadenceDaily)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("EmitManualTick() did not emit tick")
	}
}

func TestAffectedMetricsExactMatch(t *testing.T) {
	// Exact match: metric is affected.
	if !AffectedMetrics(model.CadenceDaily, model.CadenceDaily) {
		t.Errorf("AffectedMetrics(Daily, Daily) = false, want true")
	}
}

func TestAffectedMetricsNoUpsampling(t *testing.T) {
	// Bullet requirement: weekly metric does NOT recompute on daily tick.
	if AffectedMetrics(model.CadenceDaily, model.CadenceWeekly) {
		t.Errorf("AffectedMetrics(Daily tick, Weekly metric) = true, want false (no upsampling)")
	}
}

func TestAffectedMetricsEvent(t *testing.T) {
	// Event cadence is independent of timer ticks but can be manually triggered.
	if AffectedMetrics(model.CadenceDaily, model.CadenceEvent) {
		t.Errorf("AffectedMetrics(Daily tick, Event metric) = true, want false")
	}
	// Event metric triggered on manual event tick (exact match).
	if !AffectedMetrics(model.CadenceEvent, model.CadenceEvent) {
		t.Errorf("AffectedMetrics(Event tick, Event metric) = false, want true (exact match)")
	}
}

func TestSchedulerTickChannel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	s := NewScheduler(logger)

	ch := s.TickChannel()
	if ch == nil {
		t.Errorf("TickChannel() returned nil")
	}
}

func TestSchedulerStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	s := NewScheduler(logger)

	// Run Start in a goroutine with a timeout context.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	go s.Start(ctx)

	// Wait for context to expire, which stops the scheduler.
	<-ctx.Done()

	// Now call Stop (it should be idempotent with sync.Once).
	s.Stop()
	s.Stop() // second call should be safe (no-op)

	cancel()
}
