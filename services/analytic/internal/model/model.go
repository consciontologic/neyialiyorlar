package model

import (
	"crypto/md5"
	"fmt"
	"time"
)

// Confidence represents the freshness/reliability of a derived metric.
type Confidence string

const (
	ConfFresh Confidence = "fresh"
	ConfStale Confidence = "stale"
	ConfApprox Confidence = "approx"
)

// String returns the string representation of a Confidence value (for logging).
func (c Confidence) String() string {
	return string(c)
}

// Worst returns the maximum (most conservative) confidence flag.
// Ordering: approx > stale > fresh. A derived metric inherits the WORST
// flag among its inputs — no proxy is ever mistaken for a measurement.
func Worst(flags ...Confidence) Confidence {
	worst := ConfFresh
	for _, f := range flags {
		if compareConf(f) > compareConf(worst) {
			worst = f
		}
	}
	return worst
}

func compareConf(c Confidence) int {
	switch c {
	case ConfApprox:
		return 2
	case ConfStale:
		return 1
	case ConfFresh:
		return 0
	default:
		return 0
	}
}

// Cadence represents when a metric recomputes.
type Cadence string

const (
	CadenceIntraday Cadence = "intraday"
	CadenceDaily    Cadence = "daily"
	CadenceWeekly   Cadence = "weekly"
	CadenceEvent    Cadence = "event"
)

// RawPayload represents a stored raw ingestion record from the harvester.
type RawPayload struct {
	ID           int64
	Source       string    // bist, kap, mkkvap, evds
	NaturalKey   string    // ticker, entity id, metric identifier
	Payload      []byte    // raw bytes (JSON, HTML, PDF text extracted)
	ContentHash  string    // dedup key
	FetchedAt    time.Time // knowledge time
	Confidence   Confidence
	Quarantined  bool
	QuarantineReason *string
}

// Derived represents a computed metric value to be stored/cached.
type Derived struct {
	MetricKey   string     // velocity_accumulation, nimvi, etc.
	Entity      string     // stable entity id (ISIN/MKK)
	Timestamp   time.Time  // observation time
	Value       *float64   // nil = gap row
	Flag        Confidence // fresh, stale, approx
	Cadence     Cadence    // intraday, daily, weekly, event
	ComputedAt  time.Time  // when this was computed
	InputsHash  string     // hash of inputs used; part of primary key for idempotency
	LatencyMs   int64      // computation latency
}

// RawSet is a collection of raw payloads grouped logically for metric computation.
type RawSet struct {
	Rows           []*RawPayload
	WorstConfidence Confidence
}

// NewRawSet creates a RawSet and computes the worst-case confidence.
func NewRawSet(rows []*RawPayload) *RawSet {
	flags := make([]Confidence, len(rows))
	for i, r := range rows {
		flags[i] = r.Confidence
	}
	return &RawSet{
		Rows:            rows,
		WorstConfidence: Worst(flags...),
	}
}

// ComputeInputsHash creates a deterministic hash of the inputs for idempotency.
// Inputs are hashed by (source, natural_key, content_hash, fetched_at) in sorted order.
func ComputeInputsHash(rows []*RawPayload) string {
	h := md5.New()
	for _, r := range rows {
		// Simple stable hash: concatenate fields in a fixed order.
		fmt.Fprintf(h, "%s|%s|%s|%d\n", r.Source, r.NaturalKey, r.ContentHash, r.FetchedAt.Unix())
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
