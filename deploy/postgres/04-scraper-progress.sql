-- 04-scraper-progress.sql
-- Scraper run progress log and per-fund checkpoint for smart incremental fetches.
-- Populated by the fund-scraper service; read by the API admin endpoint.
--
-- ⚠️  NOT auto-applied. Only deploy/postgres/01-init-users.sql is mounted into
--     docker-entrypoint-initdb.d; schema is applied by golang-migrate from
--     services/db/migrations (mandate 14). The CANONICAL definition of these
--     tables now lives in services/db/migrations/0002_fund_holdings.up.sql —
--     this file is kept for reference only. Edit the migration, not this file.

BEGIN;

-- One row per scraper run; updated in-place as the run progresses.
CREATE TABLE IF NOT EXISTS scraper_progress (
    run_id           TEXT        NOT NULL,
    started_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at      TIMESTAMPTZ,
    status           TEXT        NOT NULL DEFAULT 'running', -- running | done | error
    scan_start       INT         NOT NULL DEFAULT 0,
    scan_end         INT         NOT NULL DEFAULT 0,
    indices_scanned  INT         NOT NULL DEFAULT 0,
    disclosures_found INT        NOT NULL DEFAULT 0,
    funds_total      INT         NOT NULL DEFAULT 0,
    funds_done       INT         NOT NULL DEFAULT 0,
    holdings_saved   INT         NOT NULL DEFAULT 0,
    errors           INT         NOT NULL DEFAULT 0,
    last_fund        TEXT,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (run_id)
);

-- Per-fund checkpoint: prevents re-downloading a PDF we already parsed.
-- (fund_code, kap_idx) is the natural key — same index == same filing.
CREATE TABLE IF NOT EXISTS scraper_checkpoint (
    fund_code   TEXT NOT NULL,
    kap_idx     INT  NOT NULL,
    as_of_date  DATE NOT NULL,
    scraped_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (fund_code, kap_idx)
);

GRANT SELECT, INSERT, UPDATE ON scraper_progress   TO neyi;
GRANT SELECT, INSERT, UPDATE ON scraper_checkpoint TO neyi;

-- Discovered disclosures from KAP index scans — never re-scanned once found.
-- Persists across container restarts so the slow scan phase is skipped.
CREATE TABLE IF NOT EXISTS scraper_scan_cache (
    idx           INT         NOT NULL,
    stock_code    TEXT        NOT NULL,
    fund_type     TEXT        NOT NULL,
    company_title TEXT        NOT NULL,
    mkk_member_oid TEXT,
    publish_date  TEXT,
    year          INT         NOT NULL DEFAULT 2025,
    attachments   JSONB       NOT NULL DEFAULT '[]',
    found_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (idx)
);

-- Tracks which (start, end, step) ranges have been fully scanned.
CREATE TABLE IF NOT EXISTS scraper_scan_range (
    scan_start INT NOT NULL,
    scan_end   INT NOT NULL,
    scan_step  INT NOT NULL,
    scanned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (scan_start, scan_end, scan_step)
);

GRANT SELECT, INSERT, UPDATE ON scraper_scan_cache TO neyi;
GRANT SELECT, INSERT, UPDATE ON scraper_scan_range TO neyi;

COMMIT;
