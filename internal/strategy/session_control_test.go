package strategy

import (
	"testing"
	"time"
)

func TestTradingScheduleNextScheduleChangeInSession(t *testing.T) {
	ts, err := NewTradingSchedule("10:00", "18:39", "Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Europe/Moscow")
	// Wednesday 12:00 MSK — inside main session
	tm := time.Date(2026, 5, 20, 12, 0, 0, 0, loc)
	next, active, label := ts.NextScheduleChange(tm)
	if !active {
		t.Fatalf("expected active session")
	}
	if label != "main" {
		t.Fatalf("label=%q want main", label)
	}
	wantEnd := time.Date(2026, 5, 20, 18, 39, 0, 0, loc)
	if !next.Equal(wantEnd) {
		t.Fatalf("next=%v want %v", next, wantEnd)
	}
}

func TestTradingScheduleNextScheduleChangeOutside(t *testing.T) {
	ts, err := NewTradingSchedule("10:00", "18:39", "Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Europe/Moscow")
	tm := time.Date(2026, 5, 20, 8, 0, 0, 0, loc)
	next, active, _ := ts.NextScheduleChange(tm)
	if active {
		t.Fatalf("expected outside session")
	}
	wantOpen := time.Date(2026, 5, 20, 10, 0, 0, 0, loc)
	if !next.Equal(wantOpen) {
		t.Fatalf("next=%v want %v", next, wantOpen)
	}
}
