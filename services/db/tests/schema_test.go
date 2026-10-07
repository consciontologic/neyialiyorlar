package tests

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// TestRawPayloadUniqueConstraint verifies duplicate (source, natural_key, content_hash) is rejected
func TestRawPayloadUniqueConstraint(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Insert first payload
	_, err := db.ExecContext(ctx, `
		INSERT INTO raw_payload (source, natural_key, content_hash, http_status, media_type, payload)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, "bist", "key1", []byte("hash1"), 200, "application/json", []byte("payload1"))
	if err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	// Try to insert duplicate (source, natural_key, content_hash)
	_, err = db.ExecContext(ctx, `
		INSERT INTO raw_payload (source, natural_key, content_hash, http_status, media_type, payload)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, "bist", "key1", []byte("hash1"), 200, "application/json", []byte("payload1"))

	// Should fail with unique constraint violation
	if err == nil {
		t.Errorf("Expected duplicate constraint error, but insert succeeded")
	} else if err == sql.ErrNoRows {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestMetricValueIdempotency verifies duplicate (metric_key, entity, ts, inputs_hash) is idempotent
func TestMetricValueIdempotency(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ts := time.Now().UTC()
	metric_key := "velocity_accumulation"
	entity := "banks"
	inputs_hash := []byte("inputs_hash_1")
	value := 1.83
	flag := "fresh"
	tier := "daily"

	// Insert first row
	_, err := db.ExecContext(ctx, `
		INSERT INTO metric_value (metric_key, entity, ts, value, flag, tier, inputs_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, metric_key, entity, ts, value, flag, tier, inputs_hash)
	if err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	// Try to insert identical row again - should fail due to primary key constraint
	_, err = db.ExecContext(ctx, `
		INSERT INTO metric_value (metric_key, entity, ts, value, flag, tier, inputs_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, metric_key, entity, ts, value, flag, tier, inputs_hash)

	if err == nil {
		t.Errorf("Expected duplicate key error, but second insert succeeded")
	}

	// Verify the row count is still 1
	var count int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM metric_value
		WHERE metric_key = $1 AND entity = $2 AND ts = $3 AND inputs_hash = $4
	`, metric_key, entity, ts, inputs_hash).Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 row, got %d", count)
	}
}

// TestCoverageIndexExists verifies mv_hot covering index exists and is functional
func TestCoverageIndexExists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.statistics
			WHERE table_name = 'metric_value' AND index_name = 'mv_hot'
		)
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("Index query failed: %v", err)
	}
	if !exists {
		t.Errorf("Expected mv_hot index to exist")
	}
}

// TestDegradedPartialIndexExists verifies mv_degraded partial index exists
func TestDegradedPartialIndexExists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.statistics
			WHERE table_name = 'metric_value' AND index_name = 'mv_degraded'
		)
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("Index query failed: %v", err)
	}
	if !exists {
		t.Errorf("Expected mv_degraded partial index to exist")
	}
}

// TestEntityRefWithValidityRanges verifies entity_ref table structure
func TestEntityRefWithValidityRanges(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Insert an entity
	_, err := db.ExecContext(ctx, `
		INSERT INTO entity_ref (entity_id, entity_type, display_ticker, valid_from, valid_to, isin)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, "ISIN_123", "security", "TSK", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
		nil, "ISIN_123")
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Verify it exists
	var entity_id string
	err = db.QueryRowContext(ctx, `
		SELECT entity_id FROM entity_ref WHERE entity_id = $1
	`, "ISIN_123").Scan(&entity_id)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if entity_id != "ISIN_123" {
		t.Errorf("Expected entity_id 'ISIN_123', got '%s'", entity_id)
	}
}

// TestCorporateActionTable verifies corporate_action table structure
func TestCorporateActionTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First insert an entity_ref
	_, err := db.ExecContext(ctx, `
		INSERT INTO entity_ref (entity_id, entity_type, display_ticker, valid_from, isin)
		VALUES ($1, $2, $3, $4, $5)
	`, "ISIN_456", "security", "ABC", "2020-01-01", "ISIN_456")
	if err != nil {
		t.Fatalf("Insert entity_ref failed: %v", err)
	}

	// Insert a corporate action
	_, err = db.ExecContext(ctx, `
		INSERT INTO corporate_action (entity_id, action_type, ex_date, factor)
		VALUES ($1, $2, $3, $4)
	`, "ISIN_456", "split", "2021-06-15", 2.0)
	if err != nil {
		t.Fatalf("Insert corporate_action failed: %v", err)
	}

	// Verify it exists
	var factor float64
	err = db.QueryRowContext(ctx, `
		SELECT factor FROM corporate_action
		WHERE entity_id = $1 AND action_type = $2
	`, "ISIN_456", "split").Scan(&factor)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if factor != 2.0 {
		t.Errorf("Expected factor 2.0, got %f", factor)
	}
}

// TestEnumTypes verifies confidence and cadence_tier enum types exist
func TestEnumTypes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tests := []struct {
		name      string
		enumType  string
		enumValue string
	}{
		{"confidence fresh", "confidence", "fresh"},
		{"confidence stale", "confidence", "stale"},
		{"confidence approx", "confidence", "approx"},
		{"cadence_tier intraday", "cadence_tier", "intraday"},
		{"cadence_tier daily", "cadence_tier", "daily"},
		{"cadence_tier weekly", "cadence_tier", "weekly"},
		{"cadence_tier event", "cadence_tier", "event"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var exists bool
			err := db.QueryRowContext(ctx, fmt.Sprintf(`
				SELECT EXISTS (
					SELECT 1 FROM pg_enum
					WHERE enumtypid = (SELECT oid FROM pg_type WHERE typname = $1)
					AND enumlabel = $2
				)
			`), tt.enumType, tt.enumValue).Scan(&exists)
			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}
			if !exists {
				t.Errorf("Expected enum value %s.%s to exist", tt.enumType, tt.enumValue)
			}
		})
	}
}

// Helper function to set up a test database connection
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// Get connection string from environment or use default
	// This assumes the test is run with docker-compose up
	connStr := "host=localhost user=neyi password=neyidev dbname=neyialiyorlar sslmode=disable"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("Could not connect to test database: %v", err)
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("Could not ping test database: %v", err)
	}

	return db
}
