package parse

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ParseTurkish decodes a Turkish-formatted number (dot-thousands, comma-decimal)
// into a float64. Examples: "1.234,56" → 1234.56, "123,4" → 123.4.
// Rationale (mandate 22): Turkish sources render numbers with dot as thousands
// separator and comma as decimal. A naive strconv.ParseFloat mis-scales such
// values by ~10³ or errors outright. This parser corrects the interpretation.
//
// Algorithm:
// 1. Remove whitespace.
// 2. If contains both '.' and ',' locate which is the decimal: the rightmost is.
// 3. Strip thousands separators, replace decimal with '.' for strconv.
// 4. Parse via strconv.ParseFloat.
func ParseTurkish(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}

	// Normalize: remove spaces
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)

	// Find positions of '.' and ','
	dotIdx := strings.LastIndex(s, ".")
	commaIdx := strings.LastIndex(s, ",")

	// Determine which is the decimal separator (the rightmost wins)
	var decimal, thousands rune
	if dotIdx > commaIdx {
		// "1.234" or "1.234,56" — dot is thousands, comma is decimal
		// or "1.234" — dot is decimal
		decimal = '.'
		thousands = ','
	} else if commaIdx > dotIdx {
		// "1,234" or "1,234.56" — comma is thousands, dot is decimal
		// or "1,234" — comma is decimal
		decimal = ','
		thousands = '.'
	} else if dotIdx >= 0 {
		// Only dot present
		decimal = '.'
		thousands = ','
	} else if commaIdx >= 0 {
		// Only comma present
		decimal = ','
		thousands = '.'
	} else {
		// No separator; must be an integer
		return strconv.ParseFloat(s, 64)
	}

	// Heuristic: if the rightmost separator comes after 3 digits from the right,
	// it's more likely a thousands separator (e.g., "1.000" = thousand, not 1000.0).
	// If it's the last or second-to-last position, it's a decimal.
	rightmostSepIdx := dotIdx
	if commaIdx > dotIdx {
		rightmostSepIdx = commaIdx
	}

	digitsAfterRightmost := len(s) - rightmostSepIdx - 1
	isMostlyDecimal := digitsAfterRightmost <= 2 // 1 or 2 digits after → likely decimal

	if !isMostlyDecimal {
		// More than 2 digits after: this is a thousands separator.
		// Swap interpretation.
		decimal, thousands = thousands, decimal
	}

	// Strip thousands separators, replace decimal with '.'
	s = strings.ReplaceAll(s, string(thousands), "")
	s = strings.ReplaceAll(s, string(decimal), ".")

	return strconv.ParseFloat(s, 64)
}
