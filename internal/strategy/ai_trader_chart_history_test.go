package strategy

import (
	"testing"
	"time"

	"github.com/24alert/trading-bot/internal/marketdata"
)

func TestDedupeMarketCandles(t *testing.T) {
	t0 := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	in := []marketdata.Candle{
		{Time: t0, Close: 1},
		{Time: t0, Close: 2},
		{Time: t0.Add(time.Minute), Close: 3},
	}
	out := dedupeMarketCandles(in)
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
	if out[0].Close != 2 || out[1].Close != 3 {
		t.Fatalf("got %+v", out)
	}
}

func TestTrimChartBars(t *testing.T) {
	bars := make([]AITraderCandleBar, 5)
	for i := range bars {
		bars[i] = AITraderCandleBar{Time: time.Unix(int64(i), 0).UTC().Format(time.RFC3339)}
	}
	got := trimChartBars(bars, 3)
	if len(got) != 3 || got[0].Time != bars[2].Time {
		t.Fatalf("trim: %+v", got)
	}
}
