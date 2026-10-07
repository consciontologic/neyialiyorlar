"""
fund_scraper.py — KAP Altı Aylık Rapor FPD holdings scraper.

Discovers fund portfolio holdings by scanning KAP disclosure indices
for period=6AB (semi-annual reports) and parsing the FPD PDF attachment.

Strategy:
  1. Scan /tr/api/notification/attachment-detail/{idx} for disclosures
     with period='6AB' (Altı Aylık = semi-annual) in a known index range.
  2. For each found disclosure, identify the FPD PDF attachment
     ("Fon Portföy Dağılımı ve Net Varlık Değeri Tablosu").
  3. Download and parse the FPD PDF — it contains individual BIST equity
     holdings with ISIN codes and FTD% (% of total fund assets).
  4. Map Turkish equity ISINs (TRA prefix) to BIST tickers.
  5. Upsert fund_ref and fund_holding tables.

Notes:
  - KAP wraps every download in a Java-serialized byte[] envelope
    (magic 0xACED0005); the real %PDF payload is sliced out before parsing.
  - FPD templates vary by fund manager: some glue the FTD% to the ISIN
    (AEH idx=1476695), others print the ISIN in its own column with the
    weight elsewhere (NHM idx=1475622). Equity ISINs are therefore matched
    layout-agnostically; weight is recorded when available, else NULL.

Schema:
  fund_ref(fund_code PK, fund_title, fund_manager, fund_type, as_of_date)
  fund_holding(fund_code, stock_id TEXT, weight_pct, as_of_date, source)
    PK: (fund_code, stock_id, as_of_date, source)
"""
from __future__ import annotations

import json
import logging
import os
import re
import sys
import time
import uuid
from datetime import datetime, timezone
from io import BytesIO
from typing import Optional

import psycopg
import requests
from pypdf import PdfReader

import ai_pdf_parser

# ── Logging setup ─────────────────────────────────────────────────────────────

_EMOJI = {
    logging.DEBUG:    '🔍',
    logging.INFO:     '📋',
    logging.WARNING:  '⚠️ ',
    logging.ERROR:    '❌',
    logging.CRITICAL: '🔥',
}


class _EmojiFormatter(logging.Formatter):
    """Single-line formatter: HH:MM:SS EMOJI LEVEL  message  [key=value …]"""

    def format(self, record: logging.LogRecord) -> str:
        emoji = _EMOJI.get(record.levelno, '  ')
        ts = self.formatTime(record, datefmt='%H:%M:%S')
        level = record.levelname.ljust(8)
        msg = record.getMessage()
        # Include exception info if present
        if record.exc_info:
            msg += '\n' + self.formatException(record.exc_info)
        return f'{ts} {emoji} {level} {msg}'


def _setup_logging() -> None:
    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(_EmojiFormatter())
    root = logging.getLogger()
    root.handlers.clear()
    root.addHandler(handler)
    level = os.environ.get('LOG_LEVEL', 'INFO').upper()
    root.setLevel(getattr(logging, level, logging.INFO))


log = logging.getLogger(__name__)

# ── Configuration ──────────────────────────────────────────────────────────────

KAP_BASE = "https://www.kap.org.tr"
KAP_HEADERS = {
    "User-Agent": (
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
        "(KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
    ),
    "Referer": "https://www.kap.org.tr/tr/fon-portfoy-bildirimleri",
    "Accept": "application/json, text/plain, */*",
    "Accept-Language": "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7",
    "Accept-Encoding": "gzip, deflate, br",
    "Cache-Control": "no-cache",
    "Pragma": "no-cache",
    "sec-ch-ua": '"Chromium";v="136", "Google Chrome";v="136", "Not.A/Brand";v="99"',
    "sec-ch-ua-mobile": "?0",
    "sec-ch-ua-platform": '"Windows"',
    "Sec-Fetch-Dest": "empty",
    "Sec-Fetch-Mode": "cors",
    "Sec-Fetch-Site": "same-origin",
}

# Scan range: the mid-2025 fund portfolio-report band. Funds file portfolio
# distribution reports MONTHLY (period "AB"), plus quarterly ("3AB") and
# semi-annual ("6AB"); all three carry the same holdings table. The July-2025
# batch (filed Aug 2025) clusters densely across ~1472000–1478500 — a probe of
# this band found ~49% of indices to be fund (FON) disclosures, spanning every
# fund type, not just the ~175 pension funds near 1474440–1476695.
# SCAN_STEP MUST be 1: disclosure indices are not on a mod-N grid, so any
# step > 1 silently skips most funds (step=3 misses ~2/3). Widen
# SCAN_START/SCAN_END via env to sweep further; the scan is resumable
# (scraper_scan_cache + scraper_checkpoint), so partial runs are safe.
SCAN_START = int(os.environ.get("SCAN_START", "1472000"))
SCAN_END = int(os.environ.get("SCAN_END", "1479000"))
SCAN_STEP = int(os.environ.get("SCAN_STEP", "1"))

# Disclosure periods that carry a fund portfolio-distribution table. KAP files
# these monthly (AB), quarterly (3AB) and semi-annually (6AB); keeping only
# "6AB" — as the scraper originally did — discarded the far more numerous
# monthly reports and left almost every stock with no funds. "HZ" (İzahname /
# prospectus) and other FON periods are not portfolio reports and are skipped.
PORTFOLIO_PERIODS = frozenset({"AB", "3AB", "6AB"})

# KAP disclosureType for investment-fund filings. Company disclosures reuse the
# same portfolio period codes (3AB/6AB, monthly AB, …) but carry disclosureType
# "FR" (financial report), "DG" (general), … — only "FON" disclosures hold a
# fund portfolio-distribution table. Filtering on this keeps a company's
# financial statement from ever being mis-ingested as a bogus fund holding.
FON_DISCLOSURE_TYPE = "FON"

# Seconds between individual HTTP requests (keep below WAF threshold).
REQUEST_DELAY = float(os.environ.get("REQUEST_DELAY", "0.5"))
# Seconds to sleep after a burst of WAF/non-JSON responses.
WAF_BACKOFF = float(os.environ.get("WAF_BACKOFF", "30.0"))
# Consecutive non-JSON responses before triggering WAF backoff.
WAF_THRESHOLD = int(os.environ.get("WAF_THRESHOLD", "20"))

# Watchdog: abort if total runtime exceeds this many minutes.
MAX_RUNTIME_MINUTES = int(os.environ.get("MAX_RUNTIME_MINUTES", "90"))

# Loop mode: if set, the scraper re-runs every RESCAN_INTERVAL_H hours after completing.
# Set to 0 to run once and exit.
RESCAN_INTERVAL_H = int(os.environ.get("RESCAN_INTERVAL_H", "0"))

DB_DSN = os.environ.get(
    "DATABASE_URL",
    "host=postgres dbname=neyialiyorlar user=postgres",
)

# ── PDF Parsing ───────────────────────────────────────────────────────────────

# Some FPD templates glue the FTD% value directly before the ISIN
# (e.g. "8,09TRAAKBNK91N6" → ftd%=8.09, isin=TRAAKBNK91N6); this captures the
# weight when present.
ISIN_FTD_RE = re.compile(r"(\d+,\d+)(TRA[A-Z][A-Z0-9]{8})")

# Any Turkish equity ISIN (TRA + 9 alphanumerics = 12 chars). Matched against
# whitespace-collapsed text so a holding is captured regardless of the PDF
# template's column layout — including KAP's alternate template, which prints
# the ISIN on its own line with the check digit wrapped onto the next one.
EQUITY_ISIN_RE = re.compile(r"TRA[A-Z][A-Z0-9]{8}")

# BIST ticker is the uppercase alpha prefix after "TRA"
TICKER_RE = re.compile(r"^TRA([A-Z]+)")

# Plausibility bound for parsed FTD% weights. A real portfolio is a percentage
# distribution: each holding ≤ 100% and the captured equity slice sums to ≤ ~100
# (WEIGHT_SUM_MAX leaves margin for rounding). Some FPD templates glue a market
# value or share count (not the weight) before the ISIN, which sums to thousands;
# when the numbers fail this bound they are discarded (the fund→stock link is
# kept, the weight stored as NULL) rather than persisted as an impossible
# >100% allocation.
WEIGHT_SUM_MAX = 110.0


def _extract_pdf_payload(raw: bytes) -> bytes:
    """Return the real PDF bytes from a KAP download.

    KAP's /tr/api/file/download endpoint wraps the file in a Java-serialized
    byte[] envelope (magic 0xACED0005), so the response does not start with
    "%PDF". Slice out the embedded %PDF…%%EOF payload. If the bytes are already
    a bare PDF, this is a no-op.
    """
    start = raw.find(b"%PDF")
    if start < 0:
        return raw
    end = raw.rfind(b"%%EOF")
    return raw[start : end + 5] if end >= 0 else raw[start:]


def _holdings_from_text(full_text: str) -> dict[str, Optional[float]]:
    """Extract {isin: weight_pct} from FPD PDF text, template-agnostically.

    Captures every BIST equity ISIN regardless of the PDF's column layout, and
    attaches the FTD% weight when the template glues it to the ISIN. Holdings
    whose weight cannot be paired keep a None weight (the fund→stock link is
    still recorded). Weights are summed when a fund holds a stock in multiple
    lots; zero/negative glued weights are treated as unknown (None). When the
    glued numbers fail the percentage-distribution sanity check (sum exceeds
    WEIGHT_SUM_MAX, or any single value > 100%), they are a market-value / share
    column rather than weights and are all dropped to None — the links survive,
    but an impossible >100% allocation is never stored.
    """
    # Weights from the glued template: {isin: summed FTD%}.
    weighted: dict[str, float] = {}
    for ftd_str, isin in ISIN_FTD_RE.findall(full_text):
        try:
            ftd = float(ftd_str.replace(",", "."))
        except ValueError:
            continue
        if ftd > 0:
            weighted[isin] = weighted.get(isin, 0.0) + ftd

    # Only trust the glued numbers when they read as a percentage distribution.
    # A value/share column sums to thousands or carries a single >100% entry;
    # in that case drop every weight (the ISINs below are still recorded with a
    # None weight) so no impossible allocation is persisted.
    if weighted and (
        sum(weighted.values()) > WEIGHT_SUM_MAX
        or any(w > 100.0 for w in weighted.values())
    ):
        weighted = {}

    # Every equity ISIN, layout-agnostic. Collapse whitespace first so an ISIN
    # whose check digit wrapped onto the next line is still matched.
    compact = re.sub(r"\s+", "", full_text)
    holdings: dict[str, Optional[float]] = {}
    for isin in EQUITY_ISIN_RE.findall(compact):
        holdings.setdefault(isin, weighted.get(isin))
    return holdings


# A standalone uppercase token that may be a BIST ticker. The look-around bounds
# stop it matching inside an ISIN (e.g. the "AKBNK" in "TRAAKBNK91N6") or inside
# a longer alphanumeric run, so only free-standing codes are considered.
TICKER_TOKEN_RE = re.compile(r"(?<![A-Z0-9])[A-Z]{3,6}(?![A-Z0-9])")


def _tickers_from_text(full_text: str, known_tickers: set[str]) -> set[str]:
    """Return the known BIST tickers that appear as standalone tokens in text.

    Many fund families (ATA, AKTİF, …) print their holdings by ticker — or with
    the newer TRE-prefixed ISIN format the TRA regex deliberately ignores — so
    the ISIN pass alone records nothing for them. Matching free-standing tokens
    against the known BIST universe recovers the fund→stock link regardless of
    template. Membership in ``known_tickers`` is the gate, so a Turkish header
    word or company name never registers as a holding. Weights are intentionally
    not paired here: these templates scatter the weight away from the ticker, and
    a recorded link without a weight is the correct degradation (ADR-0004).
    """
    if not known_tickers:
        return set()
    return {t for t in TICKER_TOKEN_RE.findall(full_text) if t in known_tickers}


def _pdf_to_text(pdf_bytes: bytes) -> str:
    """Extract the text layer from a KAP-wrapped FPD PDF, or '' on failure."""
    try:
        reader = PdfReader(BytesIO(_extract_pdf_payload(pdf_bytes)))
        return "\n".join(page.extract_text() or "" for page in reader.pages)
    except Exception as exc:
        log.warning("PDF parse error: %s", exc)
        return ""


def parse_fpd_pdf(pdf_bytes: bytes) -> dict[str, Optional[float]]:
    """
    Parse a KAP FPD (Fon Portföy Dağılımı) PDF.

    Returns {isin: weight_pct} covering every BIST equity ISIN in the report;
    weight_pct is None when the template does not glue the FTD% to the ISIN.
    """
    full_text = _pdf_to_text(pdf_bytes)
    if not full_text:
        return {}
    return _holdings_from_text(full_text)


def isin_to_ticker(isin: str) -> Optional[str]:
    """Extract the BIST ticker from a Turkish equity ISIN (TRA… prefix)."""
    m = TICKER_RE.match(isin)
    return m.group(1) if m else None


# ── KAP HTTP helpers ──────────────────────────────────────────────────────────


def _is_json_response(resp: requests.Response) -> bool:
    ct = resp.headers.get("content-type", "")
    if "json" in ct:
        return True
    text = resp.text.lstrip()
    return text.startswith("[") or text.startswith("{")


# Sentinel returned by kap_json when the request was rate-limited (429/503/666)
# even after retries — lets the scan loop distinguish "empty index" from "blocked".
_WAF_BLOCKED = object()


def kap_json(session: requests.Session, path: str, retries: int = 2):
    """GET a KAP API path and return the parsed JSON, or None / _WAF_BLOCKED.

    Return values:
      - parsed JSON (list/dict) — success.
      - None                   — index doesn't exist (404 / HTML page).
      - _WAF_BLOCKED           — WAF rate-limited on every attempt.
    """
    url = f"{KAP_BASE}{path}"
    was_waf_blocked = False
    for attempt in range(retries):
        try:
            resp = session.get(url, timeout=10)
            if resp.status_code == 200 and _is_json_response(resp):
                return resp.json()
            # Actual WAF/rate-limit block: back off and retry.
            if resp.status_code in (429, 503, 666):
                was_waf_blocked = True
                if attempt < retries - 1:
                    time.sleep(WAF_BACKOFF)
                continue
            # 404 or non-JSON 200 (Next.js HTML for unknown index): no retry.
            return None
        except requests.exceptions.Timeout:
            if attempt < retries - 1:
                time.sleep(1.0)
        except Exception as exc:
            log.debug("kap_json error %s: %s", path, exc)
            if attempt < retries - 1:
                time.sleep(1.0)
    return _WAF_BLOCKED if was_waf_blocked else None


def kap_download(session: requests.Session, obj_id: str) -> Optional[bytes]:
    """Download a KAP file (PDF) by its object ID, with WAF retry."""
    url = f"{KAP_BASE}/tr/api/file/download/{obj_id}"
    for attempt in range(2):
        try:
            resp = session.get(url, timeout=30)
            if resp.status_code == 200:
                return resp.content
            if resp.status_code in (429, 503, 666):
                if attempt == 0:
                    log.warning("⏸️  PDF WAF engeli %s — %.0fs bekleniyor", obj_id, WAF_BACKOFF)
                    time.sleep(WAF_BACKOFF)
                continue
        except Exception as exc:
            log.warning("PDF download failed for %s: %s", obj_id, exc)
            if attempt == 0:
                time.sleep(1.0)
    return None


# ── Disclosure scan ────────────────────────────────────────────────────────────


def find_fpd_obj_id(attachments: list[dict]) -> Optional[str]:
    """
    Choose the portfolio-distribution PDF from a disclosure's attachment list.

    Multi-PDF semi-annual (6AB) filings ship the FPD as a named or 2nd PDF;
    monthly (AB) filings usually ship a single PDF named after the fund and
    month (e.g. "CPU_2025.07.pdf", "TCB TEMMUZ 2025 DAĞILIM.pdf", "YDI.pdf")
    with no portfolio keyword — so a lone PDF is taken as the report.
    """
    pdfs = [a for a in attachments if a.get("fileExtension", "").lower() == "pdf"]
    if not pdfs:
        return None
    # Prefer a PDF whose name signals a portfolio / distribution table.
    for att in pdfs:
        name = att.get("fileName", "").upper()
        if any(kw in name for kw in ("FPD", "NET VAR", "PORTF", "DAĞILIM", "DAGILIM")):
            return att["objId"]
    # Monthly reports almost always carry exactly one PDF — use it.
    if len(pdfs) == 1:
        return pdfs[0]["objId"]
    # Multi-PDF fallback (typically: 1=activity, 2=FPD, 3=PSR).
    return pdfs[1]["objId"]


def _disclosure_record(idx: int, db: dict, atts: list[dict]) -> Optional[dict]:
    """
    Build a scan record from a parsed ``disclosureBasic`` dict, or None to skip.

    Only fund (``disclosureType == "FON"``) filings carry a portfolio table.
    Company disclosures — financial reports ("FR"), general ("DG"), … — reuse
    the same portfolio period codes (3AB/6AB), so without this guard a company's
    financial statement would be mis-ingested as a fund holding. Returns
    ``None`` for any non-FON disclosure.

    KAP returns JSON ``null`` for several fields on some monthly filings, and
    ``dict.get(key, default)`` yields ``None`` (not the default) when the key is
    present-but-null — which then violates the NOT NULL scan-cache columns. So
    every string field is coerced with ``or ""``. Returns ``None`` when the fund
    (stock) code is missing, since the report can't be attributed to a fund.
    """
    if (db.get("disclosureType") or "") != FON_DISCLOSURE_TYPE:
        return None
    stock_code = db.get("stockCode") or ""
    if not stock_code:
        return None
    return {
        "index": idx,
        "stock_code": stock_code,
        "fund_type": db.get("fundType") or "",
        "period": db.get("period") or "",
        "company_title": db.get("companyTitle") or "",
        "mkk_member_oid": db.get("mkkMemberOid") or "",
        "publish_date": (db.get("publishDate") or "")[:10],
        "year": db.get("year") or 2025,
        "attachments": atts,
    }


def scan_for_fund_reports(
    session: requests.Session,
    start: int,
    end: int,
    step: int = 2,
) -> list[dict]:
    """
    Scan disclosure indices [start, end) step=step for fund portfolio reports.

    Keeps every disclosure whose period is a portfolio report (monthly "AB",
    quarterly "3AB", or semi-annual "6AB" — see PORTFOLIO_PERIODS); these all
    carry a holdings table. Returns a list of dicts for each found disclosure:
      index, stock_code, fund_type, period, company_title, mkk_member_oid,
      publish_date, year, attachments
    """
    found: list[dict] = []
    consec_waf = 0      # consecutive WAF-blocked requests (429)
    total = (end - start + step - 1) // step
    log.info("🔍 %d indis tarandı %d–%d adım=%d …", total, start, end, step)

    for i, idx in enumerate(range(start, end, step)):
        data = kap_json(session, f"/tr/api/notification/attachment-detail/{idx}")

        if data is _WAF_BLOCKED:
            # Genuine rate-limit block — all retries exhausted with 429.
            consec_waf += 1
            if consec_waf >= WAF_THRESHOLD:
                log.info(
                    "⏸️  WAF sürekli engel (%d istek) idx=%d — %.0fs bekleniyor",
                    consec_waf, idx, WAF_BACKOFF,
                )
                time.sleep(WAF_BACKOFF)
                consec_waf = 0
            time.sleep(REQUEST_DELAY)
            continue

        if data is None:
            # Empty index (404/HTML) — normal, no backoff needed.
            consec_waf = 0
            time.sleep(REQUEST_DELAY)
            continue

        consec_waf = 0

        if not (isinstance(data, list) and data):
            time.sleep(REQUEST_DELAY)
            continue

        d = data[0]
        db = d.get("disclosure", {}).get("disclosureBasic", {})
        period = db.get("period", "")
        if period not in PORTFOLIO_PERIODS:
            time.sleep(REQUEST_DELAY)
            continue

        atts = d.get("attachments", [])
        record = _disclosure_record(idx, db, atts)
        if record is None:
            # Not a fund (FON) disclosure, or missing fund code — skip.
            time.sleep(REQUEST_DELAY)
            continue

        found.append(record)
        log.info(
            "  ✅ %s (%s/%s) idx=%d  ek=%d",
            record["stock_code"], record["fund_type"], record["period"],
            idx, len(atts),
        )

        time.sleep(REQUEST_DELAY)

        if (i + 1) % 200 == 0:
            log.info("  📊 İlerleme: %d/%d kontrol edildi, %d bulundu", i + 1, total, len(found))

    log.info("🏁 Tarama tamamlandı: aralıkta %d bildiri bulundu", len(found))
    return found


# ── Database helpers ──────────────────────────────────────────────────────────


def db_connect() -> psycopg.Connection:
    return psycopg.connect(DB_DSN)


def load_known_tickers(conn: psycopg.Connection) -> set[str]:
    """Return the set of active BIST security tickers from entity_ref.

    Used to ground the AI parser: a holding whose ticker is not in this set is
    treated as a hallucination and dropped, never stored.
    """
    with conn.cursor() as cur:
        cur.execute(
            "SELECT entity_id FROM entity_ref "
            "WHERE entity_type = 'security' AND valid_to IS NULL"
        )
        return {row[0] for row in cur.fetchall()}


def upsert_fund_ref(
    conn: psycopg.Connection,
    code: str,
    title: str,
    fund_type: str,
    as_of: str,
) -> None:
    # Extract fund manager: the A.Ş. entity before the fund name
    manager = ""
    m = re.match(r"^(.+?A\.Ş\.)", title, re.IGNORECASE)
    if m:
        manager = m.group(1).strip()

    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO fund_ref (fund_code, fund_title, fund_manager, fund_type, as_of_date)
            VALUES (%s, %s, %s, %s, %s)
            ON CONFLICT (fund_code) DO UPDATE SET
                fund_title   = EXCLUDED.fund_title,
                fund_manager = CASE
                    WHEN fund_ref.fund_manager = '' THEN EXCLUDED.fund_manager
                    ELSE fund_ref.fund_manager
                END,
                as_of_date   = EXCLUDED.as_of_date,
                updated_at   = NOW()
            """,
            (code, title, manager, fund_type, as_of),
        )


def upsert_holding(
    conn: psycopg.Connection,
    fund_code: str,
    ticker: str,
    weight_pct: Optional[float],
    as_of: str,
) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO fund_holding
                (fund_code, stock_id, weight_pct, as_of_date, source, scraped_at)
            VALUES (%s, %s, %s, %s, 'kap', NOW())
            ON CONFLICT (fund_code, stock_id, as_of_date, source) DO UPDATE SET
                weight_pct = EXCLUDED.weight_pct,
                scraped_at = NOW()
            """,
            (fund_code, ticker, weight_pct, as_of),
        )


# ── Scraper progress / checkpoint helpers ─────────────────────────────────────


def progress_start(conn: psycopg.Connection, run_id: str, scan_start: int, scan_end: int) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO scraper_progress (run_id, started_at, status, scan_start, scan_end)
            VALUES (%s, NOW(), 'running', %s, %s)
            ON CONFLICT (run_id) DO NOTHING
            """,
            (run_id, scan_start, scan_end),
        )
    conn.commit()


def progress_update(
    conn: psycopg.Connection,
    run_id: str,
    *,
    indices_scanned: int = 0,
    disclosures_found: int = 0,
    funds_total: int = 0,
    funds_done: int = 0,
    holdings_saved: int = 0,
    errors: int = 0,
    last_fund: str = "",
    status: str = "running",
    finished: bool = False,
) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE scraper_progress SET
                indices_scanned  = %s,
                disclosures_found = %s,
                funds_total      = %s,
                funds_done       = %s,
                holdings_saved   = %s,
                errors           = %s,
                last_fund        = NULLIF(%s, ''),
                status           = %s,
                finished_at      = CASE WHEN %s THEN NOW() ELSE NULL END,
                updated_at       = NOW()
            WHERE run_id = %s
            """,
            (
                indices_scanned, disclosures_found, funds_total, funds_done,
                holdings_saved, errors, last_fund, status, finished, run_id,
            ),
        )
    conn.commit()


def load_checkpoint(conn: psycopg.Connection) -> set[tuple[str, int]]:
    """Return set of (fund_code, kap_idx) already successfully processed."""
    with conn.cursor() as cur:
        cur.execute("SELECT fund_code, kap_idx FROM scraper_checkpoint")
        return {(row[0], row[1]) for row in cur.fetchall()}


def save_checkpoint(conn: psycopg.Connection, fund_code: str, kap_idx: int, as_of: str) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO scraper_checkpoint (fund_code, kap_idx, as_of_date)
            VALUES (%s, %s, %s)
            ON CONFLICT (fund_code, kap_idx) DO UPDATE SET scraped_at = NOW()
            """,
            (fund_code, kap_idx, as_of),
        )
    conn.commit()


# ── DB scan cache (avoids re-scanning KAP on every run) ──────────────────────


def load_scan_cache(conn: psycopg.Connection, start: int, end: int) -> list[dict]:
    """Return cached disclosures in [start, end) from DB, or empty list."""
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT idx, stock_code, fund_type, company_title,
                   mkk_member_oid, publish_date, year, attachments
            FROM scraper_scan_cache
            WHERE idx >= %s AND idx < %s
            ORDER BY idx
            """,
            (start, end),
        )
        rows = cur.fetchall()

    return [
        {
            "index": r[0],
            "stock_code": r[1],
            "fund_type": r[2],
            "company_title": r[3],
            "mkk_member_oid": r[4] or "",
            "publish_date": r[5] or "",
            "year": r[6],
            "attachments": r[7] if isinstance(r[7], list) else [],
        }
        for r in rows
    ]


def is_range_scanned(conn: psycopg.Connection, start: int, end: int, step: int) -> bool:
    """True if this exact (start, end, step) combo was previously fully scanned."""
    with conn.cursor() as cur:
        cur.execute(
            "SELECT 1 FROM scraper_scan_range WHERE scan_start=%s AND scan_end=%s AND scan_step<=%s LIMIT 1",
            (start, end, step),
        )
        return cur.fetchone() is not None


def save_scan_results(conn: psycopg.Connection, disclosures: list[dict], start: int, end: int, step: int) -> None:
    """Persist discovered disclosures and mark the range as scanned."""
    import json as _json
    with conn.cursor() as cur:
        for d in disclosures:
            cur.execute(
                """
                INSERT INTO scraper_scan_cache
                    (idx, stock_code, fund_type, company_title, mkk_member_oid, publish_date, year, attachments)
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
                ON CONFLICT (idx) DO NOTHING
                """,
                (
                    d["index"], d["stock_code"], d["fund_type"], d["company_title"],
                    d.get("mkk_member_oid", ""), d.get("publish_date", ""),
                    d.get("year", 2025), _json.dumps(d.get("attachments", [])),
                ),
            )
        cur.execute(
            """
            INSERT INTO scraper_scan_range (scan_start, scan_end, scan_step)
            VALUES (%s, %s, %s)
            ON CONFLICT DO NOTHING
            """,
            (start, end, step),
        )
    conn.commit()
    log.info("💾 %d bildiri DB önbelleğine kaydedildi (aralık %d-%d)", len(disclosures), start, end)


# ── Main ──────────────────────────────────────────────────────────────────────


def main() -> None:
    _setup_logging()
    run_id = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "-" + uuid.uuid4().hex[:6]
    start_time = time.time()
    max_runtime_s = MAX_RUNTIME_MINUTES * 60
    log.info("🚀 KAP fon portföy tarayıcısı başlatılıyor")
    log.info("📍 Tarama aralığı: %d – %d  adım=%d  gecikme=%.1fs",
             SCAN_START, SCAN_END, SCAN_STEP, REQUEST_DELAY)

    session = requests.Session()
    session.headers.update(KAP_HEADERS)

    # ── Pre-flight: confirm KAP API is reachable ───────────────────────────────
    PREFLIGHT_IDX = 1476695  # Known valid 6AB disclosure (AEH fund)
    log.info("🌐 Ön kontrol: GET attachment-detail/%d", PREFLIGHT_IDX)
    preflight = kap_json(session, f"/tr/api/notification/attachment-detail/{PREFLIGHT_IDX}")
    if preflight is _WAF_BLOCKED:
        log.error(
            "🚫 KAP API bu IP'yi engelliyor (429). "
            "En az 30 dakika SIFIR istek gönderin, sonra tekrar deneyin."
        )
        sys.exit(1)
    if preflight is None:
        log.error("❓ Bilinen geçerli indeks için veri gelmedi — beklenmedik durum, iptal.")
        sys.exit(1)
    log.info("✅ Ön kontrol tamam — API erişilebilir")
    time.sleep(REQUEST_DELAY)

    # ── Step 1: Connect to database ────────────────────────────────────────────
    conn = db_connect()
    log.info("🗄️  Veritabanı bağlantısı kuruldu")

    # Register this run in DB and load skip-checkpoint
    progress_start(conn, run_id, SCAN_START, SCAN_END)
    checkpoint = load_checkpoint(conn)
    log.info("📌 %d fon zaten işlenmiş — atlanacak", len(checkpoint))

    # Known BIST universe — used to ground every parsed ticker (drop foreign
    # equities and OCR junk that are not tradeable BIST securities) and, when
    # enabled, to constrain the AI fallback parser. Loaded unconditionally so
    # grounding works even with the AI parser disabled.
    known_tickers: set[str] = load_known_tickers(conn)
    log.info("📋 %d bilinen BIST hissesi yüklendi", len(known_tickers))

    # ── AI PDF fallback parser (optional, local Ollama) ───────────────────────
    # Activates only when the deterministic regex parser is helpless on a
    # document that looks like an equity fund. Self-disables when no local
    # Ollama daemon is reachable, so the scraper runs regex-only out of the box.
    ai_cfg: Optional[ai_pdf_parser.OllamaConfig] = ai_pdf_parser.config_from_env()
    ai_session: Optional[requests.Session] = None
    if ai_cfg.enabled:
        ai_session = requests.Session()
        if ai_pdf_parser.is_available(ai_cfg, session=ai_session):
            log.info(
                "🤖 AI ayrıştırıcı etkin — Ollama %s model=%s (%d bilinen hisse)",
                ai_cfg.host, ai_cfg.model, len(known_tickers),
            )
        else:
            log.info("🤖 AI ayrıştırıcı pasif — Ollama erişilemiyor (%s)", ai_cfg.host)
            ai_cfg = None
    else:
        ai_cfg = None

    # ── Step 2: Scan or load DB-cached scan results ────────────────────────────
    if is_range_scanned(conn, SCAN_START, SCAN_END, SCAN_STEP):
        disclosures: list[dict] = load_scan_cache(conn, SCAN_START, SCAN_END)
        log.info("✅ %d bildiri DB önbelleğinden yüklendi — tarama atlandı", len(disclosures))
    else:
        disclosures = scan_for_fund_reports(session, SCAN_START, SCAN_END, SCAN_STEP)
        save_scan_results(conn, disclosures, SCAN_START, SCAN_END, SCAN_STEP)

    if not disclosures:
        log.error("🚫 Fon bildirisi bulunamadı — iptal")
        conn.close()
        return

    # A fund files a portfolio report every month, so one fund code can appear
    # many times across the scanned band. Keep only its most recent filing
    # (highest disclosure index) → each fund is processed once with its latest
    # holdings; the API already surfaces the newest as_of per fund.
    latest_by_code: dict[str, dict] = {}
    for disc in disclosures:
        prev = latest_by_code.get(disc["stock_code"])
        if prev is None or disc["index"] > prev["index"]:
            latest_by_code[disc["stock_code"]] = disc
    if len(latest_by_code) < len(disclosures):
        log.info(
            "🧹 %d bildiri → %d benzersiz fon (en güncel rapor tutuldu)",
            len(disclosures), len(latest_by_code),
        )
    disclosures = sorted(latest_by_code.values(), key=lambda d: d["index"])

    log.info("📊 %d fon işlenecek", len(disclosures))

    # Update scan stats in DB now that scan is done
    progress_update(conn, run_id,
                    indices_scanned=(SCAN_END - SCAN_START) // SCAN_STEP,
                    disclosures_found=len(disclosures),
                    funds_total=len(disclosures))

    # ── Step 3: For each fund, download FPD PDF and parse holdings ─────────────
    processed = total_holdings = errors = 0
    funds_done = 0

    for disc in disclosures:
        # ⛔ Watchdog: abort if we've been running too long
        if time.time() - start_time > max_runtime_s:
            log.warning("⏱️  İzin verilen çalışma süresi aşıldı (%d dk)", MAX_RUNTIME_MINUTES)
            progress_update(conn, run_id, indices_scanned=(SCAN_END - SCAN_START) // SCAN_STEP,
                            disclosures_found=len(disclosures), funds_total=len(disclosures),
                            funds_done=funds_done, holdings_saved=total_holdings,
                            errors=errors, status="timeout", finished=True)
            conn.close()
            sys.exit(1)  # non-zero → Docker restart policy kicks in

        code = disc["stock_code"]
        fund_type = disc["fund_type"]
        year = disc.get("year", 2025)
        # Date the snapshot by its KAP publish date ("2025.08.12" → "2025-08-12")
        # so monthly reports carry distinct, ordered as_of dates; fall back to
        # H1 year-end only when the publish date is missing or malformed.
        pub = disc.get("publish_date", "")
        as_of = pub.replace(".", "-") if len(pub) == 10 and pub[:4].isdigit() else f"{year}-06-30"
        kap_idx = disc["index"]

        # Smart skip: already processed this exact filing
        if (code, kap_idx) in checkpoint:
            log.info("⏭️  %s idx=%d — zaten işlendi, atlanıyor", code, kap_idx)
            funds_done += 1
            continue

        log.info("📂 %s (%s)  idx=%d", code, fund_type, kap_idx)

        # Find FPD attachment
        fpd_oid = find_fpd_obj_id(disc["attachments"])
        if not fpd_oid:
            log.warning("  %s: FPD PDF eki bulunamadı (ek_sayı=%d)", code, len(disc["attachments"]))
            save_checkpoint(conn, code, kap_idx, as_of)
            checkpoint.add((code, kap_idx))
            continue

        # Download PDF
        time.sleep(REQUEST_DELAY)
        pdf_bytes = kap_download(session, fpd_oid)
        if not pdf_bytes:
            log.warning("  %s: PDF indirilemedi (oid=%s)", code, fpd_oid)
            errors += 1
            funds_done += 1
            progress_update(conn, run_id, indices_scanned=(SCAN_END - SCAN_START) // SCAN_STEP,
                            disclosures_found=len(disclosures), funds_total=len(disclosures),
                            funds_done=funds_done, holdings_saved=total_holdings,
                            errors=errors, last_fund=code)
            continue

        # Parse holdings: deterministic regex first (cheap, no AI). Extract the
        # text once and feed it to both the ISIN regex (which carries weights)
        # and the ticker-token matcher below.
        pdf_text = _pdf_to_text(pdf_bytes)
        regex_holdings = _holdings_from_text(pdf_text)
        holdings_by_ticker: dict[str, Optional[float]] = {
            t: w
            for isin, w in regex_holdings.items()
            if (t := isin_to_ticker(isin)) and (not known_tickers or t in known_tickers)
        }
        parser_used = "regex"

        # Ticker fallback: many fund families (ATA, AKTİF, …) print holdings by
        # BIST ticker, or with the new TRE-prefixed ISIN the TRA regex skips.
        # Add any known ticker the ISIN pass missed (link only, weight None —
        # the column layout makes weight pairing unreliable).
        before = len(holdings_by_ticker)
        for ticker in _tickers_from_text(pdf_text, known_tickers):
            holdings_by_ticker.setdefault(ticker, None)
        if len(holdings_by_ticker) > before:
            parser_used = "regex+ticker" if before else "ticker"

        # AI fallback: regex+ticker were helpless but the document looks like an
        # equity fund → let local Ollama read the unfamiliar layout. Output is
        # already validated against the known BIST universe; degrades to {} on
        # any error.
        if not holdings_by_ticker and ai_cfg is not None:
            if ai_pdf_parser.looks_like_equity_fund(pdf_text):
                ai_holdings = ai_pdf_parser.extract_holdings(
                    pdf_text, known_tickers, ai_cfg, session=ai_session
                )
                if ai_holdings:
                    holdings_by_ticker = ai_holdings
                    parser_used = "ai"

        if not holdings_by_ticker:
            log.info("  %s: Hisse senedi yok (hisse senedi fonu değil)", code)
            upsert_fund_ref(conn, code, disc["company_title"], fund_type, as_of)
            conn.commit()
            save_checkpoint(conn, code, kap_idx, as_of)
            checkpoint.add((code, kap_idx))
            funds_done += 1
            progress_update(conn, run_id, indices_scanned=(SCAN_END - SCAN_START) // SCAN_STEP,
                            disclosures_found=len(disclosures), funds_total=len(disclosures),
                            funds_done=funds_done, holdings_saved=total_holdings,
                            errors=errors, last_fund=code)
            continue

        log.info("  %s: %d holding ayrıştırıldı (%s)", code, len(holdings_by_ticker), parser_used)

        # Upsert fund_ref
        upsert_fund_ref(conn, code, disc["company_title"], fund_type, as_of)

        # Upsert holdings — stock_id = BIST ticker
        saved = 0
        for ticker, weight_pct in holdings_by_ticker.items():
            upsert_holding(conn, code, ticker, weight_pct, as_of)
            saved += 1

        conn.commit()
        log.info("  ✅ %s: %d/%d holding kaydedildi", code, saved, len(holdings_by_ticker))
        total_holdings += saved
        processed += 1

        save_checkpoint(conn, code, kap_idx, as_of)
        checkpoint.add((code, kap_idx))
        funds_done = processed + errors
        progress_update(conn, run_id, indices_scanned=(SCAN_END - SCAN_START) // SCAN_STEP,
                        disclosures_found=len(disclosures), funds_total=len(disclosures),
                        funds_done=funds_done, holdings_saved=total_holdings,
                        errors=errors, last_fund=code)

    conn.close()

    # Mark run as done in DB
    progress_update(conn_final := db_connect(), run_id,
                    indices_scanned=(SCAN_END - SCAN_START) // SCAN_STEP,
                    disclosures_found=len(disclosures), funds_total=len(disclosures),
                    funds_done=processed + errors, holdings_saved=total_holdings,
                    errors=errors, last_fund="", status="done", finished=True)
    conn_final.close()

    log.info(
        "🏁 Tamamlandı: %d fon · %d holding · %d hata",
        processed,
        total_holdings,
        errors,
    )


if __name__ == "__main__":
    if RESCAN_INTERVAL_H > 0:
        while True:
            try:
                main()
            except SystemExit as e:
                # non-zero exit (watchdog timeout) — let Docker restart the container
                if e.code != 0:
                    raise
            log.info("😴 Sonraki tarama: %d saat sonra", RESCAN_INTERVAL_H)
            time.sleep(RESCAN_INTERVAL_H * 3600)
    else:
        main()
