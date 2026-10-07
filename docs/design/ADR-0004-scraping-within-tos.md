<!-- docs/design/ADR-0004-scraping-within-tos.md -->

# 📐 ADR-0004: Self-adaptive scraping **within ToS** — no evasion, ever

- **Status**: accepted
- **Date**: 2026-06-14
- **Deciders**: Architecture
- **Related**: [DATA_SOURCES.md](DATA_SOURCES.md), Charter §Scope (out)

## Context

Robustness and evasion are different things. We need extractors that survive
benign drift (field renames, DOM reshuffles, PDF layout shifts). We do **not**
want — and will not build — anything that defeats bot protection. Some Turkish
portals (certain rating feeds, some VAP detail) sit behind active WAF/CAPTCHA.

## Decision

Build a **self-adaptive but polite** extraction tier: polymorphic JSON envelope
guards, heuristic HTML selector fallback anchored on **stable Turkish labels**,
and PDF parsing anchored on **stable headers** — all of which *flag and
quarantine* on drift rather than crash. Establish a **hard boundary**: no
WAF/CAPTCHA/TLS-fingerprint/proxy-rotation evasion. WAF-protected portals are
**excluded** from the automated Harvester.

## Consequences

- ➕ Resilient to the *common* breakage (drift) without crossing a ToS/legal line.
- ➕ Maintainable: anchors on human-meaningful labels age better than brittle
  CSS paths.
- ➕ Legally and ethically clean; safe to run unattended on one host.
- ➖ Some data (metric 7's ratings leg) stays **unavailable/degraded** — accepted;
  it ships `approx` or as a gap, never fabricated.
- 🔁 Escape hatch: if WAF-gated data is ever essential, it is fetched **only**
  via an optional, documented, **human-driven** browser export — never automated.

## Considered options

- **Polite self-adaptive scraping + hard no-evasion boundary** (chosen).
- **Full evasion stack (rotating proxies, TLS spoofing, CAPTCHA solving)** —
  rejected: out of scope by mandate, ToS/legal risk, high maintenance, brittle.
- **Rigid scrapers, no adaptivity** — rejected: break on every benign drift,
  violating reliability mandate 6.
