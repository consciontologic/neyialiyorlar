-- 0006_incident.down.sql
-- Reverse 0006_incident: drop the incident timeline child first (FK), then the
-- incident table itself (indexes / sequences go with their tables).
BEGIN;

DROP TABLE IF EXISTS incident_event;
DROP TABLE IF EXISTS incident;

COMMIT;
