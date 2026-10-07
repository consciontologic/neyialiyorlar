package parse

import (
	"testing"

	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// TestPDFDocValidHeader tests PDF header validation.
func TestPDFDocValidHeader(t *testing.T) {
	pdf := []byte("%PDF-1.4\n%fake pdf content")

	extractor := NewPDFDoc(100, map[string]string{
		"nav": "Net Asset Value",
	})

	result := extractor.Parse(pdf)

	if result.Confidence != model.Fresh {
		t.Errorf("expected Fresh confidence for valid PDF header, got %v", result.Confidence)
	}

	if result.Error != nil {
		t.Logf("Note: Parse returned error (expected in spike phase): %v", result.Error)
	}
}

// TestPDFDocInvalidHeader tests rejection of non-PDF bytes.
func TestPDFDocInvalidHeader(t *testing.T) {
	notPDF := []byte("This is not a PDF at all")

	extractor := NewPDFDoc(100, map[string]string{
		"field": "Label",
	})

	result := extractor.Parse(notPDF)

	if result.Confidence != model.Stale {
		t.Errorf("expected Stale confidence for non-PDF, got %v", result.Confidence)
	}

	if result.Reason != "pdf_invalid" {
		t.Errorf("expected pdf_invalid reason, got %v", result.Reason)
	}
}

// TestPDFDocEmptyBytes tests empty input handling.
func TestPDFDocEmptyBytes(t *testing.T) {
	extractor := NewPDFDoc(100, map[string]string{})

	result := extractor.Parse([]byte{})

	if result.Confidence != model.Stale {
		t.Errorf("expected Stale confidence for empty PDF, got %v", result.Confidence)
	}

	if result.Reason != "pdf_empty" {
		t.Errorf("expected pdf_empty reason, got %v", result.Reason)
	}
}

// TestPDFDocMaxPages tests page count cap (untrusted-input guard).
func TestPDFDocMaxPages(t *testing.T) {
	extractor := NewPDFDoc(5, map[string]string{})

	// Note: full page validation requires library implementation
	// This test documents the guard's intent

	if extractor.MaxPages != 5 {
		t.Errorf("expected MaxPages=5, got %d", extractor.MaxPages)
	}
}

// TestPDFDocDefaultMaxPages tests default page cap.
func TestPDFDocDefaultMaxPages(t *testing.T) {
	extractor := NewPDFDoc(0, map[string]string{}) // 0 should use default

	if extractor.MaxPages <= 0 {
		t.Errorf("expected positive default MaxPages, got %d", extractor.MaxPages)
	}

	if extractor.MaxPages != 100 {
		t.Errorf("expected MaxPages=100 default, got %d", extractor.MaxPages)
	}
}

// TestPDFDocExtractText tests text extraction interface.
func TestPDFDocExtractText(t *testing.T) {
	pdf := []byte("%PDF-1.4\nfake content")
	extractor := NewPDFDoc(100, map[string]string{})

	text, err := extractor.ExtractText(pdf)

	if err != nil {
		t.Logf("ExtractText returned error (expected in spike): %v", err)
	}

	if text == "" {
		t.Logf("ExtractText returned empty string (expected in spike phase)")
	}
}

// TestPDFDocExtractTextEmpty tests empty input to ExtractText.
func TestPDFDocExtractTextEmpty(t *testing.T) {
	extractor := NewPDFDoc(100, map[string]string{})

	_, err := extractor.ExtractText([]byte{})

	if err == nil {
		t.Errorf("expected error for empty PDF in ExtractText")
	}
}
