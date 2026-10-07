package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// HighWater tracks the read position (cursor) for each source.
// Rationale (mandate 11): on restart, resume from the last committed position
// without duplicating fetches. The cursor is transactional: only advanced after
// the raw upsert succeeds. A simulated crash + restart proves resumption.
type HighWater struct {
	SourceID   string    `db:"source_id"`
	Cursor     string    `db:"cursor"`     // opaque: could be a page token, timestamp, offset, ID
	FetchedAt  time.Time `db:"fetched_at"` // when this cursor was acquired
	UpdatedAt  time.Time `db:"updated_at"` // when last advanced
}

// HighWaterStore manages high-water marks in the database.
type HighWaterStore struct {
	db *sql.DB
}

// NewHighWaterStore creates a store for managing cursors.
func NewHighWaterStore(db *sql.DB) *HighWaterStore {
	return &HighWaterStore{db: db}
}

// GetCursor retrieves the current cursor for a source, or "" if not yet recorded.
func (hw *HighWaterStore) GetCursor(ctx context.Context, sourceID string) (string, error) {
	query := `
		SELECT cursor FROM source_cursor
		WHERE source_id = $1
		ORDER BY updated_at DESC
		LIMIT 1
	`

	var cursor sql.NullString
	err := hw.db.QueryRowContext(ctx, query, sourceID).Scan(&cursor)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // Not yet recorded; return empty cursor
		}
		return "", fmt.Errorf("get cursor: %w", err)
	}

	if cursor.Valid {
		return cursor.String, nil
	}
	return "", nil
}

// UpdateCursor advances the cursor for a source. Idempotent: same cursor
// value is inserted only once (ON CONFLICT DO UPDATE).
func (hw *HighWaterStore) UpdateCursor(ctx context.Context, sourceID, newCursor string) error {
	query := `
		INSERT INTO source_cursor (source_id, cursor, fetched_at, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (source_id, cursor) DO UPDATE
			SET updated_at = CURRENT_TIMESTAMP
	`

	_, err := hw.db.ExecContext(ctx, query, sourceID, newCursor)
	if err != nil {
		return fmt.Errorf("update cursor: %w", err)
	}

	return nil
}

// ListCursors returns all current cursors (useful for health checks).
func (hw *HighWaterStore) ListCursors(ctx context.Context) (map[string]string, error) {
	query := `
		SELECT DISTINCT ON (source_id) source_id, cursor, updated_at
		FROM source_cursor
		ORDER BY source_id, updated_at DESC
	`

	rows, err := hw.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list cursors: %w", err)
	}
	defer rows.Close()

	cursors := make(map[string]string)
	for rows.Next() {
		var sourceID, cursor string
		var updatedAt time.Time
		if err := rows.Scan(&sourceID, &cursor, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan cursor: %w", err)
		}
		cursors[sourceID] = cursor
	}

	return cursors, rows.Err()
}

// GetCursorHistory returns all historical cursors for a source (for audit/replay).
func (hw *HighWaterStore) GetCursorHistory(ctx context.Context, sourceID string, limit int) ([]HighWater, error) {
	query := `
		SELECT source_id, cursor, fetched_at, updated_at
		FROM source_cursor
		WHERE source_id = $1
		ORDER BY updated_at DESC
	`

	args := []interface{}{sourceID}
	if limit > 0 {
		query += ` LIMIT $2`
		args = append(args, limit)
	}

	rows, err := hw.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get cursor history: %w", err)
	}
	defer rows.Close()

	var history []HighWater
	for rows.Next() {
		hw := HighWater{}
		if err := rows.Scan(&hw.SourceID, &hw.Cursor, &hw.FetchedAt, &hw.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan cursor history: %w", err)
		}
		history = append(history, hw)
	}

	return history, rows.Err()
}
