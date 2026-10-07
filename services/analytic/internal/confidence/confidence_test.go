package confidence

import (
	"testing"

	"github.com/neyialiyorlar/services/analytic/internal/model"
)

func TestWorstFreshStale(t *testing.T) {
	result := Worst(model.ConfFresh, model.ConfStale)
	if result != model.ConfStale {
		t.Errorf("Worst(Fresh, Stale) = %s, want %s", result, model.ConfStale)
	}
}

func TestWorstStaleApprox(t *testing.T) {
	result := Worst(model.ConfStale, model.ConfApprox)
	if result != model.ConfApprox {
		t.Errorf("Worst(Stale, Approx) = %s, want %s", result, model.ConfApprox)
	}
}

func TestWorstFreshApprox(t *testing.T) {
	result := Worst(model.ConfFresh, model.ConfApprox)
	if result != model.ConfApprox {
		t.Errorf("Worst(Fresh, Approx) = %s, want %s", result, model.ConfApprox)
	}
}

func TestWorstMultipleFresh(t *testing.T) {
	result := Worst(model.ConfFresh, model.ConfFresh, model.ConfFresh)
	if result != model.ConfFresh {
		t.Errorf("Worst(Fresh, Fresh, Fresh) = %s, want %s", result, model.ConfFresh)
	}
}

func TestWorstMultipleMixed(t *testing.T) {
	result := Worst(model.ConfFresh, model.ConfStale, model.ConfApprox, model.ConfFresh)
	if result != model.ConfApprox {
		t.Errorf("Worst(Fresh, Stale, Approx, Fresh) = %s, want %s", result, model.ConfApprox)
	}
}

func TestIsProxyMetricVelocity(t *testing.T) {
	if !IsProxyMetric("m01_velocity_accumulation") {
		t.Errorf("IsProxyMetric(m01_velocity_accumulation) = false, want true")
	}
}

func TestIsProxyMetricOverlap(t *testing.T) {
	if !IsProxyMetric("m02_holdings_overlap") {
		t.Errorf("IsProxyMetric(m02_holdings_overlap) = false, want true")
	}
}

func TestIsProxyMetricNonProxy(t *testing.T) {
	if IsProxyMetric("m06_transaction_diversity") {
		t.Errorf("IsProxyMetric(m06_transaction_diversity) = true, want false")
	}
}

func TestSeedConfidenceProxyMetric(t *testing.T) {
	inputs := model.NewRawSet([]*model.RawPayload{
		{Confidence: model.ConfFresh},
	})
	result := SeedConfidence("m01_velocity_accumulation", inputs)
	if result != model.ConfApprox {
		t.Errorf("SeedConfidence(proxy metric) = %s, want %s", result, model.ConfApprox)
	}
}

func TestSeedConfidenceNonProxyFresh(t *testing.T) {
	inputs := model.NewRawSet([]*model.RawPayload{
		{Confidence: model.ConfFresh},
	})
	result := SeedConfidence("m06_transaction_diversity", inputs)
	if result != model.ConfFresh {
		t.Errorf("SeedConfidence(non-proxy, Fresh) = %s, want %s", result, model.ConfFresh)
	}
}

func TestSeedConfidenceNonProxyStale(t *testing.T) {
	inputs := model.NewRawSet([]*model.RawPayload{
		{Confidence: model.ConfFresh},
		{Confidence: model.ConfStale},
	})
	result := SeedConfidence("m06_transaction_diversity", inputs)
	if result != model.ConfStale {
		t.Errorf("SeedConfidence(non-proxy, Stale) = %s, want %s", result, model.ConfStale)
	}
}

func TestSeedConfidenceNilInputs(t *testing.T) {
	result := SeedConfidence("m06_transaction_diversity", nil)
	if result != model.ConfFresh {
		t.Errorf("SeedConfidence(nil inputs) = %s, want %s", result, model.ConfFresh)
	}
}

func TestPropagateConfidenceFreshFresh(t *testing.T) {
	result := PropagateConfidence(model.ConfFresh, model.ConfFresh)
	if result != model.ConfFresh {
		t.Errorf("PropagateConfidence(Fresh, Fresh) = %s, want %s", result, model.ConfFresh)
	}
}

func TestPropagateConfidenceFreshApprox(t *testing.T) {
	result := PropagateConfidence(model.ConfFresh, model.ConfApprox)
	if result != model.ConfApprox {
		t.Errorf("PropagateConfidence(Fresh, Approx) = %s, want %s", result, model.ConfApprox)
	}
}

func TestPropagateConfidenceStaleApprox(t *testing.T) {
	result := PropagateConfidence(model.ConfStale, model.ConfApprox)
	if result != model.ConfApprox {
		t.Errorf("PropagateConfidence(Stale, Approx) = %s, want %s", result, model.ConfApprox)
	}
}

func TestProxyMetricsList(t *testing.T) {
	metrics := ProxyMetricsList()
	if len(metrics) == 0 {
		t.Errorf("ProxyMetricsList() returned empty list, want 5 metrics")
	}
	// Check that at least the known proxy metrics are present.
	found := make(map[string]bool)
	for _, m := range metrics {
		found[m] = true
	}
	if !found["m01_velocity_accumulation"] {
		t.Errorf("ProxyMetricsList() missing m01_velocity_accumulation")
	}
	if !found["m02_holdings_overlap"] {
		t.Errorf("ProxyMetricsList() missing m02_holdings_overlap")
	}
}
