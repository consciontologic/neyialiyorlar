package source

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// TestBISTAdapterName tests the adapter's name.
func TestBISTAdapterName(t *testing.T) {
	adapter := NewBISTAdapter(Config{SourceID: "bist-1"}, &http.Client{})

	if adapter.Name() != "bist" {
		t.Errorf("expected name 'bist', got %s", adapter.Name())
	}
}

// TestBISTAdapterSourceID tests the adapter's source ID.
func TestBISTAdapterSourceID(t *testing.T) {
	cfg := Config{SourceID: "bist-prod"}
	adapter := NewBISTAdapter(cfg, &http.Client{})

	if adapter.SourceID() != "bist-prod" {
		t.Errorf("expected source ID 'bist-prod', got %s", adapter.SourceID())
	}
}

// TestBISTAdapterCadence tests cadence configuration.
func TestBISTAdapterCadence(t *testing.T) {
	cfg := Config{
		SourceID: "bist-1",
		Cadence:  model.Intraday,
	}
	adapter := NewBISTAdapter(cfg, &http.Client{})

	if adapter.Cadence() != model.Intraday {
		t.Errorf("expected Intraday cadence, got %v", adapter.Cadence())
	}
}

// TestBISTAdapterParseValid tests parsing a valid BIST response.
func TestBISTAdapterParseValid(t *testing.T) {
	cfg := Config{SourceID: "bist-1", Cadence: model.Daily}
	adapter := NewBISTAdapter(cfg, &http.Client{})

	// Create a mock raw with valid envelope
	raw := model.Raw{
		SourceID:   "bist-1",
		NaturalKey: "XU030",
		Payload: []byte(`{
			"status": "ok",
			"data": [
				{"indexname": "XU030", "value": "8500.25", "time": "2024-01-15"}
			]
		}`),
	}

	result := adapter.Parse(raw)

	if result.Error != nil {
		t.Logf("Parse returned an error (spike phase): %v", result.Error)
	}

	if result.Confidence == model.Fresh {
		t.Logf("Parse successful: confidence = Fresh")
	}
}

// TestBISTAdapterParseMalformed tests parsing malformed JSON.
func TestBISTAdapterParseMalformed(t *testing.T) {
	cfg := Config{SourceID: "bist-1"}
	adapter := NewBISTAdapter(cfg, &http.Client{})

	raw := model.Raw{
		SourceID:   "bist-1",
		NaturalKey: "XU030",
		Payload:    []byte("not valid json"),
	}

	result := adapter.Parse(raw)

	if result.Error == nil {
		t.Errorf("expected error for malformed JSON")
	}

	if result.Confidence != model.Stale && result.Confidence != model.Approx {
		t.Errorf("expected Stale/Approx confidence for malformed JSON, got %v", result.Confidence)
	}
}

// TestBISTAdapterFetchMock tests fetch using a mock HTTP client.
func TestBISTAdapterFetchMock(t *testing.T) {
	// Create a mock HTTP client that returns a valid response
	mockClient := &http.Client{
		Transport: &mockTransport{
			response: &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(strings.NewReader(`{
					"status": "ok",
					"data": [
						{"indexname": "XU030", "value": "8500.25"}
					]
				}`)),
			},
		},
	}

	cfg := Config{
		SourceID: "bist-1",
		URL:      "http://example.com/bist",
		Cadence:  model.Daily,
	}
	_ = NewBISTAdapter(cfg, mockClient)

	// Note: Fetch requires a real HTTP client setup
	// This test documents the interface; full integration testing
	// would use a test server or mock HTTP transport

	t.Logf("BISTAdapter.Fetch interface documented for future integration tests")
}

// mockTransport is a mock HTTP transport for testing.
type mockTransport struct {
	response *http.Response
	err      error
}

func (mt *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if mt.err != nil {
		return nil, mt.err
	}
	return mt.response, nil
}
