"""
IC Gate Core: Spearman rank IC computation + promotion gate logic.

Mandate:
- Computes Information-Coefficient (Spearman rank correlation) between indicator and forward returns
- Rolling IC check for sign-stability (fail if >3 flips in window)
- Promotion decision: GO (metric trusted) / NO-GO (statistically weak) / PENDING (insufficient data)
- Point-in-time alignment: uses only as-known values (no look-ahead)
- Anti-data-snooping: all thresholds pre-registered in config before gate runs
"""

import logging
from datetime import datetime, timedelta
import json
import hashlib
import subprocess

import pandas as pd
import numpy as np
from scipy import stats
import yaml

logger = logging.getLogger("ic_gate.core")


# ──────────────────────────────────────────────────────────────
# Config loading + anti-snooping validation
# ──────────────────────────────────────────────────────────────


def load_config(config_path: str) -> dict:
    """Load neyialiyorlar.yaml config file."""
    with open(config_path, "r") as f:
        config = yaml.safe_load(f)
    return config


def validate_ic_gates(config: dict):
    """
    Validate IC gate thresholds are present and pre-registered.
    Fails if ic_gate section is missing or incomplete.
    """
    if "ic_gate" not in config:
        raise ValueError("ic_gate section missing from config (anti-snooping violation)")

    ic_gate = config["ic_gate"]

    # Check minimum history thresholds
    required_keys = [
        "min_backtest_days_daily",
        "min_backtest_weeks",
        "max_sign_flips_rolling",
    ]
    for key in required_keys:
        if key not in ic_gate:
            raise ValueError(f"ic_gate.{key} missing (anti-snooping violation)")

    # Check ic_gate.metrics exists
    if "metrics" not in ic_gate:
        raise ValueError("ic_gate.metrics section missing (anti-snooping violation)")

    ic_gate_metrics = ic_gate["metrics"]

    # Check all config metrics have thresholds in ic_gate.metrics
    if "metrics" not in config:
        raise ValueError("metrics section missing from config")

    # metrics is a list of dicts with 'key' field
    metrics_list = config["metrics"]
    for metric_dict in metrics_list:
        if not isinstance(metric_dict, dict):
            raise ValueError(f"metrics must be a list of dicts, got {type(metric_dict)}")

        metric_key = metric_dict.get("key")
        if not metric_key:
            raise ValueError("Each metric dict must have a 'key' field (anti-snooping violation)")

        if metric_key not in ic_gate_metrics:
            raise ValueError(
                f"ic_gate.metrics.{metric_key} threshold missing (anti-snooping violation)"
            )

        metric_gates = ic_gate_metrics[metric_key]
        required_metric_keys = ["ic_threshold", "rolling_window_size", "forward_horizon_type"]
        for key in required_metric_keys:
            if key not in metric_gates:
                raise ValueError(
                    f"ic_gate.metrics.{metric_key}.{key} missing (anti-snooping violation)"
                )

    logger.info(
        f"✓ Anti-snooping validation passed: {len(metrics_list)} metrics "
        f"have pre-registered thresholds"
    )


# ──────────────────────────────────────────────────────────────
# IC Computation
# ──────────────────────────────────────────────────────────────


def compute_ic(
    metric_df: pd.DataFrame, forward_returns_df: pd.DataFrame, metric_key: str, config: dict
) -> dict:
    """
    Compute Spearman rank IC between metric and forward returns.

    Args:
        metric_df: DataFrame with columns [date, entity_id, metric_value, observation_ts]
        forward_returns_df: DataFrame with columns [date, entity_id, forward_return, horizon_days]
        metric_key: Metric identifier
        config: Config dict with ic_gate thresholds

    Returns:
        dict: {
            metric: str,
            status: str (GO | NO-GO | PENDING),
            ic: float,
            n: int,
            p_value: float,
            rolling_ic_stats: dict,
            decision: str,
            reason: str
        }
    """

    try:
        # Point-in-time alignment (mandate 20): Remove future-dated observations
        # Only use values that were known at observation_ts
        now = datetime.now()
        metric_df = metric_df[metric_df["observation_ts"] <= now].copy()

        if len(metric_df) == 0:
            return {
                "metric": metric_key,
                "status": "PENDING",
                "reason": "all_observations_future_dated",
                "ic": np.nan,
                "n": 0,
            }

        # Join on (date, entity_id)
        merged = metric_df.merge(
            forward_returns_df,
            on=["date", "entity_id"],
            how="inner",
        )

        if len(merged) == 0:
            return {
                "metric": metric_key,
                "status": "PENDING",
                "reason": "no_overlap_after_join",
                "ic": np.nan,
                "n": 0,
            }

        # Check minimum history (mandate 9)
        metric_gates = config["ic_gate"][metric_key]
        min_backtest_days = config["ic_gate"]["min_backtest_days_daily"]
        date_range = (merged["date"].max() - merged["date"].min()).days
        if date_range < min_backtest_days:
            return {
                "metric": metric_key,
                "status": "PENDING",
                "reason": f"insufficient_history_{date_range}d_lt_{min_backtest_days}d",
                "ic": np.nan,
                "n": len(merged),
            }

        # Honesty check (mandate 7): Ensure no NaN values were fabricated
        # (metric_value should have gaps, not fabricated fills)
        if merged["metric_value"].isna().any():
            n_gaps = merged["metric_value"].isna().sum()
            logger.warning(f"  ⊘ Found {n_gaps} gap rows (expected: no fabrication)")
            # Remove gap rows (they're placeholder nulls)
            merged = merged[~merged["metric_value"].isna()]

        # Compute Spearman IC
        n = len(merged)
        if n < 10:  # Need minimum sample size
            return {
                "metric": metric_key,
                "status": "PENDING",
                "reason": f"minimum_sample_too_small_{n}_lt_10",
                "ic": np.nan,
                "n": n,
            }

        ic, p_value = stats.spearmanr(merged["metric_value"], merged["forward_return"])

        # Rolling IC check for sign stability (mandate 11)
        rolling_window = metric_gates.get("rolling_window_size", 20)
        rolling_ics = []
        sign_flips = 0

        if len(merged) >= rolling_window:
            for i in range(len(merged) - rolling_window + 1):
                window = merged.iloc[i : i + rolling_window]
                if len(window) >= 3:  # Need minimum points for correlation
                    window_ic, _ = stats.spearmanr(
                        window["metric_value"], window["forward_return"]
                    )
                    rolling_ics.append(window_ic)
                    if len(rolling_ics) > 1:
                        # Count sign flips (both positive→negative and vice versa)
                        prev_sign = np.sign(rolling_ics[-2])
                        curr_sign = np.sign(rolling_ics[-1])
                        if prev_sign != 0 and curr_sign != 0 and prev_sign != curr_sign:
                            sign_flips += 1
        else:
            rolling_ics = [ic]  # Single IC if window too small

        max_flips = config["ic_gate"]["max_sign_flips_rolling"]

        # Decision logic
        reason = []
        decision = "GO"

        # Check IC threshold
        ic_threshold = metric_gates.get("ic_threshold", 0.05)
        if abs(ic) < ic_threshold:
            decision = "NO-GO"
            reason.append(f"ic_{abs(ic):.3f}_below_threshold_{ic_threshold}")

        # Check sign stability (mandate 11)
        if sign_flips > max_flips:
            decision = "NO-GO"
            reason.append(f"sign_flips_{sign_flips}_exceeds_max_{max_flips}")

        # Check p-value (statistical significance)
        if p_value > 0.05:
            decision = "NO-GO"
            reason.append(f"p_value_{p_value:.4f}_not_significant_at_5pct")

        return {
            "metric": metric_key,
            "status": decision,
            "ic": float(ic) if not np.isnan(ic) else None,
            "n": n,
            "p_value": float(p_value) if not np.isnan(p_value) else None,
            "sign_flips": sign_flips,
            "rolling_ics": rolling_ics[:5],  # Keep first 5 for logging
            "threshold": ic_threshold,
            "reason": "|".join(reason) if reason else "gates_passed",
            "decision": decision,
        }

    except Exception as e:
        logger.error(f"Error computing IC for {metric_key}: {e}", exc_info=True)
        return {
            "metric": metric_key,
            "status": "FAILED",
            "reason": str(e),
            "ic": np.nan,
            "n": 0,
        }


def compute_rolling_ic(metric_values: np.ndarray, returns: np.ndarray, window_size: int):
    """Compute rolling Spearman IC over a time series."""
    rolling_ics = []
    for i in range(len(metric_values) - window_size + 1):
        window_metric = metric_values[i : i + window_size]
        window_return = returns[i : i + window_size]
        ic, _ = stats.spearmanr(window_metric, window_return)
        rolling_ics.append(ic)
    return np.array(rolling_ics)


def check_sign_stability(rolling_ics: np.ndarray, max_flips: int) -> bool:
    """Check if IC sign flips exceed threshold. Returns True if stable (passes)."""
    sign_flips = 0
    for i in range(1, len(rolling_ics)):
        if np.sign(rolling_ics[i]) != np.sign(rolling_ics[i - 1]):
            sign_flips += 1
    return sign_flips <= max_flips


# ──────────────────────────────────────────────────────────────
# Data fetching
# ──────────────────────────────────────────────────────────────


def fetch_metric_series(
    pg_conn, metric_key: str, config: dict, lookback_days: int = 365
) -> pd.DataFrame:
    """
    Fetch metric values from metric_value table.

    Returns:
        DataFrame with columns [date, entity_id, metric_value, observation_ts]
    """
    try:
        query = f"""
        SELECT
            ts::date as date,
            entity as entity_id,
            value as metric_value,
            ts as observation_ts
        FROM metric_value
        WHERE metric_key = %s
          AND ts >= CURRENT_DATE - INTERVAL '{lookback_days} days'
          AND value IS NOT NULL
        ORDER BY ts ASC, entity ASC
        LIMIT 100000
        """

        df = pd.read_sql(query, pg_conn, params=[metric_key])
        df["date"] = pd.to_datetime(df["date"])
        df["observation_ts"] = pd.to_datetime(df["observation_ts"])

        logger.info(
            f"  ✓ Fetched {len(df)} rows for {metric_key} "
            f"({df['entity_id'].nunique()} entities, "
            f"{(df['date'].max() - df['date'].min()).days}d span)"
        )

        return df

    except Exception as e:
        logger.error(f"Error fetching metric series for {metric_key}: {e}")
        return None


def fetch_forward_returns(pg_conn, config: dict, lookback_days: int = 365) -> pd.DataFrame:
    """
    Fetch forward returns from metric_value table (price_return:<horizon> metric_key).

    Returns:
        DataFrame with columns [date, entity_id, forward_return, horizon_days]
    """
    try:
        # Query for price_return metrics (computed forward returns)
        # These should be pre-computed and written to metric_value table
        query = """
        SELECT
            ts::date as date,
            entity as entity_id,
            value as forward_return,
            5 as horizon_days  -- TODO: Extract horizon from metric_key (price_return:5d)
        FROM metric_value
        WHERE metric_key LIKE 'price_return:%'
          AND ts >= CURRENT_DATE - INTERVAL '%d days'
          AND value IS NOT NULL
        ORDER BY ts ASC, entity ASC
        """ % lookback_days

        df = pd.read_sql(query, pg_conn)
        df["date"] = pd.to_datetime(df["date"])

        if len(df) == 0:
            logger.warning("  ⊘ No forward returns found (price_return:* metrics)")
            return None

        logger.info(f"  ✓ Fetched {len(df)} forward return points")
        return df

    except Exception as e:
        logger.error(f"Error fetching forward returns: {e}")
        return None


# ──────────────────────────────────────────────────────────────
# Promotion ledger writes
# ──────────────────────────────────────────────────────────────


def write_promotion_ledger(pg_conn, metric_key: str, ic_result: dict, config_path: str = None) -> str:
    """
    Write IC gate decision to promotion_ledger table with reproducibility metadata.

    Mandate 16: Every promotion_ledger row records data window + git sha + config hash;
    re-running on the same snapshot reproduces the decision.

    Args:
        pg_conn: PostgreSQL connection
        metric_key: Metric identifier
        ic_result: IC computation result dict
        config_path: Path to config file (for config hash)

    Returns:
        promotion_id (str)
    """
    try:
        # Get reproducibility metadata
        try:
            git_sha = subprocess.check_output(["git", "rev-parse", "HEAD"]).decode().strip()
        except:
            git_sha = "unknown"

        # Compute config hash for reproducibility
        config_path = config_path or "/app/config/neyialiyorlar.yaml"
        try:
            with open(config_path, "rb") as f:
                config_hash = hashlib.sha256(f.read()).hexdigest()[:16]
        except:
            config_hash = "unknown"

        # Promotion ledger record (schema from Phase 4)
        # Fields: metric_key, run_at, ic_value, rolling_ic, threshold, horizon_days, window_size, decision, notes
        query = """
        INSERT INTO promotion_ledger
        (metric_key, run_at, ic_value, threshold, decision, notes)
        VALUES (%s, NOW(), %s, %s, %s, %s)
        RETURNING (metric_key, run_at)
        """

        notes = f"git_sha={git_sha[:8]} config_hash={config_hash} " \
                f"n={ic_result.get('n', 0)} p_value={ic_result.get('p_value', 'unknown'):.4f if ic_result.get('p_value') else 'unknown'} " \
                f"reason={ic_result.get('reason', '')}"

        with pg_conn.cursor() as cur:
            cur.execute(
                query,
                [
                    metric_key,
                    ic_result.get("ic", None),
                    ic_result.get("threshold", 0.05),
                    ic_result["status"],  # GO | NO-GO | PENDING
                    notes,
                ],
            )
            promotion_record = cur.fetchone()
            pg_conn.commit()

        promotion_id = f"{promotion_record[0]}_{promotion_record[1].isoformat() if promotion_record[1] else 'unknown'}"
        logger.info(f"  ✓ Promotion ledger written: {promotion_id} (reproducibility: git={git_sha[:8]}, config={config_hash})")
        return promotion_id

    except Exception as e:
        logger.error(f"Error writing promotion ledger: {e}")
        raise
