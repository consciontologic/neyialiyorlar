-- 0002_fund_holdings.up.sql
-- Fund reference catalogue, per-fund stock holdings, and the fund-scraper
-- progress / checkpoint / scan-cache tables.
--
-- These were previously defined ONLY in deploy/postgres/03-fund-holdings.sql
-- and deploy/postgres/04-scraper-progress.sql, neither of which is applied:
-- postgres mounts only 01-init-users.sql into docker-entrypoint-initdb.d, and
-- golang-migrate runs migrations from this directory. As a result the tables
-- were never created, so the fund-scraper crashed on its first
-- `INSERT INTO scraper_progress` and the API's GET /api/v1/entities/{id}/funds
-- query failed against the missing fund_holding / fund_ref tables — the UI
-- therefore listed no funds for any stock. This migration is the single source
-- of truth for that schema.
BEGIN;

-- ── Fund catalogue + holdings (mirrors deploy/postgres/03-fund-holdings.sql) ──

-- Fund catalogue: one row per TEFAS/KAP fund code.
CREATE TABLE IF NOT EXISTS fund_ref (
    fund_code      TEXT        NOT NULL,          -- TEFAS short code, e.g. 'YAC'
    fund_title     TEXT        NOT NULL,          -- full Turkish name
    fund_manager   TEXT        NOT NULL,          -- portföy yönetim şirketi
    fund_type      TEXT        NOT NULL DEFAULT 'EMK', -- 'EMK' = emeklilik
    as_of_date     DATE        NOT NULL,          -- last successful scrape date
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (fund_code)
);

-- Individual stock holdings inside each fund.
-- One row per (fund, stock, as_of_date, source). stock_id is the BIST ticker
-- (matches entity_ref.entity_id); the scraper derives it from the equity ISIN.
CREATE TABLE IF NOT EXISTS fund_holding (
    fund_code      TEXT        NOT NULL REFERENCES fund_ref(fund_code)
                                        ON DELETE CASCADE,
    stock_id       TEXT        NOT NULL,          -- matches entity_ref.entity_id
    weight_pct     DOUBLE PRECISION,              -- % of fund AUM; NULL if unknown
    as_of_date     DATE        NOT NULL,
    source         TEXT        NOT NULL DEFAULT 'tefas', -- 'tefas' | 'kap'
    scraped_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (fund_code, stock_id, as_of_date, source)
);

CREATE INDEX IF NOT EXISTS fund_holding_stock_idx
    ON fund_holding (stock_id, as_of_date DESC);

CREATE INDEX IF NOT EXISTS fund_holding_fund_idx
    ON fund_holding (fund_code, as_of_date DESC);

GRANT SELECT, INSERT, UPDATE, DELETE ON fund_ref     TO neyi;
GRANT SELECT, INSERT, UPDATE, DELETE ON fund_holding TO neyi;

-- ── Scraper progress / checkpoint / scan cache (mirrors 04-scraper-progress) ──

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
