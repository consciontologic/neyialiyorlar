"""Database connection utilities."""

import os
import logging
import psycopg
import redis

logger = logging.getLogger("utils.db")


def connect_postgres():
    """Connect to PostgreSQL using environment variables."""
    try:
        conn = psycopg.connect(
            host=os.getenv("POSTGRES_HOST", "postgres"),
            port=int(os.getenv("POSTGRES_PORT", 5432)),
            dbname=os.getenv("POSTGRES_DB", "neyialiyorlar"),
            user=os.getenv("POSTGRES_USER", "postgres"),
            password=os.getenv("POSTGRES_PASSWORD", "postgres"),
        )
        logger.info(
            f"✓ Connected to PostgreSQL: {os.getenv('POSTGRES_HOST')}:{os.getenv('POSTGRES_PORT')}/{os.getenv('POSTGRES_DB')}"
        )
        return conn
    except Exception as e:
        logger.error(f"✗ PostgreSQL connection failed: {e}")
        raise


def connect_redis(optional: bool = False):
    """Connect to Redis using environment variables. If optional=True, returns None on failure."""
    try:
        conn = redis.Redis(
            host=os.getenv("REDIS_HOST", "redis"),
            port=int(os.getenv("REDIS_PORT", 6379)),
            db=int(os.getenv("REDIS_DB", 0)),
            decode_responses=True,
        )
        conn.ping()
        logger.info(
            f"✓ Connected to Redis: {os.getenv('REDIS_HOST')}:{os.getenv('REDIS_PORT')}/{os.getenv('REDIS_DB')}"
        )
        return conn
    except Exception as e:
        if optional:
            logger.warning(f"⊘ Redis connection skipped (optional): {e}")
            return None
        else:
            logger.error(f"✗ Redis connection failed: {e}")
            raise


def write_promotion_ledger(
    pg_conn,
    metric_key: str,
    decision: str,
    ic: float,
    n: int,
    rolling_ic_stats: dict,
    threshold: float,
    reason: str,
    git_sha: str,
    config_hash: str,
    data_window_start: str,
    data_window_end: str,
    notes: str = "",
):
    """
    Write IC gate decision to promotion_ledger table.

    Args:
        pg_conn: PostgreSQL connection
        metric_key: Metric identifier (e.g., "nimvi")
        decision: GO | NO-GO | PENDING
        ic: Information coefficient value (Spearman rank correlation)
        n: Sample size (number of observation pairs)
        rolling_ic_stats: Dict with rolling IC summary {mean: float, std: float, min: float, max: float}
        threshold: Pre-registered IC threshold (from config)
        reason: Human-readable decision reason (pipe-separated if multiple)
        git_sha: Git commit SHA of config (for reproducibility)
        config_hash: SHA256 hash of IC gate config section (for reproducibility)
        data_window_start: ISO date string of first sample
        data_window_end: ISO date string of last sample
        notes: Optional notes (e.g., "threshold revised post-hoc" for protocol violations)
    """
    try:
        with pg_conn.cursor() as cur:
            cur.execute(
                """
                INSERT INTO promotion_ledger (
                    metric_key,
                    run_at,
                    decision,
                    ic,
                    sample_size,
                    rolling_ic_mean,
                    rolling_ic_std,
                    threshold,
                    reason,
                    git_sha,
                    config_hash,
                    data_window_start,
                    data_window_end,
                    notes
                ) VALUES (%s, NOW(), %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                """,
                (
                    metric_key,
                    decision,
                    ic,
                    n,
                    rolling_ic_stats.get("mean"),
                    rolling_ic_stats.get("std"),
                    threshold,
                    reason,
                    git_sha,
                    config_hash,
                    data_window_start,
                    data_window_end,
                    notes,
                ),
            )
        pg_conn.commit()
        logger.info(f"✓ Wrote promotion_ledger row: {metric_key} = {decision}")
    except Exception as e:
        logger.error(f"✗ Failed to write promotion_ledger: {e}")
        pg_conn.rollback()
        raise
