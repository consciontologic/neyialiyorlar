"""
bist_universe.py — populate entity_ref with the full BIST equity universe.

The Hisseler (stocks) page lists whatever is in entity_ref. Historically only a
handful of blue-chips (the disabled synthetic seed) were present, so most
BIST-listed stocks never appeared. This module fetches the authoritative list
of BIST companies from KAP's public "BİST Şirketleri" page and upserts every
ticker into entity_ref as an active security.

Source (ToS-compliant, no auth — a single polite GET, never the high-volume
scan that trips KAP's WAF):

    https://www.kap.org.tr/tr/bist-sirketler

KAP's site is a Next.js app that server-renders the company list into the page
HTML as a JSON payload. Each company record exposes stable field names:

    stockCode        BIST ticker        -> entity_ref.entity_id / display_ticker
    kapMemberTitle   company legal name
    kapMemberType    'IGS' = traded company
    mkkMemberOid     MKK member id

We anchor on those field names (not DOM coordinates), so layout drift does not
break extraction: a record without a stockCode is skipped, and an empty parse is
a safe no-op that never wipes existing rows. Degradation, not fabrication.
"""
from __future__ import annotations

import logging
import os
import re
from typing import NamedTuple

import psycopg
import requests

log = logging.getLogger(__name__)

KAP_COMPANIES_URL = "https://www.kap.org.tr/tr/bist-sirketler"
HTTP_TIMEOUT = float(os.environ.get("BIST_HTTP_TIMEOUT", "30"))
DB_DSN = os.environ.get(
    "DATABASE_URL",
    "host=postgres dbname=neyialiyorlar user=postgres",
)

_HEADERS = {
    "User-Agent": (
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
        "(KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
    ),
    "Accept": "text/html,application/xhtml+xml",
    "Accept-Language": "tr-TR,tr;q=0.9,en;q=0.8",
}

# Anchored on KAP's stable JSON field names, tolerant of field-order drift.
# stockCode is captured case-insensitively then normalised to upper-case so the
# entity_id matches fund_holding.stock_id and the Yahoo ".IS" pricing key.
_STOCK_CODE_RE = re.compile(r'"stockCode":"([A-Za-z0-9]*)"')
_TITLE_RE = re.compile(r'"kapMemberTitle":"((?:[^"\\]|\\.)*)"')
_TYPE_RE = re.compile(r'"kapMemberType":"([A-Z]*)"')


class Company(NamedTuple):
    """A single BIST-listed company extracted from the KAP companies page."""

    stock_code: str
    title: str
    member_type: str


def parse_companies(html: str) -> list[Company]:
    """Extract every BIST company (with a non-empty stockCode) from KAP HTML.

    Robust to field-order and layout drift: it anchors on KAP's stable JSON
    field names rather than DOM structure. Returns a de-duplicated list; an
    unrecognised or empty page yields ``[]`` so callers can no-op safely.
    """
    # The company list is embedded inside a JS string literal, so its JSON
    # quotes arrive escaped (\"). Unescape once, then split into per-company
    # chunks on the stable leading field of each record.
    text = html.replace('\\"', '"').replace("\\\\", "\\")
    companies: list[Company] = []
    seen: set[str] = set()
    for chunk in text.split('"mkkMemberOid":')[1:]:
        m_code = _STOCK_CODE_RE.search(chunk)
        if not m_code:
            continue
        code = m_code.group(1).strip().upper()
        if not code or code in seen:
            continue
        m_title = _TITLE_RE.search(chunk)
        m_type = _TYPE_RE.search(chunk)
        seen.add(code)
        companies.append(
            Company(
                stock_code=code,
                title=(m_title.group(1).strip() if m_title else ""),
                member_type=(m_type.group(1) if m_type else ""),
            )
        )
    return companies


def fetch_companies_html(session: requests.Session) -> str:
    """GET the KAP BİST Şirketleri page and return its HTML."""
    resp = session.get(KAP_COMPANIES_URL, timeout=HTTP_TIMEOUT)
    resp.raise_for_status()
    return resp.text


def upsert_entity(conn: psycopg.Connection, code: str) -> None:
    """Upsert one BIST ticker into entity_ref as an active security.

    The existing ``isin`` is left untouched on conflict (so the synthetic-seed
    ISINs survive), and a row that re-appears in the listing is re-activated
    (``valid_to`` -> NULL).
    """
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO entity_ref
                (entity_id, entity_type, display_ticker, valid_from, valid_to, isin)
            VALUES (%s, 'security', %s, CURRENT_DATE, NULL, NULL)
            ON CONFLICT (entity_id) DO UPDATE SET
                entity_type    = 'security',
                display_ticker = EXCLUDED.display_ticker,
                valid_to       = NULL
            """,
            (code, code),
        )


def ingest(conn: psycopg.Connection, session: requests.Session) -> int:
    """Fetch the BIST universe and upsert every ticker into entity_ref.

    Returns the number of securities upserted. A parse that yields no companies
    leaves entity_ref untouched (never deletes), so a bad fetch degrades to a
    no-op instead of wiping the catalogue.
    """
    html = fetch_companies_html(session)
    companies = parse_companies(html)
    if not companies:
        log.warning(
            "BIST universe: parsed 0 companies — leaving entity_ref untouched"
        )
        return 0
    for company in companies:
        upsert_entity(conn, company.stock_code)
    conn.commit()
    log.info(
        "BIST universe: upserted %d securities into entity_ref", len(companies)
    )
    return len(companies)


def main() -> int:
    logging.basicConfig(
        level=os.environ.get("LOG_LEVEL", "INFO"),
        format="%(asctime)s %(levelname)s %(message)s",
    )
    session = requests.Session()
    session.headers.update(_HEADERS)
    with psycopg.connect(DB_DSN) as conn:
        count = ingest(conn, session)
    print(f"entity_ref securities upserted: {count}")
    return 0 if count else 1


if __name__ == "__main__":
    raise SystemExit(main())
