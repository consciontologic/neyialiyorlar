-- 0004_seed_basket_membership.down.sql
-- Remove the XU030 / XU100 memberships seeded by the up migration.
-- Scoped to the seeded valid_from so a future, differently-dated source of
-- membership data is left untouched.
BEGIN;

DELETE FROM basket_member
WHERE basket_id IN ('XU030', 'XU100')
  AND valid_from = DATE '2020-01-01';

COMMIT;
