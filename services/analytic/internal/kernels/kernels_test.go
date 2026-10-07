package kernels

import (
	"math"
	"testing"
)

// Ratio tests
func TestRatioValid(t *testing.T) {
	result := Ratio(10, 2)
	if result == nil || *result != 5 {
		t.Errorf("Ratio(10, 2) = %v, want 5", result)
	}
}

func TestRatioZeroDenominator(t *testing.T) {
	result := Ratio(10, 0)
	if result != nil {
		t.Errorf("Ratio(10, 0) = %v, want nil (gap)", result)
	}
}

func TestRatioNaNDenominator(t *testing.T) {
	result := Ratio(10, math.NaN())
	if result != nil {
		t.Errorf("Ratio(10, NaN) = %v, want nil (gap)", result)
	}
}

func TestRatioInfDenominator(t *testing.T) {
	result := Ratio(10, math.Inf(1))
	if result != nil {
		t.Errorf("Ratio(10, Inf) = %v, want nil (gap)", result)
	}
}

func TestRatioNegative(t *testing.T) {
	result := Ratio(-10, 2)
	if result == nil || *result != -5 {
		t.Errorf("Ratio(-10, 2) = %v, want -5", result)
	}
}

// Delta tests
func TestDeltaPositive(t *testing.T) {
	result := Delta(100, 50)
	if result == nil || *result != 50 {
		t.Errorf("Delta(100, 50) = %v, want 50", result)
	}
}

func TestDeltaNegative(t *testing.T) {
	result := Delta(50, 100)
	if result == nil || *result != -50 {
		t.Errorf("Delta(50, 100) = %v, want -50", result)
	}
}

func TestDeltaZero(t *testing.T) {
	result := Delta(100, 100)
	if result == nil || *result != 0 {
		t.Errorf("Delta(100, 100) = %v, want 0", result)
	}
}

func TestDeltaNaNCurrent(t *testing.T) {
	result := Delta(math.NaN(), 50)
	if result != nil {
		t.Errorf("Delta(NaN, 50) = %v, want nil (gap)", result)
	}
}

func TestDeltaNaNPrevious(t *testing.T) {
	result := Delta(100, math.NaN())
	if result != nil {
		t.Errorf("Delta(100, NaN) = %v, want nil (gap)", result)
	}
}

func TestDeltaInfCurrent(t *testing.T) {
	result := Delta(math.Inf(1), 50)
	if result != nil {
		t.Errorf("Delta(Inf, 50) = %v, want nil (gap)", result)
	}
}

// DeltaWeightedTime tests
func TestDeltaWeightedTimeValid(t *testing.T) {
	result := DeltaWeightedTime(100, 2)
	if result == nil || *result != 50 {
		t.Errorf("DeltaWeightedTime(100, 2) = %v, want 50", result)
	}
}

func TestDeltaWeightedTimeZeroTime(t *testing.T) {
	result := DeltaWeightedTime(100, 0)
	if result != nil {
		t.Errorf("DeltaWeightedTime(100, 0) = %v, want nil (gap)", result)
	}
}

func TestDeltaWeightedTimeNegativeTime(t *testing.T) {
	result := DeltaWeightedTime(100, -2)
	if result != nil {
		t.Errorf("DeltaWeightedTime(100, -2) = %v, want nil (gap)", result)
	}
}

func TestDeltaWeightedTimeNaNWeight(t *testing.T) {
	result := DeltaWeightedTime(math.NaN(), 2)
	if result != nil {
		t.Errorf("DeltaWeightedTime(NaN, 2) = %v, want nil (gap)", result)
	}
}

// ZScore tests
func TestZScoreValid(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	// mean = 3, stddev = sqrt(2) ≈ 1.414
	// zscore(5) = (5-3)/1.414 ≈ 1.414
	result := ZScore(5, values, 2)
	if result == nil || *result <= 1.3 || *result >= 1.5 {
		t.Errorf("ZScore(5, [1..5], 2) = %v, want ~1.414", result)
	}
}

func TestZScoreNegative(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	// zscore(1) = (1-3)/1.414 ≈ -1.414
	result := ZScore(1, values, 2)
	if result == nil || *result >= -1.3 || *result <= -1.5 {
		t.Errorf("ZScore(1, [1..5], 2) = %v, want ~-1.414", result)
	}
}

func TestZScoreZeroVariance(t *testing.T) {
	values := []float64{3, 3, 3, 3}
	result := ZScore(3, values, 2)
	if result != nil {
		t.Errorf("ZScore(3, [3,3,3,3], 2) = %v, want nil (zero variance)", result)
	}
}

func TestZScoreInsufficientSamples(t *testing.T) {
	values := []float64{1}
	result := ZScore(1, values, 2)
	if result != nil {
		t.Errorf("ZScore(1, [1], 2) = %v, want nil (insufficient samples)", result)
	}
}

func TestZScoreNaNInput(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	result := ZScore(math.NaN(), values, 2)
	if result != nil {
		t.Errorf("ZScore(NaN, [1..5], 2) = %v, want nil (gap)", result)
	}
}

func TestZScoreNaNInValues(t *testing.T) {
	values := []float64{1, 2, math.NaN(), 4, 5}
	result := ZScore(3, values, 2)
	if result != nil {
		t.Errorf("ZScore(3, [1,2,NaN,4,5], 2) = %v, want nil (invalid input)", result)
	}
}

// Rolling tests
func TestRollingValid(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	result := Rolling(values, 2)
	if result.Mean == nil || *result.Mean != 3 {
		t.Errorf("Rolling([1..5], 2).Mean = %v, want 3", result.Mean)
	}
	// variance = 2.0 (population variance of [1,2,3,4,5])
	if result.Variance == nil || *result.Variance <= 1.9 || *result.Variance >= 2.1 {
		t.Errorf("Rolling([1..5], 2).Variance = %v, want ~2.0", result.Variance)
	}
}

func TestRollingConstantValues(t *testing.T) {
	values := []float64{5, 5, 5, 5}
	result := Rolling(values, 2)
	if result.Mean == nil || *result.Mean != 5 {
		t.Errorf("Rolling([5,5,5,5], 2).Mean = %v, want 5", result.Mean)
	}
	if result.Variance == nil || *result.Variance != 0 {
		t.Errorf("Rolling([5,5,5,5], 2).Variance = %v, want 0", result.Variance)
	}
}

func TestRollingInsufficientSamples(t *testing.T) {
	values := []float64{1}
	result := Rolling(values, 2)
	if result.Mean != nil || result.Variance != nil {
		t.Errorf("Rolling([1], 2) = (%v, %v), want (nil, nil)", result.Mean, result.Variance)
	}
}

func TestRollingNaNInValues(t *testing.T) {
	values := []float64{1, 2, math.NaN(), 4, 5}
	result := Rolling(values, 2)
	if result.Mean != nil || result.Variance != nil {
		t.Errorf("Rolling([1,2,NaN,4,5], 2) = (%v, %v), want (nil, nil)", result.Mean, result.Variance)
	}
}

// Overlap tests
func TestOverlapJaccardIdentical(t *testing.T) {
	setA := []string{"GARAN", "ASELS", "AKBNK"}
	setB := []string{"GARAN", "ASELS", "AKBNK"}
	result := OverlapJaccard(setA, setB)
	if result == nil || *result != 1.0 {
		t.Errorf("OverlapJaccard(identical sets) = %v, want 1.0", result)
	}
}

func TestOverlapJaccardPartial(t *testing.T) {
	setA := []string{"GARAN", "ASELS", "AKBNK"}
	setB := []string{"GARAN", "ASELS", "YKBNK", "SISE"}
	// intersection = 2, union = 5
	result := OverlapJaccard(setA, setB)
	if result == nil || *result != 0.4 {
		t.Errorf("OverlapJaccard(partial) = %v, want 0.4", result)
	}
}

func TestOverlapJaccardDisjoint(t *testing.T) {
	setA := []string{"GARAN", "ASELS"}
	setB := []string{"YKBNK", "SISE"}
	// intersection = 0, union = 4
	result := OverlapJaccard(setA, setB)
	if result == nil || *result != 0 {
		t.Errorf("OverlapJaccard(disjoint) = %v, want 0", result)
	}
}

func TestOverlapJaccardEmpty(t *testing.T) {
	result := OverlapJaccard([]string{}, []string{"GARAN"})
	if result != nil {
		t.Errorf("OverlapJaccard(empty setA) = %v, want nil", result)
	}
}

func TestOverlapWeightCosineIdentical(t *testing.T) {
	weightsA := map[string]float64{"GARAN": 0.5, "ASELS": 0.3, "AKBNK": 0.2}
	weightsB := map[string]float64{"GARAN": 0.5, "ASELS": 0.3, "AKBNK": 0.2}
	// All weights identical: numerator = 0.5+0.3+0.2 = 1.0, denominator = 1.0
	result := OverlapWeightCosine(weightsA, weightsB)
	if result == nil || *result != 1.0 {
		t.Errorf("OverlapWeightCosine(identical) = %v, want 1.0", result)
	}
}

func TestOverlapWeightCosinePartial(t *testing.T) {
	weightsA := map[string]float64{"GARAN": 0.6, "ASELS": 0.4}
	weightsB := map[string]float64{"GARAN": 0.5, "YKBNK": 0.5}
	// GARAN: min(0.6, 0.5) = 0.5, max(0.6, 0.5) = 0.6
	// ASELS: max(0.4) = 0.4
	// YKBNK: max(0.5) = 0.5
	// numerator = 0.5, denominator = 0.6+0.4+0.5 = 1.5
	result := OverlapWeightCosine(weightsA, weightsB)
	if result == nil || *result <= 0.32 || *result >= 0.34 {
		t.Errorf("OverlapWeightCosine(partial) = %v, want ~0.333", result)
	}
}

func TestOverlapWeightCosineNegativeWeight(t *testing.T) {
	weightsA := map[string]float64{"GARAN": -0.1, "ASELS": 0.4}
	weightsB := map[string]float64{"GARAN": 0.5, "YKBNK": 0.5}
	result := OverlapWeightCosine(weightsA, weightsB)
	if result != nil {
		t.Errorf("OverlapWeightCosine(negative weight) = %v, want nil", result)
	}
}

func TestOverlapWeightCosineEmpty(t *testing.T) {
	result := OverlapWeightCosine(map[string]float64{}, map[string]float64{"GARAN": 0.5})
	if result != nil {
		t.Errorf("OverlapWeightCosine(empty map) = %v, want nil", result)
	}
}

// CanonicalSort tests
func TestCanonicalSortAscending(t *testing.T) {
	values := []float64{3, 1, 4, 1, 5, 9, 2, 6}
	CanonicalSort(values)
	expected := []float64{1, 1, 2, 3, 4, 5, 6, 9}
	for i, v := range values {
		if v != expected[i] {
			t.Errorf("CanonicalSort[%d] = %f, want %f", i, v, expected[i])
		}
	}
}

func TestCanonicalSortWithNaN(t *testing.T) {
	values := []float64{3, math.NaN(), 1, 5}
	CanonicalSort(values)
	// NaN should be at the end
	if !math.IsNaN(values[len(values)-1]) {
		t.Errorf("CanonicalSort with NaN: last element = %f, want NaN", values[len(values)-1])
	}
}

func TestCanonicalSortWithInf(t *testing.T) {
	values := []float64{3, math.Inf(1), 1, 5}
	CanonicalSort(values)
	// Inf should be at the end
	if !math.IsInf(values[len(values)-1], 1) {
		t.Errorf("CanonicalSort with Inf: last element = %f, want Inf", values[len(values)-1])
	}
}

// CanonicalStringSort tests
func TestCanonicalStringSort(t *testing.T) {
	values := []string{"SISE", "GARAN", "AKBNK", "ASELS"}
	CanonicalStringSort(values)
	expected := []string{"AKBNK", "ASELS", "GARAN", "SISE"}
	for i, v := range values {
		if v != expected[i] {
			t.Errorf("CanonicalStringSort[%d] = %s, want %s", i, v, expected[i])
		}
	}
}

// ValidateFloat tests
func TestValidateFloatValid(t *testing.T) {
	if !ValidateFloat(3.14) {
		t.Errorf("ValidateFloat(3.14) = false, want true")
	}
}

func TestValidateFloatNaN(t *testing.T) {
	if ValidateFloat(math.NaN()) {
		t.Errorf("ValidateFloat(NaN) = true, want false")
	}
}

func TestValidateFloatInf(t *testing.T) {
	if ValidateFloat(math.Inf(1)) {
		t.Errorf("ValidateFloat(Inf) = true, want false")
	}
}
