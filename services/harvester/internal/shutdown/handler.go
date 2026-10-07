package shutdown

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Handler manages graceful shutdown of the harvester.
// On SIGTERM: drain in-flight fetches, flush state, exit cleanly within timeout.
type Handler struct {
	timeout     time.Duration
	logger      *slog.Logger
	drainFunc   func(ctx context.Context) error // user-provided drain callback
	flushFunc   func(ctx context.Context) error // user-provided flush callback
	shutdownMu  sync.Mutex
	shutdownSig chan struct{}
	isShutdown  bool
}

// NewHandler creates a shutdown handler with configured timeout.
func NewHandler(timeout time.Duration, logger *slog.Logger) *Handler {
	return &Handler{
		timeout:     timeout,
		logger:      logger,
		shutdownSig: make(chan struct{}),
	}
}

// SetDrainFunc sets the callback to drain in-flight operations.
func (h *Handler) SetDrainFunc(fn func(ctx context.Context) error) {
	h.drainFunc = fn
}

// SetFlushFunc sets the callback to flush state (e.g., circuit breaker state).
func (h *Handler) SetFlushFunc(fn func(ctx context.Context) error) {
	h.flushFunc = fn
}

// Start begins listening for SIGTERM/SIGINT and initiates graceful shutdown.
func (h *Handler) Start() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigChan
		h.logger.Info("shutdown signal received", slog.String("signal", sig.String()))

		h.shutdownMu.Lock()
		if h.isShutdown {
			h.shutdownMu.Unlock()
			return
		}
		h.isShutdown = true
		h.shutdownMu.Unlock()

		close(h.shutdownSig)

		// Execute graceful shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
		defer cancel()

		h.performShutdown(ctx)
	}()
}

// IsShuttingDown returns whether shutdown has been initiated.
func (h *Handler) IsShuttingDown() bool {
	select {
	case <-h.shutdownSig:
		return true
	default:
		return false
	}
}

// Wait blocks until shutdown is initiated.
func (h *Handler) Wait() {
	<-h.shutdownSig
}

// performShutdown executes the graceful shutdown sequence.
func (h *Handler) performShutdown(ctx context.Context) {
	start := time.Now()

	h.logger.Info("graceful shutdown started", slog.Duration("timeout", h.timeout))

	// Step 1: Drain in-flight fetches
	if h.drainFunc != nil {
		h.logger.Info("draining in-flight fetches...")
		if err := h.drainFunc(ctx); err != nil {
			h.logger.Warn("drain error (continuing)", slog.String("error", err.Error()))
		}
	}

	remaining := h.timeout - time.Since(start)
	if remaining <= 0 {
		h.logger.Warn("shutdown timeout exceeded during drain")
		os.Exit(1)
	}

	// Step 2: Flush state (circuit breaker, health, etc.)
	if h.flushFunc != nil {
		flushCtx, cancel := context.WithTimeout(ctx, remaining)
		h.logger.Info("flushing state to database...")
		if err := h.flushFunc(flushCtx); err != nil {
			h.logger.Warn("flush error (continuing)", slog.String("error", err.Error()))
		}
		cancel()
	}

	elapsed := time.Since(start)
	h.logger.Info("graceful shutdown completed", slog.Duration("elapsed", elapsed))
	os.Exit(0)
}

// Shutdown triggers an immediate graceful shutdown (useful for testing).
func (h *Handler) Shutdown() error {
	h.shutdownMu.Lock()
	if h.isShutdown {
		h.shutdownMu.Unlock()
		return fmt.Errorf("already shutdown")
	}
	h.isShutdown = true
	h.shutdownMu.Unlock()

	close(h.shutdownSig)

	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	h.performShutdown(ctx)
	return nil
}
