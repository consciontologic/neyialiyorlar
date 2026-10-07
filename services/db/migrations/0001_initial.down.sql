-- 0001_initial.down.sql
-- Rollback the complete schema (Phase 2 + Phase 4)
BEGIN;

-- Drop Phase 4 tables in dependency order (reverse of creation)
DROP TABLE IF EXISTS corporate_action CASCADE;
DROP TABLE IF EXISTS basket_member CASCADE;
DROP TABLE IF EXISTS entity_ref CASCADE;
DROP TABLE IF EXISTS promotion_ledger CASCADE;
DROP TABLE IF EXISTS source_health CASCADE;
DROP TABLE IF EXISTS metric_value CASCADE;
DROP TABLE IF EXISTS raw_payload CASCADE;

-- Drop Phase 4 enums
DROP TYPE IF EXISTS cadence_tier CASCADE;
DROP TYPE IF EXISTS confidence CASCADE;

-- Drop Phase 2 tables in dependency order
DROP TABLE IF EXISTS quarantine_replay_log CASCADE;
DROP TABLE IF EXISTS source_cursor CASCADE;
DROP TABLE IF EXISTS raw CASCADE;

-- Note: schema_migrations table is managed by golang-migrate and is dropped separately

COMMIT;
