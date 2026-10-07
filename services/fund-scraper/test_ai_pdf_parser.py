"""Unit tests for ai_pdf_parser — the local-Ollama FPD fallback parser.

These cover the contract that keeps the AI tier safe under the project mandate:

  1. It activates only on documents with a Turkish equity anchor
     (``looks_like_equity_fund``) — pure bond/MM funds are never sent.
  2. It grounds the model: tickers absent from the known BIST universe are
     dropped, never stored (no fabrication).
  3. It degrades: an unreachable daemon / model error / unparseable reply
     yields ``{}`` so the caller keeps the regex result.

Every Ollama interaction is injected via a fake ``requests``-like session, so
these run with no network and no Ollama installed.
"""
from __future__ import annotations

import json
from unittest.mock import Mock

import ai_pdf_parser as ai


# ── config_from_env ────────────────────────────────────────────────────────────


def test_config_from_env_defaults():
    cfg = ai.config_from_env(env={})
    assert cfg.host == ai.DEFAULT_HOST
    assert cfg.model == ai.DEFAULT_MODEL
    assert cfg.timeout == ai.DEFAULT_TIMEOUT
    assert cfg.enabled is True


def test_config_from_env_overrides_and_disable():
    cfg = ai.config_from_env(
        env={
            "OLLAMA_HOST": "http://ollama:11434/",
            "OLLAMA_MODEL": "qwen2.5",
            "OLLAMA_TIMEOUT": "30",
            "AI_PARSER_ENABLED": "0",
        }
    )
    assert cfg.host == "http://ollama:11434"  # trailing slash stripped
    assert cfg.model == "qwen2.5"
    assert cfg.timeout == 30.0
    assert cfg.enabled is False


# ── looks_like_equity_fund ─────────────────────────────────────────────────────


def test_looks_like_equity_fund_detects_turkish_anchors():
    assert ai.looks_like_equity_fund("... Hisse Senedi ... AKBNK ...") is True
    assert ai.looks_like_equity_fund("Ortaklık Payları Tablosu") is True


def test_looks_like_equity_fund_false_on_bond_fund():
    txt = "Devlet Tahvili ve Ters Repo portföyü — para piyasası fonu"
    assert ai.looks_like_equity_fund(txt) is False


# ── build_prompt ───────────────────────────────────────────────────────────────


def test_build_prompt_includes_doc_and_json_shape():
    prompt = ai.build_prompt("HOLDING ROW AKBNK 8,09")
    assert "HOLDING ROW AKBNK 8,09" in prompt
    assert '"holdings"' in prompt


def test_build_prompt_truncates_long_text():
    long_text = "x" * (ai.MAX_TEXT_CHARS + 5000)
    prompt = ai.build_prompt(long_text)
    # Isolate the document segment between the markers and assert it is capped.
    doc = prompt.split("<<<DOC\n", 1)[1].rsplit("\nDOC>>>", 1)[0]
    assert len(doc) == ai.MAX_TEXT_CHARS
    assert set(doc) == {"x"}


# ── parse_ai_response ──────────────────────────────────────────────────────────


def test_parse_ai_response_extracts_ticker_and_weight():
    raw = json.dumps(
        {"holdings": [{"ticker": "akbnk", "weight": "8,09"}, {"ticker": "THYAO", "weight": 5}]}
    )
    assert ai.parse_ai_response(raw) == [("AKBNK", 8.09), ("THYAO", 5.0)]


def test_parse_ai_response_accepts_bare_list():
    raw = json.dumps([{"ticker": "GARAN", "weight": 3.2}])
    assert ai.parse_ai_response(raw) == [("GARAN", 3.2)]


def test_parse_ai_response_null_and_out_of_range_weight_become_none():
    raw = json.dumps(
        {"holdings": [
            {"ticker": "AAA", "weight": None},
            {"ticker": "BBB", "weight": 0},
            {"ticker": "CCC", "weight": 150},
        ]}
    )
    assert ai.parse_ai_response(raw) == [("AAA", None), ("BBB", None), ("CCC", None)]


def test_parse_ai_response_invalid_json_returns_empty():
    assert ai.parse_ai_response("not json at all") == []
    assert ai.parse_ai_response('{"holdings": "oops"}') == []


# ── is_available ───────────────────────────────────────────────────────────────


def test_is_available_true_on_200():
    session = Mock()
    session.get.return_value = Mock(status_code=200)
    assert ai.is_available(ai.config_from_env(env={}), session=session) is True


def test_is_available_false_on_exception():
    session = Mock()
    session.get.side_effect = OSError("connection refused")
    assert ai.is_available(ai.config_from_env(env={}), session=session) is False


# ── call_ollama ────────────────────────────────────────────────────────────────


def test_call_ollama_posts_generate_and_returns_response():
    cfg = ai.config_from_env(env={"OLLAMA_HOST": "http://h:11434", "OLLAMA_MODEL": "m"})
    resp = Mock()
    resp.raise_for_status.return_value = None
    resp.json.return_value = {"response": '{"holdings": []}'}
    session = Mock()
    session.post.return_value = resp

    out = ai.call_ollama(cfg, "PROMPT", session=session)

    assert out == '{"holdings": []}'
    url, kwargs = session.post.call_args[0][0], session.post.call_args[1]
    assert url == "http://h:11434/api/generate"
    assert kwargs["json"]["model"] == "m"
    assert kwargs["json"]["stream"] is False
    assert kwargs["json"]["format"] == "json"
    assert kwargs["json"]["options"]["temperature"] == 0


# ── extract_holdings (orchestration) ───────────────────────────────────────────


def _session_returning(payload_obj) -> Mock:
    resp = Mock()
    resp.raise_for_status.return_value = None
    resp.json.return_value = {"response": json.dumps(payload_obj)}
    session = Mock()
    session.post.return_value = resp
    return session


def test_extract_holdings_validates_against_known_universe():
    session = _session_returning(
        {"holdings": [
            {"ticker": "AKBNK", "weight": 8.09},
            {"ticker": "FAKE1", "weight": 2.0},  # not in universe → dropped
        ]}
    )
    out = ai.extract_holdings(
        "Hisse Senedi tablosu", {"AKBNK", "THYAO"}, ai.config_from_env(env={}), session=session
    )
    assert out == {"AKBNK": 8.09}


def test_extract_holdings_no_universe_keeps_valid_tickers():
    session = _session_returning({"holdings": [{"ticker": "ZZZ", "weight": 1.0}]})
    out = ai.extract_holdings("Hisse Senedi", set(), ai.config_from_env(env={}), session=session)
    assert out == {"ZZZ": 1.0}


def test_extract_holdings_degrades_to_empty_on_ollama_error():
    session = Mock()
    session.post.side_effect = OSError("connection refused")
    out = ai.extract_holdings(
        "Hisse Senedi", {"AKBNK"}, ai.config_from_env(env={}), session=session
    )
    assert out == {}


def test_extract_holdings_empty_text_returns_empty():
    session = Mock()
    out = ai.extract_holdings("", {"AKBNK"}, ai.config_from_env(env={}), session=session)
    assert out == {}
    session.post.assert_not_called()


def test_extract_holdings_dedups_keeping_first_real_weight():
    session = _session_returning(
        {"holdings": [
            {"ticker": "AKBNK", "weight": None},
            {"ticker": "AKBNK", "weight": 8.09},
        ]}
    )
    out = ai.extract_holdings("Hisse Senedi", {"AKBNK"}, ai.config_from_env(env={}), session=session)
    assert out == {"AKBNK": 8.09}
