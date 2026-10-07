"""
Survivorship completeness: Retain delisted/suspended entities in IC sample.

Phase 6 mandate 25: The IC sample retains delisted/suspended names via entity_ref validity ranges;
a test shows dropping non-survivors inflates IC and the harness does not.
"""

import logging
from datetime import datetime, date

import pandas as pd

logger = logging.getLogger("ic_gate.survivorship")


def retain_delisted_entities(
    metric_df: pd.DataFrame,
    entity_ref_df: pd.DataFrame = None
) -> pd.DataFrame:
    """
    Retain delisted and suspended entities in the IC sample.

    Arguments:
        metric_df: DataFrame with [date, entity_id, metric_value, ...]
        entity_ref_df: DataFrame with [entity_id, valid_from, valid_to]
                       (valid_to = NULL means still active; set on delist/suspend)

    Returns:
        Filtered DataFrame that includes delisted entities (within their validity ranges)
    """
    try:
        if entity_ref_df is None or len(entity_ref_df) == 0:
            logger.warning("  ⊘ No entity_ref data available; proceeding with all entities")
            return metric_df

        logger.info(f"  → Applying survivorship filter (retaining delisted entities)")

        # Filter metric_df to rows within each entity's validity range
        valid_rows = []

        for _, entity_row in entity_ref_df.iterrows():
            entity_id = entity_row["entity_id"]
            valid_from = pd.to_datetime(entity_row["valid_from"]).date()
            valid_to = entity_row["valid_to"]
            if pd.notna(valid_to):
                valid_to = pd.to_datetime(valid_to).date()
            else:
                valid_to = date.max  # Still active

            # Get all metric rows for this entity
            entity_metrics = metric_df[metric_df["entity_id"] == entity_id].copy()

            if len(entity_metrics) == 0:
                continue

            # Filter to within validity range
            entity_metrics["date_only"] = pd.to_datetime(entity_metrics["date"]).dt.date
            entity_metrics = entity_metrics[
                (entity_metrics["date_only"] >= valid_from) &
                (entity_metrics["date_only"] <= valid_to)
            ]
            valid_rows.append(entity_metrics.drop(columns=["date_only"]))

        result = pd.concat(valid_rows, ignore_index=True) if valid_rows else metric_df

        logger.info(
            f"  ✓ Retained {len(result)} rows across {len(entity_ref_df)} entities "
            f"(including delisted/suspended within their windows)"
        )
        return result

    except Exception as e:
        logger.error(f"Error in survivorship filter: {e}")
        return metric_df


def check_survivorship_bias(
    ic_with_survivors: float,
    ic_with_all_entities: float,
    p_value_survivors: float,
    p_value_all: float
) -> dict:
    """
    Test mandate 25: Verify that dropping non-survivors inflates IC.

    If the IC computed on survivors-only (alive entities) is significantly higher
    than the IC on all entities (including delisted), that indicates survivorship bias.

    Arguments:
        ic_with_survivors: IC computed on active entities only
        ic_with_all_entities: IC computed on all entities (including delisted)
        p_value_survivors: P-value for survivors-only IC
        p_value_all: P-value for all-entities IC

    Returns:
        dict with assessment results
    """
    try:
        ic_diff = ic_with_survivors - ic_with_all_entities
        pct_diff = (ic_diff / abs(ic_with_all_entities) * 100) if ic_with_all_entities != 0 else 0

        # Assessment
        assessment = {
            "ic_survivors": ic_with_survivors,
            "ic_all": ic_with_all_entities,
            "ic_difference": ic_diff,
            "pct_difference": pct_diff,
            "has_bias": abs(ic_diff) > 0.05,  # Threshold for meaningful bias
            "p_value_survivors": p_value_survivors,
            "p_value_all": p_value_all,
        }

        if assessment["has_bias"]:
            logger.warning(
                f"  ⊘ Potential survivorship bias detected: "
                f"survivors IC={ic_with_survivors:.3f} vs all IC={ic_with_all_entities:.3f} "
                f"(diff={ic_diff:.3f}, {pct_diff:.1f}%)"
            )
        else:
            logger.info(
                f"  ✓ Survivorship bias check passed: "
                f"survivors IC={ic_with_survivors:.3f} vs all IC={ic_with_all_entities:.3f} "
                f"(diff={ic_diff:.3f})"
            )

        return assessment

    except Exception as e:
        logger.error(f"Error checking survivorship bias: {e}")
        return {"error": str(e)}


def get_entity_validity_ranges(pg_conn) -> pd.DataFrame:
    """
    Fetch entity_ref table with validity ranges (valid_from, valid_to).

    Returns:
        DataFrame with [entity_id, entity_type, valid_from, valid_to]
    """
    try:
        query = """
        SELECT
            entity_id,
            entity_type,
            valid_from,
            valid_to
        FROM entity_ref
        ORDER BY entity_id, valid_from
        """

        df = pd.read_sql(query, pg_conn)
        logger.info(f"  ✓ Fetched validity ranges for {len(df)} entities")
        return df

    except Exception as e:
        logger.error(f"Error fetching entity_ref: {e}")
        return None
