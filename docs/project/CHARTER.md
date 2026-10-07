<!--
docs/project/CHARTER.md — the one-screen "why" for neyialiyorlar.
Authoritative for scope and success criteria. The sequenced "how" lives in
docs/planning/ROADMAP.md; the source reality lives in docs/design/DATA_SOURCES.md.
-->

# 📜 Charter: `neyialiyorlar`

> Local-only research system (`https://neyialiyorlar.local`) that ingests,
> processes, and surfaces **institutional-liquidity indicators** for the
> Turkish equity & fund market from **verified-live public feeds**:
> **BIST, KAP, MKK/VAP, CBRT EVDS**.

## Vision

Give a single analyst a fast, dense, trustworthy terminal that turns slow,
fragmented Turkish market-microstructure disclosures into a small set of
**institutional-liquidity indicators** — each one honestly labelled by how
fresh and how direct its underlying data is, and none of them mistaken for
validated alpha until it has earned that label.

## Scope (in)

- Ingest four **live, free, public** sources (BIST JSON, KAP disclosures,
  MKK/VAP report pages, CBRT EVDS) on their natural cadences (intraday /
  daily / weekly / event-driven). See [DATA_SOURCES.md](../design/DATA_SOURCES.md).
- Compute the **10 buildable metrics** of the Alpha Matrix — 5 🟢 CORE,
  5 🟡 PROXY — every value carrying a `fresh|stale|approx` confidence flag.
  See [METRICS_CATALOG.md](../design/METRICS_CATALOG.md).
- Persist **raw payloads first, derived metrics second** (idempotent
  recompute) in PostgreSQL; serve sub-millisecond hot reads from Redis.
- Surface everything through a Go Web API (REST + WebSocket) and a Flutter
  PWA financial-terminal dashboard (web-first; native mobile later).
- Run the whole system from a single `docker-compose.yml` behind Nginx with
  local trusted TLS — **no local install assumptions**.
- Maintain a living `docs/` set and the append-only `ai/tracking.csv` ledger
  across the project life cycle.

## Scope (out)

- ❌ **No WAF / CAPTCHA / TLS-fingerprint / proxy-rotation evasion.** Sources
  behind active bot protection are *excluded from automation*, never bypassed
  (see [ADR-0004](../design/ADR-0004-scraping-within-tos.md)). Reason: legality,
  ToS, and long-term maintainability.
- ❌ **No fabricated data.** An unavailable source yields a `stale`/`approx`
  flag or a gap — never a synthesised value. Reason: a fabricated indicator is
  worse than a missing one.
- ❌ **No HFT-extreme engineering** (zero-copy deserialization, lock-free
  queues, AVX-512, GPU/NPU). Reason: the workload is KB–MB/day; see
  [ADR-0001](../design/ADR-0001-go-primary-backend.md).
- ❌ **No paid feeds** (e.g. VİOP open interest). Metrics that depended only on
  dead/paywalled sources were dropped before this charter.
- ❌ **No production hardening yet** (secrets policy, multi-tenant authz,
  cloud constraints). Dev/experimental only; production is a later concern.

## Audience

- **Primary**: the analyst/quant running the terminal locally.
- **Secondary**: future B2B/B2C consumers of the read API (designed for, not
  yet exposed).
- **Operators**: the same developer, running `docker compose up` on one host.

## Success criteria

1. `docker compose up` brings the full stack to healthy behind
   `https://neyialiyorlar.local` with a trusted local cert — zero host installs
   beyond Docker + mkcert.
2. All 4 sources ingest on cadence; a source outage degrades its metrics to
   `stale`/`approx` **without** crashing the pipeline or fabricating values.
3. All 10 metrics compute idempotently from stored raw payloads and expose a
   correct `fresh|stale|approx` flag end-to-end (DB → API → dashboard).
4. The dashboard renders the full metric matrix and refreshes hot tiles from
   Redis-cached reads.
5. The Phase-6 **IC gate** runs: every metric is rank-correlated against
   forward returns and stays labelled **exploratory** until it earns a GO.

## Non-goals & explicit trade-offs

- **Correctness & honesty over coverage.** 5 dependable metrics beat 20
  fragile ones. We ship the proxy 5 behind flags, not disguised as fact.
- **Steadiness over peak throughput.** Right-sized for one host; batch, not
  busy-poll. See [ADR-0001](../design/ADR-0001-go-primary-backend.md).
- **Clarity over cleverness.** Tiny source files, structured comments
  explaining the *performance rationale*, clean architecture boundaries.

## Constraints

Hard constraints (full text in [ROADMAP.md](../planning/ROADMAP.md) §Mandates):
full containerization · per-service `.env` (compose stays credential-free) ·
single centralized config blueprint · clean-architecture separation · TDD
first · absolute reliability (backoff, self-healing schemas, recovery states)
· right-sized efficiency · local TLS at `neyialiyorlar.local` · self-adaptive
scraping **within ToS** · dev-only (no secret policy yet).

## Definition of "exploratory" (non-negotiable)

Every metric is **exploratory**, not alpha, until it passes the Phase-6
information-coefficient gate. The label travels with the data through every
layer. Promotion to "trusted" is a deliberate, gated, per-metric decision —
never a default.
