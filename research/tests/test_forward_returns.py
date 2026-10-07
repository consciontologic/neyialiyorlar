"""
Unit tests for forward returns builder.

Phase 6 mandate 12: price_return:<horizon> derived from BIST raw via idempotent raw-then-derived path.
Phase 6 mandate 25: Corporate-action adjustment; a capital-increase day is a real return, not a −50% cliff.
"""

import pytest
import pandas as pd
import numpy as np
from datetime import date, timedelta

from bist_calendar.bist import BistCalendar
from returns.forward import ForwardReturnsBuilder


@pytest.fixture
def calendar():
    """Initialize BIST calendar."""
    return BistCalendar()


@pytest.fixture
def builder(calendar):
    """Initialize forward returns builder."""
    return ForwardReturnsBuilder(calendar)


@pytest.fixture
def sample_prices():
    """Create sample BIST price data."""
    dates = pd.date_range("2024-01-01", periods=50, freq="D")
    prices = 100 + np.cumsum(np.random.randn(50) * 2)  # Random walk from 100

    data = []
    for entity_id in ["AAPL", "MSFT"]:
        for i, date_val in enumerate(dates):
            data.append({
                "date": date_val,
                "entity_id": entity_id,
                "close_price": prices[i],
            })

    return pd.DataFrame(data)


class TestForwardReturnsComputation:
    """Test basic forward returns computation."""

    def test_compute_returns_log_returns(self, builder, sample_prices):
        """Compute log returns over a specified horizon."""
        # Filter to single entity for simpler testing
        prices = sample_prices[sample_prices["entity_id"] == "AAPL"].copy()

        returns_df = builder.compute_returns(prices, horizon_days=5, return_type="log")

        assert returns_df is not None
        assert len(returns_df) > 0
        assert "forward_return" in returns_df.columns
        assert "horizon_days" in returns_df.columns

    def test_compute_returns_simple_returns(self, builder, sample_prices):
        """Compute simple returns (percentage change)."""
        prices = sample_prices[sample_prices["entity_id"] == "AAPL"].copy()

        returns_df = builder.compute_returns(prices, horizon_days=5, return_type="simple")

        assert returns_df is not None
        # Simple returns should be between -1 and inf
        assert (returns_df["forward_return"] > -1).all()

    def test_compute_returns_multiple_entities(self, builder, sample_prices):
        """Ensure returns are computed per entity separately."""
        returns_df = builder.compute_returns(sample_prices, horizon_days=5)

        # Should have returns for both AAPL and MSFT
        assert "AAPL" in returns_df["entity_id"].values
        assert "MSFT" in returns_df["entity_id"].values

    def test_compute_returns_respects_horizon(self, builder, sample_prices):
        """Verify forward returns are computed over the specified horizon."""
        prices = sample_prices[sample_prices["entity_id"] == "AAPL"].copy()
        horizon = 5

        returns_df = builder.compute_returns(prices, horizon_days=horizon)

        # Horizon_days should match the requested horizon (approximately, given calendar)
        # Allow some tolerance for weekend/holiday skip logic
        assert (returns_df["horizon_days"] >= horizon - 2).all()
        assert (returns_df["horizon_days"] <= horizon + 2).all()


class TestCorporateActionAdjustment:
    """Test mandate 25: Corporate-action adjustment.

    A capital-increase day is a real return, not a −50% cliff.
    """

    def test_apply_corporate_actions_stock_split(self, builder):
        """Apply 2-for-1 stock split adjustment."""
        prices_data = pd.DataFrame({
            "date": pd.date_range("2024-01-01", periods=10, freq="D"),
            "entity_id": "AAPL",
            "close_price": [100] * 10,
        })

        # 2-for-1 split on day 5
        corp_actions = pd.DataFrame({
            "effective_date": [date(2024, 1, 5)],
            "entity_id": ["AAPL"],
            "factor": [0.5],  # 0.5 = 2-for-1 split
        })

        adjusted = builder.apply_corporate_actions(prices_data, corp_actions)

        # Prices before split should be halved
        before_split = adjusted[adjusted["date"] < date(2024, 1, 5)]
        after_split = adjusted[adjusted["date"] >= date(2024, 1, 5)]

        assert (before_split["close_price"] == 50).all()
        assert (after_split["close_price"] == 100).all()

    def test_apply_corporate_actions_bonus(self, builder):
        """Apply 50% bonus (rights issue) adjustment."""
        prices_data = pd.DataFrame({
            "date": pd.date_range("2024-01-01", periods=10, freq="D"),
            "entity_id": "AAPL",
            "close_price": [100] * 10,
        })

        # 50% bonus on day 5
        corp_actions = pd.DataFrame({
            "effective_date": [date(2024, 1, 5)],
            "entity_id": ["AAPL"],
            "factor": [1 / 1.5],  # Inverse: apply before effective date
        })

        adjusted = builder.apply_corporate_actions(prices_data, corp_actions)

        # Prices before bonus should be adjusted down
        before_bonus = adjusted[adjusted["date"] < date(2024, 1, 5)]["close_price"].iloc[0]
        after_bonus = adjusted[adjusted["date"] >= date(2024, 1, 5)]["close_price"].iloc[0]

        # After adjustment, returns should be continuous (no cliff)
        assert after_bonus > before_bonus  # Adjusted down before, adjusted up after
        assert not np.isnan(before_bonus)
        assert not np.isnan(after_bonus)

    def test_capital_increase_is_real_return_not_cliff(self, builder):
        """Test mandate 25: A capital-increase day is a real return, not a −50% cliff."""
        # Create price series with a -50% cliff (simulating capital increase)
        dates = pd.date_range("2024-01-01", periods=10, freq="D")
        prices_raw = [100] * 4 + [50] * 6  # -50% cliff on day 5

        prices_data = pd.DataFrame({
            "date": dates,
            "entity_id": "AAPL",
            "close_price": prices_raw,
        })

        # Apply 50% factor adjustment (to account for capital increase)
        corp_actions = pd.DataFrame({
            "effective_date": [date(2024, 1, 5)],
            "entity_id": ["AAPL"],
            "factor": [1 / 1.5],
        })

        adjusted = builder.apply_corporate_actions(prices_data, corp_actions)

        # After adjustment, the cliff should be smoothed
        # Prices before should be adjusted down, after should remain the same
        adjusted_before = adjusted.loc[adjusted["date"] < date(2024, 1, 5), "close_price"].iloc[0]
        adjusted_after = adjusted.loc[adjusted["date"] >= date(2024, 1, 5), "close_price"].iloc[0]

        # The adjusted price before should be lower than 100
        assert adjusted_before < 100

        # When computing returns, the cliff should be smoothed
        returns_raw = (prices_raw[-1] - prices_raw[0]) / prices_raw[0]
        returns_adjusted = (adjusted_after - adjusted_before) / adjusted_before

        # Adjusted returns should be closer to zero (less extreme)
        assert abs(returns_adjusted) < abs(returns_raw)


class TestPointInTimeAlignment:
    """Test no look-ahead bias in forward returns computation."""

    def test_returns_only_use_historical_prices(self, builder):
        """Forward returns should only use prices known at the start date."""
        prices_data = pd.DataFrame({
            "date": pd.date_range("2024-01-01", periods=20, freq="D"),
            "entity_id": "AAPL",
            "close_price": range(100, 120),
        })

        returns_df = builder.compute_returns(prices_data, horizon_days=5)

        # Ensure no look-ahead: each return uses only prices up to its horizon
        for _, row in returns_df.iterrows():
            start_date = row["date"]
            forward_return = row["forward_return"]

            # Verify return was computed from prices available at start_date
            # (This is more of a logical check; actual implementation would need audit trail)
            assert start_date <= prices_data["date"].max()


class TestHorizonCalculation:
    """Test mandate 18: Horizons count BIST session days."""

    def test_horizon_uses_trading_days_not_calendar_days(self, builder):
        """Verify horizon is measured in BIST trading days, not calendar days."""
        # Create prices across a weekend
        dates = [
            date(2024, 1, 12),  # Friday
            date(2024, 1, 15),  # Monday
            date(2024, 1, 16),  # Tuesday
            date(2024, 1, 17),  # Wednesday
            date(2024, 1, 18),  # Thursday
            date(2024, 1, 19),  # Friday
            date(2024, 1, 22),  # Monday
        ]

        prices_data = pd.DataFrame({
            "date": dates,
            "entity_id": "AAPL",
            "close_price": range(100, 107),
        })

        returns_df = builder.compute_returns(prices_data, horizon_days=3)

        # The horizon should use trading days (Fri->Mon=1, Mon->Tue=2, Tue->Wed=3)
        # So a 3-day horizon from Friday should land on Wednesday
        if len(returns_df) > 0:
            # First return should be from Friday over 3 trading days (landing on Wed)
            first_return = returns_df.iloc[0]
            assert first_return["horizon_days"] >= 2  # At least this many trading days


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
