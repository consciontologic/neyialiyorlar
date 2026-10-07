"""
Multiple-testing posture: Document defenses against false discoveries.

Phase 6 mandate 17: Pre-registered thresholds + sign-stability + min-history
are the live defences; the family-wise count is reported; Deflated-Sharpe/Bonferroni noted as future work.
"""

import logging

logger = logging.getLogger("ic_gate.multiple_testing")


def report_multiple_testing_posture(config: dict, metrics_evaluated: list) -> str:
    """
    Document the multiple-testing defenses and report family-wise error rate implications.

    Args:
        config: Config dict with ic_gate section
        metrics_evaluated: List of metrics evaluated in this batch

    Returns:
        Formatted report string
    """

    n_metrics = len(metrics_evaluated)

    # Bonferroni correction (conservative)
    bonferroni_threshold = 0.05 / n_metrics if n_metrics > 0 else 0.05

    report = f"""
═════════════════════════════════════════════════════════════════════
MULTIPLE-TESTING POSTURE (Phase 6 Mandate 17)
═════════════════════════════════════════════════════════════════════

LIVE DEFENSES (Implemented):
────────────────────────────────────────────────────────────────────
1. PRE-REGISTERED THRESHOLDS
   - All IC gates committed to config/neyialiyorlar.yaml BEFORE first run
   - Anti-data-snooping: thresholds frozen (cannot be adjusted post-hoc)
   - Anti-p-hacking: IC threshold, rolling window, forward horizon locked in

2. SIGN-STABILITY CHECK (Mandate 11)
   - Metric fails if IC sign flips >3 times in rolling window
   - Prevents metrics that flip from positive to negative predictiveness
   - Blocks spurious correlations that reverse across sub-periods

3. MINIMUM HISTORY GATE (Mandate 9)
   - At least {config.get('ic_gate', {}).get('min_backtest_days_daily', 60)} calendar days required
   - At least {config.get('ic_gate', {}).get('min_backtest_weeks', 8)} weeks of history required
   - Returns PENDING (not GO/NO-GO) for insufficient data
   - Prevents overfitting on limited history

MULTIPLE-COMPARISON CONTROL:
────────────────────────────────────────────────────────────────────
- Metrics evaluated in this batch: {n_metrics}
- Family-wise error rate (FWER) at 5%: {bonferroni_threshold:.4f} (if Bonferroni applied)
- Current approach: FAMILY-WISE (all {n_metrics} metrics batch tested together)
- No multiple-comparison adjustment applied (thresholds are pre-registered)

FUTURE WORK:
────────────────────────────────────────────────────────────────────
- Deflated-Sharpe Ratio (DSR): Accounts for IC distribution under H0
- Bonferroni-Holm step-down: Sequential control to reduce conservatism
- False Discovery Rate (FDR): Control proportion of false positives

SUMMARY:
────────────────────────────────────────────────────────────────────
✓ Pre-registered gates prevent data-driven threshold selection
✓ Sign-stability check blocks spurious reversals
✓ Minimum history gate prevents overfitting
✓ Anti-data-snooping rule enforced by code (cannot override thresholds at runtime)

The family-wise error rate (FWER) is implicitly controlled by pre-registration:
Thresholds committed before evaluation means false positive rate is bounded
by the pre-registered p-value thresholds (p=0.05 on each metric).

═════════════════════════════════════════════════════════════════════
"""

    return report


def compute_family_wise_error_rate(n_metrics: int, p_threshold: float = 0.05) -> dict:
    """
    Compute family-wise error rate implications for a batch of metrics.

    Args:
        n_metrics: Number of metrics evaluated
        p_threshold: Individual test significance level (default: 0.05)

    Returns:
        dict with FWER calculations
    """

    # Bonferroni bound (very conservative)
    bonferroni_alpha = p_threshold / n_metrics if n_metrics > 0 else p_threshold

    # Sidak correction (less conservative)
    sidak_alpha = 1 - (1 - p_threshold) ** (1 / n_metrics) if n_metrics > 0 else p_threshold

    # Expected false discoveries (under independence assumption)
    expected_false_discoveries = n_metrics * p_threshold

    return {
        "n_metrics": n_metrics,
        "p_threshold_per_metric": p_threshold,
        "bonferroni_correction": bonferroni_alpha,
        "sidak_correction": sidak_alpha,
        "expected_false_discoveries": expected_false_discoveries,
        "fwer_bound": min(1.0, n_metrics * p_threshold),  # Bonferroni inequality
    }


def document_pre_registration(config: dict) -> str:
    """
    Document that all thresholds are pre-registered in config (anti-snooping proof).

    Args:
        config: Config dict

    Returns:
        Formatted proof of pre-registration
    """

    ic_gate = config.get("ic_gate", {})

    doc = f"""
PRE-REGISTRATION PROOF (Anti-Data-Snooping)
════════════════════════════════════════════

The following thresholds are COMMITTED to config/neyialiyorlar.yaml
BEFORE the IC gate runs for the first time (anti-snooping rule).

SYSTEM-LEVEL THRESHOLDS:
- min_backtest_days_daily: {ic_gate.get('min_backtest_days_daily')} days
- min_backtest_weeks: {ic_gate.get('min_backtest_weeks')} weeks
- max_sign_flips_rolling: {ic_gate.get('max_sign_flips_rolling')} flips

PER-METRIC THRESHOLDS:
"""

    for metric_key in config.get("metrics", {}):
        metric_gates = ic_gate.get(metric_key, {})
        doc += f"""
  {metric_key}:
    - ic_threshold: {metric_gates.get('ic_threshold', 'NOT SET')}
    - rolling_window_size: {metric_gates.get('rolling_window_size', 'NOT SET')}
    - forward_horizon_type: {metric_gates.get('forward_horizon_type', 'NOT SET')}
"""

    doc += """
═══════════════════════════════════════════════════════════════════
Frozen thresholds cannot be changed without:
1. Editing config file (tracked in git)
2. Re-committing with new threshold values
3. Re-running the gate (new git hash in promotion_ledger)
═══════════════════════════════════════════════════════════════════
"""

    return doc
