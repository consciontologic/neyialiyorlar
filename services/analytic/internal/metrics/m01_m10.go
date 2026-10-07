package metrics

import (
	"context"
	"time"

	"github.com/neyialiyorlar/services/analytic/internal/confidence"
	"github.com/neyialiyorlar/services/analytic/internal/model"
)

// m01VelocityAccumulation computes portfolio accumulation velocity (proxy metric).
type m01VelocityAccumulation struct{}

func (m *m01VelocityAccumulation) Key() string {
	return "m01_velocity_accumulation"
}

func (m *m01VelocityAccumulation) Description() string {
	return "Portfolio accumulation velocity (proxy: derived from holdings)"
}

func (m *m01VelocityAccumulation) Cadence() model.Cadence {
	return model.CadenceEvent
}

func (m *m01VelocityAccumulation) RequiredSources() []string {
	return []string{"kap"}
}

func (m *m01VelocityAccumulation) Compute(ctx context.Context, rawSet *model.RawSet) (*model.Derived, error) {
	// This is a proxy metric: it will be seeded with Approx confidence.
	// Compute is a placeholder; real implementation would aggregate holdings velocity.
	if rawSet == nil || len(rawSet.Rows) == 0 {
		return &model.Derived{
			MetricKey:  m.Key(),
			Value:      nil, // gap
			Flag:       model.ConfStale,
			Cadence:    m.Cadence(),
			ComputedAt: time.Now(),
		}, nil
	}

	// For now, return a gap since we don't have real holdings data.
	return &model.Derived{
		MetricKey:  m.Key(),
		Value:      nil,
		Flag:       confidence.SeedConfidence(m.Key(), rawSet),
		Cadence:    m.Cadence(),
		ComputedAt: time.Now(),
	}, nil
}

// NewM01VelocityAccumulation creates a new instance of the velocity accumulation metric.
func NewM01VelocityAccumulation() Metric {
	return &m01VelocityAccumulation{}
}

// m02HoldingsOverlap computes the overlap between fund holdings across time (proxy metric).
type m02HoldingsOverlap struct{}

func (m *m02HoldingsOverlap) Key() string {
	return "m02_holdings_overlap"
}

func (m *m02HoldingsOverlap) Description() string {
	return "Holdings overlap (Jaccard index) across consecutive disclosures (proxy)"
}

func (m *m02HoldingsOverlap) Cadence() model.Cadence {
	return model.CadenceEvent
}

func (m *m02HoldingsOverlap) RequiredSources() []string {
	return []string{"kap"}
}

func (m *m02HoldingsOverlap) Compute(ctx context.Context, rawSet *model.RawSet) (*model.Derived, error) {
	if rawSet == nil || len(rawSet.Rows) < 2 {
		return &model.Derived{
			MetricKey:  m.Key(),
			Value:      nil,
			Flag:       model.ConfStale,
			Cadence:    m.Cadence(),
			ComputedAt: time.Now(),
		}, nil
	}

	// Placeholder: would extract holdings from consecutive disclosures and compute Jaccard.
	return &model.Derived{
		MetricKey:  m.Key(),
		Value:      nil,
		Flag:       confidence.SeedConfidence(m.Key(), rawSet),
		Cadence:    m.Cadence(),
		ComputedAt: time.Now(),
	}, nil
}

// NewM02HoldingsOverlap creates a new instance of the holdings overlap metric.
func NewM02HoldingsOverlap() Metric {
	return &m02HoldingsOverlap{}
}

// m03CategoryConcentration computes Herfindahl index of a fund's sector concentration.
type m03CategoryConcentration struct{}

func (m *m03CategoryConcentration) Key() string {
	return "m03_category_concentration"
}

func (m *m03CategoryConcentration) Description() string {
	return "Herfindahl concentration index of fund holdings by category"
}

func (m *m03CategoryConcentration) Cadence() model.Cadence {
	return model.CadenceIntraday
}

func (m *m03CategoryConcentration) RequiredSources() []string {
	return []string{"kap"}
}

func (m *m03CategoryConcentration) Compute(ctx context.Context, rawSet *model.RawSet) (*model.Derived, error) {
	if rawSet == nil || len(rawSet.Rows) == 0 {
		return &model.Derived{
			MetricKey:  m.Key(),
			Value:      nil,
			Flag:       model.ConfStale,
			Cadence:    m.Cadence(),
			ComputedAt: time.Now(),
		}, nil
	}

	return &model.Derived{
		MetricKey:  m.Key(),
		Value:      nil,
		Flag:       confidence.SeedConfidence(m.Key(), rawSet),
		Cadence:    m.Cadence(),
		ComputedAt: time.Now(),
	}, nil
}

// NewM03CategoryConcentration creates a new instance.
func NewM03CategoryConcentration() Metric {
	return &m03CategoryConcentration{}
}

// m04RebalanceFrequency computes the frequency of portfolio rebalancing events.
type m04RebalanceFrequency struct{}

func (m *m04RebalanceFrequency) Key() string {
	return "m04_rebalance_frequency"
}

func (m *m04RebalanceFrequency) Description() string {
	return "Rebalancing event frequency (proxy: inferred from disclosure timestamps)"
}

func (m *m04RebalanceFrequency) Cadence() model.Cadence {
	return model.CadenceDaily
}

func (m *m04RebalanceFrequency) RequiredSources() []string {
	return []string{"kap"}
}

func (m *m04RebalanceFrequency) Compute(ctx context.Context, rawSet *model.RawSet) (*model.Derived, error) {
	if rawSet == nil || len(rawSet.Rows) < 2 {
		return &model.Derived{
			MetricKey:  m.Key(),
			Value:      nil,
			Flag:       model.ConfStale,
			Cadence:    m.Cadence(),
			ComputedAt: time.Now(),
		}, nil
	}

	return &model.Derived{
		MetricKey:  m.Key(),
		Value:      nil,
		Flag:       confidence.SeedConfidence(m.Key(), rawSet),
		Cadence:    m.Cadence(),
		ComputedAt: time.Now(),
	}, nil
}

// NewM04RebalanceFrequency creates a new instance.
func NewM04RebalanceFrequency() Metric {
	return &m04RebalanceFrequency{}
}

// m05DiversificationRatio computes the ratio of average weight to portfolio volatility.
type m05DiversificationRatio struct{}

func (m *m05DiversificationRatio) Key() string {
	return "m05_diversification_ratio"
}

func (m *m05DiversificationRatio) Description() string {
	return "Diversification ratio: average weight / portfolio stddev"
}

func (m *m05DiversificationRatio) Cadence() model.Cadence {
	return model.CadenceDaily
}

func (m *m05DiversificationRatio) RequiredSources() []string {
	return []string{"kap"}
}

func (m *m05DiversificationRatio) Compute(ctx context.Context, rawSet *model.RawSet) (*model.Derived, error) {
	if rawSet == nil || len(rawSet.Rows) == 0 {
		return &model.Derived{
			MetricKey:  m.Key(),
			Value:      nil,
			Flag:       model.ConfStale,
			Cadence:    m.Cadence(),
			ComputedAt: time.Now(),
		}, nil
	}

	return &model.Derived{
		MetricKey:  m.Key(),
		Value:      nil,
		Flag:       confidence.SeedConfidence(m.Key(), rawSet),
		Cadence:    m.Cadence(),
		ComputedAt: time.Now(),
	}, nil
}

// NewM05DiversificationRatio creates a new instance.
func NewM05DiversificationRatio() Metric {
	return &m05DiversificationRatio{}
}

// m06TransactionDiversity computes portfolio turnover week-over-week.
type m06TransactionDiversity struct{}

func (m *m06TransactionDiversity) Key() string {
	return "m06_transaction_diversity"
}

func (m *m06TransactionDiversity) Description() string {
	return "Portfolio turnover (week-over-week transaction rate)"
}

func (m *m06TransactionDiversity) Cadence() model.Cadence {
	return model.CadenceWeekly
}

func (m *m06TransactionDiversity) RequiredSources() []string {
	return []string{"evds"}
}

func (m *m06TransactionDiversity) Compute(ctx context.Context, rawSet *model.RawSet) (*model.Derived, error) {
	if rawSet == nil || len(rawSet.Rows) == 0 {
		return &model.Derived{
			MetricKey:  m.Key(),
			Value:      nil,
			Flag:       model.ConfStale,
			Cadence:    m.Cadence(),
			ComputedAt: time.Now(),
		}, nil
	}

	return &model.Derived{
		MetricKey:  m.Key(),
		Value:      nil,
		Flag:       confidence.SeedConfidence(m.Key(), rawSet),
		Cadence:    m.Cadence(),
		ComputedAt: time.Now(),
	}, nil
}

// NewM06TransactionDiversity creates a new instance.
func NewM06TransactionDiversity() Metric {
	return &m06TransactionDiversity{}
}

// m07EstimatedAnnualReturn is a forward-looking estimate (proxy).
type m07EstimatedAnnualReturn struct{}

func (m *m07EstimatedAnnualReturn) Key() string {
	return "m07_estimated_annual_return"
}

func (m *m07EstimatedAnnualReturn) Description() string {
	return "Estimated annual return (proxy: analyst consensus or historical trajectory)"
}

func (m *m07EstimatedAnnualReturn) Cadence() model.Cadence {
	return model.CadenceDaily
}

func (m *m07EstimatedAnnualReturn) RequiredSources() []string {
	return []string{"bist"}
}

func (m *m07EstimatedAnnualReturn) Compute(ctx context.Context, rawSet *model.RawSet) (*model.Derived, error) {
	if rawSet == nil || len(rawSet.Rows) == 0 {
		return &model.Derived{
			MetricKey:  m.Key(),
			Value:      nil,
			Flag:       model.ConfStale,
			Cadence:    m.Cadence(),
			ComputedAt: time.Now(),
		}, nil
	}

	return &model.Derived{
		MetricKey:  m.Key(),
		Value:      nil,
		Flag:       confidence.SeedConfidence(m.Key(), rawSet),
		Cadence:    m.Cadence(),
		ComputedAt: time.Now(),
	}, nil
}

// NewM07EstimatedAnnualReturn creates a new instance.
func NewM07EstimatedAnnualReturn() Metric {
	return &m07EstimatedAnnualReturn{}
}

// m08RollingReturnVolatility computes rolling 90-day volatility.
type m08RollingReturnVolatility struct{}

func (m *m08RollingReturnVolatility) Key() string {
	return "m08_rolling_return_volatility"
}

func (m *m08RollingReturnVolatility) Description() string {
	return "Rolling 90-day return volatility (standard deviation)"
}

func (m *m08RollingReturnVolatility) Cadence() model.Cadence {
	return model.CadenceDaily
}

func (m *m08RollingReturnVolatility) RequiredSources() []string {
	return []string{"bist"}
}

func (m *m08RollingReturnVolatility) Compute(ctx context.Context, rawSet *model.RawSet) (*model.Derived, error) {
	if rawSet == nil || len(rawSet.Rows) < 90 {
		return &model.Derived{
			MetricKey:  m.Key(),
			Value:      nil,
			Flag:       model.ConfStale,
			Cadence:    m.Cadence(),
			ComputedAt: time.Now(),
		}, nil
	}

	return &model.Derived{
		MetricKey:  m.Key(),
		Value:      nil,
		Flag:       confidence.SeedConfidence(m.Key(), rawSet),
		Cadence:    m.Cadence(),
		ComputedAt: time.Now(),
	}, nil
}

// NewM08RollingReturnVolatility creates a new instance.
func NewM08RollingReturnVolatility() Metric {
	return &m08RollingReturnVolatility{}
}

// m09BurstDetectionScore detects volatility spikes (bursts).
type m09BurstDetectionScore struct{}

func (m *m09BurstDetectionScore) Key() string {
	return "m09_burst_detection_score"
}

func (m *m09BurstDetectionScore) Description() string {
	return "Z-score burst detection (deviation from rolling mean)"
}

func (m *m09BurstDetectionScore) Cadence() model.Cadence {
	return model.CadenceIntraday
}

func (m *m09BurstDetectionScore) RequiredSources() []string {
	return []string{"bist"}
}

func (m *m09BurstDetectionScore) Compute(ctx context.Context, rawSet *model.RawSet) (*model.Derived, error) {
	if rawSet == nil || len(rawSet.Rows) < 5 {
		return &model.Derived{
			MetricKey:  m.Key(),
			Value:      nil,
			Flag:       model.ConfStale,
			Cadence:    m.Cadence(),
			ComputedAt: time.Now(),
		}, nil
	}

	return &model.Derived{
		MetricKey:  m.Key(),
		Value:      nil,
		Flag:       confidence.SeedConfidence(m.Key(), rawSet),
		Cadence:    m.Cadence(),
		ComputedAt: time.Now(),
	}, nil
}

// NewM09BurstDetectionScore creates a new instance.
func NewM09BurstDetectionScore() Metric {
	return &m09BurstDetectionScore{}
}

// m10RealYield computes real yield (yield minus inflation).
type m10RealYield struct{}

func (m *m10RealYield) Key() string {
	return "m10_real_yield"
}

func (m *m10RealYield) Description() string {
	return "Real yield: fund yield minus nominal inflation rate"
}

func (m *m10RealYield) Cadence() model.Cadence {
	return model.CadenceDaily
}

func (m *m10RealYield) RequiredSources() []string {
	return []string{"bist", "evds"}
}

func (m *m10RealYield) Compute(ctx context.Context, rawSet *model.RawSet) (*model.Derived, error) {
	if rawSet == nil || len(rawSet.Rows) == 0 {
		return &model.Derived{
			MetricKey:  m.Key(),
			Value:      nil,
			Flag:       model.ConfStale,
			Cadence:    m.Cadence(),
			ComputedAt: time.Now(),
		}, nil
	}

	return &model.Derived{
		MetricKey:  m.Key(),
		Value:      nil,
		Flag:       confidence.SeedConfidence(m.Key(), rawSet),
		Cadence:    m.Cadence(),
		ComputedAt: time.Now(),
	}, nil
}

// NewM10RealYield creates a new instance.
func NewM10RealYield() Metric {
	return &m10RealYield{}
}
