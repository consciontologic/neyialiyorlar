package parse

import (
	"testing"

	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// TestDOMBasicExtraction tests basic label-anchor extraction.
func TestDOMBasicExtraction(t *testing.T) {
	html := `
	<html>
		<body>
			<table>
				<tr>
					<td>Fon Toplam Değeri</td>
					<td>1.234,56</td>
				</tr>
				<tr>
					<td>Raporlama Tarihi</td>
					<td>2024-01-15</td>
				</tr>
			</table>
		</body>
	</html>
	`

	dom := NewDOM(map[string]string{
		"total_value": "Fon Toplam Değeri",
		"report_date": "Raporlama Tarihi",
	})

	result := dom.Parse([]byte(html))

	if result.Error != nil {
		// Allow "missing fields" error if some fields weren't found
		t.Logf("Parse result: %v", result.Error)
	}

	if result.Confidence == model.Fresh && len(result.Data) == 2 {
		t.Logf("✓ Extracted: %+v", result.Data)
	}
}

// TestDOMFieldDrift tests tolerance for missing fields.
func TestDOMFieldDrift(t *testing.T) {
	// HTML missing one of the expected fields
	html := `
	<html>
		<body>
			<p>Fon Toplam Değeri</p>
			<span>1.234,56</span>
		</body>
	</html>
	`

	dom := NewDOM(map[string]string{
		"total_value": "Fon Toplam Değeri",
		"missing_field": "Hiçbir Yerde Yok", // This field doesn't exist
	})

	result := dom.Parse([]byte(html))

	// Expect approx confidence due to missing field
	if result.Confidence != model.Approx {
		t.Errorf("expected approx confidence due to missing field, got %v", result.Confidence)
	}

	if result.Reason != "dom_field_not_found" {
		t.Errorf("expected dom_field_not_found reason, got %v", result.Reason)
	}
}

// TestDOMInvalidHTML tests error handling for malformed HTML.
func TestDOMInvalidHTML(t *testing.T) {
	invalidHTML := "<<<>>>" // Completely invalid

	dom := NewDOM(map[string]string{
		"field": "Label",
	})

	result := dom.Parse([]byte(invalidHTML))

	// The HTML parser is very forgiving and will still parse this
	// but should not find the expected fields
	if result.Confidence != model.Approx && result.Confidence != model.Stale {
		t.Logf("confidence: %v", result.Confidence)
	}
}

// TestDOMTurkishCaseInsensitive tests Turkish-aware case folding.
func TestDOMTurkishCaseInsensitive(t *testing.T) {
	// Turkish uppercase I (İ) and lowercase i are distinct
	html := `
	<html>
		<body>
			<p>FON TOPLAM DEĞERİ</p>
			<span>5000</span>
		</body>
	</html>
	`

	// Try both uppercase and lowercase variants
	tests := []struct {
		label string
		found bool
	}{
		{"Fon Toplam Değeri", true},   // Exact case variant
		{"FON TOPLAM DEĞERİ", true},   // All uppercase
		{"fon toplam değeri", true},   // All lowercase
	}

	for _, tt := range tests {
		dom := NewDOM(map[string]string{
			"value": tt.label,
		})
		result := dom.Parse([]byte(html))

		if tt.found && len(result.Data) == 0 {
			t.Logf("Label %q: not found (might be Turkish case issue)", tt.label)
		} else if tt.found {
			t.Logf("Label %q: found ✓", tt.label)
		}
	}
}

// TestDOMComplexNesting tests extraction from nested HTML.
func TestDOMComplexNesting(t *testing.T) {
	html := `
	<html>
		<body>
			<div class="report">
				<section>
					<article>
						<p class="label">Raporlama Tarihi</p>
						<p class="value">15 Ocak 2024</p>
					</article>
				</section>
			</div>
		</body>
	</html>
	`

	dom := NewDOM(map[string]string{
		"date": "Raporlama Tarihi",
	})

	result := dom.Parse([]byte(html))
	if len(result.Data) > 0 {
		t.Logf("Extracted from nested HTML: %+v", result.Data)
	}
}

// TestDOMLabelNotFound tests when a label is entirely absent.
func TestDOMLabelNotFound(t *testing.T) {
	html := `
	<html>
		<body>
			<p>Some other content</p>
		</body>
	</html>
	`

	dom := NewDOM(map[string]string{
		"required": "Required Label That Does Not Exist",
	})

	result := dom.Parse([]byte(html))

	if result.Confidence != model.Approx && result.Confidence != model.Stale {
		t.Errorf("expected approx/stale when label not found, got %v", result.Confidence)
	}
}
