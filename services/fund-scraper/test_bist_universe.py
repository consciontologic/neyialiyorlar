"""Unit tests for bist_universe.parse_companies.

These exercise the KAP "BİST Şirketleri" extraction without any network or DB:
the sample mirrors the real server-rendered payload (JSON embedded in a JS
string literal, so quotes arrive backslash-escaped).
"""
import bist_universe as bu

# Mirrors KAP's embedded Next.js payload: JSON inside a JS string => escaped \".
# Four records: ACME, BETA, a duplicate ACME (deduped), and a stockCode-less
# member (skipped).
SAMPLE_HTML = (
    'noise<script>self.__next_f.push([1,"15:[\\"$\\",\\"div\\",null,{\\"data\\":'
    '[{\\"code\\":\\"A\\",\\"content\\":[{'
    '\\"mkkMemberOid\\":\\"OID1\\",\\"kapMemberTitle\\":\\"ACME SELÜLOZ A.Ş.\\",'
    '\\"relatedMemberTitle\\":\\"PwC A.Ş\\",\\"stockCode\\":\\"ACME\\",'
    '\\"cityName\\":\\"İSTANBUL\\",\\"kapMemberType\\":\\"IGS\\"},{'
    '\\"mkkMemberOid\\":\\"OID2\\",\\"kapMemberTitle\\":\\"BETA HOLDİNG A.Ş.\\",'
    '\\"stockCode\\":\\"BETA\\",\\"kapMemberType\\":\\"IGS\\"},{'
    '\\"mkkMemberOid\\":\\"OID1B\\",\\"kapMemberTitle\\":\\"ACME SELÜLOZ A.Ş.\\",'
    '\\"stockCode\\":\\"ACME\\",\\"kapMemberType\\":\\"IGS\\"},{'
    '\\"mkkMemberOid\\":\\"OID3\\",\\"kapMemberTitle\\":\\"GAMMA DENETİM A.Ş.\\",'
    '\\"stockCode\\":\\"\\",\\"kapMemberType\\":\\"BD\\"}]}]}]"])</script>'
)


def test_parse_companies_extracts_unique_nonempty_tickers():
    codes = [c.stock_code for c in bu.parse_companies(SAMPLE_HTML)]
    # ACME deduped to one; empty-stockCode member skipped.
    assert codes == ["ACME", "BETA"]


def test_parse_companies_preserves_title_and_type():
    acme = bu.parse_companies(SAMPLE_HTML)[0]
    assert acme.title == "ACME SELÜLOZ A.Ş."
    assert acme.member_type == "IGS"


def test_parse_companies_lowercase_code_is_uppercased():
    html = SAMPLE_HTML.replace('\\"stockCode\\":\\"BETA\\"', '\\"stockCode\\":\\"beta\\"')
    codes = [c.stock_code for c in bu.parse_companies(html)]
    assert "BETA" in codes and "beta" not in codes


def test_parse_companies_empty_on_unrecognised_page():
    assert bu.parse_companies("<html>no company data here</html>") == []


def test_parse_companies_empty_string():
    assert bu.parse_companies("") == []
