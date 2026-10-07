# 🎨 `docs/design/` — design docs & ADRs

Design docs (forward-looking proposals) and ADRs (Architecture Decision
Records — accepted decisions with rationale).

## Files

- [`DESIGN.template.md`](DESIGN.template.md) — for proposing a non-trivial change before building it.
- [`ADR.template.md`](ADR.template.md) — for recording a code-level decision after it's made.

## When to write

- **Design doc**: before a change > a few days of work, before a public API, before a cross-module refactor. Reviewed by humans + agents, signed off before code lands.
- **ADR**: after a meaningful architectural decision, so future-you can ask "why is it this way?" and get an answer.

Both live alongside the code they shape — file name embeds the topic
(`DESIGN-auth-rework.md`, `ADR-0007-use-postgres-not-mongo.md`).

## Project design docs (`neyialiyorlar`)

- [`DATA_SOURCES.md`](DATA_SOURCES.md) — verified-live source integration map (BIST/KAP/MKK-VAP/EVDS), envelopes, cadences, degradation matrix, WAF hard boundary.
- [`METRICS_CATALOG.md`](METRICS_CATALOG.md) — the Alpha Matrix: 10 metrics with formulas, source color, build priority, confidence semantics, IC target.
- [`ADR-0001-go-primary-backend.md`](ADR-0001-go-primary-backend.md) — Go-primary, right-sized backend (no HFT-extreme).
- [`ADR-0002-raw-then-derived-ingestion.md`](ADR-0002-raw-then-derived-ingestion.md) — store raw payloads first, recompute idempotently.
- [`ADR-0003-staleness-confidence-flags.md`](ADR-0003-staleness-confidence-flags.md) — `fresh|stale|approx` propagation (worst input wins).
- [`ADR-0004-scraping-within-tos.md`](ADR-0004-scraping-within-tos.md) — self-adaptive scraping within ToS; no evasion, ever.
