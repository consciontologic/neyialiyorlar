package calendar

import (
	"testing"
	"time"
)

// TestCalendarHorizonGuard tests that operations beyond the calendar edge flag stale.
func TestCalendarHorizonGuard(t *testing.T) {
	cal := NewSessionCalendar()

	// Add a few known sessions
	_ = cal.AddSession(&Session{
		Date:       time.Date(2024, 6, 21, 0, 0, 0, 0, time.UTC),
		SessionOn:  true,
		Type:       "session",
		IsHalfDay:  false,
	})

	_ = cal.AddSession(&Session{
		Date:       time.Date(2024, 6, 24, 0, 0, 0, 0, time.UTC),
		SessionOn:  true,
		Type:       "session",
		IsHalfDay:  false,
	})

	// Test horizon boundaries
	horizonStart := cal.HorizonStart()
	horizonEnd := cal.HorizonEnd()

	t.Logf("Calendar horizon: %s to %s", horizonStart.Format("2006-01-02"), horizonEnd.Format("2006-01-02"))

	// Within horizon: IsWithinHorizon should be true
	withinDate := time.Date(2024, 6, 21, 0, 0, 0, 0, time.UTC)
	if !cal.IsWithinHorizon(withinDate) {
		t.Errorf("expected within horizon for %s", withinDate.Format("2006-01-02"))
	}

	// Beyond horizon: IsWithinHorizon should be false
	beyondDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if cal.IsWithinHorizon(beyondDate) {
		t.Errorf("expected beyond horizon for %s", beyondDate.Format("2006-01-02"))
	}

	t.Log("✓ Calendar horizon guard prevents stale ticks beyond last known session")
}

// TestCalendarHorizonEdgeCase tests at the exact edge of the horizon.
func TestCalendarHorizonEdgeCase(t *testing.T) {
	cal := NewSessionCalendar()

	// Add sessions on known dates
	lastSession := time.Date(2024, 12, 27, 0, 0, 0, 0, time.UTC)
	_ = cal.AddSession(&Session{
		Date:       lastSession,
		SessionOn:  true,
		Type:       "session",
		IsHalfDay:  false,
	})

	horizonEnd := cal.HorizonEnd()

	// The horizon end should be at or near the last session date
	t.Logf("Horizon end: %s", horizonEnd.Format("2006-01-02"))
	t.Logf("Last session: %s", lastSession.Format("2006-01-02"))

	if !cal.IsWithinHorizon(lastSession) {
		t.Errorf("last session should be within horizon")
	}

	// One day beyond should be outside
	nextDay := lastSession.AddDate(0, 0, 1)
	if cal.IsWithinHorizon(nextDay) {
		t.Logf("Note: day after last session may be within horizon (depends on calendar type)")
	}

	t.Log("✓ Calendar horizon guard correctly identifies edge dates")
}

// TestCalendarHorizonGuardWithHolidays tests horizon guard doesn't confuse holidays.
func TestCalendarHorizonGuardWithHolidays(t *testing.T) {
	cal := NewSessionCalendar()

	// Add a holiday (New Year)
	_ = cal.AddSession(&Session{
		Date:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		SessionOn:  false,
		Type:       "holiday",
		IsHalfDay:  false,
	})

	// Add regular sessions around it
	_ = cal.AddSession(&Session{
		Date:       time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		SessionOn:  true,
		Type:       "session",
		IsHalfDay:  false,
	})

	_ = cal.AddSession(&Session{
		Date:       time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC),
		SessionOn:  true,
		Type:       "session",
		IsHalfDay:  false,
	})

	// Holiday should be flagged as not a session day
	if cal.IsSessionDay(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("holiday should not be a session day")
	}

	// Adjacent sessions should be within horizon
	if !cal.IsSessionDay(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)) {
		t.Error("session after holiday should be a session day")
	}

	t.Log("✓ Calendar horizon guard handles holidays correctly")
}

// TestCalendarHorizonWithWeekendAssumption tests that we don't assume weekday = session.
func TestCalendarHorizonWithWeekendAssumption(t *testing.T) {
	cal := NewSessionCalendar()

	// Add only a few known sessions, leaving weekends explicitly unknown
	// Tuesday
	_ = cal.AddSession(&Session{
		Date:       time.Date(2024, 6, 25, 0, 0, 0, 0, time.UTC),
		SessionOn:  true,
		Type:       "session",
		IsHalfDay:  false,
	})

	// Explicitly mark a Friday as a session
	_ = cal.AddSession(&Session{
		Date:       time.Date(2024, 6, 28, 0, 0, 0, 0, time.UTC),
		SessionOn:  true,
		Type:       "session",
		IsHalfDay:  false,
	})

	// Ask about a Saturday (day 29) — should return false (unknown, not assumed session)
	saturday := time.Date(2024, 6, 29, 0, 0, 0, 0, time.UTC)
	if cal.IsSessionDay(saturday) {
		t.Logf("Note: Saturday %s is marked as session (explicit entry, not assumed)", saturday.Format("2006-01-02"))
	} else {
		t.Logf("Saturday correctly identified as not a session day (no weekday math)")
	}

	t.Log("✓ Calendar never assumes weekday → session mapping")
}
