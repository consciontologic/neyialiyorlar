package ingestwatch

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

// MockListener is a test listener that records events.
type MockListener struct {
	events []*RawEvent
}

func (m *MockListener) OnRawIngested(ctx context.Context, event *RawEvent) error {
	m.events = append(m.events, event)
	return nil
}

func TestNewWatcher(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewWatcher(logger)
	if w == nil {
		t.Errorf("NewWatcher() returned nil")
	}
	if len(w.listeners) != 0 {
		t.Errorf("NewWatcher() initial listeners = %d, want 0", len(w.listeners))
	}
}

func TestWatcherRegister(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewWatcher(logger)

	listener1 := &MockListener{}
	listener2 := &MockListener{}

	w.Register(listener1)
	w.Register(listener2)

	if len(w.listeners) != 2 {
		t.Errorf("Register() listeners = %d, want 2", len(w.listeners))
	}
}

func TestWatcherPublishEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewWatcher(logger)

	listener := &MockListener{}
	w.Register(listener)

	event := &RawEvent{
		Source:      "bist",
		NaturalKey:  "GARAN",
		ContentHash: "abc123",
	}

	// Publish event in a goroutine to avoid blocking.
	go func() {
		w.PublishEvent(event)
		time.Sleep(10 * time.Millisecond)
		w.Stop()
	}()

	// Start watching.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	w.Start(ctx)

	// Event should have been received.
	if len(listener.events) != 1 {
		t.Errorf("Listener received %d events, want 1", len(listener.events))
	} else if listener.events[0].NaturalKey != "GARAN" {
		t.Errorf("Listener event = %v, want GARAN", listener.events[0].NaturalKey)
	}
}

func TestParseRawEvent(t *testing.T) {
	payload := []byte(`{"source":"kap","natural_key":"ASELS","content_hash":"xyz789"}`)
	event, err := ParseRawEvent(payload)
	if err != nil {
		t.Errorf("ParseRawEvent() returned error: %v", err)
	}
	if event.Source != "kap" {
		t.Errorf("ParseRawEvent() source = %s, want kap", event.Source)
	}
	if event.NaturalKey != "ASELS" {
		t.Errorf("ParseRawEvent() natural_key = %s, want ASELS", event.NaturalKey)
	}
}

func TestParseRawEventMissingSource(t *testing.T) {
	payload := []byte(`{"natural_key":"ASELS","content_hash":"xyz789"}`)
	_, err := ParseRawEvent(payload)
	if err == nil {
		t.Errorf("ParseRawEvent() with missing source should return error")
	}
}

func TestParseRawEventMissingKey(t *testing.T) {
	payload := []byte(`{"source":"kap","content_hash":"xyz789"}`)
	_, err := ParseRawEvent(payload)
	if err == nil {
		t.Errorf("ParseRawEvent() with missing natural_key should return error")
	}
}

func TestParseRawEventInvalidJSON(t *testing.T) {
	payload := []byte(`{invalid json}`)
	_, err := ParseRawEvent(payload)
	if err == nil {
		t.Errorf("ParseRawEvent() with invalid JSON should return error")
	}
}

func TestWatcherMultipleListeners(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewWatcher(logger)

	listener1 := &MockListener{}
	listener2 := &MockListener{}
	listener3 := &MockListener{}

	w.Register(listener1)
	w.Register(listener2)
	w.Register(listener3)

	event := &RawEvent{
		Source:      "mkkvap",
		NaturalKey:  "TCELL",
		ContentHash: "def456",
	}

	go func() {
		w.PublishEvent(event)
		time.Sleep(10 * time.Millisecond)
		w.Stop()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	w.Start(ctx)

	// All listeners should have received the event.
	if len(listener1.events) != 1 || len(listener2.events) != 1 || len(listener3.events) != 1 {
		t.Errorf("Listeners = %d, %d, %d; want 1, 1, 1",
			len(listener1.events), len(listener2.events), len(listener3.events))
	}
}
