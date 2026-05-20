package strategy

import (
	"context"
	"testing"
)

func TestCancelAllLiveOrdersClearsRunnerState(t *testing.T) {
	r := &Runner{}
	s := &AITraderSession{
		LiveState: &LiveTradingState{
			WorkingOrders: []LiveOrder{
				{ID: "lo-1", Side: "buy", Price: 99, Quantity: 1, Status: "working", BrokerOrderID: "broker-1"},
			},
		},
	}
	r.cancelAllLiveOrders(context.Background(), s)
	if len(s.LiveState.WorkingOrders) != 0 {
		t.Fatalf("expected WorkingOrders cleared, got %d", len(s.LiveState.WorkingOrders))
	}
}

func TestStartLiveTradingFromPlaybookCancelsBeforePlace(t *testing.T) {
	r := &Runner{}
	s := &AITraderSession{
		ID: "test", AccountID: "acc", InstrumentID: "uid", Ticker: "TST",
		LevelPlaybook: &LevelPlaybook{Levels: []AITraderLevel{{Price: 100, Kind: "support", Source: "test"}}},
		LiveState: &LiveTradingState{
			WorkingOrders: []LiveOrder{
				{ID: "lo-1", Side: "buy", Price: 99, Quantity: 1, Status: "working", BrokerOrderID: "broker-1"},
			},
		},
		Limits: defaultAITraderLimits(),
	}
	f := &AITraderFeatures{Mid: 100, BestBid: 99.9, BestAsk: 100.1, SpreadBPS: 2}
	r.startLiveTradingFromPlaybook(context.Background(), s, f, nil, nil)
	if len(s.LiveState.WorkingOrders) != 0 {
		t.Fatalf("expected no working orders without orderSvc, got %d", len(s.LiveState.WorkingOrders))
	}
}
