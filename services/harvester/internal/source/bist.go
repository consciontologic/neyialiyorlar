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

// BISTAdapter fetches BIST index data (S1 — JSON via HTTP GET).
// Rationale: BIST publishes index values in JSON; envelope guard tolerates field drift.
type BISTAdapter struct {
	cfg      Config
	httpClient *http.Client
	envelope *parse.Envelope
}

// NewBISTAdapter creates a BIST data source adapter.
func NewBISTAdapter(cfg Config, httpClient *http.Client) *BISTAdapter {
	// Define expected JSON fields for BIST endpoints
	fieldNames := map[string]string{
		"index_name":   "indexname",
		"value":        "value",
		"previous":     "previous",
		"change":       "chg",
		"change_pct":   "chgpct",
		"timestamp":    "time",
	}
	requiredFields := []string{"index_name", "value"}

	return &BISTAdapter{
		cfg:        cfg,
		httpClient: httpClient,
		envelope: parse.NewEnvelope(fieldNames, requiredFields),
	}
}

// Name returns the source identifier.
func (ba *BISTAdapter) Name() string {
	return "bist"
}

// SourceID returns the unique identifier for this source.
func (ba *BISTAdapter) SourceID() string {
	return ba.cfg.SourceID
}

// Cadence returns the ingestion frequency.
func (ba *BISTAdapter) Cadence() model.Cadence {
	return ba.cfg.Cadence
}

// Fetch retrieves index data from BIST API.
// Rationale: BIST endpoints are paginated via query parameters.
// The cursor is an opaque token; on initial fetch it's empty.
func (ba *BISTAdapter) Fetch(ctx context.Context, cursor string) ([]model.Raw, string, error) {
	// Construct request URL
	// SPIKE: Real BIST endpoint is undisclosed (scraped); using skeleton
	url := ba.cfg.URL
	if cursor != "" {
		// Append cursor as query param (pagination mechanism depends on BIST API)
		url = fmt.Sprintf("%s&op=%s", url, cursor)
	}

	// Perform fetch via httpClient (which handles backoff, circuit breaker, etc.)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("bist fetch: create request: %w", err)
	}

	resp, err := ba.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("bist fetch: http do: %w", err)
	}
	defer resp.Body.Close()

	// Read response body (respects max_payload_bytes via SafeReader)
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("bist fetch: read body: %w", err)
	}

	// Guard: validate envelope shape
	parseResult := ba.envelope.Validate(payload)
	if parseResult.Error != nil {
		return nil, "", fmt.Errorf("bist fetch: envelope guard: %w", parseResult.Error)
	}

	// Parse the data array
	// SPIKE: Real BIST adapter would extract index records from the data array
	fields := ba.envelope.AllFieldsMap()
	raw := model.Raw{
		SourceID:   ba.cfg.SourceID,
		NaturalKey: fmt.Sprintf("%v", fields["index_name"]), // Use index name as key
		Payload:    payload,
		FetchedAt:  time.Now(),
	}

	// SPIKE: Next cursor depends on BIST API pagination; for now, empty (no more pages)
	nextCursor := ""

	return []model.Raw{raw}, nextCursor, nil
}

// Parse processes BIST JSON into derived fields.
func (ba *BISTAdapter) Parse(raw model.Raw) model.ParseResult {
	result := ba.envelope.Validate(raw.Payload)
	if result.Error != nil {
		// Validation failed; quarantine
		return model.ParseResult{
			Confidence: model.Stale,
			Error:      result.Error,
			Reason:     "bist_envelope_error",
		}
	}

	// Extract fields
	fields := ba.envelope.AllFieldsMap()

	// Ensure required fields are present
	if _, ok := fields["index_name"]; !ok {
		return model.ParseResult{
			Confidence: model.Approx,
			Error:      fmt.Errorf("missing index_name"),
			Reason:     "bist_missing_index_name",
		}
	}

	// Return fresh data
	return model.ParseResult{
		Data:       fields,
		Confidence: model.Fresh,
	}
}
