"""
ai_pdf_parser.py — AI-assisted KAP fund-portfolio PDF parser via local Ollama.

Why this exists
---------------
The deterministic regex parser in ``fund_scraper.py`` anchors on the TRA…
equity-ISIN pattern. That covers the common FPD templates, but fund managers
publish the "Fon Portföy Dağılım Raporu" in many layouts — some list holdings
by company *name* with no ISIN, some break the table across columns the regex
cannot follow. When the regex comes back empty on a document that clearly *is*
an equity fund, this module asks a **local Ollama** model to read the raw text
and extract the BIST equity holdings, robust to layout.

Guard rails (project mandate, ADR-0004 + DATA_SOURCES.md)
---------------------------------------------------------
- **No WAF evasion, no external API.** This talks only to a *local* Ollama
  daemon the operator runs; it never reaches a bot-protected portal.
- **Degrade, never fabricate.** If Ollama is unreachable, the model errors, or
  the response is unparseable, ``extract_holdings`` returns ``{}`` and the
  caller keeps the (empty) regex result — it never invents a holding.
- **Grounded output.** Every ticker the model returns is validated against the
  known BIST universe (``entity_ref``); a hallucinated code is dropped, never
  stored.
- **Anchored activation.** The AI is only invoked when the document contains a
  stable Turkish equity-section anchor (``looks_like_equity_fund``), so pure
  bond / money-market funds are never sent to the model.

The module is import-side-effect-free (no network, no DB) so it unit-tests
anywhere; every Ollama interaction is injected via a ``requests``-like session.
"""
from __future__ import annotations

import json
import logging
import os
import re
from typing import NamedTuple, Optional

log = logging.getLogger(__name__)

# ── Configuration ──────────────────────────────────────────────────────────────

DEFAULT_HOST = "http://localhost:11434"
DEFAULT_MODEL = "llama3.1"
DEFAULT_TIMEOUT = 120.0

# Cap the document text fed to the model so the prompt stays bounded regardless
# of PDF size. FPD holding tables sit near the top of the report, so the head of
# the text is where the equities are.
MAX_TEXT_CHARS = 12000

# Stable Turkish equity-section anchors (lower-cased). Presence means the report
# has an equity section worth parsing; absence means a pure bond / money-market
# fund → skip the AI entirely. "hisse sen" matches both "Hisse Senedi" and
# "Hisse Senetleri"; "ortaklık pay" matches "Ortaklık Payları".
_EQUITY_ANCHORS = ("hisse sen", "ortaklık pay")

# A plausible BIST ticker: 2–10 upper-case letters/digits, leading letter.
_TICKER_RE = re.compile(r"^[A-Z][A-Z0-9]{1,9}$")


class OllamaConfig(NamedTuple):
    host: str
    model: str
    timeout: float
    enabled: bool


def config_from_env(env: Optional[dict] = None) -> OllamaConfig:
    """Build an :class:`OllamaConfig` from environment variables.

    ``AI_PARSER_ENABLED`` defaults to enabled; the parser still self-disables at
    runtime when the local Ollama daemon is unreachable (see ``is_available``).
    """
    e = os.environ if env is None else env
    raw_enabled = e.get("AI_PARSER_ENABLED", "1").strip().lower()
    enabled = raw_enabled not in ("0", "false", "no", "off", "")
    try:
        timeout = float(e.get("OLLAMA_TIMEOUT", str(DEFAULT_TIMEOUT)))
    except (ValueError, TypeError):
        timeout = DEFAULT_TIMEOUT
    return OllamaConfig(
        host=e.get("OLLAMA_HOST", DEFAULT_HOST).rstrip("/"),
        model=e.get("OLLAMA_MODEL", DEFAULT_MODEL),
        timeout=timeout,
        enabled=enabled,
    )


def looks_like_equity_fund(pdf_text: str) -> bool:
    """True if the report text mentions a Turkish equity section."""
    low = pdf_text.lower()
    return any(anchor in low for anchor in _EQUITY_ANCHORS)


def is_available(cfg: OllamaConfig, *, session) -> bool:
    """True if the local Ollama daemon answers ``GET /api/tags``."""
    try:
        resp = session.get(f"{cfg.host}/api/tags", timeout=5)
        return getattr(resp, "status_code", None) == 200
    except Exception as exc:  # noqa: BLE001 — any transport error means "down"
        log.debug("Ollama not reachable at %s: %s", cfg.host, exc)
        return False


# ── Prompt construction ────────────────────────────────────────────────────────

_SYSTEM_PROMPT = (
    "You are a precise financial-document extraction engine. You receive the "
    "raw text of a Turkish investment-fund portfolio disclosure (KAP 'Fon "
    "Portföy Dağılım Raporu'). Extract ONLY individual Borsa İstanbul (BIST) "
    "equity holdings — Turkish listed shares. For each holding give the BIST "
    "ticker code (the short upper-case code, e.g. AKBNK, THYAO, GARAN) and its "
    "percentage weight of the fund when stated. Ignore government bonds, "
    "treasury bills, reverse repo, time deposits, participation accounts, other "
    "funds, derivatives, and total/subtotal rows. Never invent a ticker. "
    "Respond with JSON only, no prose."
)

_PROMPT_TEMPLATE = (
    "Return a JSON object of exactly this shape:\n"
    '{{"holdings": [{{"ticker": "AKBNK", "weight": 8.09}}, '
    '{{"ticker": "THYAO", "weight": null}}]}}\n'
    "Rules: tickers are upper-case letters/digits only; use null when the "
    "weight is not stated; output an empty list if there are no BIST equities. "
    "Document text follows between the markers:\n\n"
    "<<<DOC\n{doc}\nDOC>>>"
)


def build_prompt(pdf_text: str) -> str:
    """Build the user prompt, truncating the document to ``MAX_TEXT_CHARS``."""
    return _PROMPT_TEMPLATE.format(doc=pdf_text[:MAX_TEXT_CHARS])


# ── Response parsing ───────────────────────────────────────────────────────────


def _coerce_weight(val) -> Optional[float]:
    """Coerce a model-supplied weight to a sane percentage, else ``None``.

    Accepts ``8.09``, ``"8,09"``, ``"8.09"``; rejects non-numbers and anything
    outside ``(0, 100]`` (totals, garbage) by returning ``None``.
    """
    if val is None:
        return None
    try:
        weight = float(str(val).replace(",", "."))
    except (ValueError, TypeError):
        return None
    return weight if 0 < weight <= 100 else None


def parse_ai_response(raw: str) -> list[tuple[str, Optional[float]]]:
    """Parse the model's JSON reply into ``[(ticker, weight), …]``.

    Tolerates both ``{"holdings": [...]}`` and a bare ``[...]`` list. Any JSON
    error or unexpected shape yields an empty list (the caller degrades).
    """
    try:
        obj = json.loads(raw)
    except (ValueError, TypeError):
        return []

    rows = obj.get("holdings") if isinstance(obj, dict) else obj
    if not isinstance(rows, list):
        return []

    out: list[tuple[str, Optional[float]]] = []
    for row in rows:
        if not isinstance(row, dict):
            continue
        ticker = str(row.get("ticker", "")).strip().upper()
        if not ticker:
            continue
        out.append((ticker, _coerce_weight(row.get("weight"))))
    return out


# ── Ollama call + orchestration ────────────────────────────────────────────────


def call_ollama(cfg: OllamaConfig, prompt: str, *, session) -> str:
    """POST the prompt to Ollama's ``/api/generate`` and return the raw reply.

    Uses ``format=json`` and ``temperature=0`` (plus a fixed seed) so the same
    document yields the same extraction — see the ai-output-stability skill.
    """
    payload = {
        "model": cfg.model,
        "prompt": prompt,
        "system": _SYSTEM_PROMPT,
        "stream": False,
        "format": "json",
        "options": {"temperature": 0, "seed": 42},
    }
    resp = session.post(f"{cfg.host}/api/generate", json=payload, timeout=cfg.timeout)
    resp.raise_for_status()
    data = resp.json()
    return data.get("response", "") if isinstance(data, dict) else ""


def extract_holdings(
    pdf_text: str,
    known_tickers,
    cfg: OllamaConfig,
    *,
    session,
) -> dict[str, Optional[float]]:
    """Extract ``{ticker: weight_pct}`` from a fund-portfolio PDF via Ollama.

    Every ticker is validated against ``known_tickers`` (the BIST universe) when
    that set is non-empty, so hallucinated codes are dropped. Any failure —
    unreachable daemon, model error, unparseable reply — returns ``{}`` so the
    caller keeps degrading rather than fabricating.
    """
    if not pdf_text:
        return {}

    try:
        raw = call_ollama(cfg, build_prompt(pdf_text), session=session)
    except Exception as exc:  # noqa: BLE001 — degrade on any Ollama failure
        log.warning("AI parse failed (Ollama call): %s", exc)
        return {}

    known_upper = {t.upper() for t in known_tickers} if known_tickers else None
    holdings: dict[str, Optional[float]] = {}
    for ticker, weight in parse_ai_response(raw):
        if not _TICKER_RE.match(ticker):
            continue
        if known_upper is not None and ticker not in known_upper:
            log.debug("AI ticker %s not in known BIST universe — dropped", ticker)
            continue
        if ticker in holdings:
            # Keep the first stated weight; only upgrade None → a real weight.
            if holdings[ticker] is None and weight is not None:
                holdings[ticker] = weight
        else:
            holdings[ticker] = weight
    return holdings
