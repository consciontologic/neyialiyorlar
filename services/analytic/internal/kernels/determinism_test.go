package kernels

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// TestDeterministicComputation proves that the same raw inputs in different order
// produce byte-identical floating-point output after canonical sorting.
// This is prerequisite for golden-file testing (bullet 7).
func TestDeterministicComputation(t *testing.T) {
	// Test data: rolling average of a sequence.
	input1 := []float64{1.0, 5.0, 3.0, 7.0, 2.0}
	input2 := []float64{5.0, 1.0, 7.0, 2.0, 3.0} // same values, different order
	input3 := []float64{7.0, 2.0, 5.0, 3.0, 1.0} // yet another order

	// Compute rolling stats (which uses CanonicalSort internally).
	stats1 := Rolling(input1, 2)
	stats2 := Rolling(input2, 2)
	stats3 := Rolling(input3, 2)

	// After sorting, all three should produce identical mean/variance.
	if stats1.Mean == nil || stats2.Mean == nil || stats3.Mean == nil {
		t.Errorf("Rolling() returned nil mean for valid input")
		return
	}

	// Byte-compare: encode each result as bytes and compare.
	buf1 := encodeFloat64(*stats1.Mean)
	buf2 := encodeFloat64(*stats2.Mean)
	buf3 := encodeFloat64(*stats3.Mean)

	if !bytes.Equal(buf1, buf2) {
		t.Errorf("Determinism failure: different order produced different mean. buf1=%v, buf2=%v",
			buf1, buf2)
	}
	if !bytes.Equal(buf2, buf3) {
		t.Errorf("Determinism failure: third order produced different mean. buf2=%v, buf3=%v",
			buf2, buf3)
	}
}

// TestDeterministicRatio proves ratio computation is deterministic.
func TestDeterministicRatio(t *testing.T) {
	// Same numerator and denominator in all cases.
	r1 := Ratio(15.0, 3.0)
	r2 := Ratio(15.0, 3.0)

	if r1 == nil || r2 == nil {
		t.Errorf("Ratio() returned nil")
		return
	}

	buf1 := encodeFloat64(*r1)
	buf2 := encodeFloat64(*r2)

	if !bytes.Equal(buf1, buf2) {
		t.Errorf("Ratio not deterministic: buf1=%v, buf2=%v", buf1, buf2)
	}
}

// TestDeterministicZScore proves zscore computation is deterministic.
func TestDeterministicZScore(t *testing.T) {
	values := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	valuesMixed := []float64{3.0, 1.0, 5.0, 2.0, 4.0}

	z1 := ZScore(3.0, values, 3)
	z2 := ZScore(3.0, valuesMixed, 3)

	if z1 == nil || z2 == nil {
		t.Errorf("ZScore() returned nil")
		return
	}

	buf1 := encodeFloat64(*z1)
	buf2 := encodeFloat64(*z2)

	if !bytes.Equal(buf1, buf2) {
		t.Errorf("ZScore not deterministic: input order changed result. buf1=%v, buf2=%v", buf1, buf2)
	}
}

// encodeFloat64 encodes a float64 as bytes for deterministic comparison.
func encodeFloat64(v float64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, math.Float64bits(v))
	return buf
}
