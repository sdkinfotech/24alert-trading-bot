package strategy

import (
	"testing"
	"time"
)

func TestTrackDOMChangesAddPull(t *testing.T) {
	st := newAITraderContextState()
	book1 := &AITraderDOMBook{
		Bids: []AITraderBookLevel{{Price: 100, Quantity: 50}},
		Asks: []AITraderBookLevel{{Price: 101, Quantity: 40}},
	}
	st.trackDOMChangesLocked(book1, mustParseTime("2026-05-20T10:00:00Z"))
	book2 := &AITraderDOMBook{
		Bids: []AITraderBookLevel{{Price: 100, Quantity: 50}, {Price: 99.9, Quantity: 200}},
		Asks: []AITraderBookLevel{},
	}
	st.trackDOMChangesLocked(book2, mustParseTime("2026-05-20T10:00:01Z"))
	if len(st.domEvents) < 2 {
		t.Fatalf("expected dom events, got %d", len(st.domEvents))
	}
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
