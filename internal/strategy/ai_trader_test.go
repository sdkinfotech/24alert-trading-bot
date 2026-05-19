package strategy

import (
	"testing"
	"time"

	"github.com/24alert/trading-bot/internal/marketdata"
)

func TestComputeAITraderFeatures(t *testing.T) {
	book := &marketdata.Orderbook{
		InstrumentUID: "uid1",
		Depth:         20,
		Time:          time.Now().UTC(),
		Bids: []marketdata.OrderbookRow{
			{Price: 100, Quantity: 10},
			{Price: 99.9, Quantity: 20},
		},
		Asks: []marketdata.OrderbookRow{
			{Price: 100.2, Quantity: 5},
			{Price: 100.3, Quantity: 5},
		},
	}
	f := computeAITraderFeatures(book, "TEST", 1500)
	if f.BestBid != 100 || f.BestAsk != 100.2 {
		t.Fatalf("unexpected best bid/ask: %+v", f)
	}
	if f.Mid != 100.1 {
		t.Fatalf("unexpected mid: %v", f.Mid)
	}
	if f.TopBidVolume != 30 || f.TopAskVolume != 10 {
		t.Fatalf("unexpected top volumes: bid=%d ask=%d", f.TopBidVolume, f.TopAskVolume)
	}
	if f.Imbalance <= 0 {
		t.Fatalf("expected positive imbalance: %v", f.Imbalance)
	}
	if len(f.OrderBookSnapshot.Bids) != 2 || f.OrderBookSnapshot.Bids[0].Price != 100 {
		t.Fatalf("unexpected snapshot: %+v", f.OrderBookSnapshot)
	}
}

func TestDecideAITraderRejectsStale(t *testing.T) {
	s := &AITraderSession{
		ID:     "s1",
		Mode:   AITraderModePaper,
		Limits: defaultAITraderLimits(),
	}
	f := &AITraderFeatures{Stale: true, DataFreshnessMS: 5000}
	ev := decideAITraderRules(s, f)
	if ev.Action != "block" || ev.RiskResult != "blocked_stale_feed" {
		t.Fatalf("unexpected stale decision: %+v", ev)
	}
	if ev.Summary == "" || ev.NextWatch == "" || ev.OperatorNote == "" || ev.MarketBias != "blocked" {
		t.Fatalf("expected human-readable blocked conclusion: %+v", ev)
	}
}

func TestDecideAITraderBuildsReadableConclusion(t *testing.T) {
	s := &AITraderSession{
		ID:           "s1",
		StrategyKind: AITraderStrategyLevelIntraday,
		Phase:        AITraderPhaseTrading,
		Limits:       defaultAITraderLimits(),
	}
	f := &AITraderFeatures{
		BestBid:      100,
		BestAsk:      100.1,
		SpreadBPS:    1,
		TopBidVolume: 200,
		TopAskVolume: 600,
		Imbalance:    -0.5,
	}
	ev := decideAITraderRules(s, f)
	if ev.Action != "paper_plan" || ev.MarketBias != "bearish" {
		t.Fatalf("unexpected pressure decision: %+v", ev)
	}
	if ev.Summary == "" || ev.NextWatch == "" {
		t.Fatalf("expected readable conclusion: %+v", ev)
	}
}
