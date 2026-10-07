"""Forward Returns Dataset: price_return:<horizon> metric."""

import logging
from datetime import date, timedelta
from typing import Tuple, Optional, Dict

import pandas as pd
import numpy as np
from scipy import stats

logger = logging.getLogger("returns.forward")


class ForwardReturnsBuilder:
    """
    Build and validate forward returns dataset for IC gate input.

    Mandate 12: Forward-return dataset `price_return:<horizon>` derived from
    BIST raw (S1) via idempotent raw-then-derived path is the matched RHS
    of every IC join.

    Mandate 18: Horizons count BIST session days (not calendar days).

    Mandate 25: Corporate-action adjustment applies Phase-2 corporate_action
    factors before differencing.
    """

    def __init__(self, calendar):
        self.calendar = calendar

    def fetch_prices_from_db(
        self, pg_conn, lookback_days: int = 365
    ) -> pd.DataFrame:
        """
        Fetch BIST close prices from metric_value table (S1 data).

        Returns:
            DataFrame [date, entity_id, close_price]
        """
        try:
            query = """
            SELECT
                ts::date as date,
                entity as entity_id,
                value as close_price
            FROM metric_value
            WHERE metric_key = 'price_close_unadjusted'
              AND ts >= CURRENT_DATE - INTERVAL '%d days'
              AND value IS NOT NULL
              AND flag = 'fresh'
            ORDER BY entity_id ASC, ts ASC
            """ % lookback_days

            df = pd.read_sql(query, pg_conn)
            logger.info(f"✓ Fetched {len(df)} price points for forward returns")
            return df
        except Exception as e:
            logger.error(f"Error fetching prices: {e}")
            return None

    def compute_returns(
        self,
        prices: pd.DataFrame,
        corp_actions: Optional[pd.DataFrame] = None,
        horizon_days: int = 5,
        return_type: str = "log",
    ) -> pd.DataFrame:
        """
        Compute forward returns from price series.

        Args:
            prices: DataFrame [date, entity_id, close_price]
            corp_actions: DataFrame [effective_date, entity_id, factor] (split/bonus)
            horizon_days: BIST trading days (not calendar days)
            return_type: "log" or "simple"

        Returns:
            DataFrame [date, entity_id, forward_return, horizon_days]
        """
        try:
            # Sort by entity and date
            prices = prices.sort_values(["entity_id", "date"]).reset_index(drop=True)

            # Apply corporate action adjustments if provided
            if corp_actions is not None:
                prices = self.apply_corporate_actions(prices, corp_actions)

            returns_list = []

            # Compute returns per entity
            for entity_id in prices["entity_id"].unique():
                entity_prices = prices[prices["entity_id"] == entity_id].copy()

                # Compute forward returns over the specified horizon
                for i in range(len(entity_prices) - 1):
                    today_row = entity_prices.iloc[i]

                    # Find the date horizon_days in the future (trading days)
                    target_date = self.calendar.add_trading_days(today_row["date"], horizon_days)

                    # Find the closest row at/after target date
                    future_rows = entity_prices[entity_prices["date"] >= target_date]

                    if len(future_rows) == 0:
                        continue  # No future data available

                    future_row = future_rows.iloc[0]

                    # Compute return
                    today_price = today_row["close_price"]
                    future_price = future_row["close_price"]

                    if return_type == "log":
                        fwd_return = np.log(future_price / today_price)
                    else:  # simple
                        fwd_return = (future_price - today_price) / today_price

                    # Record actual horizon achieved
                    actual_horizon = self.calendar.trading_days_between(
                        today_row["date"], future_row["date"]
                    )

                    returns_list.append({
                        "date": today_row["date"],
                        "entity_id": entity_id,
                        "forward_return": fwd_return,
                        "horizon_days": actual_horizon,
                    })

            result_df = pd.DataFrame(returns_list)
            logger.info(f"✓ Computed {len(result_df)} forward return points")
            return result_df

        except Exception as e:
            logger.error(f"Error computing returns: {e}", exc_info=True)
            return None

    def apply_corporate_actions(
        self, prices: pd.DataFrame, corp_actions: pd.DataFrame
    ) -> pd.DataFrame:
        """
        Apply corporate action factors (splits, bonuses) to price series.

        Mandate 25: A capital-increase day is a real return, not a −50 % cliff.

        Corp_actions format: [effective_date, entity_id, factor]
        Factor interpretation: 0.5 = 2-for-1 split, 1.5 = 50% bonus, etc.
        """
        prices = prices.copy()

        try:
            for _, action_row in corp_actions.iterrows():
                entity_id = action_row["entity_id"]
                effective_date = action_row["effective_date"]
                factor = action_row["factor"]

                # Adjust all prices BEFORE the effective date
                mask = (prices["entity_id"] == entity_id) & (prices["date"] < effective_date)
                prices.loc[mask, "close_price"] *= factor

            logger.info(f"✓ Applied {len(corp_actions)} corporate actions")
            return prices

        except Exception as e:
            logger.error(f"Error applying corporate actions: {e}")
            return prices

    @staticmethod
    def log_returns(prices: np.ndarray) -> np.ndarray:
        """Compute log returns: log(P_t / P_{t-1})."""
        return np.diff(np.log(prices))

    @staticmethod
    def simple_returns(prices: np.ndarray) -> np.ndarray:
        """Compute simple returns: (P_t - P_{t-1}) / P_{t-1}."""
        return np.diff(prices) / prices[:-1]

    @staticmethod
    def compute_sharpe_ratio(returns: np.ndarray, risk_free_rate: float = 0.0) -> float:
        """Compute Sharpe ratio of return series."""
        excess_returns = returns - risk_free_rate / 252  # Assume 252 trading days/year
        return np.mean(excess_returns) / np.std(excess_returns) * np.sqrt(252)

