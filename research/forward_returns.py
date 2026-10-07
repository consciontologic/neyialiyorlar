#!/usr/bin/env python3
"""
Forward Returns Builder: Compute price_return:<horizon> metrics and write to metric_value table.

Phase 6 mandate 12: Build forward-return dataset as the matched RHS of IC joins.

Usage:
  make forward-returns HORIZON=5     # Compute 5-day forward returns
  make forward-returns HORIZON=all   # Compute all horizons (5, 20, etc.)
"""

import argparse
import logging
import os
import sys
from pathlib import Path
from datetime import datetime, timedelta

import pandas as pd
import psycopg
import yaml

sys.path.insert(0, str(Path(__file__).parent))

from utils.db import connect_postgres
from bist_calendar.bist import BistCalendar
from returns.forward import ForwardReturnsBuilder

logger = logging.getLogger("forward_returns")
logging.basicConfig(
    level=os.getenv("IC_GATE_LOG_LEVEL", "INFO"),
    format="%(asctime)s [%(name)s] %(levelname)s: %(message)s",
)


def compute_and_write_forward_returns(
    horizons: list = None, config_path: str = None, dry_run: bool = False
):
    """
    Compute forward returns and write to metric_value table.

    Args:
        horizons: List of horizon days (e.g., [5, 20]) or None for defaults
        config_path: Path to config file
        dry_run: If True, log but do not write

    Returns:
        dict with results
    """

    config_path = config_path or "/app/config/neyialiyorlar.yaml"
    horizons = horizons or [5, 20]  # Default horizons
    dry_run = dry_run or os.getenv("IC_GATE_DRY_RUN", "false").lower() == "true"

    logger.info(
        f"Computing forward returns: horizons={horizons}, dry_run={dry_run}"
    )

    try:
        # Load config
        with open(config_path, "r") as f:
            config = yaml.safe_load(f)

        # Connect to DB
        pg_conn = connect_postgres()

        # Initialize calendar + builder
        calendar = BistCalendar()
        builder = ForwardReturnsBuilder(calendar)

        # Fetch prices
        prices = builder.fetch_prices_from_db(pg_conn, lookback_days=365)
        if prices is None or len(prices) == 0:
            logger.error("✗ No price data available")
            return {"all": {"status": "FAILED", "reason": "no_price_data"}}

        results = {}

        # Compute returns for each horizon
        for horizon in horizons:
            try:
                logger.info(f"\n→ Computing {horizon}-day forward returns")

                returns_df = builder.compute_returns(
                    prices,
                    horizon_days=horizon,
                    return_type="log"
                )

                if returns_df is None or len(returns_df) == 0:
                    logger.warning(f"  ✗ No returns computed for {horizon}d horizon")
                    results[horizon] = {"status": "FAILED", "reason": "no_returns"}
                    continue

                # Write to metric_value table
                if not dry_run:
                    n_written = write_forward_returns_to_db(
                        pg_conn, returns_df, horizon
                    )
                    logger.info(
                        f"  ✓ Wrote {n_written} forward return points (horizon={horizon}d)"
                    )
                    results[horizon] = {
                        "status": "OK",
                        "n_written": n_written,
                    }
                else:
                    logger.info(
                        f"  [DRY_RUN] Would write {len(returns_df)} points (horizon={horizon}d)"
                    )
                    results[horizon] = {"status": "DRY_RUN", "n_rows": len(returns_df)}

            except Exception as e:
                logger.error(f"  ✗ Error for horizon {horizon}d: {e}", exc_info=True)
                results[horizon] = {"status": "FAILED", "reason": str(e)}

        pg_conn.close()

        # Summary
        logger.info("\n" + "=" * 70)
        logger.info("FORWARD RETURNS SUMMARY")
        logger.info("=" * 70)
        for horizon, result in results.items():
            status_icon = "✓" if result["status"] in ["OK", "DRY_RUN"] else "✗"
            logger.info(
                f"{status_icon} {horizon}d horizon: {result['status']} "
                f"({result.get('n_written', result.get('n_rows', '?'))} points)"
            )

        return results

    except Exception as e:
        logger.error(f"✗ Forward returns computation failed: {e}", exc_info=True)
        return {"status": "FAILED", "reason": str(e)}


def write_forward_returns_to_db(pg_conn, returns_df: pd.DataFrame, horizon_days: int) -> int:
    """
    Write forward returns to metric_value table.

    Args:
        pg_conn: PostgreSQL connection
        returns_df: DataFrame with [date, entity_id, forward_return, horizon_days]
        horizon_days: Horizon in BIST trading days

    Returns:
        Number of rows written
    """

    try:
        metric_key = f"price_return:{horizon_days}d"

        # Convert to metric_value format
        # ts = date (use start of day as observation time)
        # entity = entity_id
        # value = forward_return
        # flag = 'fresh' (computed returns)
        # tier = 'daily'
        # inputs_hash = placeholder

        with pg_conn.cursor() as cur:
            for _, row in returns_df.iterrows():
                ts = pd.to_datetime(row["date"]).replace(hour=0, minute=0, second=0)
                entity = row["entity_id"]
                value = row["forward_return"]
                inputs_hash = b"placeholder"  # TODO: compute from input prices

                query = """
                INSERT INTO metric_value
                (metric_key, entity, ts, value, flag, tier, inputs_hash, computed_at)
                VALUES (%s, %s, %s, %s, 'fresh', 'daily', %s, NOW())
                ON CONFLICT (metric_key, entity, ts, inputs_hash) DO NOTHING
                """

                cur.execute(query, [metric_key, entity, ts, value, inputs_hash])

            pg_conn.commit()
            n_written = cur.rowcount

        logger.info(f"  ✓ Written {n_written} rows for metric_key={metric_key}")
        return n_written

    except Exception as e:
        logger.error(f"Error writing forward returns: {e}")
        raise


def main():
    parser = argparse.ArgumentParser(
        description="Compute forward returns and write to metric_value table"
    )
    parser.add_argument(
        "--horizons",
        default="5,20",
        help="BIST trading day horizons (comma-separated) [default: 5,20]",
    )
    parser.add_argument(
        "--config",
        default="/app/config/neyialiyorlar.yaml",
        help="Path to config file",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Log but do not write",
    )

    args = parser.parse_args()

    horizons = [int(h.strip()) for h in args.horizons.split(",")]
    results = compute_and_write_forward_returns(
        horizons=horizons,
        config_path=args.config,
        dry_run=args.dry_run,
    )

    # Exit code: 0 if all OK/DRY_RUN, 1 if any FAILED
    exit_code = 0
    for result in results.values() if isinstance(results, dict) else []:
        if result.get("status") == "FAILED":
            exit_code = 1
            break

    sys.exit(exit_code)


if __name__ == "__main__":
    main()
