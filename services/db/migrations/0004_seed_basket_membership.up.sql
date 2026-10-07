-- 0004_seed_basket_membership.up.sql
-- Seed BIST30 (XU030) and BIST100 (XU100) index memberships into basket_member.
--
-- These memberships were previously defined ONLY in
-- deploy/postgres/02-bist-entities.sql, which postgres runs exclusively at
-- first container init (docker-entrypoint-initdb.d). Any database created
-- before that seed existed — or restored from a volume that predates it —
-- therefore has an EMPTY basket_member table. The API's
-- loadEntitiesFromDB LEFT JOINs basket_member to build each stock's
-- basket_ids, so with no rows every stock resolved to an empty basket list
-- and the Flutter stock-detail screen rendered "Endeks: Endeks dışı"
-- (out-of-index) for index constituents like THYAO, GARAN, ASELS, etc.
-- This migration is the single applied source of truth for that data.
--
-- Note on numbering: this is 0004 rather than 0003 because the live database
-- reports schema version 3 with no corresponding 0003 file ever present in
-- git history (a stale forced version). golang-migrate's `up` only applies
-- versions strictly greater than the current one, so 0004 guarantees this
-- runs on the existing database while still applying cleanly to a fresh one.
--
-- Each INSERT joins entity_ref so that tickers absent from the security
-- universe are skipped rather than raising a foreign-key violation
-- (basket_member.entity_id REFERENCES entity_ref.entity_id). ON CONFLICT
-- keeps the migration idempotent. Compositions mirror the verified Q2-2026
-- lists in deploy/postgres/02-bist-entities.sql — no fabricated members.
BEGIN;

-- XU030 (BIST30) — verified composition Q2-2026.
INSERT INTO basket_member (basket_id, entity_id, valid_from)
SELECT m.basket_id, m.entity_id, DATE '2020-01-01'
FROM (VALUES
    ('XU030', 'AKBNK'), ('XU030', 'AKSEN'), ('XU030', 'ARCLK'),
    ('XU030', 'ASELS'), ('XU030', 'BIMAS'), ('XU030', 'DOHOL'),
    ('XU030', 'EKGYO'), ('XU030', 'ENKAI'), ('XU030', 'EREGL'),
    ('XU030', 'FROTO'), ('XU030', 'GARAN'), ('XU030', 'HALKB'),
    ('XU030', 'ISCTR'), ('XU030', 'KCHOL'), ('XU030', 'KORDS'),
    ('XU030', 'ODAS'),  ('XU030', 'PETKM'), ('XU030', 'PGSUS'),
    ('XU030', 'SAHOL'), ('XU030', 'SASA'),  ('XU030', 'SISE'),
    ('XU030', 'TAVHL'), ('XU030', 'TCELL'), ('XU030', 'THYAO'),
    ('XU030', 'TOASO'), ('XU030', 'TTKOM'), ('XU030', 'TUPRS'),
    ('XU030', 'VAKBN'), ('XU030', 'YKBNK'), ('XU030', 'GUBRF')
) AS m(basket_id, entity_id)
JOIN entity_ref e ON e.entity_id = m.entity_id AND e.valid_to IS NULL
ON CONFLICT DO NOTHING;

-- XU100 (BIST100) — XU030 plus the additional constituents.
INSERT INTO basket_member (basket_id, entity_id, valid_from)
SELECT m.basket_id, m.entity_id, DATE '2020-01-01'
FROM (VALUES
    ('XU100', 'AKBNK'), ('XU100', 'AKSEN'), ('XU100', 'ARCLK'),
    ('XU100', 'ASELS'), ('XU100', 'BIMAS'), ('XU100', 'DOHOL'),
    ('XU100', 'EKGYO'), ('XU100', 'ENKAI'), ('XU100', 'EREGL'),
    ('XU100', 'FROTO'), ('XU100', 'GARAN'), ('XU100', 'HALKB'),
    ('XU100', 'ISCTR'), ('XU100', 'KCHOL'), ('XU100', 'KORDS'),
    ('XU100', 'ODAS'),  ('XU100', 'PETKM'), ('XU100', 'PGSUS'),
    ('XU100', 'SAHOL'), ('XU100', 'SASA'),  ('XU100', 'SISE'),
    ('XU100', 'TAVHL'), ('XU100', 'TCELL'), ('XU100', 'THYAO'),
    ('XU100', 'TOASO'), ('XU100', 'TTKOM'), ('XU100', 'TUPRS'),
    ('XU100', 'VAKBN'), ('XU100', 'YKBNK'), ('XU100', 'GUBRF'),
    ('XU100', 'AEFES'), ('XU100', 'AGESA'), ('XU100', 'AGHOL'),
    ('XU100', 'ALARK'), ('XU100', 'ALBRK'), ('XU100', 'ANACM'),
    ('XU100', 'ASUZU'), ('XU100', 'AYGAZ'), ('XU100', 'BAGFS'),
    ('XU100', 'BANVT'), ('XU100', 'BRSAN'), ('XU100', 'BUCIM'),
    ('XU100', 'CCOLA'), ('XU100', 'CIMSA'), ('XU100', 'CLEBI'),
    ('XU100', 'CWENE'), ('XU100', 'DOAS'),  ('XU100', 'DEVA'),
    ('XU100', 'EGEEN'), ('XU100', 'EMKEL'), ('XU100', 'FENER'),
    ('XU100', 'GLYHO'), ('XU100', 'GOLTS'), ('XU100', 'GSRAY'),
    ('XU100', 'HLGYO'), ('XU100', 'INDES'), ('XU100', 'JANTS'),
    ('XU100', 'KARSN'), ('XU100', 'KLMSN'), ('XU100', 'KONTR'),
    ('XU100', 'KOZAA'), ('XU100', 'KOZAL'), ('XU100', 'KTSKR'),
    ('XU100', 'LOGO'),  ('XU100', 'MAVI'),  ('XU100', 'MGROS'),
    ('XU100', 'MPARK'), ('XU100', 'NETAS'), ('XU100', 'NTHOL'),
    ('XU100', 'OTKAR'), ('XU100', 'RYSAS'), ('XU100', 'SELEC'),
    ('XU100', 'SKBNK'), ('XU100', 'TATGD'), ('XU100', 'TKFEN'),
    ('XU100', 'TKNSA'), ('XU100', 'TTRAK'), ('XU100', 'TURSG'),
    ('XU100', 'ULKER'), ('XU100', 'ULAG'),  ('XU100', 'VESTL'),
    ('XU100', 'VESBE')
) AS m(basket_id, entity_id)
JOIN entity_ref e ON e.entity_id = m.entity_id AND e.valid_to IS NULL
ON CONFLICT DO NOTHING;

COMMIT;
