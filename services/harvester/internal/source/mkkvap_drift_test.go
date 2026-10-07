package source

import (
	"net/http"
	"testing"

	"github.com/neyialiyorlar/services/harvester/internal/log"
	"github.com/neyialiyorlar/services/harvester/internal/model"
	"github.com/neyialiyorlar/services/harvester/internal/parse"
)

// Compile-time guard: the harvester's structured logger must satisfy the
// detector's DriftLogger interface so drift alerts can be wired in.
var _ parse.DriftLogger = (*log.Logger)(nil)

// healthyMKKVAPReport contains all six anchor labels the adapter expects.
const healthyMKKVAPReport = `
<html><body><table>
	<tr><td>Fon Toplam Değeri</td><td>1.234,56</td></tr>
	<tr><td>Yönetim Ücrets</td><td>2,5</td></tr>
	<tr><td>Portföy Değeri</td><td>987.654,32</td></tr>
	<tr><td>NAV Değeri</td><td>10,5</td></tr>
	<tr><td>Fon Talebi</td><td>100</td></tr>
	<tr><td>Ödeme Tarihi</td><td>2026-06-26</td></tr>
</table></body></html>`

// driftedMKKVAPReport drops the "Fon Toplam Değeri" (total_value) label,
// simulating a DOM/layout change where the required anchor disappears.
const driftedMKKVAPReport = `
<html><body><table>
	<tr><td>Yönetim Ücrets</td><td>2,5</td></tr>
	<tr><td>Portföy Değeri</td><td>987.654,32</td></tr>
	<tr><td>NAV Değeri</td><td>10,5</td></tr>
	<tr><td>Fon Talebi</td><td>100</td></tr>
	<tr><td>Ödeme Tarihi</td><td>2026-06-26</td></tr>
</table></body></html>`

func newMKKVAPWithDrift() *MKKVAPAdapter {
	a := NewMKKVAPAdapter(Config{SourceID: "mkkvap"}, &http.Client{})
	a.SetDriftDetector(parse.NewDriftDetector(nil))
	return a
}

// TestMKKVAPDrift_StableReportNoFalseAlarm verifies that a steady, healthy
// report establishes a baseline and never trips the drift detector.
func TestMKKVAPDrift_StableReportNoFalseAlarm(t *testing.T) {
	a := newMKKVAPWithDrift()
	raw := model.Raw{SourceID: "mkkvap", NaturalKey: "Fund_001", Payload: []byte(healthyMKKVAPReport)}

	first := a.Parse(raw)
	if first.Confidence != model.Fresh {
		t.Fatalf("healthy report should parse Fresh (all anchors present), got %s reason=%q err=%v",
			first.Confidence, first.Reason, first.Error)
	}

	second := a.Parse(raw)
	if second.Confidence != model.Fresh || second.Error != nil {
		t.Errorf("identical report must not drift: got %s reason=%q err=%v",
			second.Confidence, second.Reason, second.Error)
	}
}

// TestMKKVAPDrift_RequiredAnchorRemovedIsBreaking verifies the reaction: once
// a known-good baseline exists, a report that drops the required total_value
// anchor is escalated to Stale and flagged/quarantined as a breaking drift,
// rather than silently degrading to Approx like a one-off partial report.
func TestMKKVAPDrift_RequiredAnchorRemovedIsBreaking(t *testing.T) {
	a := newMKKVAPWithDrift()

	// Establish the known-good baseline from a healthy report.
	if base := a.Parse(model.Raw{SourceID: "mkkvap", Payload: []byte(healthyMKKVAPReport)}); base.Confidence != model.Fresh {
		t.Fatalf("baseline report should be Fresh, got %s (%s)", base.Confidence, base.Reason)
	}

	// The required anchor disappears: systemic breaking drift.
	got := a.Parse(model.Raw{SourceID: "mkkvap", Payload: []byte(driftedMKKVAPReport)})
	if got.Confidence != model.Stale {
		t.Errorf("breaking drift must escalate to Stale, got %s", got.Confidence)
	}
	if got.Reason != "schema_drift_breaking" {
		t.Errorf("expected reason schema_drift_breaking, got %q", got.Reason)
	}
	if got.Error == nil {
		t.Errorf("breaking drift must set an error so the raw is quarantined")
	}
}

// TestMKKVAPDrift_DisabledPreservesGracefulDegradation verifies the nil-safe
// path: with no detector, a missing required anchor degrades to Approx (the
// original behaviour) and is never escalated to a breaking drift.
func TestMKKVAPDrift_DisabledPreservesGracefulDegradation(t *testing.T) {
	a := NewMKKVAPAdapter(Config{SourceID: "mkkvap"}, &http.Client{}) // no detector

	if healthy := a.Parse(model.Raw{SourceID: "mkkvap", Payload: []byte(healthyMKKVAPReport)}); healthy.Confidence != model.Fresh {
		t.Fatalf("healthy report should be Fresh without a detector, got %s (%s)", healthy.Confidence, healthy.Reason)
	}

	missing := a.Parse(model.Raw{SourceID: "mkkvap", Payload: []byte(driftedMKKVAPReport)})
	if missing.Confidence != model.Approx {
		t.Errorf("missing field without a detector should be Approx, got %s", missing.Confidence)
	}
	if missing.Reason == "schema_drift_breaking" {
		t.Errorf("drift detection must not run when no detector is set")
	}
}
