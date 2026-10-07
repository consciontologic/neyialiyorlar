package model

// EntityResolution stubs the entity resolution contract (bullet 17 / mandate 23).
// Full implementation deferred to Phase 4 when entity_ref master table is available.
//
// Contract: Metric 1 (velocity) and Metric 2 (overlap) must join raw across
// consecutive disclosures by stable entity id (ISIN/MKK code), not the display ticker.
// A ticker rename between disclosures does not split the series.
//
// Phase 4 implementation:
// 1. Add entity_ref table with (id, isin_code, mkk_code, display_ticker, valid_from, valid_to, theme_baskets)
// 2. In Compute(), resolve each raw.natural_key to its entity_id via entity_ref
// 3. Use entity_id as the Derived.entity key (not the display ticker)
// 4. Join holdings/velocity across consecutive disclosures by entity_id
//
// Example flow:
//   raw [disclosure_a, ticker="GARAN"] -> entity_ref.lookup -> entity_id=123, isin="TR0005000C93"
//   raw [disclosure_b, ticker="GARAN"] -> entity_ref.lookup -> entity_id=123, isin="TR0005000C93"
//   Metric 1: entity="TR0005000C93" (ISIN, stable)
//   Metric 2: overlap joins by entity_id=123 (stable across disclosure boundary)
//
type EntityResolutionConfig struct {
	ISINCode string // Stable ISIN code (e.g. TR0005000C93)
	MKKCode  string // Stable MKK code (alternative key)
	Ticker   string // Display ticker (churns with capital actions)
}

// ResolveEntity returns the stable entity ID for a given natural_key.
// Phase 4: queries entity_ref table. Phase 3 stub: returns the input as-is.
func ResolveEntity(naturalKey string) string {
	// Phase 3: no entity_ref table yet; return naturalKey as-is.
	// Phase 4: SELECT entity_id FROM entity_ref WHERE ... LIMIT 1
	return naturalKey
}

// MonotonicCacheWrite documents the contract for bullet 19 (hot-cache monotonicity).
// Phase 4 implementation:
//
// Contract: A late-arriving earlier observation cannot overwrite a newer `:latest` cache value.
// Guarded by computed_at/ts comparison: SET cache only if computed_at(new) > computed_at(existing).
//
// Implementation:
//   Redis EVAL script (atomic compare-and-set):
//     SCRIPT:
//       existing_ts = redis.call('hget', cache_key, 'computed_at')
//       if new_ts > existing_ts:
//         redis.call('hset', cache_key, 'computed_at', new_ts)
//         redis.call('hset', cache_key, 'value', value)
//         redis.call('hset', cache_key, 'flag', flag)
//       return existing_ts
//
// Test cases:
// 1. First write -> success
// 2. Later write with newer computed_at -> success
// 3. Late write with older computed_at -> rejected (no-op)
// 4. Concurrent writes -> Lua script ensures atomicity
//
type MonotonicCacheGuard struct {
	ComputedAtKey string // Redis hash field name
	ValueKey      string
	FlagKey       string
}

// OverlapFanoutCap documents the contract for bullet 20 (overlap degradation).
// Phase 4 implementation:
//
// Contract: Metric 2 (holdings overlap) fan-out is capped per category.
// If a category exceeds the cap (e.g., 100 fund members), the metric degrades to
// a flagged partial overlap rather than stalling the event-cadence budget.
//
// Parameters (from config.metrics[].overlap_cap):
//   - overlap_cap: max number of fund members before degradation
//   - degradation_flag: flag to apply when degrading (ConfApprox)
//   - degradation_reason: text reason (e.g., "overlap category exceeded 100 members")
//
// Implementation:
//   1. Group holdings by category (e.g., sector)
//   2. For each category, count members
//   3. If count > overlap_cap:
//     - Compute overlap on a random sample of overlap_cap members
//     - Mark result with ConfApprox flag
//     - Log degradation_reason
//   4. Otherwise, compute full overlap
//
// Test cases:
// 1. Small category (< cap) -> full overlap
// 2. Category exceeds cap -> partial overlap with ConfApprox
// 3. Empty category -> gap row
//
type OverlapFanoutConfig struct {
	CapPerCategory int  // max fund members before degradation
	ApplySampling  bool // use random sampling for oversized categories
}
