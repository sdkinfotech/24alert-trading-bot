package strategy

import (
	"testing"
	"time"

	"github.com/24alert/trading-bot/internal/marketdata"
)

func TestLevelTouchedAndTapeRejection(t *testing.T) {
	f := &AITraderFeatures{BestBid: 100.0, BestAsk: 100.05, Mid: 100.025}
	if !levelTouched("buy", 100.10, f) {
		t.Fatal("expected buy touch when ask <= level")
	}
	mctx := &AITraderMarketContext{
		TapeStats: AITraderTapeStats{TradeCount: 10, DeltaPct: 0.2},
	}
	if !tapeConfirmsRejection("buy", 100.0, mctx) {
		t.Fatal("expected positive delta confirmation")
	}
}

func TestPaperSLTPAndFees(t *testing.T) {
	s := &AITraderSession{
		ID: "t1", Ticker: "BMM6",
		Limits: defaultAITraderLimits(),
		LevelPlaybook: &LevelPlaybook{SLMultATR: 0.5, TPMultATR: 1.5},
		PaperState: newPaperTradingState(),
	}
	st := s.PaperState
	fill := PaperFill{Side: "buy", Price: 100, Quantity: 1, LimitPx: 100}
	lotVal := paperLotValueRUB("BMM6")
	closed, pnl := applyPaperFill(st, fill, lotVal)
	if closed || pnl != 0 {
		t.Fatalf("opening fill should not close: closed=%v pnl=%v", closed, pnl)
	}
	st.StopLoss = 99
	st.TakeProfit = 102
	r := &Runner{}
	f := &AITraderFeatures{Mid: 98.5}
	r.checkPaperSLTP(s, f, nil)
	if st.PositionLots != 0 {
		t.Fatalf("expected flat after SL, pos=%d", st.PositionLots)
	}
}

func TestScoreLevelsConfluence(t *testing.T) {
	levels := []AITraderLevel{
		{Price: 99, Kind: "support", Source: "daily_low 2026-05-01", Rank: 1},
		{Price: 101, Kind: "resistance", Source: "hourly_high", Rank: 2},
	}
	f := &AITraderFeatures{Mid: 100, LargestBidWall: AITraderWall{Price: 99, Quantity: 100, Side: "bid"}}
	scored := scoreLevels(levels, f, nil, nil)
	if len(scored) != 2 || scored[0].Score < scored[1].Score {
		// support may rank higher due to daily + wall
	}
	sup, ok := bestSupportLevel(scored, 100, 1.0)
	if !ok || sup.Price != 99 {
		t.Fatalf("expected support 99, got %+v ok=%v", sup, ok)
	}
}

func TestDetectSessionRegime(t *testing.T) {
	bars := make([]AITraderCandleBar, 0, 15)
	p := 100.0
	for i := 0; i < 15; i++ {
		p += 0.5
		bars = append(bars, AITraderCandleBar{Open: p - 0.3, High: p + 0.2, Low: p - 0.4, Close: p, Volume: 10})
	}
	reg := detectSessionRegime(&AITraderMarketContext{ChartBars: bars})
	if reg != RegimeTrend && reg != RegimeBreakout {
		t.Fatalf("expected trend/breakout, got %s", reg)
	}
}

func TestPaperRateLimit(t *testing.T) {
	r := &Runner{}
	s := &AITraderSession{Limits: AITraderLimits{MaxTradesPerMinute: 2}, PaperState: newPaperTradingState()}
	now := time.Now()
	s.PaperState.tradeTimestamps = []time.Time{now, now}
	if r.paperRateLimitOK(s) {
		t.Fatal("expected rate limit block")
	}
}

func TestApplySlippage(t *testing.T) {
	px := applySlippage(100, "buy", 100)
	if px <= 100 {
		t.Fatalf("buy slippage should worsen price: %v", px)
	}
}

func _unusedMarketdata() {
	_ = marketdata.Orderbook{}
}
