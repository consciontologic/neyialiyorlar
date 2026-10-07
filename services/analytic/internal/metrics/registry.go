package metrics

// Registry holds all available metrics, keyed by metric key string.
type Registry struct {
	metrics map[string]Metric
}

// NewRegistry creates a new metric registry and registers all 10 metrics.
func NewRegistry() *Registry {
	r := &Registry{
		metrics: make(map[string]Metric),
	}

	// Register all 10 metrics.
	r.Register(NewM01VelocityAccumulation())
	r.Register(NewM02HoldingsOverlap())
	r.Register(NewM03CategoryConcentration())
	r.Register(NewM04RebalanceFrequency())
	r.Register(NewM05DiversificationRatio())
	r.Register(NewM06TransactionDiversity())
	r.Register(NewM07EstimatedAnnualReturn())
	r.Register(NewM08RollingReturnVolatility())
	r.Register(NewM09BurstDetectionScore())
	r.Register(NewM10RealYield())

	return r
}

// Register adds a metric to the registry.
func (r *Registry) Register(m Metric) {
	r.metrics[m.Key()] = m
}

// Get retrieves a metric by key, or nil if not found.
func (r *Registry) Get(key string) Metric {
	return r.metrics[key]
}

// All returns all registered metrics as a slice.
func (r *Registry) All() []Metric {
	result := make([]Metric, 0, len(r.metrics))
	for _, m := range r.metrics {
		result = append(result, m)
	}
	return result
}

// Count returns the number of registered metrics.
func (r *Registry) Count() int {
	return len(r.metrics)
}

// Keys returns all metric keys.
func (r *Registry) Keys() []string {
	result := make([]string, 0, len(r.metrics))
	for key := range r.metrics {
		result = append(result, key)
	}
	return result
}
