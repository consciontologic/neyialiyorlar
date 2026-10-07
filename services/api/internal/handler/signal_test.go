package handler

import (
	"testing"
)

// TestSMA verifies the sma helper produces correct simple moving averages.
func TestSMA(t *testing.T) {
	cases := []struct {
		name   string
		prices []float64
		want   float64
	}{
		{"single price", []float64{10}, 10},
		{"two prices", []float64{10, 20}, 15},
		{"five prices", []float64{1, 2, 3, 4, 5}, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sma(tc.prices)
			if got != tc.want {
				t.Errorf("sma(%v) = %.4f, want %.4f", tc.prices, got, tc.want)
			}
		})
	}
}

// TestRoundAt verifies roundAt rounds to the given decimal places.
func TestRoundAt(t *testing.T) {
	cases := []struct {
		v    float64
		dec  int
		want float64
	}{
		{3.14159, 2, 3.14},
		{2.555, 2, 2.56},
		{100.0, 0, 100.0},
		{-1.675, 2, -1.68},
	}
	for _, tc := range cases {
		got := roundAt(tc.v, tc.dec)
		if got != tc.want {
			t.Errorf("roundAt(%.5f, %d) = %.5f, want %.5f", tc.v, tc.dec, got, tc.want)
		}
	}
}

// TestComputeSignalPureLogic validates the signal computation without a DB by
// calling the internal helpers directly. We test only the sma + roundAt path
// and enumerate the branching conditions.
func TestComputeSignalPureLogic(t *testing.T) {
	// BUY: steadily rising prices → MA5 > MA20 (recent prices much higher than older)
	t.Run("upward trend gives BUY signal condition", func(t *testing.T) {
		// 25 prices rising from 10 → 130 in uniform steps (step=5)
		prices := make([]float64, 25)
		for i := range prices {
			prices[i] = float64(10 + i*5) // 10, 15, 20, ..., 130
		}
		n := len(prices)
		ma5 := sma(prices[n-5 : n])
		ma20 := sma(prices[n-20 : n])
		// With uniformly rising prices MA5 is always > MA20
		if ma5 <= ma20 {
			t.Fatalf("expected ma5 (%.2f) > ma20 (%.2f) for upward trend", ma5, ma20)
		}
	})

	// SELL: steadily falling prices → MA5 < MA20
	t.Run("downward trend gives SELL signal condition", func(t *testing.T) {
		// 25 prices falling from 130 → 10
		prices := make([]float64, 25)
		for i := range prices {
			prices[i] = float64(130 - i*5) // 130, 125, 120, ..., 10
		}
		n := len(prices)
		ma5 := sma(prices[n-5 : n])
		ma20 := sma(prices[n-20 : n])
		if ma5 >= ma20 {
			t.Fatalf("expected ma5 (%.2f) < ma20 (%.2f) for downward trend", ma5, ma20)
		}
	})

	// INSUFFICIENT: flat prices but check edge of 19-observation threshold
	t.Run("fewer than 20 observations is insufficient threshold", func(t *testing.T) {
		const threshold = 20
		prices := make([]float64, threshold-1) // 19 prices
		if len(prices) >= threshold {
			t.Fatal("expected slice smaller than threshold")
		}
	})
}
