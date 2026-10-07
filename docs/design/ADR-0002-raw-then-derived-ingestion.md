<!-- docs/design/ADR-0002-raw-then-derived-ingestion.md -->

# 📐 ADR-0002: Raw-then-derived ingestion (store payloads first, compute second)

- **Status**: accepted
- **Date**: 2026-06-14
- **Deciders**: Architecture
- **Related**: [DATA_SOURCES.md](DATA_SOURCES.md), [ADR-0003](ADR-0003-staleness-confidence-flags.md)

## Context

Sources drift (field renames, layout changes), go late, or briefly disappear.
If we parse-and-discard, a parser bug or a schema shift permanently loses data
we already fetched, and metrics cannot be recomputed or audited. Reliability
mandate 6 requires data-corruption recovery and transaction integrity.

## Decision

Persist the **untouched raw payload first** (JSON/HTML/PDF bytes + fetch
metadata + content hash) in PostgreSQL, then compute derived metrics from the
stored raw in a separate, **idempotent** step keyed by `(source, natural_key,
content_hash)`.

## Consequences

- ➕ Every metric is **replayable** from raw — fix a parser, recompute history,
  no re-fetch.
- ➕ **Idempotent** recompute (upsert on the derived key) means re-runs never
  duplicate or drift.
- ➕ Parser layout-drift → **quarantine the raw**, not crash; metric holds at
  last-good with `approx`.
- ➕ Natural audit trail: raw + hash proves provenance, supports golden-file tests.
- ➖ Extra storage for raw payloads — negligible at KB–MB/day.
- 🔁 Reversible per metric: derived tables can be dropped and rebuilt from raw.

## Considered options

- **Raw-then-derived** (chosen) — replayable, idempotent, recovery-friendly.
- **Parse-on-ingest, store only derived** — rejected: unrecoverable on parser
  bugs/schema drift; no audit; violates reliability mandate.
- **Event-log/Kafka stream** — rejected: over-built for KB–MB/day batch cadence.
