package journal

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLite_ListEventsIncludesTimelineEvents(t *testing.T) {
	ctx := context.Background()
	j, err := OpenSQLite(filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer j.Close()

	ts := time.Date(2026, 5, 16, 11, 45, 0, 0, time.UTC)
	if err := j.RecordEvent(ctx, EventRecord{
		Type:          "signal_cancelled",
		InstanceID:    "fut-brent-mini-lb",
		InstrumentUID: "uid-brent",
		Direction:     "sell",
		Quantity:      1,
		OrderType:     "market",
		RefPrice:      110.27,
		Reason:        "reject R=110.6",
		Status:        "session_blocked",
		Message:       "weekday-only trading schedule blocked signal",
		CreatedAt:     ts,
	}); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	events, err := j.ListEvents(ctx, "fut-brent-mini-lb", 10)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}

	got := events[0]
	if got.Type != "signal_cancelled" || got.Status != "session_blocked" {
		t.Fatalf("event type/status = %q/%q, want signal_cancelled/session_blocked", got.Type, got.Status)
	}
	if got.Reason != "reject R=110.6" || got.Message == "" {
		t.Fatalf("event reason/message = %q/%q", got.Reason, got.Message)
	}
	if got.Time != ts.Format(time.RFC3339) {
		t.Fatalf("event time = %q, want %q", got.Time, ts.Format(time.RFC3339))
	}
}
