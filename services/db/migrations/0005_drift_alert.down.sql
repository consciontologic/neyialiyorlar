-- 0005_drift_alert.down.sql
-- Reverse 0005_drift_alert: drop the persisted drift-alert table (and its
-- indexes / sequence, which Postgres removes with the table).
BEGIN;

DROP TABLE IF EXISTS drift_alert;

COMMIT;
