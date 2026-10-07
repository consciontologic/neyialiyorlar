-- 0006_incident.up.sql
-- Persisted operational incidents ("hata"), surfaced on the İzleme Paneli.
--
-- Today an error only leaves an ephemeral trace: the fund-scraper bumps an
-- `errors` counter on scraper_progress, the harvester writes a slog line for
-- each fetch failure / quarantine / circuit-breaker flip / replay attempt, and
-- source_health keeps a single current snapshot. None of that answers the three
-- questions an operator actually asks when they see a "hata":
--
--   1. WHY did it happen?        -> incident.reason
--   2. WHAT did the system do?   -> incident.action_taken (+ the event timeline)
--   3. Did it help or not?       -> incident.outcome (recovered = positive,
--                                    failed = negative, pending = still open)
--
-- This migration persists every incident so it survives restarts and stays on
-- the panel until resolved, plus a child `incident_event` timeline that records
-- each step the system took in response (detected -> retry -> backoff ->
-- circuit_opened -> quarantined -> replay_attempt -> recovered/failed). The
-- timeline is what the per-incident markdown report is generated from.
--
-- Dedup: a partial unique index on signature WHERE status='open' collapses a
-- repeating failure into one open row (bumping occurrences / last_seen_at)
-- instead of flooding the panel. Once resolved, a later recurrence of the same
-- signature opens a NEW row — a repeat after a fix is a distinct incident.
BEGIN;

CREATE TABLE IF NOT EXISTS incident (
    id            BIGSERIAL   PRIMARY KEY,
    source        TEXT        NOT NULL,                 -- 'harvester' | 'scraper' | 'api' | 'analytic'
    component     TEXT        NOT NULL DEFAULT '',      -- finer site: 'fetch' | 'parse' | 'quarantine' | 'breaker' | 'replay' | 'run' | handler/job
    kind          TEXT        NOT NULL,                 -- error class: 'fetch_error' | 'circuit_breaker' | 'quarantine' | 'replay' | 'scraper_run' | 'api_error' | 'analytic_error' | 'parse_error'
    severity      TEXT        NOT NULL DEFAULT 'error', -- 'warn' | 'error' | 'critical'
    signature     TEXT        NOT NULL,                 -- stable hash for open-row dedup
    reason        TEXT        NOT NULL DEFAULT '',      -- WHY: error message / cause
    natural_key   TEXT        NOT NULL DEFAULT '',      -- entity in play (ticker / fund / isin)
    action_taken  TEXT        NOT NULL DEFAULT 'none',  -- WHAT: 'retry_backoff' | 'circuit_opened' | 'circuit_closed' | 'quarantined' | 'replayed' | 'skipped' | 'none'
    outcome       TEXT        NOT NULL DEFAULT 'pending', -- 'recovered' (positive) | 'failed' (negative) | 'pending'
    occurrences   INTEGER     NOT NULL DEFAULT 1,
    status        TEXT        NOT NULL DEFAULT 'open',  -- 'open' | 'resolved'
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at   TIMESTAMPTZ
);

-- At most one OPEN incident per signature; a recurrence bumps the existing row.
CREATE UNIQUE INDEX IF NOT EXISTS incident_open_signature_idx
    ON incident (signature) WHERE status = 'open';

-- Panel query path: open incidents, most-recently-seen first.
CREATE INDEX IF NOT EXISTS incident_status_idx
    ON incident (status, last_seen_at DESC);

-- Ordered lifecycle of what the system did about one incident. This is the
-- "report what happens in the system when a hata occurs": every step, in order,
-- with the per-step outcome where one applies.
CREATE TABLE IF NOT EXISTS incident_event (
    id          BIGSERIAL   PRIMARY KEY,
    incident_id BIGINT      NOT NULL REFERENCES incident (id) ON DELETE CASCADE,
    at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    phase       TEXT        NOT NULL,             -- 'detected' | 'retry' | 'backoff' | 'circuit_opened' | 'circuit_closed' | 'quarantined' | 'replay_attempt' | 'recovered' | 'failed' | 'resolved'
    note        TEXT        NOT NULL DEFAULT '',  -- human-readable detail for this step
    outcome     TEXT        NOT NULL DEFAULT ''   -- per-step outcome where meaningful: 'recovered' | 'failed' | ''
);

-- Timeline read path: all steps for one incident, oldest first.
CREATE INDEX IF NOT EXISTS incident_event_incident_idx
    ON incident_event (incident_id, at);

GRANT SELECT, INSERT, UPDATE, DELETE ON incident       TO neyi;
GRANT SELECT, INSERT, UPDATE, DELETE ON incident_event TO neyi;
GRANT USAGE, SELECT ON SEQUENCE incident_id_seq        TO neyi;
GRANT USAGE, SELECT ON SEQUENCE incident_event_id_seq  TO neyi;

COMMIT;
