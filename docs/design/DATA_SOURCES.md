<!--
docs/design/DATA_SOURCES.md — verified-live source integration map.
Status: accepted. This is the source-of-record for what the Harvester is
allowed to touch, the envelopes it expects, and the degradation behaviour.
Probe date of record: 2026-06-14. Re-probe and bump the date when sources move.
-->

# 🎨 Design: Verified Source Integration Map

- **Status**: accepted
- **Author**: Architecture
- **Date**: 2026-06-14 (probe-of-record)
- **Supersedes**: the original brief's source list (corrected against live probes)

## Problem

The Turkish market-data landscape is a mix of live free JSON APIs, HTML report
pages, PDF disclosures, and WAF-protected portals. A brittle scraper that
assumes one shape, or that tries to bypass bot protection, will either break
weekly or cross a ToS/legal line. We need a single, honest map of **what is
live, what is a proxy, and what is off-limits** — and the degradation behaviour
for each.

## Goals

- Enumerate every source the Harvester may touch, with its transport, envelope,
  cadence, and the metrics it feeds.
- Define the **hard boundary**: WAF/CAPTCHA-protected portals are excluded.
- Define **degradation, not fabrication**: each source's failure mode maps to a
  `stale`/`approx` flag or a gap, never a synthesised value.

## Non-goals

- Exact field-by-field schemas (those live as captured golden fixtures in the
  Harvester repo under `testdata/fixtures/<source>/`).
- Any evasion technique. Out of scope by mandate; see
  [ADR-0004](ADR-0004-scraping-within-tos.md).

## Source matrix (probed live 2026-06-14)

| # | Source | Transport | Envelope | Cadence | Auth | Status | Feeds metrics |
|---|---|---|---|---|---|---|---|
| S1 | **BIST** indices/equity/VİOP | HTTPS `GET` JSON (`bist-indexes.php?op=…`) | `{ status, data[] }` | intraday → daily | none | 🟢 live | 3, 10 |
| S2 | **KAP** disclosures (incl. fund portfolio) | HTTPS `POST` JSON (SPA backend) | `{ ...page, result[] }` | intraday (event) | none | 🟢 live | 1, 2, 7, 9 |
| S3 | **MKK / VAP** report pages | HTTPS `GET` HTML (+ embedded tables) | HTML DOM, Turkish labels | daily / T+10 | none | 🟢 live (HTML) | 4, 5, 6 |
| S4 | **CBRT EVDS** | HTTPS `GET` JSON, API key | `{ items[], totalCount }` | daily / weekly | **key** (dev) | 🟢 live | 6, 8, 10 |
| — | KAP fund **PDF** disclosure documents | HTTPS `GET` PDF | text-layer headers | as-published | none | 🟢 live (parse) | 1, 2 |
| ✗ | WAF-protected portals (rating feeds, some VAP detail) | — | — | — | bot-protected | ⛔ **excluded** | 7 (degraded) |

## Per-source integration detail

### S1 — BIST (`bist-indexes.php?op=…`) 🟢

- **Transport.** Plain `GET`, JSON response. Polite single-host client:
  shared keep-alive pool, ≤ 2 concurrent, identifying `User-Agent`, respect
  `Retry-After`.
- **Envelope.** Polymorphic guard validates `status == "ok"` and that `data`
  is a non-empty array. Field drift inside rows is tolerated: unknown keys are
  preserved into the raw payload, missing-but-required keys flag the row
  `approx` and quarantine, they do **not** crash the batch.
- **Cadence.** Index/price snapshots polled on a daily close batch; VİOP
  near-month value/volume polled intraday for metric 3. No tick stream.
- **Failure → flag.** HTTP 5xx / timeout → exponential backoff (see Reliability
  below); after breaker opens, metrics 3 & 10 recompute from last-good raw and
  flag `stale`.

### S2 — KAP disclosures (SPA / `POST` JSON) 🟢

- **Transport.** The KAP single-page app talks to a JSON backend via `POST`
  with a query body (date range, disclosure type, member filter). The Harvester
  replays that documented `POST`, it does **not** drive a headless browser.
- **Envelope.** Disclosure list returns paged `result[]`; each item references a
  disclosure id resolved to either inline JSON detail or a **PDF** document.
- **Fund portfolio disclosures.** The portfolio breakdown (holdings + weights)
  arrives as a periodic disclosure document → routed to the PDF parser, which
  anchors on **stable Turkish headers** (e.g. "Fon Portföy Değeri", holding
  rows) rather than absolute coordinates. Layout drift → quarantine + `approx`,
  never a crash.
- **Cadence.** Polled intraday on a short batch interval (a disclosure feed, not
  a tick stream). New disclosure ids since the last high-water mark are fetched.
- **Failure → flag.** Feed unreachable → metrics 1, 2, 9 recompute from stored
  disclosures and flag `stale`; a parse-quarantined document holds its metric at
  last-good with `approx`.

### S3 — MKK / VAP report pages (HTML) 🟢

- **Transport.** `GET` HTML report pages. **Heuristic selector fallback**: the
  parser anchors on **stable Turkish text labels** (e.g. fund-type, AUM, foreign
  ownership) and walks the DOM relative to them, so a class/id reshuffle does not
  break extraction. A label-not-found condition flags `approx` + quarantine.
- **Cadence.** Daily for AUM/float aggregates (metrics 4, 5); foreign-ownership
  **detail is T+10 lag** — explicitly *not* daily — feeding metric 6's slow leg.
- **Failure → flag.** Page shape unrecognised → fall back to the last good
  snapshot, flag `stale`; if the label anchors vanish entirely → `approx` +
  quarantine + alert, no fabricated rows.

### S4 — CBRT EVDS (keyed JSON) 🟢

- **Transport.** `GET` JSON with a dev API key (key lives in the service's
  `.env`, never in compose — see mandate 2; dev-only, no secret policy yet).
  Series codes requested per metric (TLREF, CPI, reserves, policy rate,
  non-resident flow).
- **Envelope.** `{ items[], totalCount }`; each item is `{date, <seriesCode>}`.
  Guard validates `totalCount >= 1` and that the requested series code is
  present; a missing series flags the dependent metric `stale`.
- **Cadence.** Daily series (rates) on the daily batch; **weekly** non-resident
  flow is the **signal-of-record** for metric 6 (do not upsample to daily).
- **Failure → flag.** EVDS 4xx (bad key/quota) → block that series, flag
  dependent metrics `stale`, surface the error; never invent a data point.

### ⛔ Excluded — WAF / CAPTCHA-protected portals

- Some rating feeds and certain VAP detail endpoints sit behind active bot
  protection. **They are excluded from the automated Harvester.** No
  TLS-fingerprint spoofing, no CAPTCHA solving, no proxy rotation. Metric 7
  (Mandate Expansion) is therefore **speculative/degraded** and ships `approx`.
- If such data is ever genuinely required, it is reachable **only** via an
  optional, documented, human-driven browser export — never automated evasion.
  See [ADR-0004](ADR-0004-scraping-within-tos.md).

## Degradation matrix (failure → flag, never fabricate)

| Condition | Harvester action | Metric flag |
|---|---|---|
| Source on cadence, envelope valid | store raw, compute | `fresh` |
| Source late but reachable | recompute from last-good raw | `stale` |
| Envelope/field drift, row recoverable | preserve unknowns, compute | `fresh` (logged) |
| Required field missing / parse drift | quarantine raw, hold metric | `approx` |
| Source unreachable (breaker open) | recompute from last-good raw | `stale` |
| Source is a proxy by nature (1–4, 7) | compute, mark proxy | `approx` (always) |
| Source permanently gone | gap (no row) + alert | gap, **never** fabricated |

## Reliability contract (applies to every source)

- **Exponential backoff with jitter** on retryable errors (HTTP 429/5xx,
  timeouts): `base · 2^n ± jitter`, capped, bounded attempts.
- **Circuit breaker** per source: open after N consecutive failures, half-open
  probe after cooldown, close on success.
- **Self-healing schema guard**: envelope validated before parse; unknown fields
  tolerated; missing required fields downgrade confidence, never panic.
- **Raw-then-derived**: the untouched payload is persisted *before* any parse so
  every metric is replayable and idempotent. See
  [ADR-0002](ADR-0002-raw-then-derived-ingestion.md).

## Open questions

- EVDS weekly flow vs MKK T+10 foreign detail: confirm the reconciliation window
  for metric 6 (EVDS is signal-of-record; MKK refines on lag).
- KAP fund-portfolio PDF templates: capture a fixture set per fund family to
  harden the header anchors.
