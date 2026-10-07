package parse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// Schema/DOM drift detection.
//
// Rationale (mandate 5): the per-record guards (Envelope, DOM) already
// degrade gracefully when a single payload is malformed — they flag
// Approx/Stale and quarantine the raw. What they do NOT do is recognise a
// *structural change* in a source as a systemic event: when KAP renames a
// JSON field or BIST flips a type or an MKK/VAP report drops a label
// anchor, every record fails identically and the operator has no signal
// that the *contract* changed versus one bad row.
//
// The drift detector closes that gap. It reduces a payload to a stable
// structural fingerprint (field path -> kind, value-independent and
// order-independent), remembers the last-known-good fingerprint per
// source, and classifies any change:
//
//   - none:     identical structure -> Fresh, no action.
//   - benign:   additive-only (new optional field, or an optional field
//               removed) -> Approx, baseline advances (the system adapts
//               to additive evolution automatically).
//   - breaking: a required field removed, or any field retyped (incl. the
//               root container type) -> Stale + quarantine + alert, and
//               the baseline is held so the known-good contract is not
//               silently overwritten by the broken shape.
//
// The "kind" vocabulary deliberately matches Envelope.checkType
// ("string"/"number"/"boolean"/"object"/"array"/"null") so a contract can
// be seeded directly from an Envelope's declared FieldNames.

// Kind is a coarse structural type used in a Shape fingerprint.
type Kind string

const (
	KindString  Kind = "string"
	KindNumber  Kind = "number"
	KindBool    Kind = "boolean"
	KindObject  Kind = "object"
	KindArray   Kind = "array"
	KindNull    Kind = "null"
	KindUnknown Kind = "unknown"
)

// rootKey is the sentinel path under which the top-level container kind is
// recorded, so an object->array (or scalar) change at the root surfaces as
// a retype rather than silently vanishing.
const rootKey = "$"

// maxShapeDepth bounds structural recursion so a hostile, deeply nested
// payload cannot blow the stack or balloon the fingerprint.
const maxShapeDepth = 4

// Shape is a structural fingerprint: a map from field path to Kind.
// It is value-independent (different numbers/strings produce the same
// Shape) and order-independent (JSON key order is irrelevant), so it
// changes only when the *structure* changes — which is exactly a drift.
type Shape map[string]Kind

// Severity classifies a detected drift.
type Severity string

const (
	SeverityNone     Severity = "none"
	SeverityBenign   Severity = "benign"
	SeverityBreaking Severity = "breaking"
)

// DriftReport is the outcome of comparing a current Shape against a baseline.
type DriftReport struct {
	Drifted  bool     // true when Severity != none
	Severity Severity // none / benign / breaking
	Added    []string // field paths present now but not in the baseline
	Removed  []string // field paths present in the baseline but not now
	Retyped  []string // field paths whose Kind changed
	Summary  string   // compact human-readable description
}

// kindOf maps a decoded value to a coarse Kind. It accepts the set
// produced by encoding/json plus the common Go numeric types that an
// upstream parser might place into an extracted field map.
func kindOf(v interface{}) Kind {
	switch v.(type) {
	case nil:
		return KindNull
	case bool:
		return KindBool
	case string:
		return KindString
	case float64, float32,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		json.Number:
		return KindNumber
	case map[string]interface{}:
		return KindObject
	case []interface{}:
		return KindArray
	default:
		return KindUnknown
	}
}

// walkShape records the kind at prefix, then recurses into objects and
// array elements up to depth levels. The root call uses prefix "" so the
// container itself is not double-recorded (the caller records rootKey).
func walkShape(prefix string, v interface{}, depth int, out Shape) {
	if prefix != "" {
		out[prefix] = kindOf(v)
	}
	if depth <= 0 {
		return
	}
	switch t := v.(type) {
	case map[string]interface{}:
		for key, val := range t {
			child := key
			if prefix != "" {
				child = prefix + "." + key
			}
			walkShape(child, val, depth-1, out)
		}
	case []interface{}:
		// Merge the shape of every element under "<prefix>[]" so a
		// heterogeneous array surfaces all of its element fields.
		elemPrefix := prefix + "[]"
		for _, el := range t {
			walkShape(elemPrefix, el, depth-1, out)
		}
	}
}

// FingerprintJSON reduces a JSON payload to its structural Shape. It returns
// an error only when the bytes are not valid JSON (mirroring Envelope,
// which quarantines on a decode error). Whitespace, key order, and scalar
// values do not affect the result.
func FingerprintJSON(payload []byte) (Shape, error) {
	var v interface{}
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("fingerprint json decode: %w", err)
	}
	out := Shape{rootKey: kindOf(v)}
	walkShape("", v, maxShapeDepth, out)
	return out, nil
}

// ShapeFromFields builds a Shape from an already-extracted field map (for
// example a DOM-resolved or Envelope-validated Data map). The presence and
// kind of each field is recorded; absent fields are simply not present.
func ShapeFromFields(fields map[string]interface{}) Shape {
	out := Shape{rootKey: KindObject}
	for k, v := range fields {
		walkShape(k, v, maxShapeDepth-1, out)
	}
	return out
}

// ShapeFromContract builds a Shape from a declared field-name -> type-name
// contract, using the same vocabulary as Envelope.FieldNames. This lets a
// source seed its known-good baseline from its declared contract instead of
// waiting to observe a healthy payload first.
func ShapeFromContract(fieldNames map[string]string) Shape {
	out := Shape{rootKey: KindObject}
	for name, typeName := range fieldNames {
		out[name] = kindFromTypeName(typeName)
	}
	return out
}

func kindFromTypeName(typeName string) Kind {
	switch typeName {
	case "string":
		return KindString
	case "number":
		return KindNumber
	case "boolean":
		return KindBool
	case "object":
		return KindObject
	case "array":
		return KindArray
	case "null":
		return KindNull
	default:
		return KindUnknown
	}
}

// CompareShapes diffs current against baseline and classifies the result.
// A field path in required makes its removal breaking; any retype (the
// kind of a still-present field changed, including the root container) is
// always breaking. Additive-only changes and removals of optional fields
// are benign.
func CompareShapes(baseline, current Shape, required []string) DriftReport {
	reqSet := make(map[string]bool, len(required))
	for _, r := range required {
		reqSet[r] = true
	}

	var added, removed, retyped []string
	for path, baseKind := range baseline {
		curKind, ok := current[path]
		switch {
		case !ok:
			removed = append(removed, path)
		case curKind != baseKind:
			retyped = append(retyped, path)
		}
	}
	for path := range current {
		if _, ok := baseline[path]; !ok {
			added = append(added, path)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(retyped)

	sev := SeverityNone
	switch {
	case len(retyped) > 0:
		sev = SeverityBreaking
	case anyRequired(removed, reqSet):
		sev = SeverityBreaking
	case len(added) > 0 || len(removed) > 0:
		sev = SeverityBenign
	}

	return DriftReport{
		Drifted:  sev != SeverityNone,
		Severity: sev,
		Added:    added,
		Removed:  removed,
		Retyped:  retyped,
		Summary:  summarize(sev, added, removed, retyped, reqSet),
	}
}

func anyRequired(paths []string, reqSet map[string]bool) bool {
	for _, p := range paths {
		if reqSet[p] {
			return true
		}
	}
	return false
}

func summarize(sev Severity, added, removed, retyped []string, reqSet map[string]bool) string {
	if sev == SeverityNone {
		return "no structural change"
	}
	var parts []string
	if len(retyped) > 0 {
		parts = append(parts, "retyped=["+strings.Join(retyped, ",")+"]")
	}
	if len(removed) > 0 {
		var reqRemoved, optRemoved []string
		for _, p := range removed {
			if reqSet[p] {
				reqRemoved = append(reqRemoved, p)
			} else {
				optRemoved = append(optRemoved, p)
			}
		}
		if len(reqRemoved) > 0 {
			parts = append(parts, "removed_required=["+strings.Join(reqRemoved, ",")+"]")
		}
		if len(optRemoved) > 0 {
			parts = append(parts, "removed_optional=["+strings.Join(optRemoved, ",")+"]")
		}
	}
	if len(added) > 0 {
		parts = append(parts, "added=["+strings.Join(added, ",")+"]")
	}
	return string(sev) + ": " + strings.Join(parts, "; ")
}

// RecommendedConfidence maps a drift severity onto the confidence a parse
// result should carry: none keeps Fresh, benign degrades to Approx, and
// breaking degrades to Stale.
func RecommendedConfidence(sev Severity) model.Confidence {
	switch sev {
	case SeverityBreaking:
		return model.Stale
	case SeverityBenign:
		return model.Approx
	default:
		return model.Fresh
	}
}

// BaselineStore persists the last-known-good Shape per source. An
// in-memory implementation is provided; a database-backed implementation
// can be substituted in production so baselines survive restarts.
type BaselineStore interface {
	Get(sourceID string) (Shape, bool)
	Set(sourceID string, shape Shape)
}

// DriftLogger receives a structured alert whenever drift is detected. The
// harvester's *log.Logger satisfies this interface; defining it here keeps
// the parse package decoupled from the log package.
type DriftLogger interface {
	LogDrift(source, severity string, added, removed, retyped []string)
}

// MultiLogger fans a single drift event out to several DriftLoggers — for
// example a slog-backed logger for the operator's log stream and a
// database-backed store that persists the alert for the monitoring panel.
// A nil element is skipped, so callers can compose loggers without
// nil-checking each one.
type MultiLogger []DriftLogger

// LogDrift forwards the event to every non-nil logger in the slice.
func (m MultiLogger) LogDrift(source, severity string, added, removed, retyped []string) {
	for _, l := range m {
		if l != nil {
			l.LogDrift(source, severity, added, removed, retyped)
		}
	}
}

// MemoryBaselineStore is a thread-safe in-memory BaselineStore. Shapes are
// copied on the way in and out so callers cannot mutate stored baselines.
type MemoryBaselineStore struct {
	mu        sync.RWMutex
	baselines map[string]Shape
}

// NewMemoryBaselineStore creates an empty in-memory baseline store.
func NewMemoryBaselineStore() *MemoryBaselineStore {
	return &MemoryBaselineStore{baselines: make(map[string]Shape)}
}

// Get returns a copy of the stored baseline for sourceID, if present.
func (m *MemoryBaselineStore) Get(sourceID string) (Shape, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.baselines[sourceID]
	if !ok {
		return nil, false
	}
	return copyShape(s), true
}

// Set stores a copy of shape as the baseline for sourceID.
func (m *MemoryBaselineStore) Set(sourceID string, shape Shape) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.baselines[sourceID] = copyShape(shape)
}

func copyShape(s Shape) Shape {
	out := make(Shape, len(s))
	for k, v := range s {
		out[k] = v
	}
	return out
}

// DriftDetector remembers the last-known-good Shape per source and
// classifies each new observation. It is safe for concurrent use.
type DriftDetector struct {
	mu     sync.Mutex
	store  BaselineStore
	logger DriftLogger
}

// NewDriftDetector creates a detector backed by store. If store is nil an
// in-memory store is used.
func NewDriftDetector(store BaselineStore) *DriftDetector {
	if store == nil {
		store = NewMemoryBaselineStore()
	}
	return &DriftDetector{store: store}
}

// SetLogger attaches a DriftLogger that is notified on every detected drift
// (benign or breaking). Passing nil disables alerting.
func (d *DriftDetector) SetLogger(l DriftLogger) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.logger = l
}

// Seed installs a known-good baseline for sourceID without treating it as
// an observation. Production wiring seeds from a source's declared contract
// so the very first live payload is compared against the intended shape
// rather than becoming the baseline by default.
func (d *DriftDetector) Seed(sourceID string, shape Shape) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.store.Set(sourceID, shape)
}

// Observe compares current against the stored baseline for sourceID and
// returns a DriftReport. On the first observation (no baseline) it records
// the baseline and reports none. On benign drift it advances the baseline
// (adapting to additive evolution). On breaking drift it holds the baseline
// so the known-good contract is not overwritten by the broken shape.
func (d *DriftDetector) Observe(sourceID string, current Shape, required []string) DriftReport {
	d.mu.Lock()
	defer d.mu.Unlock()

	baseline, ok := d.store.Get(sourceID)
	if !ok {
		d.store.Set(sourceID, current)
		return DriftReport{
			Drifted:  false,
			Severity: SeverityNone,
			Summary:  "baseline established",
		}
	}

	report := CompareShapes(baseline, current, required)
	switch report.Severity {
	case SeverityBenign:
		// Adapt: accept additive evolution and advance the contract.
		d.store.Set(sourceID, current)
	case SeverityBreaking:
		// Hold the known-good baseline; do not adopt the broken shape.
	}
	if report.Drifted && d.logger != nil {
		d.logger.LogDrift(sourceID, string(report.Severity), report.Added, report.Removed, report.Retyped)
	}
	return report
}

// ObserveJSON fingerprints a raw JSON payload and observes it. A decode
// error is returned to the caller (which should quarantine the raw), and
// is itself a strong drift signal.
func (d *DriftDetector) ObserveJSON(sourceID string, payload []byte, required []string) (DriftReport, error) {
	shape, err := FingerprintJSON(payload)
	if err != nil {
		return DriftReport{}, err
	}
	return d.Observe(sourceID, shape, required), nil
}
