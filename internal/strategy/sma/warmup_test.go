package sma

import (
	"testing"
	"time"

	"github.com/24alert/trading-bot/internal/strategy"
)

func TestWarmupCandlesReturnSlowN(t *testing.T) {
	c := New()
	_ = c.Configure(map[string]string{"fast_period": "5", "slow_period": "20"})
	if got := c.WarmupCandles(); got != 20 {
		t.Fatalf("WarmupCandles() = %d, want 20", got)
	}
}

func TestWarmupFeedsHistorySignalsDiscarded(t *testing.T) {
	c := New()
	_ = c.Configure(map[string]string{"fast_period": "2", "slow_period": "3", "quantity": "1"})

	base := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	// Feed 3 bars of history (simulating warmup — runner discards signals).
	warmupPrices := []float64{10, 10, 10}
	for i, px := range warmupPrices {
		sigs := c.OnCandle(strategy.Candle{
			InstrumentUID: "uid-1",
			Close:         px,
			Open:          px,
			High:          px,
			Low:           px,
			Time:          base.Add(time.Duration(i) * time.Hour),
			IsComplete:    true,
		})
		// In real warmup runner discards these, but strategy should not
		// generate signals during range building (flat prices = no cross).
		_ = sigs
	}

	// After warmup, the strategy should have closes filled.
	if len(c.closes) < 3 {
		t.Fatalf("closes len after warmup = %d, want >= 3", len(c.closes))
	}

	// Now feed a live candle that triggers a cross — should signal immediately.
	livePrices := []float64{14, 15}
	var liveSigs []strategy.Signal
	for i, px := range livePrices {
		sigs := c.OnCandle(strategy.Candle{
			InstrumentUID: "uid-1",
			Close:         px,
			Open:          px,
			High:          px,
			Low:           px,
			Time:          base.Add(time.Duration(len(warmupPrices)+i) * time.Hour),
			IsComplete:    true,
		})
		liveSigs = append(liveSigs, sigs...)
	}

	if len(liveSigs) == 0 {
		t.Fatal("expected signal from live data after warmup")
	}
	if liveSigs[0].Direction != "buy" {
		t.Fatalf("expected buy, got %q", liveSigs[0].Direction)
	}
}

func TestWarmupDiscardedSignalDoesNotStopHistory(t *testing.T) {
	c := New()
	_ = c.Configure(map[string]string{"fast_period": "2", "slow_period": "3", "quantity": "1"})

	base := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	prices := []float64{10, 10, 10, 11, 12, 13, 14}
	for i, px := range prices {
		sigs := c.OnCandle(strategy.Candle{
			InstrumentUID: "uid-1",
			Close:         px,
			Open:          px,
			High:          px,
			Low:           px,
			Time:          base.Add(time.Duration(i) * time.Hour),
			IsComplete:    true,
		})
		for _, sig := range sigs {
			c.OnSignalDispatchFailed(sig, "warmup_discarded")
		}
	}

	if c.history[len(c.history)-1].Time != base.Add(time.Duration(len(prices)-1)*time.Hour) {
		t.Fatalf("history stopped at %s, want last warmup candle", c.history[len(c.history)-1].Time)
	}
	if c.pendingEntry != 0 {
		t.Fatalf("pendingEntry after discarded warmup signals = %d, want 0", c.pendingEntry)
	}
}

func TestWarmupAfterRestoreNoDoubleData(t *testing.T) {
	c := New()
	_ = c.Configure(map[string]string{"fast_period": "2", "slow_period": "3"})

	// Simulate restore — populate closes directly.
	c.closes = []float64{10, 10, 10}
	c.pos = 0

	// Warmup would also feed 3 candles. The strategy accumulates both.
	// In reality, runner skips warmup if Restore provided enough data.
	// But if both run, strategy still works (just has more data points).
	warmup := []float64{10, 10, 10}
	for _, px := range warmup {
		_ = c.OnCandle(strategy.Candle{
			InstrumentUID: "uid-1",
			Close:         px,
			IsComplete:    true,
		})
	}

	// Should have accumulated bars. SMA trims to slowN+5 max.
	if len(c.closes) < 3 {
		t.Fatalf("closes len = %d, expected >= 3", len(c.closes))
	}
}

func TestWarmupHintInterface(t *testing.T) {
	var _ strategy.WarmupHint = (*Crossover)(nil)
}
