package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/neyialiyorlar/services/harvester/internal/model"
	"github.com/neyialiyorlar/services/harvester/internal/parse"
)

// MKKVAPAdapter fetches Turkish fund disclosures from MKK/VAP HTML reports.
// Rationale (mandate 3): DOM extraction via label anchors tolerates report layout drift
// without breaking; label-not-found flags Approx, never crashes.
type MKKVAPAdapter struct {
	cfg       Config
	httpCli   *http.Client
	domParser *parse.DOM
	// drift, when set, detects systemic DOM/schema changes across fetches:
	// a required anchor disappearing or a field changing type escalates the
	// result to Stale and quarantines it for replay/audit. Nil disables it,
	// preserving the per-record-only behaviour.
	drift *parse.DriftDetector
}

// NewMKKVAPAdapter creates an MKK/VAP adapter.
func NewMKKVAPAdapter(cfg Config, httpCli *http.Client) *MKKVAPAdapter {
	// Anchors map field name -> Turkish label text, the direction NewDOM
	// expects. The labels are stable across report layout changes; if a
	// label disappears the field is reported missing (a drift signal).
	anchors := map[string]string{
		"total_value":     "Fon Toplam Değeri",
		"management_fee":  "Yönetim Ücrets",
		"portfolio_value": "Portföy Değeri",
		"nav_value":       "NAV Değeri",
		"fund_demand":     "Fon Talebi",
		"payment_date":    "Ödeme Tarihi",
	}
	return &MKKVAPAdapter{
		cfg:       cfg,
		httpCli:   httpCli,
		domParser: parse.NewDOM(anchors),
	}
}

// Name returns the adapter name.
func (a *MKKVAPAdapter) Name() string {
	return "mkkvap"
}

// SourceID returns the configured source id.
func (a *MKKVAPAdapter) SourceID() string {
	return a.cfg.SourceID
}

// SetDriftDetector enables systemic schema/DOM drift detection for this
// adapter. The detector should be seeded with the adapter's known-good
// contract (see DriftContract) so the first live payload is compared
// against the intended shape.
func (a *MKKVAPAdapter) SetDriftDetector(d *parse.DriftDetector) {
	a.drift = d
}

// DriftContract returns the known-good structural Shape for this adapter,
// derived from its required DOM anchors. Use it to seed a DriftDetector.
func (a *MKKVAPAdapter) DriftContract() parse.Shape {
	return parse.Shape{
		"$":           parse.KindObject,
		"total_value": parse.KindString,
	}
}

// driftRequiredFields are the field paths whose disappearance constitutes a
// breaking DOM change for this source.
func (a *MKKVAPAdapter) driftRequiredFields() []string {
	return []string{"total_value"}
}

// Cadence returns the ingestion cadence (daily for fund reports, T+10 for foreign).
func (a *MKKVAPAdapter) Cadence() model.Cadence {
	return model.Daily
}

// Fetch retrieves the HTML report from the configured URL.
// The cursor parameter is date-based pagination (e.g., "2024-06-23" for next page).
func (a *MKKVAPAdapter) Fetch(ctx context.Context, cursor string) ([]model.Raw, string, error) {
	// Build the request URL with optional date cursor
	url := a.cfg.URL
	if cursor != "" {
		url = fmt.Sprintf("%s?date=%s", a.cfg.URL, cursor)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, cursor, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("User-Agent", "neyialiyorlar/1.0 (research)")
	resp, err := a.httpCli.Do(req)
	if err != nil {
		return nil, cursor, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, cursor, fmt.Errorf("http status: %d", resp.StatusCode)
	}

	// Read response body
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, cursor, fmt.Errorf("read body: %w", err)
	}

	// Extract fund identifier from HTML or use a default
	// In real implementation, parse the HTML to find fund name/isin
	fundName := "MKK_VAP_Fund"

	raw := model.Raw{
		SourceID:  a.cfg.SourceID,
		NaturalKey: fundName,
		Payload:   payload,
		FetchedAt: time.Now().UTC(),
	}

	// Compute content hash
	raw.ContentHash = computeSHA256(payload)

	// Parse next cursor from HTML (e.g., next date or pagination link)
	nextCursor := "" // In production: extract from pagination link

	return []model.Raw{raw}, nextCursor, nil
}

// Parse extracts fund data from the HTML report using DOM label anchors.
func (a *MKKVAPAdapter) Parse(raw model.Raw) model.ParseResult {
	result := model.ParseResult{
		Data:       make(map[string]interface{}),
		Confidence: model.Fresh,
	}

	// Parse the HTML using DOM extractor
	parseResult := a.domParser.Parse(raw.Payload)

	// A hard parse error (malformed HTML) yields nothing to inspect.
	if parseResult.Reason == "dom_parse_error" {
		result.Error = parseResult.Error
		result.Confidence = model.Approx
		result.Reason = "parse_error"
		return result
	}

	// Copy extracted data (may be partial when some labels are missing).
	for k, v := range parseResult.Data {
		result.Data[k] = v
	}

	// A missing label degrades confidence but is not fatal on its own; the
	// drift detector below decides whether the absence is a systemic break
	// (the label vanished versus the known-good contract) rather than a
	// one-off partial report.
	if parseResult.Reason == "dom_field_not_found" {
		result.Confidence = model.Approx
		result.Reason = parseResult.Reason
	}

	// Verify required fields are present
	if _, hasValue := result.Data["total_value"]; !hasValue {
		result.Confidence = model.Approx
		if result.Reason == "" {
			result.Reason = "missing_required_field_total_value"
		}
	}

	// Systemic drift check: compare this payload's structure against the
	// last-known-good contract. A breaking change (required anchor gone or
	// a field retyped) escalates to Stale and quarantines the raw so the
	// shift surfaces as an alert instead of silently degrading every record.
	if a.drift != nil {
		shape := parse.ShapeFromFields(result.Data)
		report := a.drift.Observe(a.SourceID(), shape, a.driftRequiredFields())
		switch report.Severity {
		case parse.SeverityBreaking:
			result.Confidence = model.Stale
			result.Reason = "schema_drift_breaking"
			result.Error = fmt.Errorf("schema drift (breaking) on %s: %s", a.SourceID(), report.Summary)
		case parse.SeverityBenign:
			if result.Confidence == model.Fresh {
				result.Confidence = model.Approx
			}
			if result.Reason == "" {
				result.Reason = "schema_drift_benign"
			}
		}
	}

	return result
}

// computeSHA256 is a helper to compute content hash.
// In production, use crypto/sha256.
func computeSHA256(data []byte) string {
	// Placeholder: real implementation uses sha256 sum
	return fmt.Sprintf("sha256_%d", len(data))
}
