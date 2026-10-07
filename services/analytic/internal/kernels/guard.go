package kernels

import "math"

// Guard contains numerical-integrity checks for all kernel computations.
// Mandate 21: kernels are total functions — a computation that would yield NaN, Inf,
// or a degenerate input (zero denominator, zero variance, insufficient sample) returns nil
// instead, never panics or returns an invalid float.

// GuardZeroDenominator returns nil if denominator is zero, otherwise true.
// Use this to guard ratio-like computations (dividend / divisor).
// Example: a/b where b is zero → gap (value = nil).
func GuardZeroDenominator(denominator float64) bool {
	return denominator != 0 && !math.IsNaN(denominator) && !math.IsInf(denominator, 0)
}

// GuardZeroVariance returns nil if variance is effectively zero.
// Use this to guard zscore and rolling-std computations where division by variance would occur.
// Example: zscore = (x - mean) / stddev where stddev is zero → gap.
func GuardZeroVariance(variance float64) bool {
	const minVariance = 1e-10
	return variance > minVariance && !math.IsNaN(variance) && !math.IsInf(variance, 0)
}

// GuardMinimumSample returns nil if sample count is below the required minimum.
// Use this to guard rolling and statistical computations requiring sufficient data.
// Example: rolling window has only 1 sample, but minimum is 5 → gap.
func GuardMinimumSample(sampleCount, minimumRequired int) bool {
	return sampleCount >= minimumRequired
}

// GuardValidFloat checks if a float is valid (not NaN/Inf).
// Use this as a final check before returning a computed value.
// Example: result is NaN due to intermediate computation → gap.
func GuardValidFloat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// GuardComputedValue wraps a computed result and returns it only if valid.
// This is the canonical way to guard a kernel result before returning to Compute.
// Example: ratio := a / b; return GuardComputedValue(ratio) // returns nil if NaN/Inf
func GuardComputedValue(computed float64) *float64 {
	if GuardValidFloat(computed) {
		return &computed
	}
	return nil // gap: invalid result
}

// GuardBothNotNaN returns true if both values are valid (not NaN/Inf).
// Use when a computation depends on two inputs that must both be valid.
func GuardBothNotNaN(a, b float64) bool {
	return GuardValidFloat(a) && GuardValidFloat(b)
}

// GuardAllNotNaN returns true if all values in a slice are valid.
// Use when filtering a dataset for valid inputs before aggregation.
func GuardAllNotNaN(values []float64) bool {
	for _, v := range values {
		if !GuardValidFloat(v) {
			return false
		}
	}
	return true
}
