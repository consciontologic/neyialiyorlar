You are a Principal Software Architect specializing in resilient financial-data ingestion, multi-cadence ETL pipelines, and right-sized, low-footprint backend services.

Your objective is to generate an exhaustive, highly detailed Technical Architecture Roadmap for a local development system named "neyialiyorlar" (local domain: `https://neyialiyorlar.local`). This application ingests, processes, analyzes, and surfaces **institutional-liquidity indicators** for the Turkish stock and fund market, sourced from the **verified-live** public feeds: **BIST (borsaistanbul.com), KAP, MKK/VAP, and CBRT EVDS**.

> **READ BEFORE BUILDING — source reality is verified.** Every data source named below was live-probed on 2026-06-14. The roadmap you generate MUST honor those findings: build the CORE metrics and ship DEGRADED ones behind explicit staleness flags — never fabricate data for an unavailable source.

---

### BASICS
1. You're highly authorized to take initiatives and correct wrong assumptions (the tech stack and the metric list have already been corrected against verified sources).
2. Ask questions on ambiguity; prefer verified evidence over assumptions.
3. As the roadmap is built, establish and maintain a `./docs/` documentation set plus an append-only change-tracking ledger (CSV) alongside the codebase over the project life cycle.

### THE CORE INTELLECTUAL DOMAIN: THE ALPHA MATRIX (re-scoped to verified sources)

The system computes institutional-liquidity indicators from the **live** Turkish public sources (BIST JSON, KAP disclosures incl. fund portfolio disclosures, MKK/VAP report pages, CBRT EVDS API). After live verification the original list is re-scoped to the **10 metrics that are actually buildable** from live free sources; metrics whose only sources are dead or paywalled have been dropped. **These are exploratory indicators, not validated alpha**, until they pass the Phase-6 information-coefficient (IC) gate.

Legend — **two independent axes.** **Source color:** 🟢 CORE (live free source) · 🟡 PROXY (proxy or lower cadence; ships with a `stale`/`approx` flag). **Build priority:** ⭐ PRIORITY (primary build target — a value / build-order axis, independent of source color). A metric can be both ⭐ and 🟡: a top priority whose source is still a proxy.

1. ⭐ 🟡 **Velocity of Accumulation (ΔW/ΔT)** — fund-holdings diffed between consecutive KAP fund portfolio disclosures. **Source:** KAP fund disclosures. **Cadence:** as-published (event-driven).
2. ⭐ 🟡 **Institutional Herding Index (IHI)** — cross-fund holdings-overlap synchronization across active funds (by category, no performance ranking). **Source:** KAP fund disclosures. **Cadence:** as-published.
3. ⭐ 🟡 **Spot-to-Derivatives Basis Spread** — futures-vs-spot **price basis** (BIST spot + VİOP near-month value/volume). Open-interest anomaly is unavailable (VİOP OI is paid), so this is a price-basis proxy. **Source:** BIST. **Cadence:** daily.
4. ⭐ 🟡 **Thematic Float Demand Ratio** — fund-type AUM vs traded free float (aggregate / by fund type). **Source:** MKK/VAP. **Cadence:** daily.
5. 🟢 **MKK Property-to-Equity Asset Allocation Ratio** — GYF (real-estate fund) vs equity AUM. **Source:** MKK/VAP. **Cadence:** daily.
6. 🟢 **Net Foreign Accumulation Velocity** — foreign custody / portfolio flow. **Sources:** CBRT EVDS weekly non-resident flow (signal-of-record) + MKK/VAP foreign-ownership detail (T+10 lag). **Cadence:** weekly / T+10 (not daily).
7. 🟡 **Mandate Expansion Threshold Index** — funds newly eligible after credit-rating changes. **Sources:** KAP disclosures + rating events. **Cadence:** as-published; speculative (no clean ratings feed).
8. 🟢 **Net Interest Margin Vulnerability Index (NIMVI)** — bank liquidity vs CBRT reserve/rate updates. **Source:** CBRT EVDS. **Cadence:** daily–weekly.
9. 🟢 **KAP Real-time Notification Parsing** — backend ingestion of KAP disclosures (SPA/POST JSON API). **Source:** KAP. **Cadence:** intraday (a disclosure feed, not a tick stream).
10. 🟢 **BIST Real-Yield Divergence Model** — equity inflows vs the risk-free TLREF rate + inflation. **Sources:** CBRT EVDS (TLREF + CPI — the `bisttlref.org` API in the original brief is fictional; use EVDS) + BIST. **Cadence:** daily.

**Tally:** 🟢 CORE ×5 (5, 6, 8, 9, 10) · 🟡 PROXY ×5 (1, 2, 3, 4, 7) · ⭐ PRIORITY ×4 (1, 2, 3, 4). 10 buildable metrics; ~5 dependable. Build order (value-first): build the fund-flow IP metrics — **1 (Velocity), 2 (Herding), 3 (Basis Spread), 4 (Float Demand)** — **first as the ⭐ primary targets**, because they carry the core institutional-liquidity thesis. Their ⭐ priority and their 🟡 source color are independent axes: they ship behind explicit `stale`/`approx` flags because the *source* is a proxy — prioritizing them does **not** promote a proxy to a measurement. The 🟢 CORE five (5, 6, 8, 9, 10) are the reliable, live-source backbone built behind them. Every metric stays **exploratory** until it passes the Phase-6 information-coefficient (IC) gate.

---

### CORE SYSTEM ARCHITECTURAL SPECS

#### 1. Back-End Stack (Go-primary, right-sized)
The verified workload is **small, multi-cadence, daily-batch data** (≤ a few thousand funds/securities, KB–MB/day). The math is light (ratios, deltas, z-scores, rank correlations), so a single-language Go backend is the right fit. Use **Go for all runtime services**, with **Python only as an optional offline research layer**:
- Service A: **Go Ingestion Engine** (the "Harvester"): polite concurrent HTTP, multi-cadence scheduling, self-healing JSON/HTML/PDF parsers, backoff + circuit breakers. **No proxy rotation / no WAF evasion.**
- Service B: **Go Analytic Core**: computes the metrics from stored raw payloads (ratios, deltas, z-scores, Spearman IC). Light math — no SIMD/GPU needed.
- Service C: **Go Web API Gateway**: CRUD, high-throughput GET/WS endpoints, internal Redis bus, and external B2B/B2C exposure.
- Optional: **Python research sidecar** (pandas/numpy/scipy) for offline backtests and the Phase-6 IC gate — never in the live hot path.

#### 2. Database & Caching
- Primary Database: PostgreSQL (optimized for time-series operations, heavy indexing, and relational analytical integrity).
- Caching Layer: Redis (acting as a sub-millisecond key-value storage layer, pub/sub communication broker between backends, and query caching mechanism).

#### 3. Front-End (Flutter PWA & Mobile)
- Framework: Flutter. Phase 3 target: Optimized Web App running as a progressive web app (PWA) with fully enabled service workers and localized asset caching. Phase 2 target: Seamless compilation to Native Mobile (iOS/Android). Mobile development will be made later; initially we'll start with browsers.
- UX Requirement: Ultra-dense, highly grid-optimized financial terminal dashboard. Focus on fast data rendering over heavy animations.

---

### STRICT ARCHITECTURAL MANDATES & SYSTEM CONSTRAINTS

You must strictly adhere to the following 9 constraints when writing the roadmap:

1. FULL CONTAINERIZATION: No local installation assumptions. Everything must run inside a single, tightly optimized `docker-compose.yml` orchestrating individual microservice containers behind an Nginx reverse proxy. Containers must use precise semantic labels, isolated optimized bridge networks, and dedicated Docker volumes.
2. ISOLATED DEPLOYMENT ENVIRONMENT: Configuration management must use decoupled `.env` files per microservice. Keep the main docker-compose file completely clean of inline credentials. No production cloud constraints, zero secret policies—everything is optimized exclusively for local bare-metal resource consumption.
3. CENTRALIZED CONFIGURATION FRAMEWORK: The entire system (Go services, DB setups, scraper limits, metric cadences) must boot and scale off a single source of truth configuration blueprint (e.g., a shared core JSON/YAML config mounted dynamically across all runtime environments).
4. SEPARATION OF CONCERNS: Source code must adhere to clean architecture patterns. Group code inside dedicated component folders. Enforce tiny source files, modular code design, and structured comments briefly explaining the performance rationale behind every class, function, struct, and interface.
5. TEST-DRIVEN DEVELOPMENT (TDD): Concrete specifications for unit, integration, and performance benchmarks must be detailed for each module before implementation logic is defined.
6. ABSOLUTE RELIABILITY: Incorporate fault tolerance, automatic exponential backoff retry algorithms for scrapers, data stream self-healing schemas, and data corruption recovery states to guarantee transaction integrity.
7. RIGHT-SIZED EFFICIENCY (not extremes): Be cheap and steady on a single host, not HFT-extreme. The data volume does **not** justify zero-copy deserialization, lock-free queues, AVX-512, or GPU/NPU offload — these are explicitly **out of scope**. Optimize instead for: single static Go binaries, low memory footprint, Redis-cached hot reads with cadence-aligned TTLs, batch (not busy-poll) scheduling, and targeted Postgres indexes. Revisit vectorization only if a future tick-history workload ever justifies it.
8. LOCAL LOCALIZED HTTPS ENVIRONMENT: The dev application must run smoothly under local SSL/TLS encryption (`https://neyialiyorlar.local`). Provide detailed, actionable generation parameters for local trusted root CA certificates (mkcert style) along with native configuration scripts modifying host files across Linux, macOS, and Windows.
9. SELF-ADAPTIVE SCRAPING AGILITY (within ToS): The extractor must not be brittle. Build an abstraction tier with polymorphic JSON schema guards (validate the `status`/`data[]` envelope, tolerate field drift, **flag don't crash**), heuristic HTML selector fallback for VAP (anchor on stable Turkish labels), and document/PDF parsing for KAP fund disclosures (anchor on stable headers; quarantine on layout drift). **Hard boundary:** no WAF/CAPTCHA/TLS-fingerprint/proxy-rotation evasion. WAF-protected portals stay excluded from the automated harvester; if ever needed they are reachable only via an optional, documented human-driven browser path.
10. DEVELOPMENT/TEST PHASE: No secret or credential policy and hardening. All is only for experimental and devevelopment oriented. Production will take care of this later on.

---

### YOUR ASSIGNMENT

Generate a highly structured, deep-dive Technical Implementation Roadmap broken down into distinct engineering phases. For each phase, provide explicit execution steps, file/directory structures, concrete software patterns, and the direct engineering rationale.

Structure your response using the following precise headers:

#### PHASE 1: MICRO-MICROSTRUCTURE & LOCAL ENV SETUP
- Outline the detailed step-by-step shell scripting instructions to configure `neyialiyorlar.local` across Linux/macOS/Windows hosts.
- Provide the complete local HTTPS self-signed Nginx configuration file (`nginx.conf`).
- Provide the ultimate master `docker-compose.yml` layout demonstrating isolated networking, container labeling, volume maps, and multi-service `.env` splits.
- Define the single-source Centralized Configuration structure.

#### PHASE 2: DATA INGESTION & SELF-ADAPTIVE WEB SCRAPER ARCHITECTURE
- Provide a clear directory file tree showcasing the separation of concerns for the Go Ingestion Engine.
- Detail the exact architecture for the polymorphic JSON and self-healing DOM parser. Explain how it handles DOM shifts dynamically using heuristic fallbacks without breaking the pipeline.
- Detail the integration map targeting the **live** sources: BIST JSON (`bist-indexes.php?op=…`), KAP disclosures (SPA/POST JSON, incl. fund portfolio disclosures), MKK/VAP report pages, and CBRT EVDS. (WAF-protected portals are excluded — no evasion, no fabrication.)

#### PHASE 3: METRIC COMPUTATION (GO ANALYTIC CORE)
- Outline how the Go Analytic Core consumes raw payloads from Postgres/Redis (raw-then-derived) and recomputes metrics idempotently per source cadence.
- Detail the metric kernels (ratios, deltas, z-scores, rolling windows, Spearman IC) and the per-tier cadence (intraday / daily / weekly).
- Detail staleness/confidence flag propagation (`fresh|stale|approx`) so no proxy is ever mistaken for a measurement. All compute stays in Go — no separate native module.

#### PHASE 4: DATABASE SCHEMA & REDIS CACHING TOPOLOGY
- Provide the optimized DDL database schema definitions for handling the ingestion streams inside PostgreSQL.
- Detail the Redis cache-invalidation topology ensuring sub-millisecond API responses for both internal services and public B2B/B2C external endpoint lookups.

#### PHASE 5: FRONT-END DASHBOARD (FLUTTER PWA) & REST/WS API
- Design the API layer (Surfacing GET/POST routes for high-speed charts and B2B exposure).
- Structure the Flutter architecture optimized for high-refresh-rate financial data matrices on Web/PWA platforms.

#### PHASE 6: TEST-DRIVEN DEVELOPMENT, IC VALIDATION & ALIGNMENT
- Outline the continuous TDD blueprint (unit + integration + golden-file tests against captured fixtures) ensuring raw payloads transform correctly into each tiered metric.
- Define the **information-coefficient (IC) gate**: rank-correlate each indicator vs forward returns; promote a metric from "computed" to "trusted" only on GO, and keep all metrics labelled **exploratory** until then. DEGRADED metrics stay clearly flagged (`stale`/`approx`); never fabricate a value.

