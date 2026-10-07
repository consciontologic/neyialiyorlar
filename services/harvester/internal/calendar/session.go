package calendar

import (
	"fmt"
	"time"
)

// Session represents a BIST trading session with holidays and half-days.
// Rationale (mandate 18): a single versioned calendar drives all daily-close
// and intraday ticks. This avoids naive weekday math and lets us exclude
// holidays without separate lookups.
type Session struct {
	Date       time.Time `json:"date"`        // Trading session date
	SessionOn  bool      `json:"session_on"`  // true = trading session
	Type       string    `json:"type"`        // "session", "half-day", "holiday"
	IsHalfDay  bool      `json:"is_half_day"` // true if shortened session
}

// SessionCalendar manages BIST trading session dates and holidays.
type SessionCalendar struct {
	sessions map[time.Time]*Session // keyed by date
	sorted   []time.Time             // sorted session dates for binary search
}

// NewSessionCalendar creates a new session calendar.
func NewSessionCalendar() *SessionCalendar {
	return &SessionCalendar{
		sessions: make(map[time.Time]*Session),
		sorted:   []time.Time{},
	}
}

// AddSession adds a session or holiday to the calendar.
func (sc *SessionCalendar) AddSession(sess *Session) error {
	if sess == nil {
		return fmt.Errorf("session is nil")
	}

	// Normalize date to midnight
	date := time.Date(sess.Date.Year(), sess.Date.Month(), sess.Date.Day(), 0, 0, 0, 0, sess.Date.Location())

	sc.sessions[date] = sess
	return nil
}

// IsSessionDay returns true if the given date is a trading session.
func (sc *SessionCalendar) IsSessionDay(date time.Time) bool {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())

	sess, ok := sc.sessions[date]
	if !ok {
		return false // Unknown date defaults to non-session
	}

	return sess.SessionOn
}

// IsHalfDay returns true if the given date is a half-day session.
func (sc *SessionCalendar) IsHalfDay(date time.Time) bool {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())

	sess, ok := sc.sessions[date]
	if !ok {
		return false
	}

	return sess.IsHalfDay
}

// NextSessionDay returns the next trading session after the given date.
// Returns zero time if no session found (e.g., beyond calendar horizon).
func (sc *SessionCalendar) NextSessionDay(after time.Time) time.Time {
	after = time.Date(after.Year(), after.Month(), after.Day(), 0, 0, 0, 0, after.Location())

	// Find the next session by iterating through sorted dates
	for _, date := range sc.sortedDates() {
		if date.After(after) && sc.sessions[date].SessionOn {
			return date
		}
	}

	return time.Time{} // No next session
}

// PreviousSessionDay returns the previous trading session before the given date.
// Returns zero time if no session found.
func (sc *SessionCalendar) PreviousSessionDay(before time.Time) time.Time {
	before = time.Date(before.Year(), before.Month(), before.Day(), 0, 0, 0, 0, before.Location())

	sorted := sc.sortedDates()
	// Iterate backwards
	for i := len(sorted) - 1; i >= 0; i-- {
		date := sorted[i]
		if date.Before(before) && sc.sessions[date].SessionOn {
			return date
		}
	}

	return time.Time{}
}

// HorizonStart returns the earliest session date in the calendar.
func (sc *SessionCalendar) HorizonStart() time.Time {
	sorted := sc.sortedDates()
	if len(sorted) == 0 {
		return time.Time{}
	}
	return sorted[0]
}

// HorizonEnd returns the latest session date in the calendar.
func (sc *SessionCalendar) HorizonEnd() time.Time {
	sorted := sc.sortedDates()
	if len(sorted) == 0 {
		return time.Time{}
	}
	return sorted[len(sorted)-1]
}

// IsWithinHorizon returns true if the given date is within the calendar's known range.
func (sc *SessionCalendar) IsWithinHorizon(date time.Time) bool {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	_, ok := sc.sessions[date]
	return ok
}

// sortedDates returns all calendar dates sorted chronologically.
func (sc *SessionCalendar) sortedDates() []time.Time {
	// Build a sorted slice on-the-fly
	var dates []time.Time
	for d := range sc.sessions {
		dates = append(dates, d)
	}

	// Simple bubble sort for small calendars (production would use sort.Slice)
	for i := 0; i < len(dates)-1; i++ {
		for j := i + 1; j < len(dates); j++ {
			if dates[j].Before(dates[i]) {
				dates[i], dates[j] = dates[j], dates[i]
			}
		}
	}

	return dates
}

// ListSessions returns all sessions in the calendar (for auditing).
func (sc *SessionCalendar) ListSessions() []*Session {
	var sessions []*Session
	for _, date := range sc.sortedDates() {
		sessions = append(sessions, sc.sessions[date])
	}
	return sessions
}

// LoadGoldenBISTCalendar returns a hardcoded BIST calendar for testing.
// This is a sample; production would load from config/neyialiyorlar.yaml.
func LoadGoldenBISTCalendar() *SessionCalendar {
	cal := NewSessionCalendar()

	// Sample 2024 BIST sessions and holidays
	samples := []struct {
		Date      string // "2024-01-01"
		SessionOn bool
		IsHalfDay bool
	}{
		{"2024-01-01", false, false}, // New Year
		{"2024-01-02", true, false},  // Regular session
		{"2024-01-15", true, false},  // Regular session
		{"2024-04-23", false, false}, // National holiday
		{"2024-12-31", true, true},   // Year-end half-day
	}

	istanbul, _ := time.LoadLocation("Europe/Istanbul")
	for _, s := range samples {
		date, _ := time.ParseInLocation("2006-01-02", s.Date, istanbul)
		cal.AddSession(&Session{
			Date:      date,
			SessionOn: s.SessionOn,
			Type: func() string {
				if s.IsHalfDay {
					return "half-day"
				} else if s.SessionOn {
					return "session"
				} else {
					return "holiday"
				}
			}(),
			IsHalfDay: s.IsHalfDay,
		})
	}

	return cal
}
