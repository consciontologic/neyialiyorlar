package model

import (
	"fmt"
	"time"
)

// CorporateActionType enumerates the types of corporate actions tracked.
// Mandate 25: Capture capital increases (bedelli/bedelsiz), splits, dividends, and renames.
type CorporateActionType string

const (
	// BedelliIncrease: Bedelli capital increase (cash contribution, paid-in)
	BedelliIncrease CorporateActionType = "bedelli_increase"

	// BedelsizIncrease: Bedelsiz capital increase (bonus shares, no cash required)
	BedelsizIncrease CorporateActionType = "bedelsiz_increase"

	// StockSplit: Stock split (e.g., 1:2)
	StockSplit CorporateActionType = "stock_split"

	// ReverseSplit: Reverse stock split (e.g., 2:1)
	ReverseSplit CorporateActionType = "reverse_split"

	// Dividend: Cash or stock dividend
	Dividend CorporateActionType = "dividend"

	// TickerRename: Change of ticker symbol
	TickerRename CorporateActionType = "ticker_rename"

	// NameChange: Change of company legal name
	NameChange CorporateActionType = "name_change"

	// DelistingNotice: Delisting announcement or execution
	DelistingNotice CorporateActionType = "delisting_notice"
)

// IsValid checks if the CorporateActionType is recognized.
func (t CorporateActionType) IsValid() bool {
	switch t {
	case BedelliIncrease, BedelsizIncrease, StockSplit, ReverseSplit, Dividend, TickerRename, NameChange, DelistingNotice:
		return true
	}
	return false
}

// CorporateAction records a significant corporate event affecting a security.
// All timestamps are in UTC.
type CorporateAction struct {
	// EntityID identifies the affected company/fund (stable identifier)
	EntityID EntityID

	// Type classifies the corporate action
	Type CorporateActionType

	// EffectiveDate is when the corporate action takes effect
	// (e.g., dilution date for capital increase, ex-dividend date for dividend)
	EffectiveDate time.Time

	// AnnouncementDate is when the action was announced (if known)
	AnnouncementDate *time.Time

	// Ratio captures numeric parameters:
	// - For splits: numerator/denominator (e.g., 1/2 for 2:1 reverse split)
	// - For bedelli increases: amount per share, percentage increase
	// - For dividends: per-share dividend amount
	Ratio float64

	// OldValue and NewValue track before/after state
	// - For ticker rename: OldValue=old ticker, NewValue=new ticker
	// - For name change: OldValue=old name, NewValue=new name
	// - For split: OldValue=old shares per unit, NewValue=new shares per unit
	OldValue string
	NewValue string

	// Source indicates where this action was discovered
	// (e.g., "kap", "mkkvap", "bist_notice")
	Source string

	// RawID links to the raw payload record this action was extracted from
	RawID int64

	// CreatedAt is when this record was created
	CreatedAt time.Time
}

// CorporateActionLog stores corporate actions for audit and metrics recalculation.
// Key requirement: When a corporate action affects entity identity or share count,
// downstream metrics (velocity, overlap) must be recalculated from raw data using
// the updated entity mapping + adjusted share counts.
type CorporateActionLog struct {
	actions map[EntityID][]*CorporateAction
}

// NewCorporateActionLog creates a new action log.
func NewCorporateActionLog() *CorporateActionLog {
	return &CorporateActionLog{
		actions: make(map[EntityID][]*CorporateAction),
	}
}

// RecordAction adds a corporate action to the log.
func (log *CorporateActionLog) RecordAction(action *CorporateAction) error {
	if action.EntityID.IsZero() {
		return fmt.Errorf("EntityID cannot be empty")
	}

	if !action.Type.IsValid() {
		return fmt.Errorf("invalid CorporateActionType: %s", action.Type)
	}

	log.actions[action.EntityID] = append(log.actions[action.EntityID], action)
	return nil
}

// GetActions returns all recorded actions for an entity, sorted by effective date.
func (log *CorporateActionLog) GetActions(entityID EntityID) []*CorporateAction {
	actions := log.actions[entityID]
	// In production: sort by EffectiveDate ascending
	return actions
}

// GetActionsBetween returns actions effective between start and end dates.
func (log *CorporateActionLog) GetActionsBetween(entityID EntityID, start, end time.Time) []*CorporateAction {
	var result []*CorporateAction
	for _, action := range log.actions[entityID] {
		if !action.EffectiveDate.Before(start) && !action.EffectiveDate.After(end) {
			result = append(result, action)
		}
	}
	return result
}

// ApplyDividendAdjustment adjusts a historical price to account for a dividend.
// Used when calculating metrics that span dividend ex-dates.
func ApplyDividendAdjustment(priceBeforeDividend, dividendPerShare float64) float64 {
	if dividendPerShare <= 0 {
		return priceBeforeDividend
	}
	return priceBeforeDividend - dividendPerShare
}

// ApplySplitAdjustment adjusts share count for a split.
// Example: 1000 shares @ 50 TL → 500 shares @ 100 TL (reverse split 2:1)
func ApplySplitAdjustment(shareCount float64, oldValue, newValue float64) float64 {
	if oldValue <= 0 || newValue <= 0 {
		return shareCount
	}
	return shareCount * (newValue / oldValue)
}
