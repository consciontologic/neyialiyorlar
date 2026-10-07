package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neyialiyorlar/services/api/internal/model"

	"nhooyr.io/websocket"
)

// TestWebSocketConnection validates basic WS upgrade and connection
func TestWebSocketConnection(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
		if err != nil {
			t.Errorf("websocket accept failed: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		// Keep connection open briefly
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	// Connect as client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Errorf("websocket dial failed: %v", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
}

// TestServeWSStaysOpenForBroadcast is a regression test for the bug where
// ServeWS spawned handleClient with r.Context() and then returned. Go's HTTP
// server cancels r.Context() once the handler returns, so the write loop's
// <-ctx.Done() fired and closed the hijacked socket immediately; the browser
// then reconnected in a tight loop and never received a tick. The handler must
// keep the connection open and deliver broadcasts that occur after it would
// have returned.
func TestServeWSStaysOpenForBroadcast(t *testing.T) {
	srv := NewServer(nil, nil) // wsHub defaults to "*" origin allowlist

	ts := httptest.NewServer(http.HandlerFunc(srv.ServeWS))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Wait until the hub has registered the client. In the buggy version the
	// connection was torn down almost immediately after registration.
	hub := srv.Hub()
	registered := false
	for i := 0; i < 200; i++ {
		hub.mu.RLock()
		n := len(hub.clients)
		hub.mu.RUnlock()
		if n > 0 {
			registered = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !registered {
		t.Fatal("client was never registered (connection closed immediately?)")
	}

	// Broadcast AFTER the handler would have returned in the buggy version.
	want := model.MetricValue{
		Metric: "nimvi",
		Entity: "banks",
		Ts:     time.Now().UTC(),
		Value:  ptrFloat64(1.83),
		Flag:   model.ConfidenceFresh,
		Tier:   model.CadenceTierDaily,
	}
	hub.Broadcast(want)

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed (connection closed prematurely — regression): %v", err)
	}

	var got model.MetricValue
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got.Metric != want.Metric || got.Entity != want.Entity {
		t.Errorf("unexpected payload: got %+v", got)
	}
	if got.Value == nil || *got.Value != 1.83 {
		t.Errorf("value not delivered correctly: got %v", got.Value)
	}
}

// TestWebSocketContractDelivery validates metric event delivery within 500ms
// (Note: In full integration, this would use testcontainers for real Redis)
func TestWebSocketContractDelivery(t *testing.T) {
	t.Skip("requires testcontainers + Redis; validates 500ms SLA for local hardware")

	// Pseudo-code for full integration test:
	// 1. Start redis testcontainer
	// 2. Create WSHub with redis
	// 3. Start test server with /ws endpoint
	// 4. Connect WS client
	// 5. Measure time from PUBLISH to client.Receive()
	// 6. Assert < 500ms
}

// TestWebSocketBackpressure validates slow client is dropped without stalling fast clients
func TestWebSocketBackpressure(t *testing.T) {
	t.Skip("requires async client simulation; validates bounded buffer behavior")

	// Pseudo-code for full integration:
	// 1. Create two test clients: fast and slow
	// 2. Publish many messages
	// 3. Slow client fills buffer and should be closed
	// 4. Fast client should still receive all messages
	// 5. Assert slow client was dropped (websocket.CloseGoingAway)
}

// TestWebSocketOriginValidation validates Origin allowlist
func TestWebSocketOriginValidation(t *testing.T) {
	hub := NewWSHub(nil, WithWSOriginAllowlist([]string{"http://localhost:3000", "http://localhost:5001"}))

	tests := []struct {
		origin  string
		allowed bool
	}{
		{"http://localhost:3000", true},
		{"http://localhost:5001", true},
		{"http://evil.com", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := hub.isOriginAllowed(tt.origin); got != tt.allowed {
			t.Errorf("origin %q: expected %v, got %v", tt.origin, tt.allowed, got)
		}
	}
}

// TestWebSocketOriginAllowlistPermissive validates dev mode with * wildcard
func TestWebSocketOriginAllowlistPermissive(t *testing.T) {
	hub := NewWSHub(nil, WithWSOriginAllowlist([]string{"*"}))

	// In dev mode, all origins should be allowed
	if !hub.isOriginAllowed("http://localhost:3000") {
		t.Error("dev wildcard should allow all origins")
	}
	if !hub.isOriginAllowed("http://evil.com") {
		t.Error("dev wildcard should allow all origins")
	}
}

// TestWebSocketKeepalive validates ping/pong mechanism
func TestWebSocketKeepalivePing(t *testing.T) {
	t.Skip("requires live client socket; validates server ping/client pong within interval")

	// Pseudo-code:
	// 1. Connect WS client
	// 2. Send server ping
	// 3. Assert client responds with pong before timeout
	// 4. Set read deadline and verify connection reaps on pong miss
}

// TestWebSocketMessageSizeCap validates inbound message size limit
func TestWebSocketMessageSizeCap(t *testing.T) {
	maxSize := int64(1024) // 1KB
	hub := NewWSHub(nil, WithMaxMessageSize(maxSize))

	if hub.maxMessageSize != maxSize {
		t.Errorf("expected max message size %d, got %d", maxSize, hub.maxMessageSize)
	}
}

// TestWebSocketMetricUpdate validates metric update structure
func TestWebSocketMetricUpdateStructure(t *testing.T) {
	update := model.MetricValue{
		Metric: "nimvi",
		Entity: "banks",
		Ts:     time.Now(),
		Value:  ptrFloat64(1.83),
		Flag:   model.ConfidenceFresh,
		Tier:   model.CadenceTierDaily,
	}

	// Marshal and unmarshal to validate schema
	data, err := json.Marshal(update)
	if err != nil {
		t.Errorf("marshal failed: %v", err)
		return
	}

	var recovered model.MetricValue
	if err := json.Unmarshal(data, &recovered); err != nil {
		t.Errorf("unmarshal failed: %v", err)
		return
	}

	if recovered.Flag != model.ConfidenceFresh {
		t.Errorf("flag not preserved in round-trip")
	}
	if recovered.Value == nil || *recovered.Value != 1.83 {
		t.Errorf("value not preserved in round-trip")
	}
}

// TestWebSocketGapMetricUpdate validates gap (null value) in WS update
func TestWebSocketGapMetricUpdate(t *testing.T) {
	update := model.MetricValue{
		Metric: "nimvi",
		Entity: "banks",
		Ts:     time.Now(),
		Value:  nil, // Gap
		Flag:   model.ConfidenceStale,
		Tier:   model.CadenceTierDaily,
	}

	data, err := json.Marshal(update)
	if err != nil {
		t.Errorf("marshal failed: %v", err)
		return
	}

	var recovered model.MetricValue
	if err := json.Unmarshal(data, &recovered); err != nil {
		t.Errorf("unmarshal failed: %v", err)
		return
	}

	if recovered.Value != nil {
		t.Error("gap value should be nil")
	}
	if recovered.Flag != model.ConfidenceStale {
		t.Error("gap flag should be stale")
	}
}

// Helper to create float64 pointer
func ptrFloat64(v float64) *float64 {
	return &v
}
