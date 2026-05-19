package advisor

import (
	"testing"
	"time"
)

func TestBuildFactsFromSnapshots(t *testing.T) {
	payload := `{"features":{"spread_bps":12.5,"imbalance":0.2},"market_context":{"tape_stats":{"trade_count":10,"buy_volume":100,"sell_volume":80,"delta_pct":0.11,"last_price":100.5},"recent_prints":[{"side":"buy","quantity":5,"price":100.4}],"scene_notes":["bid wall held"]},"events":[]}`
	snaps := []MicroSnapshot{{
		SessionID:   "s1",
		CapturedAt:  time.Now().UTC(),
		PayloadJSON: payload,
	}}
	events := []DecisionEvent{{
		Time:    time.Now().UTC().Format(time.RFC3339),
		Summary: "test thought",
	}}
	fb := BuildFactsFromSnapshots(snaps, events, "SBER", "10:00–10:05")
	if fb.SnapshotCount != 1 {
		t.Fatalf("snapshots=%d", fb.SnapshotCount)
	}
	if fb.SpreadMax != 12.5 {
		t.Fatalf("spread max=%v", fb.SpreadMax)
	}
	if fb.TapeSummary == "" {
		t.Fatal("expected tape summary")
	}
	if fb.TextDigest == "" {
		t.Fatal("expected digest")
	}
}

func TestLastClosedPeriodEnd5m(t *testing.T) {
	loc := MSK()
	// 10:07 MSK → last closed bucket ends at 10:05
	now := time.Date(2026, 5, 19, 10, 7, 0, 0, loc).UTC()
	end := LastClosedPeriodEnd(now, TF5m)
	want := time.Date(2026, 5, 19, 10, 5, 0, 0, loc).UTC()
	if !end.Equal(want) {
		t.Fatalf("got %v want %v", end.In(loc), want.In(loc))
	}
}
