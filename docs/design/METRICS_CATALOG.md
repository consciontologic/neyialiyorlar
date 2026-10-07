<!--
docs/design/METRICS_CATALOG.md — the Alpha Matrix, formalised.
Status: accepted. One block per metric: definition, inputs, kernel, cadence,
source color, build priority, confidence semantics, IC-gate target.
Every metric is EXPLORATORY until it passes the Phase-6 IC gate.
-->

# 🎨 Design: The Alpha Matrix — Metrics Catalog

- **Status**: accepted (definitions) · all metrics **exploratory** (not alpha)
- **Author**: Architecture
- **Date**: 2026-06-14
- **Related**: [DATA_SOURCES.md](DATA_SOURCES.md) · [ADR-0003](ADR-0003-staleness-confidence-flags.md)

## Reading the two axes

Two **independent** axes (do not conflate them):

- **Source color** — how direct the data is: 🟢 **CORE** (live free direct
  source) · 🟡 **PROXY** (proxy / lower-cadence; **always** ships `stale`/`approx`).
- **Build priority** — value/order: ⭐ **PRIORITY** (built first). A metric can
  be ⭐ **and** 🟡: a top build target whose source is still a proxy. Priority
  does **not** promote a proxy to a measurement.

**Tally:** 🟢 CORE ×5 (5, 6, 8, 9, 10) · 🟡 PROXY ×5 (1, 2, 3, 4, 7) ·
⭐ PRIORITY ×4 (1, 2, 3, 4). 10 buildable; ~5 dependable.

**Build order (value-first):** 1 → 2 → 3 → 4 (the ⭐ fund-flow IP thesis),
then the 🟢 backbone 5, 6, 8, 9, 10.

## Confidence semantics (every value carries one)

| Flag | Means | Set when |
|---|---|---|
| `fresh` | on-cadence, direct | all inputs 🟢 and within cadence |
| `stale` | data older than cadence allows | any input late / breaker open |
| `approx` | proxy / substitute input | any 🟡 input, or quarantined parse |

Propagation rule: a derived metric inherits the **worst** confidence of its
inputs (`approx` > `stale` > `fresh`). See [ADR-0003](ADR-0003-staleness-confidence-flags.md).

---

## ⭐🟡 1 — Velocity of Accumulation (ΔW/ΔT)

- **Thesis.** How fast a fund is building/shedding a position.
- **Source.** KAP fund portfolio disclosures (S2). **Cadence.** as-published.
- **Inputs.** Holding weight $W_{i,t}$ for security $i$ at disclosure time $t$.
- **Kernel.** Per holding, between consecutive disclosures $t-1, t$:

  $$V_{i} = \frac{W_{i,t} - W_{i,t-1}}{\Delta t}, \quad \Delta t = t - t_{-1}$$

  Aggregate to a fund- or security-level signal (sum of positive velocities =
  accumulation pressure).
- **Confidence.** Always `approx` (event-driven proxy for continuous flow);
  `stale` if the latest disclosure is overdue.
- **IC target.** Rank-correlate $V_i$ vs forward return of security $i$.

## ⭐🟡 2 — Institutional Herding Index (IHI)

- **Thesis.** Are funds crowding into the same names at once?
- **Source.** KAP fund disclosures (S2). **Cadence.** as-published.
- **Inputs.** Holdings sets per active fund, grouped **by category** (no
  performance ranking).
- **Kernel.** Cross-fund holdings-overlap synchronization — pairwise overlap
  (Jaccard/weight-cosine) averaged within a category, then z-scored over a
  rolling window to expose synchronization spikes:

  $$\text{IHI}_c = z\!\left( \frac{1}{|P_c|} \sum_{(a,b)\in P_c} \text{overlap}(H_a, H_b) \right)$$

- **Confidence.** Always `approx`; `stale` on overdue category disclosures.
- **IC target.** IHI spike vs forward return dispersion of the crowded names.

## ⭐🟡 3 — Spot-to-Derivatives Basis Spread

- **Thesis.** Futures richness/cheapness vs spot as a positioning tell.
- **Source.** BIST spot + VİOP near-month value/volume (S1). **Cadence.** daily.
- **Inputs.** Spot price $S$, near-month futures price $F$ (value/volume only).
- **Kernel.** **Price basis** (open-interest anomaly is *unavailable* — VİOP OI
  is paid — so this is explicitly a price-basis proxy):

  $$\text{basis} = \frac{F - S}{S}, \quad \text{signal} = z(\text{basis})$$

- **Confidence.** Always `approx` (no OI confirmation); `stale` on missing VİOP
  snapshot.
- **IC target.** Basis z-score vs forward spot return.

## ⭐🟡 4 — Thematic Float Demand Ratio

- **Thesis.** Fund appetite relative to tradable supply, by theme.
- **Source.** MKK/VAP (S3). **Cadence.** daily.
- **Inputs.** Fund-type AUM $A_k$, traded free float $FF$ (aggregate / by type).
- **Kernel.**

  $$\text{TFDR}_k = \frac{A_k}{FF}, \quad \text{signal} = z(\text{TFDR}_k)$$

- **Confidence.** `approx` (AUM↔float is an indirect demand proxy); `stale` on
  late VAP snapshot.
- **IC target.** TFDR vs forward return of the theme basket.

## 🟢 5 — MKK Property-to-Equity Asset Allocation Ratio

- **Thesis.** Capital rotation between real-estate funds and equity.
- **Source.** MKK/VAP (S3). **Cadence.** daily.
- **Inputs.** GYF (real-estate fund) AUM $A_{\text{GYF}}$, equity AUM $A_{\text{eq}}$.
- **Kernel.**

  $$\text{PER} = \frac{A_{\text{GYF}}}{A_{\text{eq}}}, \quad \text{signal} = z(\text{PER})$$

- **Confidence.** `fresh` when both AUM legs are on-cadence; `stale` otherwise.
- **IC target.** Δ(ratio) vs forward equity-vs-REIT relative return.

## 🟢 6 — Net Foreign Accumulation Velocity

- **Thesis.** Direction & speed of foreign money.
- **Sources.** **CBRT EVDS weekly non-resident flow = signal-of-record** (S4)
  + MKK/VAP foreign-ownership detail (S3, **T+10 lag**). **Cadence.** weekly / T+10
  (**not** daily — do not upsample).
- **Inputs.** Weekly net non-resident flow $f_w$; foreign-ownership level (lagged).
- **Kernel.**

  $$\text{NFAV} = \frac{f_w - f_{w-1}}{\Delta w}, \ \text{reconciled with lagged ownership}$$

- **Confidence.** `fresh` on weekly cadence; the T+10 MKK leg is `stale` by
  construction until it lands and refines.
- **IC target.** NFAV vs forward index return (weekly horizon).

## 🟡 7 — Mandate Expansion Threshold Index (speculative)

- **Thesis.** Funds that become *eligible* to buy a name after a rating change
  create forced/expanding demand.
- **Sources.** KAP disclosures (S2) + rating events. **Cadence.** as-published.
- **Reality.** No clean ratings feed; rating portals are often WAF-protected and
  **excluded** (see [DATA_SOURCES.md](DATA_SOURCES.md)). This metric is
  **speculative** and ships `approx`, sometimes a gap.
- **Kernel.** Count/▲ of funds crossing an eligibility threshold after a rating
  event, normalised by mandate universe.
- **Confidence.** Always `approx`; gap when the rating leg is unavailable (never
  fabricated).
- **IC target.** Eligibility-expansion events vs forward return of the upgraded name.

## 🟢 8 — Net Interest Margin Vulnerability Index (NIMVI)

- **Thesis.** Bank-sector sensitivity to CBRT policy moves.
- **Source.** CBRT EVDS (S4). **Cadence.** daily–weekly.
- **Inputs.** Policy/reference rates, reserve requirement, relevant liquidity series.
- **Kernel.** Composite z-score of rate/reserve deltas weighted to bank-margin
  exposure:

  $$\text{NIMVI} = \sum_j w_j \, z(\Delta r_j)$$

- **Confidence.** `fresh` on EVDS cadence; `stale` if a series is late.
- **IC target.** NIMVI vs forward return of the bank basket.

## 🟢 9 — KAP Real-time Notification Parsing

- **Thesis.** Disclosure flow itself is a signal substrate (the ingestion spine
  for 1, 2, 7).
- **Source.** KAP (S2). **Cadence.** intraday (a **disclosure feed**, not a tick
  stream).
- **Inputs.** Disclosure stream (type, member, timestamp, document ref).
- **Kernel.** Normalised disclosure-rate / category-burst detection; primarily an
  **ingestion + structuring** deliverable that the other KAP metrics consume.
- **Confidence.** `fresh` while the feed is current; `stale` on feed outage.
- **IC target.** Disclosure-burst intensity vs forward return/vol of named issuers.

## 🟢 10 — BIST Real-Yield Divergence Model

- **Thesis.** Equity demand vs the real risk-free alternative.
- **Sources.** CBRT EVDS (**TLREF + CPI** — the brief's `bisttlref.org` is
  fictional; **use EVDS**) (S4) + BIST (S1). **Cadence.** daily.
- **Inputs.** TLREF $r$, CPI/inflation $\pi$, equity inflow/return proxy from BIST.
- **Kernel.** Real yield $= r - \pi$; divergence $=$ equity signal minus real
  yield, z-scored:

  $$\text{RYD} = z\big(\text{equity\_signal} - (r - \pi)\big)$$

- **Confidence.** `fresh` when EVDS + BIST are on-cadence; `stale` otherwise.
- **IC target.** RYD vs forward index return.

---

## Phase-6 IC gate (promotion contract)

Every metric above is **exploratory** until it passes the gate:

1. Compute the indicator time series and the matched **forward returns** at the
   metric's horizon.
2. Compute the **Spearman rank IC** (and a rolling IC) between them.
3. **GO** (promote "computed" → "trusted") only if IC clears the pre-registered
   threshold with sign stability across the rolling window; otherwise the metric
   stays **exploratory**.
4. DEGRADED metrics (🟡 / late) stay flagged `stale`/`approx` throughout — a
   proxy is **never** relabelled as a measurement, and a missing value is a gap,
   **never** fabricated.

The IC gate runs in the **optional Python research sidecar** (pandas/numpy/
scipy), offline, never in the live hot path. See
[ROADMAP.md](../planning/ROADMAP.md) Phase 6.
