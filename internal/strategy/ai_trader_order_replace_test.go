package strategy

import (
	"testing"
)

func TestReplacePaperOrderIfNeeded(t *testing.T) {
	r := &Runner{}
	s := &AITraderSession{
		Limits: defaultAITraderLimits(),
		PaperState: &PaperTradingState{
			WorkingOrders: []PaperOrder{
				{ID: "po-1", Side: "buy", Price: 84.10, Quantity: 1, Status: "working"},
			},
		},
		ActivePolicy: &DynamicTradingPolicy{EntryMinConfidence: 0.5},
	}
	sig := &AITraderTradeSignal{
		Side: "buy", LevelPrice: 84.25, Confidence: 0.7,
		OrderAction: "replace_limit", Reason: "mid moved",
	}
	if !r.replacePaperOrderIfNeeded(s, sig) {
		t.Fatal("expected replace")
	}
	if len(s.PaperState.WorkingOrders) != 1 {
		t.Fatalf("expected 1 working order, got %d", len(s.PaperState.WorkingOrders))
	}
	if s.PaperState.WorkingOrders[0].Price != 84.25 {
		t.Fatalf("expected price 84.25, got %.4f", s.PaperState.WorkingOrders[0].Price)
	}
}
