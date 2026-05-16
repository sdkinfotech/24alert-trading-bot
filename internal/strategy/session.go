package strategy

import (
	"fmt"
	"strings"
	"time"
)

// TradingSchedule defines allowed trading hours. Signals outside the main
// session are blocked at the runner level (handleSignals).
type TradingSchedule struct {
	start time.Duration // offset from midnight
	end   time.Duration
	tz    *time.Location
}

// NewTradingSchedule parses "HH:MM" strings and a timezone into a schedule.
// Defaults: 10:00–18:39 Europe/Moscow (MOEX equity main session).
func NewTradingSchedule(startHHMM, endHHMM, timezone string) (*TradingSchedule, error) {
	if startHHMM == "" {
		startHHMM = "10:00"
	}
	if endHHMM == "" {
		endHHMM = "18:39"
	}
	if timezone == "" {
		timezone = "Europe/Moscow"
	}

	s, err := parseHHMM(startHHMM)
	if err != nil {
		return nil, fmt.Errorf("trading_schedule.main_start: %w", err)
	}
	e, err := parseHHMM(endHHMM)
	if err != nil {
		return nil, fmt.Errorf("trading_schedule.main_end: %w", err)
	}
	if e <= s {
		return nil, fmt.Errorf("trading_schedule: main_end (%s) must be after main_start (%s)", endHHMM, startHHMM)
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("trading_schedule.timezone: %w", err)
	}

	return &TradingSchedule{start: s, end: e, tz: loc}, nil
}

// IsMainSession returns true if t falls within [main_start, main_end) on a weekday.
func (ts *TradingSchedule) IsMainSession(t time.Time) bool {
	lt := t.In(ts.tz)
	wd := lt.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	offset := time.Duration(lt.Hour())*time.Hour +
		time.Duration(lt.Minute())*time.Minute +
		time.Duration(lt.Second())*time.Second
	return offset >= ts.start && offset < ts.end
}

// NextSessionOpen returns the start of the next main session after t.
func (ts *TradingSchedule) NextSessionOpen(t time.Time) time.Time {
	lt := t.In(ts.tz)
	today := time.Date(lt.Year(), lt.Month(), lt.Day(), 0, 0, 0, 0, ts.tz)
	candidate := today.Add(ts.start)

	if lt.Before(candidate) && isWeekday(candidate) {
		return candidate
	}

	// Advance day-by-day (skip weekends).
	next := today.AddDate(0, 0, 1)
	for i := 0; i < 7; i++ {
		if isWeekday(next) {
			return next.Add(ts.start)
		}
		next = next.AddDate(0, 0, 1)
	}
	return next.Add(ts.start)
}

func isWeekday(t time.Time) bool {
	wd := t.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}

func parseHHMM(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return 0, fmt.Errorf("expected HH:MM, got %q: %w", s, err)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid time %q", s)
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute, nil
}
