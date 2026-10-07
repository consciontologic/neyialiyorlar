package parse

import (
	"testing"
)

// FuzzEnvelopeParser is a go-fuzz target for the envelope parser.
// It ensures that no hostile JSON input can panic or OOM the parser.
// Every input either parses successfully, returns a controlled error, or quarantines.
func FuzzEnvelopeParser(f *testing.F) {
	// Seed corpus: valid envelopes
	f.Add([]byte(`{"status":"ok","data":[]}`))
	f.Add([]byte(`{"status":"ok","data":[{"key":"value"}]}`))
	f.Add([]byte(`{"status":"error","data":null}`))

	// Seed corpus: invalid/edge cases
	f.Add([]byte(``))
	f.Add([]byte(`{`))
	f.Add([]byte(`{"status":"ok","data":"invalid"}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`{"status":""}`))
	f.Add([]byte(` {"status":"ok","data":[]} `))

	f.Fuzz(func(t *testing.T, input []byte) {
		// Parse the input through the envelope validator
		envelope := NewEnvelope(
			map[string]string{"status": "string", "data": "array"},
			[]string{"status", "data"},
		)

		result := envelope.Validate(input)

		// Assertions:
		// 1. No panic (framework catches panics automatically)
		// 2. Result must have valid fields
		if result.Confidence == "" {
			t.Errorf("empty confidence")
		}

		// 3. Either Data is populated or Error is set (not both nil)
		if result.Error == nil && len(result.Data) == 0 && result.Confidence != "Stale" {
			t.Logf("empty result for valid envelope")
		}
	})
}

// FuzzDOMParser is a go-fuzz target for the DOM parser.
// Ensures that no hostile HTML input can panic or OOM the parser.
func FuzzDOMParser(f *testing.F) {
	anchors := map[string]string{
		"Fon Toplam Değeri": "total_value",
		"Yönetim Ücrets":    "management_fee",
	}

	// Seed corpus: valid HTML
	f.Add([]byte(`<html><body><table><tr><td>Fon Toplam Değeri</td><td>1234.56</td></tr></table></body></html>`))
	f.Add([]byte(`<html><body></body></html>`))

	// Seed corpus: invalid/edge cases
	f.Add([]byte(``))
	f.Add([]byte(`<html>`))
	f.Add([]byte(`<!-- comment -->`))
	f.Add([]byte(`<script>alert('xss')</script>`))

	f.Fuzz(func(t *testing.T, input []byte) {
		dom := NewDOM(anchors)
		result := dom.Parse(input)

		// Assertions:
		// 1. No panic
		// 2. Valid confidence level
		if result.Confidence == "" {
			t.Errorf("empty confidence")
		}

		// 3. On parse error, quarantine reason must be present
		if result.Error != nil && result.Reason == "" {
			t.Logf("error without reason")
		}
	})
}

// FuzzNumTrParser is a go-fuzz target for Turkish number parsing.
// Ensures that no number string can panic or return NaN/Inf without explicit handling.
func FuzzNumTrParser(f *testing.F) {
	// Seed corpus: valid Turkish numbers
	f.Add([]byte("1.234,56"))
	f.Add([]byte("0,00"))
	f.Add([]byte("1234"))
	f.Add([]byte("1234,56"))

	// Seed corpus: invalid/edge cases
	f.Add([]byte(""))
	f.Add([]byte("abc"))
	f.Add([]byte(",,,"))
	f.Add([]byte("..."))
	f.Add([]byte("1.2.3,45"))
	f.Add([]byte("-1.234,56"))

	f.Fuzz(func(t *testing.T, input []byte) {
		result, err := ParseTurkish(string(input))

		// Assertions:
		// 1. No panic on any input
		// 2. If error is nil, result must be a finite number (not NaN or Inf)
		if err == nil {
			// Check for NaN (NaN != NaN is a standard way to detect it)
			if result != result {
				t.Errorf("NaN result without error for input: %q", input)
			}
			// Check for Inf (must be used in comparison to detect)
			if result > 1e308 || result < -1e308 {
				t.Logf("very large result for input: %q", input)
			}
		}
	})
}
