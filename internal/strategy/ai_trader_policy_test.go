package strategy

import "testing"

func TestClampDynamicTradingPolicyEntryConfidence(t *testing.T) {
	p := DynamicTradingPolicy{EntryMinConfidence: 0.99, ConfluenceMinScore: 10, SLMultATR: 0.01, TPMultATR: 10}
	c := clampDynamicTradingPolicy(p)
	if c.EntryMinConfidence != defaultPolicyEntryMinConfidence {
		t.Fatalf("entry conf clamp: got %.2f want %.2f", c.EntryMinConfidence, defaultPolicyEntryMinConfidence)
	}
	if c.ConfluenceMinScore != 5.0 {
		t.Fatalf("confluence clamp: got %.1f", c.ConfluenceMinScore)
	}
	if c.SLMultATR != 0.2 {
		t.Fatalf("sl mult clamp: got %.2f", c.SLMultATR)
	}
}

func TestMergeLLMPolicyIntoSession(t *testing.T) {
	s := &AITraderSession{
		LevelPlaybook: &LevelPlaybook{SLMultATR: 0.5, TPMultATR: 1.5, MarketBias: "neutral"},
	}
	llm := &aiTraderLLMTradingPolicy{
		MarketBias:         "bullish",
		EntryMinConfidence: 0.45,
		ConfluenceMinScore: 2.0,
		SLMultATR:          0.6,
		AllowNewEntry:      boolPtr(true),
	}
	pol, changed := mergeLLMPolicyIntoSession(s, llm, s.LevelPlaybook)
	if !changed {
		t.Fatal("expected policy change")
	}
	if pol.MarketBias != "bullish" {
		t.Fatalf("bias=%s", pol.MarketBias)
	}
	if pol.EntryMinConfidence != 0.45 {
		t.Fatalf("entry conf=%.2f", pol.EntryMinConfidence)
	}
}

func TestValidateSoftStopLong(t *testing.T) {
	mid := 100.0
	stop := validateSoftStop(1, mid, 98.0)
	if stop != 98.0 {
		t.Fatalf("want 98 got %.4f", stop)
	}
	bad := validateSoftStop(1, mid, 102.0)
	if bad != 0 {
		t.Fatalf("stop above mid should reject")
	}
}

func TestApplyLLMPositionManagement(t *testing.T) {
	s := &AITraderSession{
		LiveState: &LiveTradingState{PositionLots: 1, AvgPrice: 100, StopLoss: 99, TakeProfit: 101},
	}
	sig := &AITraderTradeSignal{OrderAction: "adjust_stops", StopLoss: 98.5, TakeProfit: 102}
	if !applyLLMPositionManagement(s, sig, 100.2, true) {
		t.Fatal("expected adjustment")
	}
	if s.LiveState.StopLoss != 98.5 {
		t.Fatalf("sl=%.4f", s.LiveState.StopLoss)
	}
}

func boolPtr(b bool) *bool { return &b }
