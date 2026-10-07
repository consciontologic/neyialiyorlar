# Phase 6 Coverage Baseline

## Purpose
Enforce minimum coverage floors per package to prevent regression.
Kernels and parsers should have highest coverage.

## Research Module Coverage Floors

- `ic_gate`: 80% - Core IC computation logic
- `calendar`: 85% - BIST calendar (deterministic, fully testable)
- `returns`: 75% - Forward returns builder
- `utils` (db, identity, survivorship): 70% - Utilities and glue code
- `tests`: N/A - Test code itself

## CI Gate
```bash
make ci.research --cov=ic_gate,calendar,returns,utils --cov-report=fail-under:70
```

## Notes
- Coverage floor for ic_gate is high (80%) due to security-critical IC gate logic
- Calendar module is 85% because it's deterministic and fully testable
- Utility modules are 70% as they're more integration/glue code
- A dropped tested path fails the build (mandate 22)
