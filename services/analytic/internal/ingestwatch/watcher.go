package ingestwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"time"
)

// RawEvent represents a raw ingestion event published by the harvester.
type RawEvent struct {
	Source     string `json:"source"`
	NaturalKey string `json:"natural_key"`
	ContentHash string `json:"content_hash"`
}

// EventListener defines the interface for reacting to raw ingestion events.
type EventListener interface {
	OnRawIngested(ctx context.Context, event *RawEvent) error
}

// Watcher subscribes to raw ingestion events and dispatches them to listeners.
// In a real implementation, this would connect to Redis Pub/Sub.
// For now, this is a stub that simulates Redis behavior with an in-memory event queue.
type Watcher struct {
	listeners []EventListener
	eventChan chan *RawEvent
	done      chan struct{}
	logger    *slog.Logger

	// Redis reconnect parameters.
	reconnectBase   time.Duration
	reconnectMax    time.Duration
	reconnectFactor float64
}

// NewWatcher creates a new raw event watcher.
func NewWatcher(logger *slog.Logger) *Watcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Watcher{
		listeners:       make([]EventListener, 0),
		eventChan:       make(chan *RawEvent, 100),
		done:            make(chan struct{}),
		logger:          logger,
		reconnectBase:   500 * time.Millisecond,
		reconnectMax:    60 * time.Second,
		reconnectFactor: 2.0,
	}
}

// Register adds an event listener.
func (w *Watcher) Register(listener EventListener) {
	w.listeners = append(w.listeners, listener)
}

// PublishEvent publishes a raw ingestion event for testing/simulation.
func (w *Watcher) PublishEvent(event *RawEvent) {
	select {
	case w.eventChan <- event:
	case <-w.done:
		w.logger.Warn("Watcher stopped, event discarded")
	}
}

// Start begins listening for raw ingestion events.
// In a real implementation, this would subscribe to Redis "raw.<source>" channels.
// For now, it reads from the in-memory event channel.
func (w *Watcher) Start(ctx context.Context) error {
	for {
		select {
		case <-w.done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case event := <-w.eventChan:
			if event == nil {
				continue
			}

			// Dispatch to all listeners.
			for _, listener := range w.listeners {
				if err := listener.OnRawIngested(ctx, event); err != nil {
					w.logger.Error("listener error", "source", event.Source, "err", err)
				}
			}
		}
	}
}

// Stop gracefully shuts down the watcher.
func (w *Watcher) Stop() {
	close(w.done)
}

// simulateRedisReconnect simulates exponential backoff for Redis reconnection.
// Returns the next backoff duration with jitter.
func (w *Watcher) simulateRedisReconnect(backoff time.Duration) time.Duration {
	// Add jitter: ±10% of backoff.
	jitter := time.Duration(float64(backoff) * (0.1 * (rand.Float64() - 0.5)))
	next := time.Duration(float64(backoff) * w.reconnectFactor)
	if next > w.reconnectMax {
		next = w.reconnectMax
	}
	return next + jitter
}

// ParseRawEvent parses a JSON raw event from a Redis message payload.
func ParseRawEvent(payload []byte) (*RawEvent, error) {
	var event RawEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("parse raw event: %w", err)
	}
	if event.Source == "" {
		return nil, fmt.Errorf("raw event missing source")
	}
	if event.NaturalKey == "" {
		return nil, fmt.Errorf("raw event missing natural_key")
	}
	return &event, nil
}
