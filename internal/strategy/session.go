package strategy

import (
	"fmt"
	"strings"
	"time"

	"github.com/24alert/trading-bot/pkg/config"
)

// TradingSchedule defines allowed trading hours. Signals outside configured
// sessions are blocked at the runner level (handleSignals).
type TradingSchedule struct {
	sessions []sessionWindow
	tz       *time.Location
}

type sessionWindow struct {
	name  string
	start time.Duration // offset from midnight
	end   time.Duration
}

// NewTradingSchedule parses "HH:MM" strings and a timezone into a schedule.
// Defaults: 10:00–18:39 Europe/Moscow (legacy single-window main session).
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

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("trading_schedule.timezone: %w", err)
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

	return &TradingSchedule{sessions: []sessionWindow{{name: "main", start: s, end: e}}, tz: loc}, nil
}

func NewTradingScheduleFromConfig(cfg config.TradingScheduleConfig) (*TradingSchedule, error) {
	timezone := cfg.Timezone
	if timezone == "" {
		timezone = "Europe/Moscow"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("trading_schedule.timezone: %w", err)
	}

	if len(cfg.Sessions) == 0 {
		return NewTradingSchedule(cfg.MainStart, cfg.MainEnd, timezone)
	}

	windows := make([]sessionWindow, 0, len(cfg.Sessions))
	for i, item := range cfg.Sessions {
		s, err := parseHHMM(item.Start)
		if err != nil {
			return nil, fmt.Errorf("trading_schedule.sessions[%d].start: %w", i, err)
		}
		e, err := parseHHMM(item.End)
		if err != nil {
			return nil, fmt.Errorf("trading_schedule.sessions[%d].end: %w", i, err)
		}
		if e <= s {
			return nil, fmt.Errorf("trading_schedule.sessions[%d]: end (%s) must be after start (%s)", i, item.End, item.Start)
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = fmt.Sprintf("session_%d", i+1)
		}
		windows = append(windows, sessionWindow{name: name, start: s, end: e})
	}
	return &TradingSchedule{sessions: windows, tz: loc}, nil
}

// IsMainSession returns true if t falls within any configured session on a weekday.
func (ts *TradingSchedule) IsMainSession(t time.Time) bool {
	lt := t.In(ts.tz)
	wd := lt.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	offset := time.Duration(lt.Hour())*time.Hour +
		time.Duration(lt.Minute())*time.Minute +
		time.Duration(lt.Second())*time.Second
	for _, w := range ts.sessions {
		if offset >= w.start && offset < w.end {
			return true
		}
	}
	return false
}

// NextSessionOpen returns the start of the next configured session after t.
func (ts *TradingSchedule) NextSessionOpen(t time.Time) time.Time {
	lt := t.In(ts.tz)
	today := time.Date(lt.Year(), lt.Month(), lt.Day(), 0, 0, 0, 0, ts.tz)

	if isWeekday(today) {
		for _, w := range ts.sessions {
			candidate := today.Add(w.start)
			if lt.Before(candidate) {
				return candidate
			}
		}
	}

	// Advance day-by-day (skip weekends).
	next := today.AddDate(0, 0, 1)
	for i := 0; i < 7; i++ {
		if isWeekday(next) {
			return next.Add(ts.sessions[0].start)
		}
		next = next.AddDate(0, 0, 1)
	}
	return next.Add(ts.sessions[0].start)
}

// NextScheduleChange returns the next time the trading-window state flips (session open or close).
// active is true when t is inside a configured weekday session; label is the current or next session name.
func (ts *TradingSchedule) NextScheduleChange(t time.Time) (next time.Time, active bool, label string) {
	if ts == nil || len(ts.sessions) == 0 {
		return t, false, ""
	}
	lt := t.In(ts.tz)
	if ts.IsMainSession(t) {
		active = true
		offset := time.Duration(lt.Hour())*time.Hour +
			time.Duration(lt.Minute())*time.Minute +
			time.Duration(lt.Second())*time.Second
		today := time.Date(lt.Year(), lt.Month(), lt.Day(), 0, 0, 0, 0, ts.tz)
		for _, w := range ts.sessions {
			if offset >= w.start && offset < w.end {
				label = w.name
				return today.Add(w.end), true, label
			}
		}
	}
	label = ts.sessions[0].name
	return ts.NextSessionOpen(t), false, label
}

func (ts *TradingSchedule) WindowString() string {
	parts := make([]string, 0, len(ts.sessions))
	for _, w := range ts.sessions {
		if w.name != "" {
			parts = append(parts, fmt.Sprintf("%s %s-%s", w.name, formatHHMM(w.start), formatHHMM(w.end)))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s-%s", formatHHMM(w.start), formatHHMM(w.end)))
	}
	return strings.Join(parts, ", ")
}

func (ts *TradingSchedule) TimezoneName() string {
	if ts == nil || ts.tz == nil {
		return ""
	}
	return ts.tz.String()
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

func formatHHMM(d time.Duration) string {
	totalMinutes := int(d / time.Minute)
	return fmt.Sprintf("%02d:%02d", totalMinutes/60, totalMinutes%60)
}
