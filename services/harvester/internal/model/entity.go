package model

import (
	"fmt"
)

// EntityID is a stable, opaque identifier for a traded entity (stock, fund, etc).
// Unlike ticker symbols (which can change, are locale-specific, and rename),
// EntityID is immutable. Two disclosures with the same ISIN/MKK-code resolve
// to the same EntityID even if their ticker symbols differ (rename event).
//
// Mandate 23: Every raw row resolves to a stable entity ID, not display ticker.
// Rationale: Metrics 1 (velocity) and 2 (overlap) must match across ticker renames.
type EntityID string

// IsZero returns true if the EntityID is empty (uninitialized).
func (e EntityID) IsZero() bool {
	return len(e) == 0
}

// String returns the string representation.
func (e EntityID) String() string {
	return string(e)
}

// EntityResolution provides mappings from identifiers to stable EntityID.
// Preference order: ISIN > MKK-code > ticker (never use ticker as sole identifier).
type EntityResolution struct {
	// isinToEntity maps ISIN (e.g. "TR0123456789") to EntityID
	isinToEntity map[string]EntityID

	// mkkToEntity maps MKK code (e.g. "10006") to EntityID
	mkkToEntity map[string]EntityID

	// tickerToEntity maps ticker to EntityID (informational; should not be primary key)
	tickerToEntity map[string]EntityID

	// entityToInfo maps EntityID to metadata (ISIN, MKK, display ticker, name)
	entityToInfo map[EntityID]*EntityInfo
}

// EntityInfo holds stable metadata for an entity.
type EntityInfo struct {
	EntityID      EntityID
	ISIN          string // Stable identifier
	MKKCode       string // Stable identifier (fund-specific)
	DisplayTicker string // May change (rename event)
	Name          string
}

// NewEntityResolution creates a new resolution registry.
func NewEntityResolution() *EntityResolution {
	return &EntityResolution{
		isinToEntity:   make(map[string]EntityID),
		mkkToEntity:    make(map[string]EntityID),
		tickerToEntity: make(map[string]EntityID),
		entityToInfo:   make(map[EntityID]*EntityInfo),
	}
}

// RegisterEntity adds a mapping from ISIN/MKK/ticker to a stable EntityID.
// At least one of isin or mkkCode must be provided.
// If the entity already exists, updates ticker/name info (allows rename).
func (er *EntityResolution) RegisterEntity(entityID EntityID, info *EntityInfo) error {
	if entityID.IsZero() {
		return fmt.Errorf("entityID cannot be empty")
	}

	if info == nil {
		return fmt.Errorf("EntityInfo cannot be nil")
	}

	if info.ISIN == "" && info.MKKCode == "" {
		return fmt.Errorf("at least ISIN or MKKCode must be provided")
	}

	// Map ISIN to EntityID (if provided)
	if info.ISIN != "" {
		if existing, found := er.isinToEntity[info.ISIN]; found && existing != entityID {
			return fmt.Errorf("ISIN %q already maps to entity %s, cannot reassign to %s", info.ISIN, existing, entityID)
		}
		er.isinToEntity[info.ISIN] = entityID
	}

	// Map MKK to EntityID (if provided)
	if info.MKKCode != "" {
		if existing, found := er.mkkToEntity[info.MKKCode]; found && existing != entityID {
			return fmt.Errorf("MKK code %q already maps to entity %s, cannot reassign to %s", info.MKKCode, existing, entityID)
		}
		er.mkkToEntity[info.MKKCode] = entityID
	}

	// Map ticker to EntityID (informational; allows updates on rename)
	if info.DisplayTicker != "" {
		er.tickerToEntity[info.DisplayTicker] = entityID
	}

	// Store entity info (may update on ticker rename)
	info.EntityID = entityID
	er.entityToInfo[entityID] = info

	return nil
}

// ResolveEntity returns the EntityID for an ISIN, MKK code, or ticker (in order of preference).
// Preference: ISIN > MKK-code > ticker.
// Returns zero EntityID if not found.
func (er *EntityResolution) ResolveEntity(isin, mkkCode, ticker string) EntityID {
	// Try ISIN first (most stable)
	if isin != "" {
		if id, found := er.isinToEntity[isin]; found {
			return id
		}
	}

	// Try MKK code second
	if mkkCode != "" {
		if id, found := er.mkkToEntity[mkkCode]; found {
			return id
		}
	}

	// Try ticker last (least stable, allows updates)
	if ticker != "" {
		if id, found := er.tickerToEntity[ticker]; found {
			return id
		}
	}

	return EntityID("")
}

// GetEntityInfo returns metadata for an EntityID.
func (er *EntityResolution) GetEntityInfo(entityID EntityID) *EntityInfo {
	return er.entityToInfo[entityID]
}

// UpdateTickerName allows a ticker rename while preserving the EntityID.
// This is the key mechanism for metrics 1&2 to survive ticker changes.
func (er *EntityResolution) UpdateTickerName(entityID EntityID, newTicker, newName string) error {
	info, found := er.entityToInfo[entityID]
	if !found {
		return fmt.Errorf("entity %s not found", entityID)
	}

	// Remove old ticker mapping if it exists
	if info.DisplayTicker != "" {
		delete(er.tickerToEntity, info.DisplayTicker)
	}

	// Add new ticker mapping
	if newTicker != "" {
		er.tickerToEntity[newTicker] = entityID
	}

	// Update entity info
	info.DisplayTicker = newTicker
	if newName != "" {
		info.Name = newName
	}

	return nil
}
