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

// EVDSAdapter fetches EVDS macroeconomic data (S4 — JSON via HTTP GET).
// Rationale: EVDS publishes keyed JSON time series (e.g., USD/TRY, interest rates).
// Fetches are keyed via query parameters; pagination is cursor-based.
type EVDSAdapter struct {
	cfg      Config
	httpClient *http.Client
	envelope *parse.Envelope
}

// NewEVDSAdapter creates an EVDS data source adapter.
func NewEVDSAdapter(cfg Config, httpClient *http.Client) *EVDSAdapter {
	// Define expected JSON fields for EVDS endpoints
	fieldNames := map[string]string{
		"series_code": "code",
		"date":        "tarih",
		"value":       "deger",
		"frequency":   "sikligi",
	}
	requiredFields := []string{"series_code", "date", "value"}

	return &EVDSAdapter{
		cfg:        cfg,
		httpClient: httpClient,
		envelope: parse.NewEnvelope(fieldNames, requiredFields),
	}
}

// Name returns the source identifier.
func (ea *EVDSAdapter) Name() string {
	return "evds"
}

// SourceID returns the unique identifier for this source.
func (ea *EVDSAdapter) SourceID() string {
	return ea.cfg.SourceID
}

// Cadence returns the ingestion frequency (EVDS publishes daily/weekly).
func (ea *EVDSAdapter) Cadence() model.Cadence {
	return model.Daily
}

// Fetch retrieves EVDS time-series data via GET.
// Cursor is a date-based token for time-series pagination.
func (ea *EVDSAdapter) Fetch(ctx context.Context, cursor string) ([]model.Raw, string, error) {
	// Construct GET request URL with query parameters
	// SPIKE: Real EVDS endpoint requires API key and series code; omitted for skeleton
	url := ea.cfg.URL
	if cursor != "" {
		url = fmt.Sprintf("%s&start=%s", url, cursor)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("evds fetch: create request: %w", err)
	}

	resp, err := ea.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("evds fetch: http do: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("evds fetch: read body: %w", err)
	}

	// Guard: validate envelope shape
	parseResult := ea.envelope.Validate(payload)
	if parseResult.Error != nil {
		return nil, "", fmt.Errorf("evds fetch: envelope guard: %w", parseResult.Error)
	}

	// Extract fields
	fields := ea.envelope.AllFieldsMap()

	raw := model.Raw{
		SourceID:   ea.cfg.SourceID,
		NaturalKey: fmt.Sprintf("%v", fields["series_code"]), // e.g., "USD/TRY"
		Payload:    payload,
		FetchedAt:  time.Now(),
	}

	// SPIKE: Next cursor depends on EVDS pagination (date-based)
	nextCursor := ""

	return []model.Raw{raw}, nextCursor, nil
}

// Parse processes EVDS JSON into derived fields.
func (ea *EVDSAdapter) Parse(raw model.Raw) model.ParseResult {
	result := ea.envelope.Validate(raw.Payload)
	if result.Error != nil {
		return model.ParseResult{
			Confidence: model.Stale,
			Error:      result.Error,
			Reason:     "evds_envelope_error",
		}
	}

	fields := ea.envelope.AllFieldsMap()

	// Ensure required fields are present
	if _, ok := fields["series_code"]; !ok {
		return model.ParseResult{
			Confidence: model.Approx,
			Error:      fmt.Errorf("missing series_code"),
			Reason:     "evds_missing_series_code",
		}
	}

	if _, ok := fields["value"]; !ok {
		return model.ParseResult{
			Confidence: model.Approx,
			Error:      fmt.Errorf("missing value"),
			Reason:     "evds_missing_value",
		}
	}

	return model.ParseResult{
		Data:       fields,
		Confidence: model.Fresh,
	}
}
