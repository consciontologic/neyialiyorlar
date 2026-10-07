package model

import (
	"time"
)

// Confidence represents the freshness/reliability of a derived value.
// Three states: Fresh (current data), Approx (proxied/partial data),
// Stale (recomputed from last-good, source unavailable).
// Rationale (mandate 20): point-in-time semantics require every metric
// to carry its knowledge date and confidence origin, never a silent
// fabrication. The downstream IC gate uses this flag to exclude stale
// tails from the right-hand side.
type Confidence string

const (
	Fresh Confidence = "fresh"   // current observation, all sources answered
	Approx Confidence = "approx" // proxied/partial (field drift, missing source)
	Stale Confidence = "stale"   // recomputed from last-good, source unavailable
)

// Raw is the untouched payload persisted from a source fetch.
// Rationale (mandate 2, ADR-0002): store first, parse later. The raw is
// immutable and indexed by content hash so recovery, replay, and offline
// audit are possible. Parsing is idempotent; the raw is the source of truth.
type Raw struct {
	SourceID     string    `db:"source_id"`      // e.g., "bist", "kap", "mkkvap", "evds"
	NaturalKey   string    `db:"natural_key"`    // ticker/isin/series key from the source
	ContentHash  string    `db:"content_hash"`   // SHA256(payload); deduplication
	Payload      []byte    `db:"payload"`        // raw bytes preserved as-is
	FetchedAt    time.Time `db:"fetched_at"`     // when this was observed
	Quarantined  bool      `db:"quarantined"`    // parse failure; keep for replay
	QuarantineReason string `db:"quarantine_reason"` // "envelope_error", "dom_not_found", etc.
}

// FetchMeta carries the metadata from a single fetch round: what sources
// were polled, which succeeded, which errored, and what the next cursor
// position should be. Rationale: the scheduler uses this to adjust backoff
// and advance high-water marks transactionally.
type FetchMeta struct {
	SourceID      string        `json:"source_id"`
	RawsFetched   int           `json:"raws_fetched"`
	RawsQuarantined int         `json:"raws_quarantined"`
	LatencyMS     int64         `json:"latency_ms"`
	Error         string        `json:"error,omitempty"` // on failure
	NextCursor    string        `json:"next_cursor,omitempty"`
	RetryAfterSec int           `json:"retry_after_sec,omitempty"` // if 429/503
}

// ParseResult is the output of a parser: the extracted fields, confidence
// flag, and any error. Rationale: every parse path (envelope, DOM, PDF)
// returns the same contract so the store layer can treat them uniformly.
type ParseResult struct {
	Data       map[string]interface{} // extracted fields
	Confidence Confidence             // fresh/approx/stale
	Error      error                  // if non-nil, quarantine the raw
	Reason     string                 // quarantine reason
}

// CalendarDate represents a single BIST session or holiday.
// Rationale (mandate 18): a single versioned calendar drives all daily-close
// and intraday ticks. This avoids naive weekday math and lets us exclude
// holidays without a separate holiday list lookup.
type CalendarDate struct {
	Date       time.Time `json:"date"`        // date in Europe/Istanbul
	SessionOn  bool      `json:"session_on"`  // true = trading session, false = holiday/weekend
	Type       string    `json:"type"`        // "session", "half-day", "holiday"
}

// Cadence represents the ingestion frequency for a data source.
type Cadence string

const (
	Event    Cadence = "event"     // real-time, on occurrence
	Intraday Cadence = "intraday"  // multiple times per day (e.g., every 5 min)
	Daily    Cadence = "daily"     // once per day (at BIST close)
	Weekly   Cadence = "weekly"    // once per week
)

// IsValid checks if the cadence is a recognized value.
func (c Cadence) IsValid() bool {
	switch c {
	case Event, Intraday, Daily, Weekly:
		return true
	default:
		return false
	}
}
