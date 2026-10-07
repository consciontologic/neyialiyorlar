#!/usr/bin/env python3
"""
IC Gate: Phase 6 Information-Coefficient (IC) metric promotion harness.

Purpose:
- Joins indicator series with forward returns
- Computes Spearman rank IC + rolling IC + sign-stability check
- Writes GO/NO-GO/PENDING decisions to promotion_ledger
- Enforces anti-data-snooping: all thresholds pre-registered in config

Usage:
  make ic.run METRIC=nimvi         # Run IC gate for single metric
  make ic.run.all                  # Run for all 10 metrics
"""

# CRITICAL: Setup paths FIRST
import sys
import os
from pathlib import Path

# Ensure script directory is in sys.path for local imports (returns, utils, ic_gate)
_script_dir = str(Path(__file__).parent.absolute())
if _script_dir not in sys.path:
    sys.path.insert(0, _script_dir)

# Add calendar module path BEFORE importing 3rd-party modules
# Docker location: /app/calendar (need /app in sys.path to import 'calendar' package)
# Dev location: ./calendar (need script dir in sys.path)
_calendar_parent = '/app'
if os.path.exists(_calendar_parent + '/calendar'):
    if _calendar_parent not in sys.path:
        sys.path.insert(1, _calendar_parent)
        print(f"[DEBUG] Added {_calendar_parent} to sys.path for calendar module", file=sys.stderr)
elif os.path.exists('./calendar'):
    print("[DEBUG] Found ./calendar, using script dir", file=sys.stderr)
    pass  # Already in sys.path via _script_dir

import argparse
import logging
from datetime import datetime, timedelta
import json

# CRITICAL: Import bist_calendar BEFORE pandas (pandas imports stdlib calendar during init)
# This ensures our local bist_calendar package doesn't shadow stdlib, while still being imported early
from bist_calendar.bist import BistCalendar
from returns.forward import ForwardReturnsBuilder

# Now safe to import 3rd-party modules
import yaml
import psycopg
import redis
import pandas as pd
import numpy as np
from scipy import stats
from dateutil import tz

# Local modules
from ic_gate.core import (
    load_config,
    validate_ic_gates,
    compute_ic,
    fetch_metric_series,
    fetch_forward_returns,
    write_promotion_ledger,
)
from utils.db import connect_postgres, connect_redis
from utils.identity import join_on_stable_entity_id, check_ticker_rename_not_split
from utils.survivorship import retain_delisted_entities, get_entity_validity_ranges


# ──────────────────────────────────────────────────────────────
# Logging setup
# ──────────────────────────────────────────────────────────────

LOG_LEVEL = os.getenv("IC_GATE_LOG_LEVEL", "INFO")
logging.basicConfig(
    level=LOG_LEVEL,
    format="%(asctime)s [%(name)s] %(levelname)s: %(message)s",
    handlers=[logging.StreamHandler(sys.stdout)],
)
logger = logging.getLogger("ic_gate")


# ──────────────────────────────────────────────────────────────
# Main harness
# ──────────────────────────────────────────────────────────────


def run_ic_gate(metric_key: str, config_path: str = None, dry_run: bool = False):
    """
    Run IC gate for a single metric.

    Args:
        metric_key: Metric identifier (e.g., "nimvi") or "all" for all 10 metrics
        config_path: Path to neyialiyorlar.yaml (default: /app/config/neyialiyorlar.yaml)
        dry_run: If True, log but do not write to promotion_ledger

    Returns:
        dict: Gate decision {metric: str, status: str, ic: float, n: int, decision: str, reason: str}
    """

    config_path = config_path or "/app/config/neyialiyorlar.yaml"
    dry_run = dry_run or os.getenv("IC_GATE_DRY_RUN", "false").lower() == "true"

    logger.info(
        f"Starting IC gate: metric={metric_key}, config={config_path}, dry_run={dry_run}"
    )

    # Load config + validate anti-snooping gates
    try:
        config = load_config(config_path)
        validate_ic_gates(config)
        logger.info("✓ Config loaded + anti-snooping gates validated")
    except Exception as e:
        logger.error(f"✗ Config validation failed: {e}")
        return {"status": "FAILED", "reason": str(e)}

    # Connect to databases
    try:
        pg_conn = connect_postgres()
        redis_conn = connect_redis(optional=True)
        logger.info("✓ Connected to PostgreSQL + Redis")
    except Exception as e:
        logger.error(f"✗ Database connection failed: {e}")
        return {"status": "FAILED", "reason": str(e)}

    # Expand metric list
    if metric_key.lower() == "all":
        # config["metrics"] is a list of dicts with 'key' field
        metrics = [m["key"] for m in config["metrics"] if isinstance(m, dict) and "key" in m]
        logger.info(f"Running gate for all {len(metrics)} metrics: {metrics}")
    else:
        metrics = [metric_key]

    results = []

    for metric in metrics:
        try:
            logger.info(f"\n→ Processing metric: {metric}")

            # Fetch metric series from metric_value table
            metric_df = fetch_metric_series(pg_conn, metric, config)
            if metric_df is None or len(metric_df) == 0:
                logger.warning(f"  ✗ No data for {metric}")
                results.append(
                    {
                        "metric": metric,
                        "status": "PENDING",
                        "reason": "insufficient_data",
                    }
                )
                continue

            # Fetch forward returns
            forward_returns_df = fetch_forward_returns(pg_conn, config)
            if forward_returns_df is None or len(forward_returns_df) == 0:
                logger.warning(f"  ✗ No forward returns data")
                results.append(
                    {
                        "metric": metric,
                        "status": "PENDING",
                        "reason": "forward_returns_unavailable",
                    }
                )
                continue

            # Compute IC
            ic_result = compute_ic(metric_df, forward_returns_df, metric, config)

            # Write to promotion_ledger (unless dry_run)
            if not dry_run:
                promotion_id = write_promotion_ledger(pg_conn, metric, ic_result)
                logger.info(f"  ✓ Promotion ledger entry: {promotion_id}")
            else:
                logger.info(f"  [DRY_RUN] Would write promotion ledger entry")

            results.append(ic_result)

        except Exception as e:
            logger.error(f"  ✗ Error processing {metric}: {e}", exc_info=True)
            results.append(
                {
                    "metric": metric,
                    "status": "FAILED",
                    "reason": str(e),
                }
            )

    # Summary
    logger.info("\n" + "=" * 70)
    logger.info("IC GATE SUMMARY")
    logger.info("=" * 70)
    for r in results:
        status_icon = "✓" if r["status"] == "GO" else ("⊘" if r["status"] == "PENDING" else "✗")
        ic_val = f"{r.get('ic', '?'):.3f}" if isinstance(r.get('ic'), (int, float)) else str(r.get('ic', '?'))
        n_val = r.get('n', '?')
        logger.info(
            f"{status_icon} {r.get('metric', 'unknown'):25s} → {r['status']:10s} "
            f"(IC={ic_val}, n={n_val})"
        )

    pg_conn.close()
    if redis_conn:
        redis_conn.close()

    return results


# ──────────────────────────────────────────────────────────────
# CLI
# ──────────────────────────────────────────────────────────────


def main():
    parser = argparse.ArgumentParser(
        description="IC Gate: Information-Coefficient metric promotion harness"
    )
    parser.add_argument(
        "--metric",
        default="all",
        help="Metric key (e.g., 'nimvi', 'velocity_accumulation') or 'all' [default: all]",
    )
    parser.add_argument(
        "--config",
        default="/app/config/neyialiyorlar.yaml",
        help="Path to config file [default: /app/config/neyialiyorlar.yaml]",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Log but do not write to promotion_ledger",
    )

    args = parser.parse_args()

    results = run_ic_gate(
        metric_key=args.metric, config_path=args.config, dry_run=args.dry_run
    )

    # Exit code: 0 if all GO/PENDING, 1 if any FAILED
    exit_code = 0
    for r in results:
        if isinstance(r, dict) and r.get("status") == "FAILED":
            exit_code = 1
            break

    sys.exit(exit_code)


if __name__ == "__main__":
    main()
