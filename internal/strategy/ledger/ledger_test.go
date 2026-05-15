package ledger

import (
	"testing"
)

func TestInstanceLedger_BuyThenSell(t *testing.T) {
	l := NewInstanceLedger()
	if d := l.ApplyFill("u1", "ORDER_DIRECTION_BUY", 10, 100); d != 0 {
		t.Fatalf("expected no realized on open, got %v", d)
	}
	if d := l.ApplyFill("u1", "ORDER_DIRECTION_SELL", 10, 110); d == 0 {
		t.Fatal("expected realized on close")
	}
	q, _, r := l.Snapshot()
	if q["u1"] != 0 && len(q) != 0 {
		t.Fatalf("expected flat, qty=%v", q)
	}
	if r <= 0 {
		t.Fatalf("expected positive realized, got %v", r)
	}
}

func TestInstanceLedger_SellReducesLong(t *testing.T) {
	l := NewInstanceLedger()
	_ = l.ApplyFill("u1", "buy", 10, 100)
	d := l.ApplyFill("u1", "sell", 4, 105)
	if d <= 0 {
		t.Fatalf("expected profit on partial sell, got %v", d)
	}
	q, _, _ := l.Snapshot()
	if q["u1"] != 6 {
		t.Fatalf("qty want 6 got %v", q["u1"])
	}
}
