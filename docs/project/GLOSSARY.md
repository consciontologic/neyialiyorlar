<!--
docs/project/GLOSSARY.md — domain + system terms for neyialiyorlar.
Keep alphabetic, one line each. Source acronyms verified against the live
Turkish market data providers.
-->

# 📖 Glossary — `neyialiyorlar`

Domain (Turkish market microstructure) and system terms. One line each,
alphabetic.

## Market & domain terms

| Term | Meaning |
|---|---|
| Alpha Matrix | The curated set of 10 institutional-liquidity indicators this system builds. See [METRICS_CATALOG.md](../design/METRICS_CATALOG.md). |
| AUM | Assets Under Management — total market value a fund manages. |
| Basis (spot–futures) | Price gap between a futures contract and its spot underlying; here a **price** basis (VİOP open interest is paid, so excluded). |
| BIST | Borsa İstanbul — the Istanbul stock exchange; source of equity/index JSON. |
| CBRT | Central Bank of the Republic of Türkiye (TCMB); publishes EVDS. |
| EVDS | Elektronik Veri Dağıtım Sistemi — CBRT's Electronic Data Delivery System (keyed JSON API): rates, reserves, CPI, flows. |
| Forward return | Future price change of a security over a horizon; the target the IC gate correlates indicators against. |
| Free float | Shares actually available for public trading (excludes locked/insider holdings). |
| GYF | Gayrimenkul Yatırım Fonu — real-estate investment fund (property side of metric 5). |
| Herding | Funds moving into the same holdings in sync; quantified by the Institutional Herding Index. |
| IC | Information Coefficient — rank correlation (Spearman) between an indicator and forward returns; the Phase-6 promotion gate. |
| Institutional liquidity | Flow/positioning of large managed pools (funds, foreign custody) as opposed to retail. |
| KAP | Kamuyu Aydınlatma Platformu — Türkiye's Public Disclosure Platform; corporate & **fund portfolio** disclosures. |
| MKK | Merkezi Kayıt Kuruluşu — Central Securities Depository (operates VAP). |
| NIMVI | Net Interest Margin Vulnerability Index — bank-liquidity sensitivity to CBRT rate/reserve moves (metric 8). |
| Non-resident flow | Foreign (non-resident) net portfolio inflow/outflow; CBRT EVDS weekly series is the signal-of-record for metric 6. |
| TLREF | Turkish Lira Overnight Reference Rate — the local risk-free rate used in the Real-Yield Divergence model (metric 10). |
| VAP | Veri Analiz Platformu — MKK's data/analysis report pages (fund AUM, ownership detail). |
| VİOP | Vadeli İşlem ve Opsiyon Piyasası — BIST's derivatives market (futures/options); near-month value/volume feed metric 3. |
| Velocity of Accumulation | ΔW/ΔT — change in a fund's holdings between consecutive KAP disclosures over elapsed time (metric 1). |

## System & confidence terms

| Term | Meaning |
|---|---|
| 🟢 CORE | Source color: metric backed by a live, free, direct source. |
| 🟡 PROXY | Source color: metric backed by a proxy or lower-cadence source; **always** ships with a `stale`/`approx` flag. |
| ⭐ PRIORITY | Build-priority axis (value/order), independent of source color; the 4 fund-flow IP metrics built first. |
| `approx` | Confidence flag: value derived from a proxy/substitute input, not a direct measurement. |
| Cadence | The natural refresh rhythm of a source/metric: intraday, daily, weekly, or as-published (event-driven). |
| Circuit breaker | Reliability pattern: after N consecutive source failures, stop calling it for a cooldown to avoid hammering. |
| Confidence flag | `fresh \| stale \| approx` — travels with every metric value through DB → API → dashboard. |
| Degraded | A metric still served but flagged `stale`/`approx` because its source is late/unavailable; never fabricated. |
| Exploratory | Default status of every metric until it passes the IC gate; **not** validated alpha. |
| `fresh` | Confidence flag: value computed from on-cadence, direct source data. |
| Golden-file test | Test that asserts a parser/kernel reproduces a known-good output from a captured raw fixture. |
| Harvester | The Go Ingestion Engine (Service A) that polls sources and stores raw payloads. |
| Idempotent recompute | Re-running computation on the same raw inputs yields the same derived row (no duplication/drift). |
| Polymorphic envelope guard | Validates the `status`/`data[]` shape of a JSON source and tolerates field drift — flags, never crashes. |
| Quarantine | Holding a raw payload aside (not dropped) when a parser hits layout drift, for later replay. |
| Raw-then-derived | Persist the untouched source payload first; compute metrics from stored raw second. See [ADR-0002](../design/ADR-0002-raw-then-derived-ingestion.md). |
| `stale` | Confidence flag: value computed from data older than its cadence allows (source late/outage). |
| Staleness propagation | Carrying the worst input confidence forward into the derived metric's flag. |

## Framework terms (inherited)

| Term | Meaning |
|---|---|
| Agent | An AI coding assistant working under [AGENTS.md](../../AGENTS.md). |
| Run | One agent invocation spanning many tool calls; identified by `run_id`. |
| Scope | The named slice of work a row in `ai/tracking.csv` touches (a phase id, module slug). |
| Tracking row | One line in [`ai/tracking.csv`](../../ai/tracking.csv) recording an agent action. |
