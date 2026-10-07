package calendar

import (
	"testing"
	"time"
)

// TestSessionCalendarAdd tests adding sessions to the calendar.
func TestSessionCalendarAdd(t *testing.T) {
	cal := NewSessionCalendar()

	istanbul, _ := time.LoadLocation("Europe/Istanbul")
	date := time.Date(2024, 1, 15, 10, 30, 0, 0, istanbul)

	sess := &Session{
		Date:      date,
		SessionOn: true,
		Type:      "session",
		IsHalfDay: false,
	}

	err := cal.AddSession(sess)
	if err != nil {
		t.Errorf("failed to add session: %v", err)
	}

	if !cal.IsSessionDay(date) {
		t.Errorf("expected session day on %v", date)
	}
}

// TestSessionCalendarHoliday tests holiday handling.
func TestSessionCalendarHoliday(t *testing.T) {
	cal := NewSessionCalendar()

	istanbul, _ := time.LoadLocation("Europe/Istanbul")
	date := time.Date(2024, 1, 1, 0, 0, 0, 0, istanbul)

	sess := &Session{
		Date:      date,
		SessionOn: false,
		Type:      "holiday",
		IsHalfDay: false,
	}

	cal.AddSession(sess)

	if cal.IsSessionDay(date) {
		t.Errorf("expected holiday (non-session) on %v", date)
	}
}

// TestSessionCalendarHalfDay tests half-day session handling.
func TestSessionCalendarHalfDay(t *testing.T) {
	cal := NewSessionCalendar()

	istanbul, _ := time.LoadLocation("Europe/Istanbul")
	date := time.Date(2024, 12, 31, 0, 0, 0, 0, istanbul)

	sess := &Session{
		Date:      date,
		SessionOn: true,
		Type:      "half-day",
		IsHalfDay: true,
	}

	cal.AddSession(sess)

	if !cal.IsSessionDay(date) {
		t.Errorf("expected half-day session on %v", date)
	}

	if !cal.IsHalfDay(date) {
		t.Errorf("expected half-day flag on %v", date)
	}
}

// TestSessionCalendarNextSessionDay tests finding the next session.
func TestSessionCalendarNextSessionDay(t *testing.T) {
	cal := NewSessionCalendar()

	istanbul, _ := time.LoadLocation("Europe/Istanbul")

	dates := []string{"2024-01-01", "2024-01-02", "2024-01-15"}
	for i, dateStr := range dates {
		date, _ := time.ParseInLocation("2006-01-02", dateStr, istanbul)
		sessionOn := i > 0 // First date is holiday, rest are sessions
		cal.AddSession(&Session{
			Date:      date,
			SessionOn: sessionOn,
			Type: func() string {
				if sessionOn {
					return "session"
				} else {
					return "holiday"
				}
			}(),
		})
	}

	jan1, _ := time.ParseInLocation("2006-01-02", "2024-01-01", istanbul)
	nextSession := cal.NextSessionDay(jan1)

	jan2, _ := time.ParseInLocation("2006-01-02", "2024-01-02", istanbul)
	if !nextSession.Equal(jan2) {
		t.Errorf("expected next session on 2024-01-02, got %v", nextSession)
	}
}

// TestSessionCalendarPreviousSessionDay tests finding the previous session.
func TestSessionCalendarPreviousSessionDay(t *testing.T) {
	cal := NewSessionCalendar()

	istanbul, _ := time.LoadLocation("Europe/Istanbul")

	// Add three sessions
	dates := []string{"2024-01-02", "2024-01-15", "2024-01-22"}
	for _, dateStr := range dates {
		date, _ := time.ParseInLocation("2006-01-02", dateStr, istanbul)
		cal.AddSession(&Session{
			Date:      date,
			SessionOn: true,
			Type:      "session",
		})
	}

	jan15, _ := time.ParseInLocation("2006-01-02", "2024-01-15", istanbul)
	jan22, _ := time.ParseInLocation("2006-01-02", "2024-01-22", istanbul)

	prevSession := cal.PreviousSessionDay(jan22)

	if !prevSession.Equal(jan15) {
		t.Errorf("expected previous session on 2024-01-15, got %v", prevSession)
	}
}

// TestSessionCalendarHorizonGuard tests calendar boundary checks.
func TestSessionCalendarHorizonGuard(t *testing.T) {
	cal := NewSessionCalendar()

	istanbul, _ := time.LoadLocation("Europe/Istanbul")

	jan1, _ := time.ParseInLocation("2006-01-02", "2024-01-01", istanbul)
	jan15, _ := time.ParseInLocation("2006-01-02", "2024-01-15", istanbul)
	jan30, _ := time.ParseInLocation("2006-01-02", "2024-01-30", istanbul)

	// Only add two dates
	cal.AddSession(&Session{Date: jan1, SessionOn: true, Type: "session"})
	cal.AddSession(&Session{Date: jan15, SessionOn: true, Type: "session"})

	// Check horizon
	if !cal.IsWithinHorizon(jan1) || !cal.IsWithinHorizon(jan15) {
		t.Errorf("expected jan1 and jan15 within horizon")
	}

	if cal.IsWithinHorizon(jan30) {
		t.Errorf("expected jan30 outside horizon")
	}

	start := cal.HorizonStart()
	if !start.Equal(jan1) {
		t.Errorf("expected horizon start on 2024-01-01, got %v", start)
	}

	end := cal.HorizonEnd()
	if !end.Equal(jan15) {
		t.Errorf("expected horizon end on 2024-01-15, got %v", end)
	}
}

// TestGoldenBISTCalendar tests the golden calendar.
func TestGoldenBISTCalendar(t *testing.T) {
	cal := LoadGoldenBISTCalendar()

	istanbul, _ := time.LoadLocation("Europe/Istanbul")

	// Check known dates from golden calendar
	newYear, _ := time.ParseInLocation("2006-01-02", "2024-01-01", istanbul)
	if cal.IsSessionDay(newYear) {
		t.Errorf("expected New Year to be a holiday (non-session)")
	}

	// Check that the calendar has at least some entries
	sessions := cal.ListSessions()
	if len(sessions) == 0 {
		t.Errorf("expected golden calendar to have sessions")
	}

	// Verify horizon is valid
	start := cal.HorizonStart()
	end := cal.HorizonEnd()
	if start.After(end) {
		t.Errorf("expected horizon start before end")
	}
}

// TestSessionCalendarNormalization tests that dates are normalized to midnight.
func TestSessionCalendarNormalization(t *testing.T) {
	cal := NewSessionCalendar()

	istanbul, _ := time.LoadLocation("Europe/Istanbul")

	// Add session with non-midnight time
	date1 := time.Date(2024, 1, 15, 10, 30, 45, 100, istanbul)
	date2 := time.Date(2024, 1, 15, 0, 0, 0, 0, istanbul)

	cal.AddSession(&Session{
		Date:      date1,
		SessionOn: true,
		Type:      "session",
	})

	// Both should match after normalization
	if !cal.IsSessionDay(date2) {
		t.Errorf("expected normalized dates to match")
	}
}
