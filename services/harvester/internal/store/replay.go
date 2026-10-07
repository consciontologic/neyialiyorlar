package store

import (
	"context"
	"fmt"
	"time"

	"database/sql"

	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// ReplayStore manages quarantine replay operations.
// Rationale (mandate 15): when a parser is fixed, quarantined rows can be
// re-processed idempotently without fetching again. Replay tracks attempts
// to prevent infinite loops (max_replay_attempts guard).
type ReplayStore struct {
	db *sql.DB
}

// NewReplayStore creates a replay manager.
func NewReplayStore(db *sql.DB) *ReplayStore {
	return &ReplayStore{db: db}
}

// ReplayResult tracks the outcome of a single replay attempt.
type ReplayResult struct {
	RawID              int64
	PreviousQuarantine bool
	NewQuarantine      bool
	QuarantineReason   string
	AttemptCount       int
	MaxAttempts        int
	Success            bool
}

// ReplayQuarantined replays a quarantined raw through a parser.
// Returns the result and whether to continue retrying.
func (rs *ReplayStore) ReplayQuarantined(
	ctx context.Context,
	rawID int64,
	parser func(model.Raw) model.ParseResult,
	maxAttempts int,
) (ReplayResult, error) {
	result := ReplayResult{
		RawID:       rawID,
		MaxAttempts: maxAttempts,
	}

	// Retrieve the raw
	query := `
		SELECT id, source_id, natural_key, payload, quarantined, quarantine_reason
		FROM raw WHERE id = $1
	`

	var raw model.Raw
	err := rs.db.QueryRowContext(ctx, query, rawID).Scan(
		&raw.SourceID,
		&raw.NaturalKey,
		&raw.Payload,
		&raw.Quarantined,
		&raw.QuarantineReason,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return result, fmt.Errorf("raw not found: %d", rawID)
		}
		return result, fmt.Errorf("query raw: %w", err)
	}

	result.PreviousQuarantine = raw.Quarantined

	// Query replay attempt count
	countQuery := `
		SELECT COUNT(*) FROM quarantine_replay_log
		WHERE raw_id = $1
	`

	var attemptCount int
	err = rs.db.QueryRowContext(ctx, countQuery, rawID).Scan(&attemptCount)
	if err != nil {
		return result, fmt.Errorf("count replay attempts: %w", err)
	}

	result.AttemptCount = attemptCount + 1

	// Check if we've exceeded max attempts
	if result.AttemptCount > maxAttempts {
		// Move to dead-letter state (mark with special reason)
		updateQuery := `
			UPDATE raw
			SET quarantined = true, quarantine_reason = 'replay_exhausted'
			WHERE id = $1
		`
		_, err := rs.db.ExecContext(ctx, updateQuery, rawID)
		if err != nil {
			return result, fmt.Errorf("mark dead-letter: %w", err)
		}

		result.NewQuarantine = true
		result.QuarantineReason = "replay_exhausted"
		result.Success = false
		return result, nil
	}

	// Attempt to parse via the provided parser
	parseResult := parser(raw)

	// Log the replay attempt
	logQuery := `
		INSERT INTO quarantine_replay_log (raw_id, attempt_number, parse_success, parse_reason, logged_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
	`

	parseSuccess := parseResult.Error == nil
	_, err = rs.db.ExecContext(ctx, logQuery, rawID, result.AttemptCount, parseSuccess, parseResult.Reason)
	if err != nil {
		return result, fmt.Errorf("log replay: %w", err)
	}

	// Update raw status based on parse result
	if parseSuccess {
		// Clear quarantine on successful parse
		clearQuery := `
			UPDATE raw
			SET quarantined = false, quarantine_reason = NULL
			WHERE id = $1
		`
		_, err := rs.db.ExecContext(ctx, clearQuery, rawID)
		if err != nil {
			return result, fmt.Errorf("clear quarantine: %w", err)
		}

		result.NewQuarantine = false
		result.Success = true
		return result, nil
	}

	// Parse failed; re-quarantine with new reason
	if result.AttemptCount < maxAttempts {
		// Still has attempts left; update reason but keep quarantined
		updateQuery := `
			UPDATE raw
			SET quarantine_reason = $2, updated_at = CURRENT_TIMESTAMP
			WHERE id = $1
		`
		_, err := rs.db.ExecContext(ctx, updateQuery, rawID, parseResult.Reason)
		if err != nil {
			return result, fmt.Errorf("update quarantine reason: %w", err)
		}
	}

	result.NewQuarantine = true
	result.QuarantineReason = parseResult.Reason
	result.Success = false
	return result, nil
}

// ListQuarantinedForReplay returns quarantined raws not yet exhausted.
func (rs *ReplayStore) ListQuarantinedForReplay(
	ctx context.Context,
	sourceID string,
	maxAttempts int,
	limit int,
) ([]model.Raw, error) {
	query := `
		SELECT id, source_id, natural_key, payload, quarantined, quarantine_reason
		FROM raw
		WHERE quarantined = true AND quarantine_reason != 'replay_exhausted'
			AND source_id = $1
		ORDER BY updated_at ASC
		LIMIT $2
	`

	rows, err := rs.db.QueryContext(ctx, query, sourceID, limit)
	if err != nil {
		return nil, fmt.Errorf("list quarantined: %w", err)
	}
	defer rows.Close()

	var raws []model.Raw
	for rows.Next() {
		var raw model.Raw
		if err := rows.Scan(
			&raw.SourceID,
			&raw.NaturalKey,
			&raw.Payload,
			&raw.Quarantined,
			&raw.QuarantineReason,
		); err != nil {
			return nil, fmt.Errorf("scan raw: %w", err)
		}
		raws = append(raws, raw)
	}

	return raws, rows.Err()
}

// GetReplayHistory returns all replay attempts for a raw.
func (rs *ReplayStore) GetReplayHistory(ctx context.Context, rawID int64) ([]map[string]interface{}, error) {
	query := `
		SELECT attempt_number, parse_success, parse_reason, logged_at
		FROM quarantine_replay_log
		WHERE raw_id = $1
		ORDER BY logged_at ASC
	`

	rows, err := rs.db.QueryContext(ctx, query, rawID)
	if err != nil {
		return nil, fmt.Errorf("get replay history: %w", err)
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var attemptNumber int
		var parseSuccess bool
		var parseReason string
		var loggedAt time.Time

		if err := rows.Scan(&attemptNumber, &parseSuccess, &parseReason, &loggedAt); err != nil {
			return nil, fmt.Errorf("scan replay log: %w", err)
		}

		entry := map[string]interface{}{
			"attempt_number": attemptNumber,
			"parse_success":  parseSuccess,
			"parse_reason":   parseReason,
			"logged_at":      loggedAt,
		}
		history = append(history, entry)
	}

	return history, rows.Err()
}

// CleanupDeadLetters removes dead-letter quarantine records older than retentionDays.
func (rs *ReplayStore) CleanupDeadLetters(ctx context.Context, retentionDays int) (int64, error) {
	query := `
		DELETE FROM raw
		WHERE quarantine_reason = 'replay_exhausted'
			AND created_at < CURRENT_TIMESTAMP - INTERVAL '1 day' * $1
	`

	result, err := rs.db.ExecContext(ctx, query, retentionDays)
	if err != nil {
		return 0, fmt.Errorf("cleanup dead-letters: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	return rowsAffected, nil
}
