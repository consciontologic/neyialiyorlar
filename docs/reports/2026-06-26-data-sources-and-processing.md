# 📊 Data sources & processing — how we scrape, parse, and derive

- **Date**: 2026-06-26
- **Scope**: A brief, grounded tour of every external source the backend
  touches, the methods used to parse each, the raw→derived processing pipeline,
  and the current fund-coverage picture (including *why* some stocks show no
  funds).
- **Sources of record**: [`docs/design/DATA_SOURCES.md`](../design/DATA_SOURCES.md)
  (probe-of-record 2026-06-14), [`config/neyialiyorlar.yaml`](../../config/neyialiyorlar.yaml),
  and the service code under [`services/`](../../services/).

> Guiding principle across every source: **degrade, never fabricate.** A source
> failure maps to a `stale`/`approx` flag or a gap — never an invented value.
> See [ADR-0003](../design/ADR-0003-staleness-confidence-flags.md) and
> [ADR-0004](../design/ADR-0004-scraping-within-tos.md).

---

## 1. Data sources at a glance

All sources are public and unauthenticated except CBRT EVDS (dev API key).
WAF/CAPTCHA-protected portals are **excluded by mandate** — we do not evade bot
protection or drive headless browsers.

| Source | What it gives us | Transport | Cadence | Status |
|---|---|---|---|---|
| **BIST** (borsaistanbul.com) | Index / equity / VİOP prices & volume | HTTPS `GET` JSON (`bist-indexes.php?op=…`) | intraday → daily | 🟢 live |
| **KAP** (kap.org.tr) | Company & **fund-portfolio** disclosures | HTTPS `POST` JSON (SPA backend) | intraday (event) | 🟢 live |
| **KAP fund PDFs** | Fund holdings + weights (FPD documents) | HTTPS `GET` PDF | as-published | 🟢 live (parsed) |
| **MKK / VAP** (mkk.com.tr) | Fund AUM, foreign ownership, float | HTTPS `GET` HTML | daily / T+10 | 🟢 live (HTML) |
| **CBRT EVDS** (evds2.tcmb.gov.tr) | Rates, FX, macro series | HTTPS `GET` JSON + API key | daily / weekly | 🟢 live |
| WAF-protected portals (rating feeds, some VAP detail) | — | — | — | ⛔ **excluded** |

Source enablement and base URLs are declared in
[`config/neyialiyorlar.yaml`](../../config/neyialiyorlar.yaml) under `sources:`.

---

## 2. How we scrape — within Terms of Service

- **Polite, single-host clients.** Shared keep-alive pool, ≤ 2 concurrent
  requests per host, an identifying `User-Agent`, and `Retry-After` is honoured.
- **Documented endpoints only.** For KAP we replay the same JSON `POST` its
  single-page app already issues; we do **not** automate a browser.
- **No evasion.** WAF/CAPTCHA portals are out of scope. When a protected detail
  is unavailable, the dependent metric degrades to `approx` rather than being
  bypassed. ([ADR-0004](../design/ADR-0004-scraping-within-tos.md))
- **Reliability.** HTTP 5xx / timeouts back off exponentially behind a circuit
  breaker; once the breaker opens, affected metrics recompute from last-good raw
  and flag `stale`.

---

## 3. How we parse each source

### BIST — JSON envelope guard
A polymorphic guard validates `status == "ok"` and that `data` is a non-empty
array. Unknown keys are preserved into the raw payload; a missing required key
flags the row `approx` and quarantines it instead of crashing the batch.

### KAP disclosures — paged JSON
The disclosure list returns paged `result[]`; each item resolves to inline JSON
detail or a PDF document. New disclosure ids since the last high-water mark are
fetched intraday.

### MKK / VAP — label-anchored HTML
The HTML parser anchors on **stable Turkish text labels** (fund-type, AUM,
foreign ownership) and walks the DOM relative to them, so a CSS class/id
reshuffle does not break extraction. A label-not-found condition flags `approx`
and quarantines.

### CBRT EVDS — keyed JSON
`GET` JSON with the dev API key; `{ items[], totalCount }` envelope feeds the
rate / FX / macro metrics.

### KAP fund-portfolio PDFs — the fund-holdings pipeline
Implemented in [`services/fund-scraper/fund_scraper.py`](../../services/fund-scraper/fund_scraper.py):

1. **Fetch** the Fon Portföy Dağılım (FPD) disclosure PDF from `kap.org.tr`.
2. **Extract text** with `pypdf` (`PdfReader`) — no coordinate dependence.
3. **Find equity holdings by ISIN, layout-agnostically.** Turkish equity ISINs
   match `EQUITY_ISIN_RE = TRA[A-Z][A-Z0-9]{8}` (TRA + 9 = 12 chars). This is
   robust to templates that glue the weight to the ISIN, split it across
   columns, or wrap the check digit onto the next line.
4. **Capture the weight (FTD %) when present**, via
   `ISIN_FTD_RE = (\d+,\d+)(TRA…)` (e.g. `8,09TRAAKBNK91N6` → weight 8.09,
   ISIN `TRAAKBNK91N6`); otherwise weight is recorded as `NULL`.
5. **Map ISIN → BIST ticker** (the upper-case prefix after `TRA`), validated
   against the BIST universe so only real tickers are stored.
6. **AI fallback for non-ISIN templates.** Some managers list holdings by
   company *name* only. [`services/fund-scraper/ai_pdf_parser.py`](../../services/fund-scraper/ai_pdf_parser.py)
   asks a **local Ollama** model to read the raw text and return tickers +
   weights. It is **grounded** (every returned ticker is validated against the
   universe; the prompt forbids inventing tickers) and **deterministic**
   (temperature 0). If Ollama is unreachable or the reply is unparseable, it
   returns `{}` and the holding is simply skipped — no guesses.

Holdings land in `fund_holding(fund_code, stock_id, weight_pct, as_of_date,
source)` and the fund itself in `fund_ref(fund_code, fund_title, fund_manager,
fund_type, as_of_date)`.

---

## 4. How we process — raw, then derived

- **Raw first.** Every fetch is persisted as a raw payload before any
  computation, so derived metrics are always reproducible from stored inputs.
  ([ADR-0002](../design/ADR-0002-raw-then-derived-ingestion.md))
- **Derived metrics** are computed by the analytic service and written to
  `metric_value(metric_key, entity, ts, value, flag, tier)`. The catalogue and
  each metric's tier/colour live in
  [`config/neyialiyorlar.yaml`](../../config/neyialiyorlar.yaml).
- **Confidence is explicit.** Every value carries a `flag`:
  - `fresh` — real, current source data (e.g. `price_close` from BIST);
  - `stale` — last-good value held while a source is unreachable;
  - `approx` — a proxy / quarantined / degraded computation.

  Most behavioural metrics (velocity, herding, basis spread, …) are `proxy`
  colour and so normally carry `approx`; only directly-sourced values like the
  closing price are `fresh`. ([ADR-0003](../design/ADR-0003-staleness-confidence-flags.md))

---

## 5. Fund coverage — and why some stocks have no funds

Snapshot at the report date (a fund scan was **actively running**, so these
numbers rise over time):

| Metric | Value |
|---|---|
| Securities in the universe | 692 |
| Securities with ≥ 1 fund holding | 287 |
| Securities with **no** fund | 405 |
| …of which are BIST30/BIST100 constituents | **5** |
| Funds catalogued (`fund_ref`) | 741 |
| Funds whose holdings are scanned (`fund_holding`) | 374 |

A stock shows **no funds** for one of four honest reasons — not a UI bug:

1. **The scan is only ~half complete and still running.** 367 of 741 known funds
   have not had their holdings parsed yet. As the scraper works through them,
   coverage climbs (it moved from 238 → 287 securities during the session that
   produced this report). Many "fundless" stocks gain funds once the funds that
   hold them are scanned.
2. **Genuinely not held — small / illiquid caps.** Turkish equity and pension
   funds concentrate in liquid large-caps. **400 of the 405** fundless stocks
   sit *outside* BIST30/BIST100 — micro-caps, niche REITs, holding shells, and
   recently-listed names that no fund currently holds. That is a real signal,
   not a gap.
3. **Disclosure / template gaps.** A few funds publish portfolios whose PDFs
   expose no clean ISIN; the ISIN regex and AI fallback recover most, but where
   both fail we record nothing rather than guess (degrade, not fabricate).
4. **Only 5 index constituents are fundless** — these are the genuine
   "should be covered shortly" cases, almost certainly held by funds still in
   the unscanned backlog.

In short: the dominant causes are **scan-in-progress** (transient) and
**genuinely-unheld small caps** (real). Index membership and price data are now
populated for the major names.

---

## 6. References

- [`docs/design/DATA_SOURCES.md`](../design/DATA_SOURCES.md) — verified source map.
- [ADR-0002](../design/ADR-0002-raw-then-derived-ingestion.md) — raw-then-derived ingestion.
- [ADR-0003](../design/ADR-0003-staleness-confidence-flags.md) — staleness / confidence flags.
- [ADR-0004](../design/ADR-0004-scraping-within-tos.md) — scraping within ToS.
- [ADR-0005](../design/ADR-0005-pdf-library-selection.md) — PDF library selection.
- Code: [`services/fund-scraper/fund_scraper.py`](../../services/fund-scraper/fund_scraper.py),
  [`services/fund-scraper/ai_pdf_parser.py`](../../services/fund-scraper/ai_pdf_parser.py),
  [`services/harvester/`](../../services/harvester/), [`services/analytic/`](../../services/analytic/).
