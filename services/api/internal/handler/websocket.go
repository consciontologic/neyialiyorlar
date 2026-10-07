package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"nhooyr.io/websocket"

	"github.com/neyialiyorlar/services/api/internal/model"
)

// WSHub manages WebSocket connections and Redis subscriptions
type WSHub struct {
	redis             *redis.Client
	clients           map[*wsClient]bool
	mu                sync.RWMutex
	maxMessageSize    int64
	pingInterval      time.Duration
	wsOriginAllowlist []string
	drainTimeout      time.Duration
}

// wsClient represents a connected WebSocket client
type wsClient struct {
	conn       *websocket.Conn
	sendChan   chan interface{}
	closedOnce sync.Once
	closed     chan struct{}
}

// NewWSHub creates a new WebSocket hub
func NewWSHub(rc *redis.Client, opts ...WSOption) *WSHub {
	hub := &WSHub{
		redis:             rc,
		clients:           make(map[*wsClient]bool),
		maxMessageSize:    1024 * 1024,      // 1MB default
		pingInterval:      30 * time.Second, // Default ping interval
		drainTimeout:      5 * time.Second,  // Default drain timeout
		wsOriginAllowlist: []string{"*"},    // Dev: allow all (documented)
	}

	for _, opt := range opts {
		opt(hub)
	}

	return hub
}

// WSOption is a functional option for WSHub
type WSOption func(*WSHub)

// WithMaxMessageSize sets the max WS message size
func WithMaxMessageSize(size int64) WSOption {
	return func(h *WSHub) {
		h.maxMessageSize = size
	}
}

// WithPingInterval sets the ping interval
func WithPingInterval(interval time.Duration) WSOption {
	return func(h *WSHub) {
		h.pingInterval = interval
	}
}

// WithWSOriginAllowlist sets the Origin allowlist
func WithWSOriginAllowlist(origins []string) WSOption {
	return func(h *WSHub) {
		h.wsOriginAllowlist = origins
	}
}

// WithDrainTimeout sets the graceful shutdown drain timeout
func WithDrainTimeout(timeout time.Duration) WSOption {
	return func(h *WSHub) {
		h.drainTimeout = timeout
	}
}

// ServeWS handles WebSocket upgrade and client management
func (s *Server) ServeWS(w http.ResponseWriter, r *http.Request) {
	// Use the shared hub created in NewServer so broadcasts from the data engine
	// reach every connected client.
	hub := s.wsHub

	// Check Origin header (CSWSH prevention)
	origin := r.Header.Get("Origin")
	if !hub.isOriginAllowed(origin) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	// Upgrade connection
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		slog.Error("websocket accept failed", slog.String("error", err.Error()))
		return
	}

	// Set connection limits
	conn.SetReadLimit(hub.maxMessageSize)

	// Create client
	client := &wsClient{
		conn:     conn,
		sendChan: make(chan interface{}, 10), // Bounded buffer
		closed:   make(chan struct{}),
	}

	// Register client
	hub.registerClient(client)

	// Drive the client read/write loops. The hijacked WebSocket connection
	// outlives this HTTP handler, so we must NOT use r.Context(): Go's HTTP
	// server cancels it as soon as ServeWS returns, which would immediately
	// trip the write loop's <-ctx.Done() and close the socket (the client then
	// reconnects in a tight loop and never receives a tick). Use a
	// connection-scoped context derived from Background and block here until the
	// client disconnects; the read loop drives cleanup on disconnect.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hub.handleClient(ctx, client)
}

// registerClient adds a client to the hub
func (h *WSHub) registerClient(client *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client] = true
	slog.Debug("ws client registered", slog.Int("total_clients", len(h.clients)))
}

// unregisterClient removes a client from the hub
func (h *WSHub) unregisterClient(client *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, client)

	client.closedOnce.Do(func() {
		close(client.closed)
		close(client.sendChan)
	})

	slog.Debug("ws client unregistered", slog.Int("total_clients", len(h.clients)))
}

// handleClient runs the read/write loop for a client
func (h *WSHub) handleClient(ctx context.Context, client *wsClient) {
	defer h.unregisterClient(client)

	// Start ping/pong keepalive
	ticker := time.NewTicker(h.pingInterval)
	defer ticker.Stop()

	// Start read loop (in goroutine)
	go func() {
		defer h.unregisterClient(client)

		for {
			_, b, err := client.conn.Read(ctx)
			if err != nil {
				slog.Debug("ws read error", slog.String("error", err.Error()))
				return
			}
			var msg interface{}
			err = json.Unmarshal(b, &msg)
			if err != nil {
				slog.Debug("ws unmarshal error", slog.String("error", err.Error()))
				continue
			}
			// Handle inbound messages (subscriptions, etc.)
			// For now: just log (stub)
			slog.Debug("ws message received", slog.Any("msg", msg))
		}
	}()

	// Write loop
	for {
		select {
		case <-ctx.Done():
			// Request context cancelled
			client.conn.Close(websocket.StatusGoingAway, "context done")
			return

		case <-client.closed:
			// Client closed
			client.conn.Close(websocket.StatusNormalClosure, "")
			return

		case msg := <-client.sendChan:
			// Send queued message to client
			b, err := json.Marshal(msg)
			if err != nil {
				slog.Debug("ws marshal error", slog.String("error", err.Error()))
				continue
			}

			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = client.conn.Write(ctx, websocket.MessageText, b)
			cancel()

			if err != nil {
				slog.Debug("ws write error", slog.String("error", err.Error()))
				return
			}

		case <-ticker.C:
			// Send ping
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := client.conn.Ping(ctx)
			cancel()

			if err != nil {
				slog.Debug("ws ping error", slog.String("error", err.Error()))
				return
			}
		}
	}
}

// isOriginAllowed checks if an Origin is in the allowlist
func (h *WSHub) isOriginAllowed(origin string) bool {
	if len(h.wsOriginAllowlist) == 0 {
		return false
	}

	for _, allowed := range h.wsOriginAllowlist {
		if allowed == "*" || origin == allowed {
			return true
		}
	}

	return false
}

// BroadcastMetricUpdate broadcasts a metric update to all clients
func (h *WSHub) BroadcastMetricUpdate(update interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.sendChan <- update:
			// Queued successfully
		default:
			// Buffer full - client too slow, mark for closure
			// This is backpressure: slow clients are dropped
			slog.Debug("ws client buffer full, dropping")
			go h.unregisterClient(client)
		}
	}
}

// Broadcast pushes a metric value to all connected clients. It satisfies the
// synthetic.Broadcaster interface used by the dev data engine, sending the same
// JSON shape the dashboard's WebSocket subscriber expects.
func (h *WSHub) Broadcast(v model.MetricValue) {
	h.BroadcastMetricUpdate(v)
}

// SubscribeToMetric subscribes a client to metric updates via Redis
// This would be called from the read loop when client sends subscription message
func (h *WSHub) SubscribeToMetric(ctx context.Context, metric string) *redis.PubSub {
	return h.redis.Subscribe(ctx, "metric."+metric)
}

// Close gracefully closes all client connections
func (h *WSHub) Close(timeout time.Duration) {
	h.mu.Lock()
	clients := make([]*wsClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.Unlock()

	// Close all clients
	for _, client := range clients {
		client.conn.Close(websocket.StatusGoingAway, "server shutdown")
		h.unregisterClient(client)
	}

	// Wait for drain or timeout
	time.Sleep(timeout)
}
