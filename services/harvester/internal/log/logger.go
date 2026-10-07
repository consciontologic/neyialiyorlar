package log

import (
	"log/slog"
	"strings"
	"time"

	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// Event is a structured logging event for ingestion operations.
type Event struct {
	Operation    string        // "fetch", "parse", "store", "quarantine", "breaker"
	Source       string        // e.g., "bist", "kap", "mkkvap", "evds"
	NaturalKey   string        // ticker/isin/series identifier
	ContentHash  string        // full SHA256 hash
	Confidence   model.Confidence // Fresh/Approx/Stale
	LatencyMS    int64         // milliseconds
	Error        string        // error message if applicable
	Reason       string        // additional context
}

// Logger wraps slog for ingestion events.
type Logger struct {
	base *slog.Logger
}

// NewLogger creates a structured logger for harvester events.
func NewLogger(base *slog.Logger) *Logger {
	return &Logger{base: base}
}

// LogFetch records a fetch operation.
func (l *Logger) LogFetch(source, naturalKey string, duration time.Duration, err error) {
	attrs := []slog.Attr{
		slog.String("operation", "fetch"),
		slog.String("source", source),
		slog.String("natural_key", naturalKey),
		slog.Int64("latency_ms", duration.Milliseconds()),
	}

	level := slog.LevelInfo
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		level = slog.LevelWarn
	}

	l.base.LogAttrs(nil, level, "fetch", attrs...)
}

// LogParse records a parse operation.
func (l *Logger) LogParse(source, naturalKey string, contentHash string, confidence model.Confidence, duration time.Duration, err error) {
	attrs := []slog.Attr{
		slog.String("operation", "parse"),
		slog.String("source", source),
		slog.String("natural_key", naturalKey),
		slog.String("content_hash_prefix", hashPrefix(contentHash, 8)),
		slog.String("confidence", string(confidence)),
		slog.Int64("latency_ms", duration.Milliseconds()),
	}

	level := slog.LevelInfo
	if confidence != model.Fresh {
		level = slog.LevelDebug
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		level = slog.LevelWarn
	}

	l.base.LogAttrs(nil, level, "parse", attrs...)
}

// LogQuarantine records when a raw is quarantined.
func (l *Logger) LogQuarantine(source, naturalKey, contentHash, reason string) {
	attrs := []slog.Attr{
		slog.String("operation", "quarantine"),
		slog.String("source", source),
		slog.String("natural_key", naturalKey),
		slog.String("content_hash_prefix", hashPrefix(contentHash, 8)),
		slog.String("quarantine_reason", reason),
	}

	l.base.LogAttrs(nil, slog.LevelWarn, "quarantine", attrs...)
}

// LogBreakerStateChange records a circuit breaker state transition.
func (l *Logger) LogBreakerStateChange(source, oldState, newState string) {
	attrs := []slog.Attr{
		slog.String("operation", "breaker_state_change"),
		slog.String("source", source),
		slog.String("old_state", oldState),
		slog.String("new_state", newState),
	}

	l.base.LogAttrs(nil, slog.LevelWarn, "breaker", attrs...)
}

// LogStore records a store/upsert operation.
func (l *Logger) LogStore(source, naturalKey, contentHash string, duration time.Duration, isDuplicate bool) {
	attrs := []slog.Attr{
		slog.String("operation", "store"),
		slog.String("source", source),
		slog.String("natural_key", naturalKey),
		slog.String("content_hash_prefix", hashPrefix(contentHash, 8)),
		slog.Int64("latency_ms", duration.Milliseconds()),
		slog.Bool("is_duplicate", isDuplicate),
	}

	level := slog.LevelDebug
	if isDuplicate {
		level = slog.LevelDebug
	}

	l.base.LogAttrs(nil, level, "store", attrs...)
}

// LogReplay records a quarantine replay attempt.
func (l *Logger) LogReplay(source, naturalKey string, attemptNumber int, success bool, reason string) {
	attrs := []slog.Attr{
		slog.String("operation", "replay"),
		slog.String("source", source),
		slog.String("natural_key", naturalKey),
		slog.Int("attempt_number", attemptNumber),
		slog.Bool("success", success),
	}

	if reason != "" {
		attrs = append(attrs, slog.String("reason", reason))
	}

	level := slog.LevelDebug
	if !success {
		level = slog.LevelWarn
	}

	l.base.LogAttrs(nil, level, "replay", attrs...)
}

// LogDrift records a detected schema/DOM drift for a source. A breaking
// drift (required field removed or any field retyped) is logged at WARN so
// it surfaces as an actionable alert; a benign, additive drift is logged at
// INFO. The field diffs let an operator see exactly what changed.
func (l *Logger) LogDrift(source, severity string, added, removed, retyped []string) {
	attrs := []slog.Attr{
		slog.String("operation", "drift"),
		slog.String("source", source),
		slog.String("severity", severity),
		slog.Int("fields_added", len(added)),
		slog.Int("fields_removed", len(removed)),
		slog.Int("fields_retyped", len(retyped)),
	}
	if len(added) > 0 {
		attrs = append(attrs, slog.String("added", strings.Join(added, ",")))
	}
	if len(removed) > 0 {
		attrs = append(attrs, slog.String("removed", strings.Join(removed, ",")))
	}
	if len(retyped) > 0 {
		attrs = append(attrs, slog.String("retyped", strings.Join(retyped, ",")))
	}

	level := slog.LevelInfo
	if severity == "breaking" {
		level = slog.LevelWarn
	}

	l.base.LogAttrs(nil, level, "drift", attrs...)
}

// hashPrefix returns the first n characters of a hash for logging.
func hashPrefix(hash string, n int) string {
	if len(hash) < n {
		return hash
	}
	return hash[:n]
}
