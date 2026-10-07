-- 0001_initial.up.sql
-- Complete schema: Phase 2 ingestion + Phase 4 derived metrics & caching topology
BEGIN;

CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    dirty BOOLEAN NOT NULL
);

-- ============================================================================
-- PHASE 2: Raw ingestion & replay infrastructure
-- ============================================================================

-- Raw payloads: immutable store for audit and replay (ADR-0002)
CREATE TABLE IF NOT EXISTS raw (
    id BIGSERIAL PRIMARY KEY,
    source_id VARCHAR(64) NOT NULL,           -- e.g., "bist", "kap", "mkkvap", "evds"
    natural_key VARCHAR(256) NOT NULL,        -- ticker/isin/series from source
    content_hash VARCHAR(64) NOT NULL UNIQUE, -- SHA256 for deduplication
    payload BYTEA NOT NULL,                   -- raw bytes as-is
    fetched_at TIMESTAMP NOT NULL,            -- when observed
    quarantined BOOLEAN DEFAULT false,        -- parse failure flag
    quarantine_reason VARCHAR(256),           -- "envelope_error", "parse_error", etc.
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_raw_source_id ON raw(source_id);
CREATE INDEX IF NOT EXISTS idx_raw_content_hash ON raw(content_hash);
CREATE INDEX IF NOT EXISTS idx_raw_quarantined ON raw(quarantined);

-- High-water marks: cursor persistence for idempotent restarts (Phase 2)
CREATE TABLE IF NOT EXISTS source_cursor (
    source_id VARCHAR(64) NOT NULL,
    cursor TEXT NOT NULL,
    fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (source_id, cursor)
);

CREATE INDEX IF NOT EXISTS idx_source_cursor_source ON source_cursor(source_id);
CREATE INDEX IF NOT EXISTS idx_source_cursor_updated ON source_cursor(updated_at);

-- Quarantine replay log: tracks replay attempts for dead-letter handling (Phase 2)
CREATE TABLE IF NOT EXISTS quarantine_replay_log (
    id BIGSERIAL PRIMARY KEY,
    raw_id BIGINT NOT NULL,
    attempt_number INT NOT NULL,
    parse_success BOOLEAN NOT NULL,
    parse_reason VARCHAR(256),
    logged_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (raw_id) REFERENCES raw(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_replay_log_raw_id ON quarantine_replay_log(raw_id);
CREATE INDEX IF NOT EXISTS idx_replay_log_logged ON quarantine_replay_log(logged_at);

-- ============================================================================
-- PHASE 4: Derived metrics schema & Redis caching topology
-- ============================================================================

-- Confidence flag enum (Phase 4, mandate 3)
CREATE TYPE confidence AS ENUM ('fresh', 'stale', 'approx');

-- Cadence tier enum (Phase 4, mandate 3)
CREATE TYPE cadence_tier AS ENUM ('intraday', 'daily', 'weekly', 'event');

-- Raw payload table (Phase 4, canonical schema for raw storage)
-- This aligns with ADR-0002: content-addressed storage for idempotent ingest & replay
CREATE TABLE IF NOT EXISTS raw_payload (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source TEXT NOT NULL,                      -- bist|kap|mkk_vap|evds
    natural_key TEXT NOT NULL,                 -- e.g. disclosure id / series code / date
    content_hash BYTEA NOT NULL,               -- sha256(payload) -> dedupe + replay
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    http_status INT NOT NULL,
    media_type TEXT NOT NULL,                  -- application/json|text/html|application/pdf
    payload BYTEA NOT NULL,                    -- raw bytes (blob offload optional)
    quarantined BOOLEAN NOT NULL DEFAULT false,
    UNIQUE (source, natural_key, content_hash) -- same bytes never stored twice
);

CREATE INDEX IF NOT EXISTS raw_src_key_time ON raw_payload (source, natural_key, fetched_at DESC);
CREATE INDEX IF NOT EXISTS raw_quarantine ON raw_payload (source) WHERE quarantined;

-- Derived metrics: one row per metric/key/inputs, idempotent, read-hot (Phase 4)
CREATE TABLE IF NOT EXISTS metric_value (
    metric_key TEXT NOT NULL,                  -- velocity_accumulation, nimvi, …
    entity TEXT NOT NULL,                      -- security/fund/index/aggregate
    ts TIMESTAMPTZ NOT NULL,                   -- observation time (cadence-aligned)
    value DOUBLE PRECISION,                    -- NULL allowed = gap (never fabricated)
    flag confidence NOT NULL,
    tier cadence_tier NOT NULL,
    inputs_hash BYTEA NOT NULL,                -- hash of consumed raw rows -> idempotency
    computed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (metric_key, entity, ts, inputs_hash)
);

-- Covering index for the hot read "latest N points of metric for entity"
CREATE INDEX IF NOT EXISTS mv_hot ON metric_value (metric_key, entity, ts DESC) INCLUDE (value, flag);

-- Partial index to surface degraded values fast for the dashboard badge layer
CREATE INDEX IF NOT EXISTS mv_degraded ON metric_value (metric_key, ts DESC) WHERE flag <> 'fresh';

-- Source health: breaker/cadence state for staleness layer + ops view (Phase 4)
CREATE TABLE IF NOT EXISTS source_health (
    source TEXT PRIMARY KEY,
    last_ok_at TIMESTAMPTZ,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),   -- detect a stale health row itself
    breaker_open BOOLEAN NOT NULL DEFAULT false,
    consecutive_failures INT NOT NULL DEFAULT 0
);

-- Promotion ledger: IC gate decisions (Phase 6, prepared in Phase 4)
CREATE TABLE IF NOT EXISTS promotion_ledger (
    metric_key TEXT NOT NULL,
    run_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ic_value DOUBLE PRECISION,
    rolling_ic DOUBLE PRECISION,
    threshold DOUBLE PRECISION NOT NULL,
    horizon_days INT,
    window_size INT,
    decision TEXT NOT NULL CHECK (decision IN ('GO', 'NO-GO', 'PENDING')),
    notes TEXT,
    PRIMARY KEY (metric_key, run_at)
);

-- Entity reference master: ISIN/MKK identity with validity ranges (Phase 4, mandate 23)
CREATE TABLE IF NOT EXISTS entity_ref (
    entity_id TEXT PRIMARY KEY,                -- ISIN / MKK code (stable across renames)
    entity_type TEXT NOT NULL,                 -- security | fund | index | basket
    display_ticker TEXT,                       -- current display symbol (may change)
    valid_from DATE NOT NULL,
    valid_to DATE,                             -- NULL = active; set on delist/rename
    isin TEXT
);

-- Basket member: versioned theme-basket composition (Phase 4, mandate 23)
CREATE TABLE IF NOT EXISTS basket_member (
    basket_id TEXT NOT NULL,
    entity_id TEXT NOT NULL REFERENCES entity_ref(entity_id),
    weight DOUBLE PRECISION,
    valid_from DATE NOT NULL,
    valid_to DATE,
    PRIMARY KEY (basket_id, entity_id, valid_from)
);

-- Corporate action: split/bonus/rights/dividend/rename events (Phase 4, mandates 23, 25)
CREATE TABLE IF NOT EXISTS corporate_action (
    entity_id TEXT NOT NULL REFERENCES entity_ref(entity_id),
    action_type TEXT NOT NULL,                 -- split | bonus | rights | dividend | rename
    ex_date DATE NOT NULL,
    factor DOUBLE PRECISION NOT NULL,          -- cumulative price-adjustment factor
    PRIMARY KEY (entity_id, action_type, ex_date)
);

COMMIT;
