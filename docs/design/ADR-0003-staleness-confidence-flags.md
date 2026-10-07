<!-- docs/design/ADR-0003-staleness-confidence-flags.md -->

# 📐 ADR-0003: Confidence-flag propagation (`fresh | stale | approx`)

- **Status**: accepted
- **Date**: 2026-06-14
- **Deciders**: Architecture
- **Related**: [METRICS_CATALOG.md](METRICS_CATALOG.md)

## Context

Half the Alpha Matrix is 🟡 PROXY and several 🟢 sources can fall behind cadence
(EVDS weekly, MKK T+10). The cardinal rule is: **no proxy or stale value may
ever be mistaken for a fresh measurement, and nothing is ever fabricated.** That
honesty must survive every layer from ingestion to the dashboard tile.

## Decision

Attach a **confidence flag** `fresh | stale | approx` to every value at the
point of computation and propagate it unchanged through DB → Redis → API → UI.
A derived metric inherits the **worst** flag of its inputs
(`approx` > `stale` > `fresh`).

## Consequences

- ➕ The UI can render proxies/late data distinctly (badge/dimming) — the analyst
  is never misled.
- ➕ A single, total ordering makes propagation a trivial `max()` over inputs.
- ➕ "Never fabricate" is enforceable: an unavailable source yields a **gap** or
  `stale`/`approx`, not a synthesised number.
- ➖ Every schema, cache key, API payload, and widget must carry the flag —
  accepted as a first-class field, not an afterthought.
- 🔁 Extensible: new flags (e.g. `quarantined`) slot into the ordering later.

## Considered options

- **Three-state flag with worst-input propagation** (chosen) — simple, total,
  honest.
- **Boolean `is_stale`** — rejected: cannot distinguish "late direct" from
  "proxy by nature."
- **Numeric confidence score** — rejected: false precision; harder to reason
  about and to test than a small ordered enum.
