package strategy

import (
	"testing"
	"time"

	"github.com/24alert/trading-bot/pkg/config"
)

func mustSchedule(t *testing.T) *TradingSchedule {
	t.Helper()
	s, err := NewTradingSchedule("10:00", "18:39", "Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func mustFortsSchedule(t *testing.T) *TradingSchedule {
	t.Helper()
	s, err := NewTradingScheduleFromConfig(config.TradingScheduleConfig{
		Timezone: "Europe/Moscow",
		Sessions: []config.TradingSessionConfig{
			{Name: "forts_day_before_clearing", Start: "10:00", End: "14:00"},
			{Name: "forts_day_after_clearing", Start: "14:05", End: "18:50"},
			{Name: "forts_evening", Start: "19:00", End: "23:50"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func msk(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestIsMainSession_duringSession(t *testing.T) {
	s := mustSchedule(t)
	loc := msk(t)
	// Wednesday 14:30 MSK
	tm := time.Date(2026, 5, 13, 14, 30, 0, 0, loc)
	if !s.IsMainSession(tm) {
		t.Fatal("14:30 on Wednesday should be in main session")
	}
}

func TestIsMainSession_beforeOpen(t *testing.T) {
	s := mustSchedule(t)
	loc := msk(t)
	tm := time.Date(2026, 5, 13, 9, 59, 59, 0, loc)
	if s.IsMainSession(tm) {
		t.Fatal("09:59 should NOT be in main session")
	}
}

func TestIsMainSession_exactOpen(t *testing.T) {
	s := mustSchedule(t)
	loc := msk(t)
	tm := time.Date(2026, 5, 13, 10, 0, 0, 0, loc)
	if !s.IsMainSession(tm) {
		t.Fatal("10:00 should be the start of session")
	}
}

func TestIsMainSession_exactClose(t *testing.T) {
	s := mustSchedule(t)
	loc := msk(t)
	// 18:39:00 is NOT in session (end is exclusive)
	tm := time.Date(2026, 5, 13, 18, 39, 0, 0, loc)
	if s.IsMainSession(tm) {
		t.Fatal("18:39 should NOT be in session (end is exclusive)")
	}
}

func TestIsMainSession_lastSecond(t *testing.T) {
	s := mustSchedule(t)
	loc := msk(t)
	tm := time.Date(2026, 5, 13, 18, 38, 59, 0, loc)
	if !s.IsMainSession(tm) {
		t.Fatal("18:38:59 should still be in session")
	}
}

func TestIsMainSession_night(t *testing.T) {
	s := mustSchedule(t)
	loc := msk(t)
	tm := time.Date(2026, 5, 13, 3, 45, 0, 0, loc)
	if s.IsMainSession(tm) {
		t.Fatal("03:45 must not be in session")
	}
}

func TestIsMainSession_saturday(t *testing.T) {
	s := mustSchedule(t)
	loc := msk(t)
	tm := time.Date(2026, 5, 16, 12, 0, 0, 0, loc) // Saturday
	if s.IsMainSession(tm) {
		t.Fatal("Saturday should NOT be in session")
	}
}

func TestIsMainSession_sunday(t *testing.T) {
	s := mustSchedule(t)
	loc := msk(t)
	tm := time.Date(2026, 5, 17, 12, 0, 0, 0, loc) // Sunday
	if s.IsMainSession(tm) {
		t.Fatal("Sunday should NOT be in session")
	}
}

func TestNextSessionOpen_fromFriday(t *testing.T) {
	s := mustSchedule(t)
	loc := msk(t)
	fri := time.Date(2026, 5, 15, 19, 0, 0, 0, loc) // Friday 19:00
	next := s.NextSessionOpen(fri)
	want := time.Date(2026, 5, 18, 10, 0, 0, 0, loc) // Monday 10:00
	if !next.Equal(want) {
		t.Fatalf("next session open: want %v, got %v", want, next)
	}
}

func TestNextSessionOpen_earlyMorning(t *testing.T) {
	s := mustSchedule(t)
	loc := msk(t)
	wed := time.Date(2026, 5, 13, 8, 0, 0, 0, loc) // Wednesday 08:00
	next := s.NextSessionOpen(wed)
	want := time.Date(2026, 5, 13, 10, 0, 0, 0, loc) // same day 10:00
	if !next.Equal(want) {
		t.Fatalf("next session open: want %v, got %v", want, next)
	}
}

func TestNewTradingSchedule_defaults(t *testing.T) {
	s, err := NewTradingSchedule("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	loc := msk(t)
	tm := time.Date(2026, 5, 13, 15, 0, 0, 0, loc)
	if !s.IsMainSession(tm) {
		t.Fatal("defaults should match MOEX main session")
	}
}

func TestNewTradingSchedule_invalidEnd(t *testing.T) {
	_, err := NewTradingSchedule("18:00", "10:00", "Europe/Moscow")
	if err == nil {
		t.Fatal("end before start should error")
	}
}

func TestFortsSchedule_daySession(t *testing.T) {
	s := mustFortsSchedule(t)
	loc := msk(t)
	tm := time.Date(2026, 5, 13, 10, 0, 0, 0, loc)
	if !s.IsMainSession(tm) {
		t.Fatal("10:00 should be in FORTS day session")
	}
}

func TestFortsSchedule_clearingBreak(t *testing.T) {
	s := mustFortsSchedule(t)
	loc := msk(t)
	tm := time.Date(2026, 5, 13, 14, 2, 0, 0, loc)
	if s.IsMainSession(tm) {
		t.Fatal("14:02 should be blocked by FORTS clearing break")
	}
}

func TestFortsSchedule_afterClearing(t *testing.T) {
	s := mustFortsSchedule(t)
	loc := msk(t)
	tm := time.Date(2026, 5, 13, 14, 5, 0, 0, loc)
	if !s.IsMainSession(tm) {
		t.Fatal("14:05 should be in FORTS day session after clearing")
	}
}

func TestFortsSchedule_betweenDayAndEvening(t *testing.T) {
	s := mustFortsSchedule(t)
	loc := msk(t)
	tm := time.Date(2026, 5, 13, 18, 55, 0, 0, loc)
	if s.IsMainSession(tm) {
		t.Fatal("18:55 should be blocked between FORTS day and evening sessions")
	}
}

func TestFortsSchedule_eveningSession(t *testing.T) {
	s := mustFortsSchedule(t)
	loc := msk(t)
	tm := time.Date(2026, 5, 13, 21, 30, 0, 0, loc)
	if !s.IsMainSession(tm) {
		t.Fatal("21:30 should be in FORTS evening session")
	}
}

func TestFortsSchedule_eveningCloseExclusive(t *testing.T) {
	s := mustFortsSchedule(t)
	loc := msk(t)
	tm := time.Date(2026, 5, 13, 23, 50, 0, 0, loc)
	if s.IsMainSession(tm) {
		t.Fatal("23:50 should NOT be in session (end is exclusive)")
	}
}

func TestFortsNextSessionOpen_fromDayBreak(t *testing.T) {
	s := mustFortsSchedule(t)
	loc := msk(t)
	tm := time.Date(2026, 5, 13, 14, 2, 0, 0, loc)
	next := s.NextSessionOpen(tm)
	want := time.Date(2026, 5, 13, 14, 5, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("next session open: want %v, got %v", want, next)
	}
}
