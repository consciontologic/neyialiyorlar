---
name: drift-alert-resolution
description: "Drift alert resolution. You just fixed an upstream schema/DOM change that the harvester flagged as a breaking drift alert; the alert persists in the İzleme Paneli until you resolve it."
---

# Drift alert resolution

## When to use

The harvester detects when an upstream source (Yahoo chart JSON today,
more adapters over time) changes its **structural contract** — a required
field removed or a field retyped — and persists a **breaking** drift alert
to the `drift_alert` table. These alerts surface in the dedicated *Şema
Kayması Uyarıları* section of the İzleme Paneli and **stay there until
explicitly resolved**.

Read this skill when **you have just fixed** the code that consumes a
drifted payload (updated a struct, a JSON path, a parser, a selector). The
fix alone does not clear the alert — a fixed-but-unresolved alert is a
false positive that keeps screaming on the panel.

## Why alerts persist

Persistence is the whole point of the feature: an operator must be able to
see an outstanding contract break across restarts, not a log line that
scrolls away. The lifecycle is:

1. Harvester observes a breaking structural change → UPSERT an `open` row
   (deduped by `signature`; recurrence bumps `occurrences` + `last_seen_at`).
2. The alert shows on the panel with field-level diffs (`added` `+`,
   `removed` `−`, `retyped` `~`) until someone resolves it.
3. **You** fix the upstream-consuming code **and** resolve the alert.
4. A *later* recurrence of the same signature after resolution is a **new**
   incident (a fresh `open` row), because the unique index only covers
   `status='open'`.

## Procedure

After your code fix is committed/staged and the underlying drift is
genuinely handled:

1. **Find the alert id.** From the panel, or:
   ```
   GET /api/v1/admin/drift          # open alerts, breaking-first
   ```
   or SQL: `SELECT id, source, summary FROM drift_alert WHERE status='open';`

2. **Resolve it.** Either path is fine:
   - HTTP: `POST /api/v1/admin/drift/{id}/resolve`
     → `200 {"resolved": true, "id": N}`; `404` if it is already resolved
     or the id is unknown (nothing open to clear).
   - SQL (when the API is not running):
     ```sql
     UPDATE drift_alert
     SET status = 'resolved', resolved_at = now()
     WHERE id = $1 AND status = 'open';
     ```

3. **Record it.** Append a tracking row noting the resolution alongside the
   fix, e.g. `action=note, scope=drift, summary="resolve drift alert N
   (<source>: <what changed>)"`.

4. **Confirm.** Re-`GET /api/v1/admin/drift` (or reload the panel) and
   verify the alert is gone.

## Anti-patterns

- **Fixing the code but not resolving the alert.** The panel still shows a
  red breaking alert; the next operator wastes time re-investigating a
  solved problem.
- **Resolving without fixing.** Never clear an alert you have not actually
  addressed — that hides a live contract break. Resolution asserts "handled".
- **Editing `status` by hand to anything other than `resolved`.** The only
  valid states are `open` and `resolved`; the partial unique index and the
  panel query both depend on that.
- **Loosening `yahooRequiredFields`** (or another adapter's required set)
  just to silence an alert. If a field genuinely left the contract, fix the
  consumer; if it was never actually required, that is a deliberate config
  change, not a silencing tactic — justify it.
