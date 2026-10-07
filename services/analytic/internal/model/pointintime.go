package model

import "time"

// FilterByFetchedAt returns a new RawSet containing only rows with fetched_at <= observationTs.
// This implements mandate 20: point-in-time integrity.
// A late refinement (MKK T+10, revised disclosure) creates a new derived row at its knowledge date,
// never rewriting the original row whose inputs_hash was locked at the original fetched_at.
func (rs *RawSet) FilterByFetchedAt(observationTs time.Time) *RawSet {
	if rs == nil {
		return nil
	}

	filtered := make([]*RawPayload, 0, len(rs.Rows))
	for _, row := range rs.Rows {
		if !row.FetchedAt.After(observationTs) {
			filtered = append(filtered, row)
		}
	}

	// Recompute worst confidence for the filtered set.
	worstConf := ConfFresh
	for _, row := range filtered {
		if Worst(worstConf, row.Confidence) > worstConf {
			worstConf = Worst(worstConf, row.Confidence)
		}
	}

	return &RawSet{
		Rows:            filtered,
		WorstConfidence: worstConf,
	}
}

// HasRequiredSources checks if the raw set contains at least one row from each required source.
// Used to determine whether a metric should write a gap row (unavailable sources).
func (rs *RawSet) HasRequiredSources(required []string) bool {
	if rs == nil || len(rs.Rows) == 0 {
		return false
	}

	sourceMap := make(map[string]bool)
	for _, row := range rs.Rows {
		sourceMap[row.Source] = true
	}

	for _, req := range required {
		if !sourceMap[req] {
			return false
		}
	}

	return true
}
