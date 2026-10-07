<!-- docs/design/ADR-0001-go-primary-backend.md -->

# 📐 ADR-0001: Go-primary, single-language, right-sized backend

- **Status**: accepted
- **Date**: 2026-06-14
- **Deciders**: Architecture
- **Supersedes**: the original brief's polyglot/HFT-extreme stack assumption

## Context

The verified workload is **small, multi-cadence, daily-batch data**: ≤ a few
thousand funds/securities, KB–MB/day, refreshed intraday/daily/weekly/event.
The math is light — ratios, deltas, z-scores, rolling windows, Spearman rank
correlation. The original brief implied HFT-grade machinery (zero-copy
deserialization, lock-free queues, AVX-512/SIMD, GPU/NPU). None of that is
justified by this data volume.

## Decision

Use **Go for all three runtime services** (Ingestion Engine, Analytic Core,
Web API Gateway), with **Python only as an optional offline research sidecar**
(backtests + the Phase-6 IC gate), never in the live hot path.

## Consequences

- ➕ One language, one toolchain, single **static binaries** → tiny images, fast
  cold start, low memory footprint, trivial containerization.
- ➕ Go's goroutines cover the *only* real concurrency need (polite concurrent
  HTTP) without exotic primitives.
- ➕ Clear hot-path / research-path split keeps numpy/scipy out of production.
- ➖ Heavy statistical research in Go is awkward — accepted, hence the Python
  sidecar for offline work.
- 🔁 Reversible: if a future **tick-history** workload ever appears, revisit
  vectorization/columnar compute then — not now.

## Considered options

- **Go-only runtime + Python research sidecar** (chosen) — right-sized; matches
  the data volume and the light math; one runtime language.
- **Polyglot microservices (Go + Python + Rust core)** — rejected: operational
  and build complexity with no workload justification.
- **HFT-extreme single service (SIMD/GPU/lock-free)** — rejected: explicitly
  out of scope per mandate 7; solves a problem we do not have.
