package kernels

import (
	"math"
	"testing"
)

func TestGuardZeroDenominator(t *testing.T) {
	if !GuardZeroDenominator(5.0) {
		t.Errorf("GuardZeroDenominator(5) = false, want true")
	}
	if GuardZeroDenominator(0) {
		t.Errorf("GuardZeroDenominator(0) = true, want false")
	}
	if GuardZeroDenominator(math.NaN()) {
		t.Errorf("GuardZeroDenominator(NaN) = true, want false")
	}
	if GuardZeroDenominator(math.Inf(1)) {
		t.Errorf("GuardZeroDenominator(Inf) = true, want false")
	}
}

func TestGuardZeroVariance(t *testing.T) {
	if !GuardZeroVariance(1.5) {
		t.Errorf("GuardZeroVariance(1.5) = false, want true")
	}
	if GuardZeroVariance(1e-15) {
		t.Errorf("GuardZeroVariance(1e-15) = true, want false (below min)")
	}
	if GuardZeroVariance(0) {
		t.Errorf("GuardZeroVariance(0) = true, want false")
	}
	if GuardZeroVariance(math.NaN()) {
		t.Errorf("GuardZeroVariance(NaN) = true, want false")
	}
}

func TestGuardMinimumSample(t *testing.T) {
	if !GuardMinimumSample(10, 5) {
		t.Errorf("GuardMinimumSample(10, 5) = false, want true")
	}
	if GuardMinimumSample(4, 5) {
		t.Errorf("GuardMinimumSample(4, 5) = true, want false")
	}
	if !GuardMinimumSample(5, 5) {
		t.Errorf("GuardMinimumSample(5, 5) = false, want true (equal is OK)")
	}
}

func TestGuardValidFloat(t *testing.T) {
	if !GuardValidFloat(42.5) {
		t.Errorf("GuardValidFloat(42.5) = false, want true")
	}
	if GuardValidFloat(math.NaN()) {
		t.Errorf("GuardValidFloat(NaN) = true, want false")
	}
	if GuardValidFloat(math.Inf(1)) {
		t.Errorf("GuardValidFloat(Inf) = true, want false")
	}
	if GuardValidFloat(math.Inf(-1)) {
		t.Errorf("GuardValidFloat(-Inf) = true, want false")
	}
}

func TestGuardComputedValue(t *testing.T) {
	v := 42.5
	result := GuardComputedValue(v)
	if result == nil || *result != v {
		t.Errorf("GuardComputedValue(42.5) = %v, want &42.5", result)
	}

	resultNaN := GuardComputedValue(math.NaN())
	if resultNaN != nil {
		t.Errorf("GuardComputedValue(NaN) = %v, want nil", resultNaN)
	}

	resultInf := GuardComputedValue(math.Inf(1))
	if resultInf != nil {
		t.Errorf("GuardComputedValue(Inf) = %v, want nil", resultInf)
	}
}

func TestGuardBothNotNaN(t *testing.T) {
	if !GuardBothNotNaN(1.0, 2.0) {
		t.Errorf("GuardBothNotNaN(1, 2) = false, want true")
	}
	if GuardBothNotNaN(math.NaN(), 2.0) {
		t.Errorf("GuardBothNotNaN(NaN, 2) = true, want false")
	}
	if GuardBothNotNaN(1.0, math.NaN()) {
		t.Errorf("GuardBothNotNaN(1, NaN) = true, want false")
	}
}

func TestGuardAllNotNaN(t *testing.T) {
	if !GuardAllNotNaN([]float64{1, 2, 3, 4, 5}) {
		t.Errorf("GuardAllNotNaN([1,2,3,4,5]) = false, want true")
	}
	if GuardAllNotNaN([]float64{1, 2, math.NaN(), 4, 5}) {
		t.Errorf("GuardAllNotNaN with NaN = true, want false")
	}
	if !GuardAllNotNaN([]float64{}) {
		t.Errorf("GuardAllNotNaN([]) = false, want true (empty is valid)")
	}
}
