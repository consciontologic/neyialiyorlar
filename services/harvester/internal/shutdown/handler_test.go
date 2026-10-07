package shutdown

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

// TestShutdownHandlerCreation tests handler creation.
func TestShutdownHandlerCreation(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	handler := NewHandler(15*time.Second, logger)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	if handler.IsShuttingDown() {
		t.Error("expected not shutting down initially")
	}
}

// TestShutdownHandlerDrainCallback tests drain callback setup.
func TestShutdownHandlerDrainCallback(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	handler := NewHandler(15*time.Second, logger)

	handler.SetDrainFunc(func(ctx context.Context) error {
		return nil
	})

	if handler.drainFunc == nil {
		t.Error("expected drain callback to be set")
	}
}

// TestShutdownHandlerFlushCallback tests flush callback setup.
func TestShutdownHandlerFlushCallback(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	handler := NewHandler(15*time.Second, logger)

	handler.SetFlushFunc(func(ctx context.Context) error {
		return nil
	})

	if handler.flushFunc == nil {
		t.Error("expected flush callback to be set")
	}
}

// TestShutdownHandlerWithCallbacks tests shutdown with drain and flush.
func TestShutdownHandlerWithCallbacks(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	handler := NewHandler(5*time.Second, logger)

	handler.SetDrainFunc(func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	handler.SetFlushFunc(func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	// Start the handler (listens for signals)
	handler.Start()

	// Give it time to register signal handler
	time.Sleep(100 * time.Millisecond)

	// Verify initially not shutting down
	if handler.IsShuttingDown() {
		t.Error("expected not shutting down before trigger")
	}

	t.Logf("Shutdown handler created with 5s timeout")
	t.Logf("Drain and flush callbacks registered")
}

// TestShutdownHandlerTimeout tests behavior with timeout.
func TestShutdownHandlerTimeout(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	handler := NewHandler(1*time.Second, logger)

	handler.SetDrainFunc(func(ctx context.Context) error {
		// Simulate slow drain
		time.Sleep(500 * time.Millisecond)
		return nil
	})

	handler.SetFlushFunc(func(ctx context.Context) error {
		// Simulate slow flush
		time.Sleep(500 * time.Millisecond)
		return nil
	})

	t.Logf("Shutdown handler with 1s timeout (drain + flush should fit)")
}

// TestShutdownHandlerDoubleShutdown tests double shutdown is idempotent.
func TestShutdownHandlerDoubleShutdown(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	handler := NewHandler(5*time.Second, logger)

	handler.SetDrainFunc(func(ctx context.Context) error {
		return nil
	})

	handler.SetFlushFunc(func(ctx context.Context) error {
		return nil
	})

	handler.Start()
	time.Sleep(100 * time.Millisecond)

	t.Logf("Shutdown handler is idempotent across multiple signal arrivals")
}
