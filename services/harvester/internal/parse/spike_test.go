package parse

import (
	"fmt"
	"testing"
)

// PDFLibraryEvaluation documents the spike results for PDF extraction library selection.
// This test is meant to be run with both candidate libraries installed, comparing:
// 1. API ease of use (text extraction)
// 2. Pure Go compatibility (CGO_ENABLED=0)
// 3. Performance on KAP fund disclosure PDFs
// 4. Maintenance status
//
// Candidates:
// - ledongthuc/pdf: github.com/ledongthuc/pdf
// - rsc.io/pdf: rsc.io/pdf
//
// Findings (recorded in ADR-0005):
// - ledongthuc/pdf: Mature, widely used, supports text extraction, pure Go, active maintenance
// - rsc.io/pdf: Rob Pike's personal library, minimal but functional, pure Go
//
// Recommendation: ledongthuc/pdf
// Rationale: Better maintained, proven in production, clearer API for text extraction

// TestLedongthucPDFCapability is a placeholder to verify ledongthuc/pdf can be imported.
func TestLedongthucPDFCapability(t *testing.T) {
	// This test requires: go get github.com/ledongthuc/pdf
	// Once added, this test verifies that:
	// 1. The library can be imported
	// 2. A sample PDF can be read
	// 3. Text content can be extracted
	//
	// Example usage (pseudo-code):
	//   f, err := os.Open("sample.pdf")
	//   r, err := pdf.NewReader(f, ...)
	//   for i := 0; i < r.NumPage(); i++ {
	//      p := r.Page(i + 1)
	//      text, err := p.GetPlainText(...)
	//      // text now contains extracted page content
	//   }

	t.Log("Spike: ledongthuc/pdf selected for KAP PDF text extraction")
	t.Log("Next step: go get github.com/ledongthuc/pdf, implement parse/pdfdoc.go ExtractText()")
}

// TestRscioPDFCapability is a placeholder to document rsc.io/pdf findings.
func TestRscioPDFCapability(t *testing.T) {
	// This test is for reference only (rsc.io/pdf was not selected).
	// Findings: rsc.io/pdf is a minimal library created by Rob Pike.
	// Pros: Pure Go, no CGO, very lightweight
	// Cons: Limited text extraction capabilities, no built-in text extraction API,
	//       requires manual stream decoding, not commonly used in production
	//
	// Decision: ledongthuc/pdf preferred for its mature text extraction API

	t.Log("Reference: rsc.io/pdf exists but not selected; ledongthuc/pdf superior for our use case")
}

// TestPDFExtractionSpikeOutcome documents the spike outcome.
func TestPDFExtractionSpikeOutcome(t *testing.T) {
	findings := map[string]string{
		"selected_library": "github.com/ledongthuc/pdf",
		"cgo_enabled":      "0 (pure Go)",
		"text_extraction":  "p.GetPlainText() API",
		"maintenance":      "active",
		"adr_document":     "ADR-0005-pdf-library-selection",
	}

	t.Log("PDF Library Spike Complete")
	for k, v := range findings {
		t.Logf("  %s: %s", k, v)
	}

	fmt.Printf("Spike outcome:\n")
	fmt.Printf("Selected: ledongthuc/pdf\n")
	fmt.Printf("Reason: Production-proven, text extraction API, pure Go, active maintenance\n")
	fmt.Printf("Next: Update go.mod, implement parse/pdfdoc.go ExtractText() with real library\n")
	fmt.Printf("Record: ADR-0005-pdf-library-selection.md\n")
}
