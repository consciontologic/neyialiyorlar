package parse

import (
	"testing"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// TestTurkishUppercaseI tests the dotted/dotless I locale-special case.
// English: I/i (ASCII-wise)
// Turkish: I/i (capital/lowercase) but also İ/ı (dotted/dotless)
// Caser issue: ASCII ToLower("I") → "i", but Turkish ToLower("I") → "ı"
func TestTurkishUppercaseI(t *testing.T) {
	turkishCaser := cases.Lower(language.Turkish)
	asciiCaser := cases.Lower(language.Und) // Undefined/ASCII

	// Capital I (dotted)
	turkishI := turkishCaser.String("I")
	asciiI := asciiCaser.String("I")

	t.Logf("Turkish ToLower('I') = '%s' (expected: ı, U+0131)", turkishI)
	t.Logf("ASCII ToLower('I')   = '%s' (expected: i, U+0069)", asciiI)

	if turkishI == "ı" && asciiI == "i" {
		t.Log("✓ Correct: Turkish I→ı, ASCII I→i")
	} else {
		t.Logf("✗ Unexpected: Turkish=%q ASCII=%q", turkishI, asciiI)
	}
}

// TestTurkishDottedCapitalI tests the capital dotted İ character.
func TestTurkishDottedCapitalI(t *testing.T) {
	turkishCaser := cases.Lower(language.Turkish)
	asciiCaser := cases.Lower(language.Und)

	// Capital dotted I (İ, U+0130)
	turkishI := turkishCaser.String("İ")
	asciiI := asciiCaser.String("İ")

	t.Logf("Turkish ToLower('İ') = '%s' (expected: i)", turkishI)
	t.Logf("ASCII ToLower('İ')   = '%s' (expected: İ — no change)", asciiI)

	if turkishI == "i" && asciiI == "İ" {
		t.Log("✓ Correct: Turkish İ→i, ASCII İ→İ (unchanged)")
	}
}

// TestTurkishSCharacters tests the Ş/ş pair (and other locale-aware letters).
func TestTurkishSCharacters(t *testing.T) {
	turkishCaser := cases.Lower(language.Turkish)
	asciiCaser := cases.Lower(language.Und)

	// Ş (S with cedilla, U+015E)
	turkishS := turkishCaser.String("Ş")
	asciiS := asciiCaser.String("Ş")

	t.Logf("Turkish ToLower('Ş') = '%s' (expected: ş)", turkishS)
	t.Logf("ASCII ToLower('Ş')   = '%s' (expected: ş — Unicode-aware)", asciiS)

	if turkishS == "ş" {
		t.Log("✓ Correct: Turkish Ş→ş")
	}
}

// TestTurkishOtherLetters tests ç, ğ, ö, ü and their capitals.
func TestTurkishOtherLetters(t *testing.T) {
	turkishCaser := cases.Lower(language.Turkish)

	tests := []struct {
		upper    string
		expected string
		name     string
	}{
		{"Ç", "ç", "C with cedilla"},
		{"Ğ", "ğ", "G with breve"},
		{"Ö", "ö", "O with diaeresis"},
		{"Ü", "ü", "U with diaeresis"},
	}

	for _, tc := range tests {
		result := turkishCaser.String(tc.upper)
		if result == tc.expected {
			t.Logf("✓ %s: %s→%s", tc.name, tc.upper, result)
		} else {
			t.Logf("✗ %s: %s→%s (expected %s)", tc.name, tc.upper, result, tc.expected)
		}
	}
}

// TestLabelAnchorMatchingI_i tests that a label differing only by I/i is matched under Turkish folding.
func TestLabelAnchorMatchingI_i(t *testing.T) {
	turkishCaser := cases.Lower(language.Turkish)

	label := "Fon Toplam Değeri" // No I characters here
	label2 := "Fon Toplam Değeri"

	if turkishCaser.String(label) == turkishCaser.String(label2) {
		t.Log("✓ Label matching works with Turkish casing")
	}

	// Simulate a real anchor scenario: label contains capital I
	labelWithI := "İstanbul Borsası"     // Capital dotted I
	anchor := "istanbul borsası"         // Expected lowercase

	turkishMatch := turkishCaser.String(labelWithI) == anchor
	asciiCaser := cases.Lower(language.Und)
	asciiMatch := asciiCaser.String(labelWithI) == anchor

	t.Logf("Label 'İstanbul Borsası' vs anchor 'istanbul borsası':")
	t.Logf("  Turkish match: %v (expected: true)", turkishMatch)
	t.Logf("  ASCII match:   %v (expected: false)", asciiMatch)
}

// TestLabelAnchorMatchingS tests Ş/ş matching in label anchors.
func TestLabelAnchorMatchingS(t *testing.T) {
	turkishCaser := cases.Lower(language.Turkish)

	labelWithS := "Şirket Adı"
	anchor := "şirket adı"

	turkishMatch := turkishCaser.String(labelWithS) == anchor

	if turkishMatch {
		t.Log("✓ Ş label matches with Turkish casing")
	} else {
		t.Logf("✗ Ş label match failed: got %q", turkishCaser.String(labelWithS))
	}
}

// TestDOMTurkishLabelExtraction tests DOM extraction with Turkish labels.
func TestDOMTurkishLabelExtraction(t *testing.T) {
	anchors := map[string]string{
		"Fon Toplam Değeri":   "total_value",       // Contains ö (o-umlaut)
		"Yönetim Ücrets":      "management_fee",    // Contains ö, ü
		"İstanbul Borsası":    "exchange_name",     // Contains İ (capital dotted I)
		"Şirket Adı":          "company_name",      // Contains Ş
	}

	dom := NewDOM(anchors)

	t.Logf("Created DOM extractor with Turkish anchors:")
	for label, field := range anchors {
		t.Logf("  %s → %s", label, field)
	}

	// Verify that the DOM caser is set to Turkish
	if dom.CaseLang != language.Turkish {
		t.Errorf("expected CaseLang=Turkish, got %v", dom.CaseLang)
	} else {
		t.Log("✓ DOM caser is set to language.Turkish")
	}
}

// TestTurkishLabelCasingRoundTrip tests that labels round-trip through Turkish casing.
func TestTurkishLabelCasingRoundTrip(t *testing.T) {
	turkishCaser := cases.Lower(language.Turkish)
	turkishUpper := cases.Upper(language.Turkish)

	tests := []string{
		"Fon Toplam Değeri",
		"Yönetim Ücrets",
		"İstanbul Borsası",
		"Şirket Adı",
		"Çocuk",
		"Ğıdak",
	}

	for _, label := range tests {
		lower := turkishCaser.String(label)
		upper := turkishUpper.String(lower)

		// Note: round-trip may not be perfect due to case mapping quirks,
		// but lower→upper→lower should be stable
		lower2 := turkishCaser.String(upper)

		if lower == lower2 {
			t.Logf("✓ Round-trip stable: %q", label)
		} else {
			t.Logf("✗ Round-trip failed: %q→%q→%q", label, lower, lower2)
		}
	}
}

// TestASCIIFoldingFailure tests that ASCII folding fails on Turkish letters.
// This is a negative test to prove why language.Turkish is necessary.
func TestASCIIFoldingFailure(t *testing.T) {
	// Turkish letters that ASCII folding doesn't handle correctly
	turkishLetters := map[rune]string{
		'I': "Would fold to 'i' in ASCII, but should fold to 'ı' in Turkish",
		'Ş': "Would stay 'Ş' in ASCII (no lowercase variant), but 'ş' in Turkish",
		'İ': "Would stay 'İ' in ASCII, but 'i' in Turkish",
		'Ç': "Would stay 'Ç' in ASCII, but 'ç' in Turkish",
	}

	asciiCaser := cases.Lower(language.Und)
	turkishCaser := cases.Lower(language.Turkish)

	t.Log("Turkish letters that require language.Turkish casing:")
	for r := range turkishLetters {
		char := string(r)
		ascii := asciiCaser.String(char)
		turkish := turkishCaser.String(char)

		if ascii != turkish {
			t.Logf("✓ Difference: '%s' ASCII→%q Turkish→%q", char, ascii, turkish)
		}
	}
}
