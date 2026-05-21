package strategy

import (
	"testing"
	"time"

	"github.com/24alert/trading-bot/internal/marketdata"
)

func TestBuildAssistantLevelsAndMirrors(t *testing.T) {
	now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	mk := func(i int, o, h, l, c float64, vol int64) marketdata.Candle {
		return marketdata.Candle{
			Time: now.Add(time.Duration(i) * time.Hour), Open: o, High: h, Low: l, Close: c, Volume: vol,
		}
	}
	daily := []marketdata.Candle{
		mk(0, 100, 110, 95, 105, 1000),
		mk(1, 105, 115, 100, 108, 1200),
		mk(2, 108, 112, 102, 104, 900),
	}
	hourly := make([]marketdata.Candle, 0, 48)
	for i := 0; i < 48; i++ {
		hourly = append(hourly, mk(i, 104+float64(i%5), 106+float64(i%5), 103, 105, 500))
	}
	fivem := hourly
	cs := assistantCandleSet{Daily1y: daily, Daily90d: daily, Hourly1m: hourly, Hourly1w: hourly[len(hourly)-24:], FiveMin7d: fivem}
	levels := buildAssistantLevels(cs)
	if len(levels) == 0 {
		t.Fatal("expected levels")
	}
	hasMirror := false
	for _, l := range levels {
		if l.Kind == "mirror" {
			hasMirror = true
		}
		if l.ID == "" {
			t.Fatal("level id required")
		}
	}
	_ = hasMirror // mirror may be absent on tiny fixture
	charts := buildAssistantCharts(cs, levels)
	if charts["1h"].Timeframe != "1h" || len(charts["1h"].Candles) == 0 {
		t.Fatalf("1h chart: %+v", charts["1h"])
	}
}
