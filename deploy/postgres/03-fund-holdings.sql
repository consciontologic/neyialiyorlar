-- 03-fund-holdings.sql
-- Fund reference catalogue and per-fund stock holding records.
-- Source: TEFAS (Playwright scrape of FonAnaliz.aspx) +
--         KAP monthly PDF disclosure cross-validation.
-- Populated by the fund-scraper service; read by the API layer.
--
-- ⚠️  NOT auto-applied. Only deploy/postgres/01-init-users.sql is mounted into
--     docker-entrypoint-initdb.d; schema is applied by golang-migrate from
--     services/db/migrations (mandate 14). The CANONICAL definition of these
--     tables now lives in services/db/migrations/0002_fund_holdings.up.sql —
--     this file is kept for reference only. Edit the migration, not this file.

BEGIN;

-- Fund catalogue: one row per TEFAS fund code.
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
-- One row per (fund, stock, as_of_date) from TEFAS.
-- KAP-sourced rows have source='kap' and carry an official weight.
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

-- Grant to app user
GRANT SELECT, INSERT, UPDATE, DELETE ON fund_ref     TO neyi;
GRANT SELECT, INSERT, UPDATE, DELETE ON fund_holding TO neyi;

COMMIT;
