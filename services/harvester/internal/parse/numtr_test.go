package parse

import (
	"math"
	"testing"
)

// TestParseTurkishBasic tests common Turkish number formats.
func TestParseTurkishBasic(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		// Standard Turkish format: dot = thousands, comma = decimal
		{input: "1.234,56", expected: 1234.56},
		{input: "1.000,00", expected: 1000.00},
		{input: "10,5", expected: 10.5},
		{input: "123,4", expected: 123.4},
		{input: "1.234.567,89", expected: 1234567.89},

		// Edge cases: only one separator
		{input: "1000", expected: 1000.0},
		{input: "1,5", expected: 1.5},
		{input: "1.000", expected: 1000.0}, // Heuristic: no digits after dot; treat as thousands

		// With whitespace
		{input: " 1.234,56 ", expected: 1234.56},
		{input: "1 . 234 , 56", expected: 1234.56},

		// Negative numbers (with leading minus)
		// Note: the parser doesn't handle negatives; the caller should strip it first.
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTurkish(tt.input)
			if err != nil {
				t.Errorf("ParseTurkish(%q): %v", tt.input, err)
				return
			}
			if math.Abs(got-tt.expected) > 1e-9 {
				t.Errorf("ParseTurkish(%q): expected %v, got %v", tt.input, tt.expected, got)
			}
		})
	}
}

// TestParseTurkishVsNaive demonstrates the difference from strconv.ParseFloat.
func TestParseTurkishVsNaive(t *testing.T) {
	// Turkish "1.234,56" should parse to 1234.56
	// But naive strconv.ParseFloat would interpret "1.234,56" as 1.234 (stops at comma)
	// or error.

	// Our Turkish parser
	got, err := ParseTurkish("1.234,56")
	if err != nil {
		t.Fatalf("ParseTurkish: %v", err)
	}

	expected := 1234.56
	if math.Abs(got-expected) > 1e-9 {
		t.Errorf("ParseTurkish: expected %v, got %v", expected, got)
	}

	// Show what strconv.ParseFloat does (for reference; this test doesn't call it)
	// strconv.ParseFloat("1.234,56", 64) would return (1.234, nil)
	// which is wrong — off by a factor of ~1000.
	t.Logf("Correct Turkish parse: 1.234,56 → %v", got)
	t.Logf("(Naive strconv.ParseFloat would incorrectly parse as 1.234)")
}

// TestParseTurkishErrors tests error cases.
func TestParseTurkishErrors(t *testing.T) {
	tests := []string{
		"",
		"abc",
		"1.234,56a",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := ParseTurkish(input)
			if err == nil {
				t.Errorf("ParseTurkish(%q): expected error, got none", input)
			}
		})
	}
}

// TestParseTurkishLargeNumbers tests million/billion-scale numbers.
func TestParseTurkishLargeNumbers(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{input: "1.000.000,00", expected: 1000000.00},
		{input: "1.234.567.890,12", expected: 1234567890.12},
		{input: "0,1", expected: 0.1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTurkish(tt.input)
			if err != nil {
				t.Errorf("ParseTurkish(%q): %v", tt.input, err)
				return
			}
			if math.Abs(got-tt.expected) > 1e-9 {
				t.Errorf("ParseTurkish(%q): expected %v, got %v", tt.input, tt.expected, got)
			}
		})
	}
}
