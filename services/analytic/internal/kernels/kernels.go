package kernels

import (
	"math"
	"sort"
)

// Ratio computes a/b. Zero denominator yields a gap (nil).
// Mandates: never NaN/Inf, always total.
func Ratio(a, b float64) *float64 {
	if b == 0 || math.IsNaN(b) || math.IsInf(b, 0) {
		return nil // gap: zero denominator
	}
	result := a / b
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return nil
	}
	return &result
}

// Delta computes x_t - x_{t-1}, or a change between two time-ordered observations.
// Returns nil if either value is invalid or if the order is wrong.
func Delta(current, previous float64) *float64 {
	if math.IsNaN(current) || math.IsNaN(previous) ||
		math.IsInf(current, 0) || math.IsInf(previous, 0) {
		return nil
	}
	result := current - previous
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return nil
	}
	return &result
}

// DeltaWeightedTime computes ΔW/ΔT (change in weight / change in time).
// Both deltas must be non-zero; time cannot be negative.
// Returns nil on division by zero, invalid values, or negative time delta.
func DeltaWeightedTime(weightDelta float64, timeDeltaSeconds float64) *float64 {
	if timeDeltaSeconds <= 0 || math.IsNaN(timeDeltaSeconds) || math.IsInf(timeDeltaSeconds, 0) {
		return nil
	}
	if math.IsNaN(weightDelta) || math.IsInf(weightDelta, 0) {
		return nil
	}
	result := weightDelta / timeDeltaSeconds
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return nil
	}
	return &result
}

// ZScore computes (x - mean) / stddev. Requires at least 2 samples.
// Zero variance yields a gap (nil). Returns nil for sub-minimum sample size.
func ZScore(x float64, values []float64, minSampleSize int) *float64 {
	if len(values) < minSampleSize || minSampleSize < 2 {
		return nil // insufficient sample
	}
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return nil
	}

	// Compute mean.
	sum := 0.0
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil // invalid input
		}
		sum += v
	}
	mean := sum / float64(len(values))

	// Compute standard deviation.
	sumSq := 0.0
	for _, v := range values {
		delta := v - mean
		sumSq += delta * delta
	}
	variance := sumSq / float64(len(values))

	if variance == 0 {
		return nil // zero variance: all values equal
	}

	stddev := math.Sqrt(variance)
	if stddev == 0 || math.IsNaN(stddev) || math.IsInf(stddev, 0) {
		return nil
	}

	result := (x - mean) / stddev
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return nil
	}
	return &result
}

// RollingStats computes windowed mean and variance over a sorted time window.
// Returns (mean, variance) or (nil, nil) if insufficient data or invalid values.
type RollingStats struct {
	Mean     *float64
	Variance *float64
}

func Rolling(values []float64, minSampleSize int) RollingStats {
	if len(values) < minSampleSize || minSampleSize < 1 {
		return RollingStats{Mean: nil, Variance: nil}
	}

	// Validate all inputs.
	sum := 0.0
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return RollingStats{Mean: nil, Variance: nil}
		}
		sum += v
	}

	mean := sum / float64(len(values))
	meanPtr := &mean

	// Compute variance.
	sumSq := 0.0
	for _, v := range values {
		delta := v - mean
		sumSq += delta * delta
	}
	variance := sumSq / float64(len(values))

	if variance < 0 || math.IsNaN(variance) || math.IsInf(variance, 0) {
		return RollingStats{Mean: meanPtr, Variance: nil}
	}

	variancePtr := &variance
	return RollingStats{Mean: meanPtr, Variance: variancePtr}
}

// Overlap computes the Jaccard index (set-based) or weight-cosine (continuous) overlap
// between two collections. For holdings overlap (metric 2), holdings are a set of entity IDs.
//
// Jaccard(A, B) = |A ∩ B| / |A ∪ B|
//
// For weighted overlap (e.g., portfolio weights):
// weight_cosine = (sum of min weights) / (sum of max weights)
//
// Returns nil on empty sets or invalid inputs.
func OverlapJaccard(setA, setB []string) *float64 {
	if len(setA) == 0 || len(setB) == 0 {
		return nil
	}

	// Convert to map for O(1) lookup.
	mapA := make(map[string]bool)
	for _, id := range setA {
		mapA[id] = true
	}

	// Count intersection and union.
	intersection := 0
	for _, id := range setB {
		if mapA[id] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return nil
	}

	result := float64(intersection) / float64(union)
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return nil
	}
	return &result
}

// OverlapWeightCosine computes weight-based overlap given paired (id, weight) values.
// For each id present in both sets, the overlap contribution is min(weight_a, weight_b).
// The denominator is the sum of max(weight_a, weight_b) for all pairs.
//
// Returns nil on empty maps or invalid weights.
func OverlapWeightCosine(weightsA, weightsB map[string]float64) *float64 {
	if len(weightsA) == 0 || len(weightsB) == 0 {
		return nil
	}

	// Validate all weights.
	for _, w := range weightsA {
		if math.IsNaN(w) || math.IsInf(w, 0) || w < 0 {
			return nil
		}
	}
	for _, w := range weightsB {
		if math.IsNaN(w) || math.IsInf(w, 0) || w < 0 {
			return nil
		}
	}

	numerator := 0.0   // sum of min weights for common ids
	denominator := 0.0 // sum of max weights for all ids

	// Intersection: sum min weights.
	for id, wA := range weightsA {
		if wB, ok := weightsB[id]; ok {
			numerator += math.Min(wA, wB)
		}
	}

	// For denominator: all unique ids, max weight in either set.
	seen := make(map[string]bool)
	for id, wA := range weightsA {
		seen[id] = true
		if wB, ok := weightsB[id]; ok {
			denominator += math.Max(wA, wB)
		} else {
			denominator += wA
		}
	}
	for id, wB := range weightsB {
		if !seen[id] {
			denominator += wB
		}
	}

	if denominator == 0 {
		return nil
	}

	result := numerator / denominator
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return nil
	}
	return &result
}

// CanonicalSort sorts a slice of float64 values in ascending order for deterministic computation.
// Invalid values (NaN, Inf) are moved to the end.
func CanonicalSort(values []float64) {
	sort.SliceStable(values, func(i, j int) bool {
		vi, vj := values[i], values[j]
		// NaN and Inf sort to the end.
		if math.IsNaN(vi) || math.IsInf(vi, 0) {
			return false
		}
		if math.IsNaN(vj) || math.IsInf(vj, 0) {
			return true
		}
		return vi < vj
	})
}

// CanonicalStringSort sorts a slice of strings for deterministic computation.
func CanonicalStringSort(values []string) {
	sort.Strings(values)
}

// ValidateFloat checks if a value is a valid number (not NaN/Inf).
func ValidateFloat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
