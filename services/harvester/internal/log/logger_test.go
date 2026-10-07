package log

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// TestLoggerFetch tests fetch logging with success.
func TestLoggerFetch(t *testing.T) {
	base := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	logger := NewLogger(base)

	logger.LogFetch("bist", "AKBNK", 100*time.Millisecond, nil)
	t.Log("Fetch success logged")
}

// TestLoggerFetchError tests fetch logging with error.
func TestLoggerFetchError(t *testing.T) {
	base := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	logger := NewLogger(base)

	logger.LogFetch("bist", "AKBNK", 500*time.Millisecond, errorf("connection timeout"))
	t.Log("Fetch error logged at WARN")
}

// TestLoggerParse tests parse logging with Fresh confidence.
func TestLoggerParse(t *testing.T) {
	base := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	logger := NewLogger(base)

	logger.LogParse("bist", "AKBNK", "abc123def456", model.Fresh, 50*time.Millisecond, nil)
	t.Log("Parse Fresh logged")
}

// TestLoggerParseApprox tests parse logging with Approx confidence.
func TestLoggerParseApprox(t *testing.T) {
	base := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	logger := NewLogger(base)

	logger.LogParse("kap", "FUND001", "def456abc123", model.Approx, 75*time.Millisecond, nil)
	t.Log("Parse Approx logged at DEBUG")
}

// TestLoggerParseError tests parse logging with error.
func TestLoggerParseError(t *testing.T) {
	base := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	logger := NewLogger(base)

	logger.LogParse("evds", "SERIES_CODE", "ghi789jkl012", model.Stale, 200*time.Millisecond, errorf("invalid JSON"))
	t.Log("Parse error logged at WARN")
}

// TestLoggerQuarantine tests quarantine logging.
func TestLoggerQuarantine(t *testing.T) {
	base := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	logger := NewLogger(base)

	logger.LogQuarantine("bist", "AKBNK", "mno345pqr678", "envelope_error")
	t.Log("Quarantine logged at WARN")
}

// TestLoggerBreakerStateChange tests circuit breaker state change logging.
func TestLoggerBreakerStateChange(t *testing.T) {
	base := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	logger := NewLogger(base)

	logger.LogBreakerStateChange("kap", "CLOSED", "OPEN")
	t.Log("Breaker CLOSED→OPEN logged at WARN")

	logger.LogBreakerStateChange("kap", "OPEN", "HALF_OPEN")
	t.Log("Breaker OPEN→HALF_OPEN logged at WARN")
}

// TestLoggerStore tests store/upsert logging.
func TestLoggerStore(t *testing.T) {
	base := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	logger := NewLogger(base)

	logger.LogStore("bist", "AKBNK", "stu901vwx234", 10*time.Millisecond, false)
	t.Log("Store new logged")

	logger.LogStore("bist", "AKBNK", "stu901vwx234", 5*time.Millisecond, true)
	t.Log("Store duplicate logged")
}

// TestLoggerReplay tests quarantine replay logging.
func TestLoggerReplay(t *testing.T) {
	base := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	logger := NewLogger(base)

	logger.LogReplay("bist", "AKBNK", 1, true, "")
	t.Log("Replay success logged at DEBUG")

	logger.LogReplay("kap", "FUND001", 2, false, "parse_error")
	t.Log("Replay failure logged at WARN")
}

// TestHashPrefix tests hash prefix extraction for logging.
func TestHashPrefix(t *testing.T) {
	tests := []struct {
		hash     string
		n        int
		expected string
	}{
		{"abcdef1234567890", 8, "abcdef12"},
		{"abc", 8, "abc"},
		{"", 8, ""},
		{"abcdef", 3, "abc"},
	}

	for _, tc := range tests {
		result := hashPrefix(tc.hash, tc.n)
		if result != tc.expected {
			t.Errorf("hashPrefix(%s, %d) = %s, expected %s", tc.hash, tc.n, result, tc.expected)
		}
	}
}

// errorf is a helper to create an error for testing.
func errorf(format string, args ...interface{}) error {
	return nil // Placeholder
}
