"""BIST Trading Calendar (Borsa İstanbul)."""

from datetime import datetime, date, timedelta
from typing import Set, List
import logging

logger = logging.getLogger("calendar.bist")


class BistCalendar:
    """
    BIST session trading calendar.

    Mandate 18: Horizons count BIST session days (not calendar days);
    a holiday span does not shorten the realized horizon.

    Uses official BIST holiday calendar (fixed holidays + Islamic holidays via lunar calculation).
    """

    # Fixed holidays (Republic Day, National Day, etc.)
    FIXED_HOLIDAYS = {
        # 2024
        (2024, 1, 1),   # New Year's Day
        (2024, 4, 23),  # National Sovereignty Day
        (2024, 5, 1),   # Labor Day
        (2024, 7, 15),  # Democracy Day
        (2024, 8, 30),  # Victory Day
        (2024, 10, 29), # Republic Day
        # 2025
        (2025, 1, 1),   # New Year's Day
        (2025, 4, 23),  # National Sovereignty Day
        (2025, 5, 1),   # Labor Day
        (2025, 7, 15),  # Democracy Day
        (2025, 8, 30),  # Victory Day
        (2025, 10, 29), # Republic Day
        # 2026
        (2026, 1, 1),   # New Year's Day
        (2026, 4, 23),  # National Sovereignty Day
        (2026, 5, 1),   # Labor Day
        (2026, 7, 15),  # Democracy Day
        (2026, 8, 30),  # Victory Day
        (2026, 10, 29), # Republic Day
    }

    # Islamic holidays (Ramadan Feast [3 days] + Sacrifice Feast [4 days])
    # Dates approximate (lunar calendar); listed as first day
    ISLAMIC_HOLIDAYS = {
        # 2024
        (2024, 4, 10), (2024, 4, 11), (2024, 4, 12),  # Ramadan Feast 2024
        (2024, 6, 16), (2024, 6, 17), (2024, 6, 18), (2024, 6, 19),  # Sacrifice Feast 2024
        # 2025
        (2025, 3, 30), (2025, 3, 31), (2025, 4, 1),   # Ramadan Feast 2025
        (2025, 6, 6), (2025, 6, 7), (2025, 6, 8), (2025, 6, 9),     # Sacrifice Feast 2025
        # 2026
        (2026, 3, 20), (2026, 3, 21), (2026, 3, 22),  # Ramadan Feast 2026
        (2026, 5, 26), (2026, 5, 27), (2026, 5, 28), (2026, 5, 29), # Sacrifice Feast 2026
    }

    # Combined holiday set
    ALL_HOLIDAYS = FIXED_HOLIDAYS | ISLAMIC_HOLIDAYS

    def __init__(self):
        self.cached_holidays: Set[date] = set()

    def is_trading_day(self, d: date) -> bool:
        """Check if a date is a BIST trading day (not weekend, not holiday)."""
        if d.weekday() >= 5:  # Saturday(5) / Sunday(6)
            return False
        if (d.year, d.month, d.day) in self.ALL_HOLIDAYS:
            return False
        return True

    def trading_days_between(self, start: date, end: date) -> int:
        """Count trading days between start and end (inclusive)."""
        count = 0
        current = start
        while current <= end:
            if self.is_trading_day(current):
                count += 1
            current += timedelta(days=1)
        return count

    def add_trading_days(self, start: date, num_days: int) -> date:
        """Return the date num_days trading days after start."""
        count = 0
        current = start + timedelta(days=1)
        while count < num_days:
            if self.is_trading_day(current):
                count += 1
            if count < num_days:
                current += timedelta(days=1)
        return current

    def nearest_trading_day_before(self, d: date) -> date:
        """Return the nearest trading day on or before d."""
        while not self.is_trading_day(d):
            d -= timedelta(days=1)
        return d

    def nearest_trading_day_after(self, d: date) -> date:
        """Return the nearest trading day on or after d."""
        while not self.is_trading_day(d):
            d += timedelta(days=1)
        return d

    def get_holidays_in_range(self, start: date, end: date) -> List[date]:
        """Get list of all holidays between start and end (inclusive)."""
        holidays = []
        current = start
        while current <= end:
            if not self.is_trading_day(current) and current.weekday() < 5:
                holidays.append(current)
            current += timedelta(days=1)
        return holidays

    def is_holiday(self, d: date) -> bool:
        """Check if a date is a holiday (not including weekends)."""
        return (d.year, d.month, d.day) in self.ALL_HOLIDAYS

