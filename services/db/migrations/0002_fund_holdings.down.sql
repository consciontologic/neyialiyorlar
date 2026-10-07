-- 0002_fund_holdings.down.sql
-- Rollback the fund-holdings + fund-scraper schema (reverse dependency order).
BEGIN;

-- Scraper bookkeeping tables (no inter-table FKs).
DROP TABLE IF EXISTS scraper_scan_range CASCADE;
DROP TABLE IF EXISTS scraper_scan_cache CASCADE;
DROP TABLE IF EXISTS scraper_checkpoint CASCADE;
DROP TABLE IF EXISTS scraper_progress CASCADE;

-- fund_holding references fund_ref, so drop it first.
DROP TABLE IF EXISTS fund_holding CASCADE;
DROP TABLE IF EXISTS fund_ref CASCADE;

COMMIT;
