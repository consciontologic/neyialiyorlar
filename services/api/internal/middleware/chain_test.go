package middleware

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// statusCapture must satisfy http.Hijacker so that connection-upgrade handlers
// (WebSocket) keep working when wrapped by the logging middleware.
var _ http.Hijacker = (*statusCapture)(nil)

// hijackableRecorder is an httptest.ResponseRecorder that also implements
// http.Hijacker, simulating the real net/http HTTP/1.1 ResponseWriter.
type hijackableRecorder struct {
	*httptest.ResponseRecorder
	hijacked bool
}

func (h *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	c1, c2 := net.Pipe()
	_ = c2.Close()
	return c1, bufio.NewReadWriter(bufio.NewReader(c1), bufio.NewWriter(c1)), nil
}

func TestStatusCaptureHijackDelegates(t *testing.T) {
	rec := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	w := &statusCapture{ResponseWriter: rec, statusCode: http.StatusOK}

	hj, ok := http.ResponseWriter(w).(http.Hijacker)
	if !ok {
		t.Fatal("statusCapture does not implement http.Hijacker")
	}

	conn, _, err := hj.Hijack()
	if err != nil {
		t.Fatalf("Hijack returned error: %v", err)
	}
	if conn != nil {
		_ = conn.Close()
	}
	if !rec.hijacked {
		t.Fatal("Hijack did not delegate to the underlying ResponseWriter")
	}
}

func TestStatusCaptureHijackUnsupported(t *testing.T) {
	// httptest.ResponseRecorder does not implement http.Hijacker.
	w := &statusCapture{ResponseWriter: httptest.NewRecorder(), statusCode: http.StatusOK}

	if _, _, err := w.Hijack(); err == nil {
		t.Fatal("expected an error when the underlying writer is not an http.Hijacker")
	}
}
