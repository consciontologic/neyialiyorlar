package middleware

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ResponseWriter wrapper to capture status code
type statusCapture struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (w *statusCapture) WriteHeader(code int) {
	if !w.written {
		w.statusCode = code
		w.written = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *statusCapture) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Hijack implements http.Hijacker by delegating to the wrapped ResponseWriter.
// Without this, connection-upgrade handlers (e.g. WebSocket) fail with
// "501 Not Implemented" because this wrapper would otherwise hide the
// underlying writer's Hijack method.
func (w *statusCapture) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return hj.Hijack()
}

// PanicRecovery recovers from panics and logs them
func PanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger := slog.Default()
				logger.Error("panic recovered",
					slog.String("error", fmt.Sprintf("%v", err)),
					slog.String("path", r.URL.Path),
					slog.String("method", r.Method),
				)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Internal server error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RequestIDKey is the context key for request ID
type contextKey string

const RequestIDKey contextKey = "request_id"

// RequestIDInjection adds a UUID request ID to each request
func RequestIDInjection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		w.Header().Set("X-Request-ID", requestID)

		// Inject into context and slog context
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// StructuredLogging logs all requests with structured fields
func StructuredLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusCapture{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		latency := time.Since(start).Milliseconds()
		requestID := ""
		if id := r.Context().Value(RequestIDKey); id != nil {
			requestID = id.(string)
		}

		slog.Default().Info("request completed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", wrapped.statusCode),
			slog.Int64("latency_ms", latency),
			slog.String("request_id", requestID),
		)
	})
}

// Timeout wraps a handler with a request deadline
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			// Create a channel to signal completion
			done := make(chan struct{})
			go func() {
				defer func() { done <- struct{}{} }()
				next.ServeHTTP(w, r.WithContext(ctx))
			}()

			select {
			case <-done:
				// Request completed successfully
			case <-ctx.Done():
				// Timeout reached
				w.Header().Set("Retry-After", "60")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Service unavailable (request timeout)",
				})
			}
		})
	}
}

// Chain combines multiple middlewares into a single handler
func Chain(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	// Apply middlewares in reverse order (so they are applied in the order specified)
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}
