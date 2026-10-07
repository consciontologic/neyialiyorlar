"""Unit tests for the fund_scraper FPD-PDF parsing helpers.

These cover the two defects that made the scraper record almost no holdings:

  1. KAP's /tr/api/file/download endpoint wraps every file in a Java-serialized
     byte[] envelope (magic 0xACED0005), so the bytes do not start with "%PDF".
     ``_extract_pdf_payload`` must slice out the real PDF before pypdf sees it.

  2. FPD PDF templates differ by fund manager. The original parser only matched
     the "FTD% glued directly before the ISIN" template (AEH) and silently
     dropped every fund that printed the ISIN in its own column with the check
     digit wrapped onto the next line (NHM), mislabelling them "not an equity
     fund". ``_holdings_from_text`` must capture equity ISINs regardless of
     layout, attaching the weight only when it can be paired.

Importing fund_scraper has no side effects beyond compiling regexes and reading
env defaults — no DB or network access — so these run anywhere.
"""
from __future__ import annotations

import fund_scraper as fs


# ── _extract_pdf_payload ──────────────────────────────────────────────────────


def test_extract_pdf_payload_unwraps_java_envelope():
    pdf = b"%PDF-1.5\nbody bytes\n%%EOF"
    wrapped = b"\xac\xed\x00\x05ur\x00\x02[B\x00\x00" + pdf + b"\x90\x91trailing"
    assert fs._extract_pdf_payload(wrapped) == pdf


def test_extract_pdf_payload_passthrough_bare_pdf():
    pdf = b"%PDF-1.7\nbody\n%%EOF"
    assert fs._extract_pdf_payload(pdf) == pdf


def test_extract_pdf_payload_without_pdf_returns_input():
    junk = b"\xac\xed\x00\x05 no pdf marker here"
    assert fs._extract_pdf_payload(junk) == junk


# ── _holdings_from_text ───────────────────────────────────────────────────────


def test_glued_template_captures_weight():
    # AEH-style: FTD% glued directly before the ISIN.
    text = "8,09TRAAKBNK91N6 0,06TRAANSGR91O1"
    assert fs._holdings_from_text(text) == {
        "TRAAKBNK91N6": 8.09,
        "TRAANSGR91O1": 0.06,
    }


def test_split_isin_template_captures_link_without_weight():
    # NHM-style: ISIN in its own column, check digit wrapped to the next line,
    # weight not adjacent. The fund->stock link must still be recorded (None).
    text = "GUBRF G\u00dcBRE\nTRAGUBRF91E\n2 50,000.000\n"
    assert fs._holdings_from_text(text) == {"TRAGUBRF91E2": None}


def test_non_equity_isins_excluded():
    # TRE (treasury bond) / TRT (bill) ISINs are not BIST equity.
    text = "TREKCAE0003 0.00 TRT011025T16 0.00"
    assert fs._holdings_from_text(text) == {}


def test_same_isin_multiple_lots_summed():
    text = "5,00TRAAKBNK91N6 lot two 3,50TRAAKBNK91N6"
    assert fs._holdings_from_text(text) == {"TRAAKBNK91N6": 8.5}


def test_zero_weight_keeps_link_with_unknown_weight():
    # A glued weight of 0 is not a usable weight, but the holding still exists.
    assert fs._holdings_from_text("0,00TRAAKBNK91N6") == {"TRAAKBNK91N6": None}


def test_single_weight_over_100_is_dropped_to_none():
    # A glued >100% number is a market value / share count, not a weight. The
    # link is kept, the impossible weight discarded (None) — never stored.
    assert fs._holdings_from_text("565,50TRAAKBNK91N6") == {"TRAAKBNK91N6": None}


def test_value_column_summing_over_100_drops_all_weights():
    # A value/share template: each number ≤ 100 individually but the slice sums
    # to an impossible >100% — discard every weight, keep every link.
    text = "60,00TRAAKBNK91N6 60,00TRATHYAO91M5"
    assert fs._holdings_from_text(text) == {
        "TRAAKBNK91N6": None,
        "TRATHYAO91M5": None,
    }


def test_plausible_distribution_near_100_is_kept():
    # A real distribution that sums to ~100 stays intact (boundary check).
    text = "40,00TRAAKBNK91N6 55,00TRATHYAO91M5"
    assert fs._holdings_from_text(text) == {
        "TRAAKBNK91N6": 40.0,
        "TRATHYAO91M5": 55.0,
    }


def test_empty_text_returns_empty():
    assert fs._holdings_from_text("") == {}


# ── _tickers_from_text ────────────────────────────────────────────────────────
#
# Some fund families (ATA, AKTİF, …) print holdings by ticker, or with the new
# TRE-prefixed ISIN the TRA regex skips — so the ISIN pass records nothing for
# them. Matching known tickers in the text recovers the link. These pin that the
# match is gated by the known universe and never fires inside an ISIN.


def test_tickers_from_text_matches_known_standalone_tokens():
    known = {"AKBNK", "THYAO", "GARAN"}
    text = "İhraççı AKBANK T.A.S.\nAKBNK\nTHYAO PAY\nGARAN"
    assert fs._tickers_from_text(text, known) == {"AKBNK", "THYAO", "GARAN"}


def test_tickers_from_text_ignores_unknown_words():
    # Turkish header words / company names must never register as holdings.
    known = {"AKBNK"}
    text = "HİSSE SENEDİ PAY GRUBU ORTAKLIK İhraççı Rayiç"
    assert fs._tickers_from_text(text, known) == set()


def test_tickers_from_text_does_not_match_ticker_inside_isin():
    # The look-around bounds keep the "AKBNK" inside an ISIN from counting as a
    # standalone ticker — the ISIN pass owns that path.
    known = {"AKBNK"}
    assert fs._tickers_from_text("8,09TRAAKBNK91N6", known) == set()


def test_tickers_from_text_empty_universe_returns_empty():
    assert fs._tickers_from_text("AKBNK THYAO", set()) == set()


# ── isin_to_ticker ────────────────────────────────────────────────────────────


def test_isin_to_ticker_strips_suffix():
    assert fs.isin_to_ticker("TRAGUBRF91E2") == "GUBRF"
    assert fs.isin_to_ticker("TRAAKBNK91N6") == "AKBNK"


def test_isin_to_ticker_rejects_non_equity():
    assert fs.isin_to_ticker("TREKCAE0003") is None


# ── find_fpd_obj_id ───────────────────────────────────────────────────────────
#
# Coverage was failing not because of parsing but because of *discovery*: funds
# file monthly portfolio reports that usually ship a single, un-keyworded PDF
# named after the fund and month. The old selector required either a portfolio
# keyword or a 2nd PDF and so returned nothing for those — leaving most stocks
# with no funds. These pin the broadened selection rules.


def _att(name: str, *, ext: str = "pdf", oid: str | None = None) -> dict:
    return {"fileName": name, "fileExtension": ext, "objId": oid or name}


def test_find_fpd_keyword_match_wins():
    atts = [_att("ACTIVITY.pdf", oid="a"), _att("FPD_2025.pdf", oid="b")]
    assert fs.find_fpd_obj_id(atts) == "b"


def test_find_fpd_dagilim_keyword_match():
    # Monthly report named "<CODE> <MONTH> <YEAR> DAĞILIM.pdf" (dotted-İ casing).
    atts = [_att("EK.pdf", oid="x"), _att("TCB TEMMUZ 2025 DAĞILIM.pdf", oid="y")]
    assert fs.find_fpd_obj_id(atts) == "y"


def test_find_fpd_single_pdf_taken_even_without_keyword():
    # The regression: a lone "CPU_2025.07.pdf" must be selected, not skipped.
    atts = [_att("CPU_2025.07.pdf", oid="only")]
    assert fs.find_fpd_obj_id(atts) == "only"


def test_find_fpd_single_pdf_ignores_non_pdf_siblings():
    atts = [_att("cover.xlsx", ext="xlsx", oid="s"), _att("YDI.pdf", oid="p")]
    assert fs.find_fpd_obj_id(atts) == "p"


def test_find_fpd_multi_pdf_no_keyword_falls_back_to_second():
    atts = [_att("one.pdf", oid="1"), _att("two.pdf", oid="2"), _att("three.pdf", oid="3")]
    assert fs.find_fpd_obj_id(atts) == "2"


def test_find_fpd_no_pdf_returns_none():
    assert fs.find_fpd_obj_id([_att("data.xlsx", ext="xlsx")]) is None
    assert fs.find_fpd_obj_id([]) is None


# ── PORTFOLIO_PERIODS contract ────────────────────────────────────────────────
#
# The scan must keep monthly (AB) and quarterly (3AB) reports, not only the
# semi-annual (6AB) ones — that single-period filter was the root cause of the
# empty coverage. These guard against a regression back to 6AB-only.


def test_portfolio_periods_includes_monthly_and_quarterly():
    assert {"AB", "3AB", "6AB"} <= fs.PORTFOLIO_PERIODS


def test_portfolio_periods_excludes_prospectus():
    # "HZ" (İzahname / prospectus) is not a portfolio-distribution report.
    assert "HZ" not in fs.PORTFOLIO_PERIODS


# ── _disclosure_record ────────────────────────────────────────────────────────
#
# KAP returns JSON null for fundType (and others) on some monthly filings.
# dict.get(key, "") returns None — not "" — for a present-but-null key, which
# crashed the scan against the NOT NULL scan-cache columns. These pin the
# coercion and the skip-when-no-code rule.


def test_disclosure_record_coerces_null_fields():
    db = {
        "stockCode": "STE",
        "disclosureType": "FON",
        "fundType": None,        # present-but-null → must become ""
        "period": "3AB",
        "companyTitle": None,
        "mkkMemberOid": None,
        "publishDate": None,
        "year": None,
    }
    rec = fs._disclosure_record(1474561, db, [])
    assert rec is not None
    assert rec["fund_type"] == ""
    assert rec["company_title"] == ""
    assert rec["mkk_member_oid"] == ""
    assert rec["publish_date"] == ""
    assert rec["year"] == 2025
    assert rec["stock_code"] == "STE"


def test_disclosure_record_truncates_publish_date():
    db = {
        "stockCode": "AEH",
        "disclosureType": "FON",
        "publishDate": "2025.08.12 14:30:00",
        "period": "6AB",
    }
    assert fs._disclosure_record(1, db, [])["publish_date"] == "2025.08.12"


def test_disclosure_record_skips_when_no_stock_code():
    assert fs._disclosure_record(1, {"period": "AB", "disclosureType": "FON"}, []) is None
    assert (
        fs._disclosure_record(
            1, {"stockCode": None, "period": "AB", "disclosureType": "FON"}, []
        )
        is None
    )


def test_disclosure_record_keeps_fon_disclosure():
    db = {
        "stockCode": "BHL",
        "disclosureType": "FON",
        "fundType": "SYF",
        "period": "AB",
        "companyTitle": "İŞ PORTFÖY BİRİNCİ HİSSE SENEDİ SERBEST FON",
    }
    rec = fs._disclosure_record(1472119, db, [])
    assert rec is not None
    assert rec["stock_code"] == "BHL"
    assert rec["fund_type"] == "SYF"


def test_disclosure_record_skips_company_financial_report():
    # BEGYO files a "Finansal Rapor" (disclosureType "FR") on a 3AB period — a
    # company report, not a fund portfolio. Must be skipped despite the
    # portfolio-looking period, or the company statement becomes a bogus fund.
    db = {"stockCode": "BEGYO", "fundType": None, "period": "3AB", "disclosureType": "FR"}
    assert fs._disclosure_record(1472021, db, []) is None


def test_disclosure_record_skips_general_disclosure():
    # KAYSE files a general ("DG") disclosure on a 3AB period — also not a fund.
    db = {"stockCode": "KAYSE", "fundType": None, "period": "3AB", "disclosureType": "DG"}
    assert fs._disclosure_record(1472018, db, []) is None


def test_disclosure_record_skips_when_disclosure_type_missing():
    # No disclosureType at all → cannot confirm a fund filing → skip.
    assert fs._disclosure_record(1, {"stockCode": "BHL", "period": "AB"}, []) is None


# ── load_scan_cache ───────────────────────────────────────────────────────────
#
# Regression: main()'s cached-resume branch called load_scan_cache with a stray
# 4th argument (SCAN_STEP), copy-pasted from the sibling scan/save calls. The
# cache read is bounded by [start, end) only — step is applied at scan time, not
# on read — so the function takes exactly (conn, start, end). The extra arg
# raised "TypeError: takes 3 positional arguments but 4 were given" the moment a
# band was re-run against an already-scanned range, breaking every resume.


class _FakeCursor:
    """Minimal psycopg-cursor stand-in: records query params, returns rows."""

    def __init__(self, rows: list, captured: dict) -> None:
        self._rows = rows
        self._captured = captured

    def __enter__(self) -> "_FakeCursor":
        return self

    def __exit__(self, *exc: object) -> bool:
        return False

    def execute(self, sql: str, params: tuple) -> None:
        self._captured["params"] = params

    def fetchall(self) -> list:
        return self._rows


class _FakeConn:
    """psycopg-connection stand-in — no DB, keeps tests network-free."""

    def __init__(self, rows: list) -> None:
        self._rows = rows
        self.captured: dict = {}

    def cursor(self) -> _FakeCursor:
        return _FakeCursor(self._rows, self.captured)


def test_load_scan_cache_takes_three_args_and_maps_rows():
    rows = [
        (1472200, "AEH", "HSF", "HEDEF PORTFÖY", "OID-1", "2025-06-30", 2025,
         [{"objId": "x", "fileName": "FPD.pdf"}]),
        # Null oid/publish_date and null attachments must coerce, not crash.
        (1472250, "NHM", "HSF", "İŞ PORTFÖY", None, None, 2025, None),
    ]
    conn = _FakeConn(rows)
    # Exactly three positional args — a 4th would raise TypeError (the bug).
    out = fs.load_scan_cache(conn, 1472000, 1473000)
    assert conn.captured["params"] == (1472000, 1473000)
    assert out == [
        {
            "index": 1472200,
            "stock_code": "AEH",
            "fund_type": "HSF",
            "company_title": "HEDEF PORTFÖY",
            "mkk_member_oid": "OID-1",
            "publish_date": "2025-06-30",
            "year": 2025,
            "attachments": [{"objId": "x", "fileName": "FPD.pdf"}],
        },
        {
            "index": 1472250,
            "stock_code": "NHM",
            "fund_type": "HSF",
            "company_title": "İŞ PORTFÖY",
            "mkk_member_oid": "",
            "publish_date": "",
            "year": 2025,
            "attachments": [],
        },
    ]


# ── checkpoint / scan-range skip — the "fetch once" guarantee ──────────────────
#
# These lock in the incremental contract the scraper relies on: a filing already
# recorded in scraper_checkpoint is skipped (never re-downloaded or re-parsed),
# and an index range already in scraper_scan_range is served from the DB cache
# instead of re-hitting KAP. A fund is therefore only fetched again when it
# publishes a NEW disclosure (a new kap_idx) — i.e. when it actually changes.


class _RecordingCursor:
    """Cursor stand-in that records the last SQL+params and serves canned rows."""

    def __init__(self, rows: list, captured: dict) -> None:
        self._rows = rows
        self._captured = captured

    def __enter__(self) -> "_RecordingCursor":
        return self

    def __exit__(self, *exc: object) -> bool:
        return False

    def execute(self, sql: str, params: tuple | None = None) -> None:
        self._captured["sql"] = sql
        self._captured["params"] = params

    def fetchall(self) -> list:
        return self._rows

    def fetchone(self):
        return self._rows[0] if self._rows else None


class _RecordingConn:
    """psycopg-connection stand-in — no DB, counts commits, keeps tests offline."""

    def __init__(self, rows: list | None = None) -> None:
        self._rows = rows or []
        self.captured: dict = {}
        self.commits = 0

    def cursor(self) -> _RecordingCursor:
        return _RecordingCursor(self._rows, self.captured)

    def commit(self) -> None:
        self.commits += 1


def test_load_checkpoint_returns_processed_filing_keys():
    # Each processed filing comes back as a (fund_code, kap_idx) tuple so main()
    # can skip it in O(1).
    conn = _RecordingConn([("AEH", 1474100), ("NHM", 1475200)])
    assert fs.load_checkpoint(conn) == {("AEH", 1474100), ("NHM", 1475200)}


def test_load_checkpoint_empty_when_nothing_processed():
    assert fs.load_checkpoint(_RecordingConn([])) == set()


def test_save_checkpoint_is_idempotent_upsert_and_commits():
    conn = _RecordingConn()
    fs.save_checkpoint(conn, "AEH", 1474100, "2025-06-30")
    sql = conn.captured["sql"]
    assert "scraper_checkpoint" in sql
    # ON CONFLICT keeps the natural (fund_code, kap_idx) filing key unique, so a
    # re-run never inserts a duplicate or re-processes the same filing.
    assert "ON CONFLICT" in sql
    assert conn.captured["params"] == ("AEH", 1474100, "2025-06-30")
    assert conn.commits == 1


def test_is_range_scanned_true_when_range_cached():
    # A matching scraper_scan_range row → the slow KAP index scan is skipped and
    # disclosures are served from scraper_scan_cache instead.
    conn = _RecordingConn([(1,)])
    assert fs.is_range_scanned(conn, 1474000, 1477000, 1) is True
    assert conn.captured["params"] == (1474000, 1477000, 1)


def test_is_range_scanned_false_when_range_absent():
    # No row → the range was never fully scanned, so the scraper must scan it.
    assert fs.is_range_scanned(_RecordingConn([]), 1474000, 1477000, 1) is False


