package model

import (
	"testing"
	"time"
)

// TestCorporateActionTypeValid tests type validation.
func TestCorporateActionTypeValid(t *testing.T) {
	validTypes := []CorporateActionType{
		BedelliIncrease, BedelsizIncrease, StockSplit, ReverseSplit,
		Dividend, TickerRename, NameChange, DelistingNotice,
	}

	for _, typ := range validTypes {
		if !typ.IsValid() {
			t.Errorf("expected %s to be valid", typ)
		}
	}

	invalidType := CorporateActionType("invalid_action")
	if invalidType.IsValid() {
		t.Errorf("expected %s to be invalid", invalidType)
	}

	t.Log("✓ Corporate action type validation works")
}

// TestBedelliIncreaseAction tests recording a bedelli capital increase.
func TestBedelliIncreaseAction(t *testing.T) {
	log := NewCorporateActionLog()

	entityID := EntityID("stock_bedelli")
	action := &CorporateAction{
		EntityID:        entityID,
		Type:            BedelliIncrease,
		EffectiveDate:   time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
		Ratio:           10.0, // 10 TL per existing share
		OldValue:        "1000000",
		NewValue:        "1500000", // Shares issued
		Source:          "kap",
		RawID:           12345,
		CreatedAt:       time.Now(),
	}

	err := log.RecordAction(action)
	if err != nil {
		t.Fatalf("RecordAction failed: %v", err)
	}

	actions := log.GetActions(entityID)
	if len(actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(actions))
	}

	if actions[0].Type != BedelliIncrease {
		t.Errorf("expected BedelliIncrease, got %s", actions[0].Type)
	}

	t.Log("✓ Bedelli increase action recorded")
}

// TestBedelsizIncreaseAction tests recording a bedelsiz capital increase (bonus).
func TestBedelsizIncreaseAction(t *testing.T) {
	log := NewCorporateActionLog()

	entityID := EntityID("fund_bedelsiz")
	action := &CorporateAction{
		EntityID:      entityID,
		Type:          BedelsizIncrease,
		EffectiveDate: time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
		Ratio:         0.25, // 1 bonus share per 4 existing
		OldValue:      "10000000",
		NewValue:      "12500000",
		Source:        "kap",
		CreatedAt:     time.Now(),
	}

	err := log.RecordAction(action)
	if err != nil {
		t.Fatalf("RecordAction failed: %v", err)
	}

	t.Log("✓ Bedelsiz increase action recorded")
}

// TestStockSplitAction tests recording a stock split.
func TestStockSplitAction(t *testing.T) {
	log := NewCorporateActionLog()

	entityID := EntityID("stock_split_test")
	action := &CorporateAction{
		EntityID:      entityID,
		Type:          StockSplit,
		EffectiveDate: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		Ratio:         2.0, // 1:2 split (double shares, half price)
		OldValue:      "1",
		NewValue:      "2",
		Source:        "bist",
		CreatedAt:     time.Now(),
	}

	_ = log.RecordAction(action)

	// Test ApplySplitAdjustment: 1000 shares at 1:2 split → 2000 shares
	newShareCount := ApplySplitAdjustment(1000, 1, 2)
	if newShareCount != 2000 {
		t.Errorf("expected 2000 shares, got %f", newShareCount)
	}

	t.Log("✓ Stock split action recorded and adjustment works")
}

// TestReverseSplitAction tests recording a reverse split.
func TestReverseSplitAction(t *testing.T) {
	log := NewCorporateActionLog()

	entityID := EntityID("stock_reverse_split")
	action := &CorporateAction{
		EntityID:      entityID,
		Type:          ReverseSplit,
		EffectiveDate: time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		Ratio:         0.5, // 2:1 reverse split (half shares, double price)
		OldValue:      "2",
		NewValue:      "1",
		Source:        "bist",
		CreatedAt:     time.Now(),
	}

	_ = log.RecordAction(action)

	// Test ApplySplitAdjustment: 2000 shares at 2:1 reverse split → 1000 shares
	newShareCount := ApplySplitAdjustment(2000, 2, 1)
	if newShareCount != 1000 {
		t.Errorf("expected 1000 shares, got %f", newShareCount)
	}

	t.Log("✓ Reverse split action recorded and adjustment works")
}

// TestDividendAction tests recording a dividend.
func TestDividendAction(t *testing.T) {
	log := NewCorporateActionLog()

	entityID := EntityID("stock_dividend")
	exDate := time.Date(2025, 5, 15, 0, 0, 0, 0, time.UTC)
	announcementDate := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)

	action := &CorporateAction{
		EntityID:         entityID,
		Type:             Dividend,
		EffectiveDate:    exDate,
		AnnouncementDate: &announcementDate,
		Ratio:            2.5, // 2.5 TL per share
		Source:           "kap",
		CreatedAt:        time.Now(),
	}

	_ = log.RecordAction(action)

	// Test dividend adjustment: price 100 TL before ex-date, adjust for 2.5 TL dividend
	adjustedPrice := ApplyDividendAdjustment(100.0, 2.5)
	if adjustedPrice != 97.5 {
		t.Errorf("expected 97.5, got %f", adjustedPrice)
	}

	t.Log("✓ Dividend action recorded and adjustment works")
}

// TestTickerRenameAction tests recording a ticker symbol change.
func TestTickerRenameAction(t *testing.T) {
	log := NewCorporateActionLog()

	entityID := EntityID("stock_rename")
	action := &CorporateAction{
		EntityID:      entityID,
		Type:          TickerRename,
		EffectiveDate: time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
		OldValue:      "OLDTICK",
		NewValue:      "NEWTICK",
		Source:        "bist",
		CreatedAt:     time.Now(),
	}

	_ = log.RecordAction(action)

	t.Log("✓ Ticker rename action recorded")
}

// TestNameChangeAction tests recording a company name change.
func TestNameChangeAction(t *testing.T) {
	log := NewCorporateActionLog()

	entityID := EntityID("company_rename")
	action := &CorporateAction{
		EntityID:      entityID,
		Type:          NameChange,
		EffectiveDate: time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC),
		OldValue:      "Old Company Inc",
		NewValue:      "New Company Inc",
		Source:        "kap",
		CreatedAt:     time.Now(),
	}

	_ = log.RecordAction(action)

	t.Log("✓ Name change action recorded")
}

// TestMultipleActionsPerEntity tests recording multiple actions for the same entity.
func TestMultipleActionsPerEntity(t *testing.T) {
	log := NewCorporateActionLog()

	entityID := EntityID("stock_multi_action")

	// Record multiple actions
	actions := []*CorporateAction{
		{
			EntityID:      entityID,
			Type:          BedelliIncrease,
			EffectiveDate: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			Source:        "kap",
			CreatedAt:     time.Now(),
		},
		{
			EntityID:      entityID,
			Type:          Dividend,
			EffectiveDate: time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
			Source:        "kap",
			CreatedAt:     time.Now(),
		},
		{
			EntityID:      entityID,
			Type:          TickerRename,
			EffectiveDate: time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
			Source:        "bist",
			CreatedAt:     time.Now(),
		},
	}

	for _, action := range actions {
		_ = log.RecordAction(action)
	}

	recorded := log.GetActions(entityID)
	if len(recorded) != 3 {
		t.Errorf("expected 3 actions, got %d", len(recorded))
	}

	t.Log("✓ Multiple actions per entity recorded")
}

// TestGetActionsBetween tests filtering actions by date range.
func TestGetActionsBetween(t *testing.T) {
	log := NewCorporateActionLog()

	entityID := EntityID("stock_date_filter")

	// Record actions on different dates
	_ = log.RecordAction(&CorporateAction{
		EntityID:      entityID,
		Type:          BedelliIncrease,
		EffectiveDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		CreatedAt:     time.Now(),
	})

	_ = log.RecordAction(&CorporateAction{
		EntityID:      entityID,
		Type:          Dividend,
		EffectiveDate: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		CreatedAt:     time.Now(),
	})

	_ = log.RecordAction(&CorporateAction{
		EntityID:      entityID,
		Type:          TickerRename,
		EffectiveDate: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
		CreatedAt:     time.Now(),
	})

	// Query for actions in Q2 2025
	start := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)

	filtered := log.GetActionsBetween(entityID, start, end)
	if len(filtered) != 1 {
		t.Errorf("expected 1 action in Q2 2025, got %d", len(filtered))
	}

	if filtered[0].Type != Dividend {
		t.Errorf("expected Dividend action, got %s", filtered[0].Type)
	}

	t.Log("✓ Action date filtering works")
}

// TestCorporateActionValidation tests that invalid actions are rejected.
func TestCorporateActionValidation(t *testing.T) {
	log := NewCorporateActionLog()

	// Missing EntityID
	err := log.RecordAction(&CorporateAction{
		Type: BedelliIncrease,
	})
	if err == nil {
		t.Error("expected error for missing EntityID")
	}

	// Invalid type
	err = log.RecordAction(&CorporateAction{
		EntityID: EntityID("test"),
		Type:     CorporateActionType("invalid"),
	})
	if err == nil {
		t.Error("expected error for invalid type")
	}

	t.Log("✓ Corporate action validation enforced")
}
