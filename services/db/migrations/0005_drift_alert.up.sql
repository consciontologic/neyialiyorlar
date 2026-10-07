-- 0005_drift_alert.up.sql
-- Persisted schema/DOM drift alerts, surfaced on the İzleme Paneli.
--
-- The harvester's drift detector (services/harvester/internal/parse/drift.go)
-- reduces every upstream payload to a structural fingerprint and classifies any
-- change as benign or breaking. A BREAKING drift (a required field removed, or
-- any field retyped) is a WARN-level operational alert: the upstream contract
-- changed and ingestion may be silently degraded. Previously these alerts were
-- only logged (ephemeral). This table persists every breaking alert so it
-- survives restarts and stays visible on the monitoring panel until an operator
-- (or the agent) fixes the underlying drift and marks it resolved.
--
-- Dedup: a partial unique index on signature WHERE status='open' collapses
-- repeated identical drift into one open row (bumping occurrences / last_seen_at)
-- instead of flooding the panel. Once an alert is resolved, a later recurrence
-- of the same signature inserts a NEW open row — a re-drift after a fix is a
-- distinct incident, not a reopening of the closed one.
BEGIN;

CREATE TABLE IF NOT EXISTS drift_alert (
    id            BIGSERIAL   PRIMARY KEY,
    source        TEXT        NOT NULL,              -- e.g. 'yahoo', 'mkkvap'
    severity      TEXT        NOT NULL,              -- 'benign' | 'breaking'
    signature     TEXT        NOT NULL,              -- stable hash of source+severity+diffs
    summary       TEXT        NOT NULL DEFAULT '',   -- compact human-readable description
    added         TEXT[]      NOT NULL DEFAULT '{}', -- field paths present now but not before
    removed       TEXT[]      NOT NULL DEFAULT '{}', -- field paths present before but not now
    retyped       TEXT[]      NOT NULL DEFAULT '{}', -- field paths whose kind changed
    status        TEXT        NOT NULL DEFAULT 'open', -- 'open' | 'resolved'
    occurrences   INTEGER     NOT NULL DEFAULT 1,    -- times this drift was observed
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at   TIMESTAMPTZ
);

-- At most one OPEN alert per signature; a recurrence bumps the existing row.
CREATE UNIQUE INDEX IF NOT EXISTS drift_alert_open_signature_idx
    ON drift_alert (signature) WHERE status = 'open';

-- Panel query path: open alerts, most-recently-seen first.
CREATE INDEX IF NOT EXISTS drift_alert_status_idx
    ON drift_alert (status, last_seen_at DESC);

GRANT SELECT, INSERT, UPDATE, DELETE ON drift_alert TO neyi;
GRANT USAGE, SELECT ON SEQUENCE drift_alert_id_seq TO neyi;

COMMIT;
