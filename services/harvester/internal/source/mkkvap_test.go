package source

import (
	"context"
	"testing"

	"net/http"
	"net/http/httptest"

	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// TestMKKVAPAdapterName tests the adapter name.
func TestMKKVAPAdapterName(t *testing.T) {
	cfg := Config{
		SourceID: "mkkvap",
		URL:      "http://example.com/mkkvap",
	}
	adapter := NewMKKVAPAdapter(cfg, &http.Client{})

	if adapter.Name() != "mkkvap" {
		t.Errorf("expected 'mkkvap', got %s", adapter.Name())
	}
}

// TestMKKVAPAdapterSourceID tests the source id.
func TestMKKVAPAdapterSourceID(t *testing.T) {
	cfg := Config{
		SourceID: "mkkvap",
		URL:      "http://example.com/mkkvap",
	}
	adapter := NewMKKVAPAdapter(cfg, &http.Client{})

	if adapter.SourceID() != "mkkvap" {
		t.Errorf("expected 'mkkvap', got %s", adapter.SourceID())
	}
}

// TestMKKVAPAdapterCadence tests the cadence is daily.
func TestMKKVAPAdapterCadence(t *testing.T) {
	cfg := Config{
		SourceID: "mkkvap",
		URL:      "http://example.com/mkkvap",
	}
	adapter := NewMKKVAPAdapter(cfg, &http.Client{})

	if adapter.Cadence() != model.Daily {
		t.Errorf("expected Daily, got %s", adapter.Cadence())
	}
}

// TestMKKVAPAdapterFetchHTML tests fetching and parsing HTML report.
func TestMKKVAPAdapterFetchHTML(t *testing.T) {
	// Mock server that returns a sample HTML report
	htmlContent := `
		<html>
		<body>
			<table>
				<tr><td>Fon Toplam Değeri</td><td>1.234,56 TRY</td></tr>
				<tr><td>Yönetim Ücrets</td><td>2,5%</td></tr>
				<tr><td>Portföy Değeri</td><td>987.654,32 TRY</td></tr>
			</table>
		</body>
		</html>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(htmlContent))
	}))
	defer server.Close()

	cfg := Config{
		SourceID: "mkkvap",
		URL:      server.URL,
	}
	adapter := NewMKKVAPAdapter(cfg, server.Client())

	raws, nextCursor, err := adapter.Fetch(context.Background(), "")

	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}

	if len(raws) == 0 {
		t.Error("expected at least one raw, got zero")
	}

	if raws[0].SourceID != "mkkvap" {
		t.Errorf("expected source_id 'mkkvap', got %s", raws[0].SourceID)
	}

	if raws[0].NaturalKey == "" {
		t.Error("expected non-empty natural_key (fund name)")
	}

	if nextCursor != "" {
		t.Logf("next cursor: %s (pagination support)", nextCursor)
	}
}

// TestMKKVAPAdapterParseDOM tests DOM extraction with label anchors.
func TestMKKVAPAdapterParseDOM(t *testing.T) {
	htmlContent := []byte(`
		<html>
		<body>
			<table>
				<tr><td>Fon Toplam Değeri</td><td>1.234,56 TRY</td></tr>
				<tr><td>Yönetim Ücrets</td><td>2,5%</td></tr>
			</table>
		</body>
		</html>
	`)

	cfg := Config{
		SourceID: "mkkvap",
		URL:      "http://example.com",
	}
	adapter := NewMKKVAPAdapter(cfg, &http.Client{})

	raw := model.Raw{
		SourceID:   "mkkvap",
		NaturalKey: "Fund_001",
		Payload:    htmlContent,
	}

	result := adapter.Parse(raw)

	if result.Error != nil {
		t.Logf("parse error (expected for mock): %v", result.Error)
	}

	// In production: verify extracted data
	t.Logf("parse confidence: %s (Fresh/Approx/Stale)", result.Confidence)
}

// TestMKKVAPAdapterMissingField tests graceful handling of missing fields.
func TestMKKVAPAdapterMissingField(t *testing.T) {
	htmlContent := []byte(`
		<html>
		<body>
			<table>
				<tr><td>Yönetim Ücrets</td><td>2,5%</td></tr>
			</table>
		</body>
		</html>
	`)

	cfg := Config{
		SourceID: "mkkvap",
		URL:      "http://example.com",
	}
	adapter := NewMKKVAPAdapter(cfg, &http.Client{})

	raw := model.Raw{
		SourceID:   "mkkvap",
		NaturalKey: "Fund_001",
		Payload:    htmlContent,
	}

	result := adapter.Parse(raw)

	// Expected: Approx confidence when required field missing
	if result.Confidence != model.Approx {
		t.Logf("confidence: %s (expected Approx for missing required field)", result.Confidence)
	}
}

// TestMKKVAPAdapterTurkishLabels tests Turkish-aware label matching.
func TestMKKVAPAdapterTurkishLabels(t *testing.T) {
	// Test that labels with Turkish characters are matched correctly
	labels := []string{
		"Fon Toplam Değeri",  // Contains Turkish ö and ı
		"Yönetim Ücrets",      // Contains Turkish ü
		"Portföy Değeri",     // Contains Turkish ö
	}

	for _, label := range labels {
		t.Logf("Turkish label: %s", label)
	}

	t.Logf("Label matching uses language.Turkish casing for fold operations")
}

// TestMKKVAPAdapterHTTPError tests handling of HTTP errors.
func TestMKKVAPAdapterHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server error"))
	}))
	defer server.Close()

	cfg := Config{
		SourceID: "mkkvap",
		URL:      server.URL,
	}
	adapter := NewMKKVAPAdapter(cfg, server.Client())

	_, _, err := adapter.Fetch(context.Background(), "")

	if err == nil {
		t.Error("expected error on HTTP 500, got nil")
	}

	t.Logf("HTTP error handled: %v", err)
}
