# Flutter Golden Test Baselines

This directory contains baseline PNG files for Flutter widget golden tests.
These validate that widgets render consistently across runs and platforms.

## Baselines

### Flag Badge Goldens
- `flag_badge_fresh.png` — Confidence.fresh renders as solid green circle with checkmark
- `flag_badge_stale.png` — Confidence.stale renders as amber circle with warning icon
- `flag_badge_approx.png` — Confidence.approx renders as orange circle with info icon
- `flag_badge_small.png` — Small size (16px) renders correctly

### Gap Rendering Goldens
- `dense_cell_gap.png` — Gap value (null) renders as "—" (dash), never "0"
- `dense_cell_value.png` — Present value renders with 2 decimal places
- `dense_cells_gap_vs_value.png` — Gap and value side-by-side show distinct visual treatment

## Regenerating Baselines

To update baselines after intentional widget changes:

```bash
cd app
flutter test --update-goldens test/widgets/flag_badge_golden_test.dart
```

**Warning:** Never run `--update-goldens` in CI. Baselines must be reviewed and committed.

## Guarantees

- **Deterministic:** Fonts and animations are pinned so output is byte-stable
- **Platform-independent:** Screenshots are generated on stable CI environment only
- **Regression detection:** Any unintended change to widget appearance fails the test
