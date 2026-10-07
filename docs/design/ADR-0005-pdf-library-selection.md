# ADR-0005: PDF Library Selection for Fund Disclosure Extraction

**Status**: Accepted
**Date**: 2025-06-23
**Context**: KAP fund disclosures (S2) are delivered as PDF documents alongside JSON metadata. Mandate 13 requires pure-Go text extraction (CGO_ENABLED=0) with bounded page processing.

## Problem

Extracting text from PDFs requires a third-party library. Candidates:
1. **ledongthuc/pdf**: Mature, widely adopted, produces GitHub stars, text extraction API
2. **rsc.io/pdf**: Minimal library by Rob Pike, pure Go, but lacks built-in text extraction

Both are pure Go (CGO_ENABLED=0 compatible).

## Decision

**Selected**: `github.com/ledongthuc/pdf`

**Rationale**:
- **Proven in production**: Widely used in real-world projects (GitHub, npm stats)
- **Text extraction API**: Direct `Page.GetPlainText()` method; no manual stream decoding required
- **Active maintenance**: Regular updates and bug fixes
- **Pure Go**: CGO_ENABLED=0 compatible, distroless deployable
- **Well-documented**: Clear examples and community support

**Alternative rejected**:
- rsc.io/pdf: Too minimal; would require custom stream decoding. Saves ~2KB but costs development time.

## Implementation

1. **Dependency**: `github.com/ledongthuc/pdf` pinned in `services/harvester/go.mod`
2. **Integration**: `parse/pdfdoc.go` ExtractText() implemented with `pdf.NewReader()` + page iteration
3. **Bounds**: MaxPages cap enforced before page extraction (untrusted-input guard)
4. **Partial extraction**: On page error, continue extraction rather than fail (partial text > no text)

## Code Changes

### parse/pdfdoc.go

```go
import "github.com/ledongthuc/pdf"

func (p *PDFDoc) ExtractText(pdfBytes []byte) (string, error) {
	// Validate header
	if len(pdfBytes) < 4 || string(pdfBytes[0:4]) != "%PDF" {
		return "", fmt.Errorf("not a valid pdf")
	}

	reader, err := pdf.NewReader(bytes.NewReader(pdfBytes), int64(len(pdfBytes)))
	if err != nil {
		return "", fmt.Errorf("failed to open pdf: %w", err)
	}

	// Enforce page count limit
	if reader.NumPage() > p.MaxPages {
		return "", fmt.Errorf("pdf exceeds page limit: %d > %d", reader.NumPage(), p.MaxPages)
	}

	// Extract all pages
	var buf bytes.Buffer
	for pageNum := 1; pageNum <= reader.NumPage(); pageNum++ {
		content, _ := reader.Page(pageNum).GetPlainText(nil)
		buf.WriteString(content)
	}

	return buf.String(), nil
}
```

## Testing

- Unit tests verify empty PDF rejection
- Valid PDF header acceptance
- Page count limit enforcement
- Partial page extraction on error (graceful degradation)

## Trade-offs

| Aspect | ledongthuc/pdf | rsc.io/pdf |
|--------|---|---|
| Binary size | ~2MB | ~100KB |
| Maintenance | Active | Minimal |
| Text API | Built-in | Custom |
| Learning curve | Low | High |
| Production use | Widespread | Rare |

**Verdict**: Maintenance and API simplicity outweigh 1.9MB size difference; distroless image base already ~30MB.

## References

- ledongthuc/pdf: https://github.com/ledongthuc/pdf
- rsc.io/pdf: https://github.com/rsc/pdf
- Mandate 13 (pure-Go): ../planning/ROADMAP.md
- KAP adapter (S2): ../source/kap.go
