package parse

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ledongthuc/pdf"
	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// PDFDoc provides PDF text extraction for fund disclosures (KAP).
// Rationale (mandate 13): pure-Go extraction (CGO_ENABLED=0). Anchored on
// stable headers; layout drift quarantines the raw, never crashes.
// Library selection is post-spike (candidates: ledongthuc/pdf, rsc.io/pdf).
type PDFDoc struct {
	// MaxPages caps the page count to prevent OOM (untrusted-input guard)
	MaxPages int

	// Anchors: map of field name -> header text to search for
	// Example: {"nav_value": "Net Asset Value", "report_date": "As of"}
	Anchors map[string]string
}

// NewPDFDoc creates a PDF extractor with bounded page processing.
func NewPDFDoc(maxPages int, anchors map[string]string) *PDFDoc {
	if maxPages <= 0 {
		maxPages = 100 // Default cap
	}
	return &PDFDoc{
		MaxPages: maxPages,
		Anchors:  anchors,
	}
}

// Parse extracts fields from a PDF document.
// Returns a ParseResult with extracted fields or an error reason.
// On layout drift or extraction failure, returns approx confidence with quarantine reason.
func (p *PDFDoc) Parse(pdfBytes []byte) model.ParseResult {
	// SPIKE: This is a skeleton implementation.
	// Full implementation depends on library selection:
	// - ledongthuc/pdf: github.com/ledongthuc/pdf
	// - rsc.io/pdf: golang.org/x/tools/cmd/pdfsearch

	// For now, return a mock result to allow compilation.
	// Production implementation will:
	// 1. Open the PDF
	// 2. Validate page count <= MaxPages
	// 3. Extract all text per page
	// 4. Search for anchors in extracted text
	// 5. On error, return approx + quarantine reason

	if len(pdfBytes) == 0 {
		return model.ParseResult{
			Confidence: model.Stale,
			Error:      fmt.Errorf("empty pdf"),
			Reason:     "pdf_empty",
		}
	}

	// Placeholder: return Fresh if we can identify PDF header
	if len(pdfBytes) >= 4 && string(pdfBytes[0:4]) == "%PDF" {
		data := make(map[string]interface{})
		// Placeholder: would extract fields here after library selection
		for fieldName := range p.Anchors {
			data[fieldName] = "[extracted from PDF]"
		}
		return model.ParseResult{
			Data:       data,
			Confidence: model.Fresh,
		}
	}

	return model.ParseResult{
		Confidence: model.Stale,
		Error:      fmt.Errorf("not a pdf: invalid header"),
		Reason:     "pdf_invalid",
	}
}

// ExtractText returns all text from the PDF using ledongthuc/pdf library.
// Page count is bounded by MaxPages (untrusted-input guard).
// On extraction error, returns empty string with error (parse fails, quarantines raw).
func (p *PDFDoc) ExtractText(pdfBytes []byte) (string, error) {
	if len(pdfBytes) == 0 {
		return "", fmt.Errorf("empty pdf")
	}

	// Validate PDF header
	if len(pdfBytes) < 4 || string(pdfBytes[0:4]) != "%PDF" {
		return "", fmt.Errorf("not a valid pdf: invalid header")
	}

	// Open PDF from bytes
	reader, err := pdf.NewReader(bytes.NewReader(pdfBytes), int64(len(pdfBytes)))
	if err != nil {
		return "", fmt.Errorf("failed to open pdf: %w", err)
	}

	// Enforce page count limit
	numPages := reader.NumPage()
	if numPages > p.MaxPages {
		return "", fmt.Errorf("pdf exceeds page limit: %d > %d", numPages, p.MaxPages)
	}

	// Extract text from all pages
	var buf bytes.Buffer

	for pageNum := 1; pageNum <= numPages; pageNum++ {
		page := reader.Page(pageNum)

		// Get plaintext from page
		content, err := page.GetPlainText(nil)
		if err != nil {
			// Log but continue extraction (partial text is better than failure)
			fmt.Printf("warning: failed to extract page %d: %v\n", pageNum, err)
			continue
		}

		if _, err := io.WriteString(&buf, content); err != nil {
			return "", fmt.Errorf("failed to write page %d text: %w", pageNum, err)
		}
		buf.WriteString("\n---PAGE---\n")
	}

	return buf.String(), nil
}
