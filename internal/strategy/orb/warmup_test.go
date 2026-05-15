package orb

import (
	"testing"
	"time"

	"github.com/24alert/trading-bot/internal/strategy"
)

func TestWarmupCandlesReturnRangeCandles(t *testing.T) {
	b := New()
	_ = b.Configure(map[string]string{"range_candles": "3"})
	if got := b.WarmupCandles(); got != 3 {
		t.Fatalf("WarmupCandles() = %d, want 3", got)
	}
}

func TestWarmupFeedsHistoryFormRange(t *testing.T) {
	b := New()
	_ = b.Configure(map[string]string{
		"range_candles": "2",
		"quantity":      "1",
		"cutoff_hour":   "18",
		"cutoff_min":    "30",
	})

	base := time.Date(2026, 5, 15, 10, 0, 0, 0, b.tz)

	// Warmup: 2 candles form the opening range.
	warmupCandles := []strategy.Candle{
		{InstrumentUID: "uid-1", Open: 100, High: 105, Low: 98, Close: 102, Time: base, IsComplete: true},
		{InstrumentUID: "uid-1", Open: 102, High: 110, Low: 99, Close: 108, Time: base.Add(15 * time.Minute), IsComplete: true},
	}

	for _, c := range warmupCandles {
		sigs := b.OnCandle(c)
		if len(sigs) > 0 {
			t.Fatalf("no signal expected during range formation, got %+v", sigs)
		}
	}

	if !b.rangeFormed {
		t.Fatal("range should be formed after warmup")
	}
	if b.rangeHigh != 110 {
		t.Fatalf("rangeHigh = %.1f, want 110", b.rangeHigh)
	}
	if b.rangeLow != 98 {
		t.Fatalf("rangeLow = %.1f, want 98", b.rangeLow)
	}

	// Live candle: breakout above range.
	live := strategy.Candle{
		InstrumentUID: "uid-1",
		Open:          109, High: 115, Low: 109, Close: 112,
		Time:       base.Add(30 * time.Minute),
		IsComplete: true,
	}
	sigs := b.OnCandle(live)
	if len(sigs) != 1 || sigs[0].Direction != "buy" {
		t.Fatalf("expected buy signal after warmup + breakout, got %+v", sigs)
	}
}

func TestWarmupHintInterface(t *testing.T) {
	var _ strategy.WarmupHint = (*Breakout)(nil)
}
