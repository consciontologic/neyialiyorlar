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

// KAPAdapter fetches KAP fund disclosures (S2 — JSON + PDF via HTTP POST).
// Rationale (ADR-0001): KAP publishes both JSON endpoints (corp actions, prices)
// and PDF documents (fund reports). This adapter handles both via POST to SPA endpoints.
type KAPAdapter struct {
	cfg      Config
	httpClient *http.Client
	envelope *parse.Envelope
	pdfParser *parse.PDFDoc
}

// NewKAPAdapter creates a KAP data source adapter.
func NewKAPAdapter(cfg Config, httpClient *http.Client) *KAPAdapter {
	// Define expected JSON fields for KAP endpoints
	fieldNames := map[string]string{
		"document_type": "doctype",
		"company_code":  "companycode",
		"filing_date":   "filing_date",
		"content_url":   "url",
	}
	requiredFields := []string{"document_type", "company_code"}

	pdfAnchors := map[string]string{
		"report_date": "Raporlama Tarihi",
		"nav":         "Net Aktif Değer",
	}

	return &KAPAdapter{
		cfg:        cfg,
		httpClient: httpClient,
		envelope: parse.NewEnvelope(fieldNames, requiredFields),
		pdfParser: parse.NewPDFDoc(100, pdfAnchors),
	}
}

// Name returns the source identifier.
func (ka *KAPAdapter) Name() string {
	return "kap"
}

// SourceID returns the unique identifier for this source.
func (ka *KAPAdapter) SourceID() string {
	return ka.cfg.SourceID
}

// Cadence returns the ingestion frequency (KAP publishes event-driven).
func (ka *KAPAdapter) Cadence() model.Cadence {
	return model.Event
}

// Fetch retrieves documents from KAP SPA endpoint.
// Rationale: KAP uses POST to submit queries; cursor is a timestamp-based token.
func (ka *KAPAdapter) Fetch(ctx context.Context, cursor string) ([]model.Raw, string, error) {
	// Construct POST request URL
	url := ka.cfg.URL

	// Create POST request
	// SPIKE: Real KAP endpoint requires specific JSON payload; omitted for skeleton
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("kap fetch: create request: %w", err)
	}

	resp, err := ka.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("kap fetch: http do: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("kap fetch: read body: %w", err)
	}

	// Guard: validate envelope shape
	parseResult := ka.envelope.Validate(payload)
	if parseResult.Error != nil {
		return nil, "", fmt.Errorf("kap fetch: envelope guard: %w", parseResult.Error)
	}

	// SPIKE: Real KAP adapter would parse document metadata and fetch PDFs
	fields := ka.envelope.AllFieldsMap()
	raw := model.Raw{
		SourceID:   ka.cfg.SourceID,
		NaturalKey: fmt.Sprintf("%v", fields["company_code"]),
		Payload:    payload,
		FetchedAt:  time.Now(),
	}

	// SPIKE: Next cursor depends on KAP pagination
	nextCursor := ""

	return []model.Raw{raw}, nextCursor, nil
}

// Parse processes KAP JSON + PDF into derived fields.
func (ka *KAPAdapter) Parse(raw model.Raw) model.ParseResult {
	// First, validate envelope
	result := ka.envelope.Validate(raw.Payload)
	if result.Error != nil {
		return model.ParseResult{
			Confidence: model.Stale,
			Error:      result.Error,
			Reason:     "kap_envelope_error",
		}
	}

	fields := ka.envelope.AllFieldsMap()

	// Check for required fields
	if _, ok := fields["company_code"]; !ok {
		return model.ParseResult{
			Confidence: model.Approx,
			Error:      fmt.Errorf("missing company_code"),
			Reason:     "kap_missing_company_code",
		}
	}

	// If this is a PDF document, attempt PDF extraction
	// SPIKE: Real implementation would fetch and parse PDF from fields["content_url"]
	if docType, ok := fields["document_type"]; ok && docType == "PDF" {
		// In production, fetch PDF from URL and parse it
		// For now, just return approx (PDF not yet parsed)
		return model.ParseResult{
			Data:       fields,
			Confidence: model.Approx,
			Reason:     "kap_pdf_not_parsed_yet",
		}
	}

	return model.ParseResult{
		Data:       fields,
		Confidence: model.Fresh,
	}
}
