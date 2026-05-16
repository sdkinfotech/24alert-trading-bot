package sma

import (
	"testing"

	"github.com/24alert/trading-bot/internal/strategy"
)

func TestCrossoverGoldenCross(t *testing.T) {
	c := New()
	if err := c.Configure(map[string]string{
		"fast_period": "2",
		"slow_period": "3",
		"quantity":    "1",
	}); err != nil {
		t.Fatal(err)
	}
	// Build uptrend so fast SMA crosses above slow on last bar.
	prices := []float64{10, 10, 10, 11, 12, 13, 14}
	var sigs []strategy.Signal
	for _, px := range prices {
		if s := c.OnCandle(strategy.Candle{
			InstrumentUID: "uid-1",
			Close:         px,
			IsComplete:    true,
		}); len(s) > 0 {
			sigs = append(sigs, s...)
		}
	}
	if len(sigs) == 0 {
		t.Fatal("expected at least one signal")
	}
	last := sigs[len(sigs)-1]
	if last.Direction != "buy" {
		t.Fatalf("want buy, got %q", last.Direction)
	}
}

func TestCrossoverDeathCross(t *testing.T) {
	c := New()
	if err := c.Configure(map[string]string{
		"fast_period": "2",
		"slow_period": "3",
		"quantity":    "1",
	}); err != nil {
		t.Fatal(err)
	}
	prices := []float64{10, 10, 10, 9, 8, 7, 6}
	var sigs []strategy.Signal
	for _, px := range prices {
		if s := c.OnCandle(strategy.Candle{
			InstrumentUID: "uid-1",
			Close:         px,
			IsComplete:    true,
		}); len(s) > 0 {
			sigs = append(sigs, s...)
		}
	}
	if len(sigs) == 0 {
		t.Fatal("expected at least one signal")
	}
	last := sigs[len(sigs)-1]
	if last.Direction != "sell" {
		t.Fatalf("want sell, got %q", last.Direction)
	}
}

func TestCrossoverNoDuplicate(t *testing.T) {
	c := New()
	_ = c.Configure(map[string]string{"fast_period": "2", "slow_period": "3"})
	prices := []float64{10, 10, 10, 11, 12, 13, 14, 15}
	var buys int
	for _, px := range prices {
		if s := c.OnCandle(strategy.Candle{
			InstrumentUID: "uid-1",
			Close:         px,
			IsComplete:    true,
		}); len(s) > 0 {
			for _, sig := range s {
				if sig.Direction == "buy" {
					buys++
				}
			}
		}
	}
	if buys > 1 {
		t.Fatalf("should not send duplicate buy signals, got %d", buys)
	}
}

func TestCrossoverNoSignalBeforeWarmup(t *testing.T) {
	c := New()
	_ = c.Configure(map[string]string{"fast_period": "2", "slow_period": "3"})
	s := c.OnCandle(strategy.Candle{InstrumentUID: "u", Close: 10, IsComplete: true})
	if s != nil {
		t.Fatalf("unexpected signal: %+v", s)
	}
}

func goldenCrossSetup(t *testing.T) (*Crossover, []strategy.Signal) {
	t.Helper()
	c := New()
	_ = c.Configure(map[string]string{"fast_period": "2", "slow_period": "3", "quantity": "1"})
	prices := []float64{10, 10, 10, 11, 12, 13, 14}
	var sigs []strategy.Signal
	for _, px := range prices {
		if s := c.OnCandle(strategy.Candle{InstrumentUID: "uid-1", Close: px, IsComplete: true}); len(s) > 0 {
			sigs = append(sigs, s...)
		}
	}
	return c, sigs
}

func TestCrossover_pendingEntry_setOnSignal(t *testing.T) {
	c, sigs := goldenCrossSetup(t)
	if len(sigs) == 0 {
		t.Fatal("expected buy signal")
	}
	if c.pendingEntry != 1 {
		t.Fatalf("pendingEntry want 1 got %d", c.pendingEntry)
	}
	if c.pos != 0 {
		t.Fatalf("pos should stay 0 until fill, got %d", c.pos)
	}
}

func TestCrossover_pendingEntry_blocksNewSignals(t *testing.T) {
	c, _ := goldenCrossSetup(t)
	// While pending, new candles should not generate signals
	sigs := c.OnCandle(strategy.Candle{InstrumentUID: "uid-1", Close: 15, IsComplete: true})
	if sigs != nil {
		t.Fatalf("should block signals while pendingEntry != 0, got %+v", sigs)
	}
}

func TestCrossover_OnExecution_filled_setsPos(t *testing.T) {
	c, _ := goldenCrossSetup(t)
	c.OnExecution(strategy.ExecutionEvent{Status: "filled"})
	if c.pos != 1 || c.pendingEntry != 0 {
		t.Fatalf("after fill: want pos=1 pending=0, got pos=%d pending=%d", c.pos, c.pendingEntry)
	}
}

func TestCrossover_OnExecution_rejected_clearsPending(t *testing.T) {
	c, _ := goldenCrossSetup(t)
	c.OnExecution(strategy.ExecutionEvent{Status: "rejected", Message: "order reject: insufficient funds"})
	if c.pendingEntry != 0 {
		t.Fatalf("pending should clear on reject, got %d", c.pendingEntry)
	}
	if c.pos != 0 {
		t.Fatalf("pos should stay 0 on reject, got %d", c.pos)
	}
}

func TestCrossover_OnSignalDispatchFailed_clearsPending(t *testing.T) {
	c, sigs := goldenCrossSetup(t)
	if len(sigs) == 0 {
		t.Fatal("expected signal")
	}
	c.OnSignalDispatchFailed(sigs[0], "risk_rejected")
	if c.pendingEntry != 0 {
		t.Fatalf("pendingEntry should be 0 after dispatch failure, got %d", c.pendingEntry)
	}
}

func TestCrossover_ResetTradingState_clearsPending(t *testing.T) {
	c := New()
	c.pendingEntry = 1
	c.pos = 1
	c.ResetTradingStateAfterWarmup()
	if c.pos != 0 || c.pendingEntry != 0 {
		t.Fatalf("want pos=0 pending=0 after reset, got pos=%d pending=%d", c.pos, c.pendingEntry)
	}
}

func TestCrossover_Snapshot_includesPendingEntry(t *testing.T) {
	c, _ := goldenCrossSetup(t)
	blob, err := c.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	c2 := New()
	_ = c2.Configure(map[string]string{"fast_period": "2", "slow_period": "3"})
	if err := c2.Restore(blob); err != nil {
		t.Fatal(err)
	}
	if c2.pendingEntry != c.pendingEntry {
		t.Fatalf("pendingEntry not restored: want %d got %d", c.pendingEntry, c2.pendingEntry)
	}
}
