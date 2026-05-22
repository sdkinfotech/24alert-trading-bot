package sma

import (
	"math"
	"testing"
	"time"

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
	c.signals = []SignalPoint{{Direction: "buy"}}
	c.ResetTradingStateAfterWarmup()
	if c.pos != 0 || c.pendingEntry != 0 {
		t.Fatalf("want pos=0 pending=0 after reset, got pos=%d pending=%d", c.pos, c.pendingEntry)
	}
	if len(c.signals) != 0 {
		t.Fatalf("warmup signals should be cleared, got %d", len(c.signals))
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

func TestCrossover_Restore_keepsConfiguredPeriods(t *testing.T) {
	c := New()
	if err := c.Configure(map[string]string{"fast_period": "9", "slow_period": "26", "quantity": "1"}); err != nil {
		t.Fatal(err)
	}
	blob, err := c.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	c2 := New()
	if err := c2.Configure(map[string]string{"fast_period": "5", "slow_period": "17", "quantity": "1"}); err != nil {
		t.Fatal(err)
	}
	if err := c2.Restore(blob); err != nil {
		t.Fatal(err)
	}

	if c2.fastN != 5 || c2.slowN != 17 {
		t.Fatalf("Restore must keep configured periods, got fast=%d slow=%d", c2.fastN, c2.slowN)
	}
}

func TestCrossover_TrailingStop_LongExitsFlat(t *testing.T) {
	c := New()
	if err := c.Configure(map[string]string{
		"fast_period":       "2",
		"slow_period":       "3",
		"quantity":          "1",
		"trailing_stop_pct": "0.10",
	}); err != nil {
		t.Fatal(err)
	}
	c.SyncBrokerPosition("uid-1", 1, 100, 100)
	c.closes = []float64{100, 100}

	if sigs := c.OnCandle(strategy.Candle{
		InstrumentUID: "uid-1",
		High:          110,
		Low:           109,
		Close:         109,
		IsComplete:    true,
	}); len(sigs) != 0 {
		t.Fatalf("unexpected signal before trail break: %+v", sigs)
	}
	sigs := c.OnCandle(strategy.Candle{
		InstrumentUID: "uid-1",
		High:          109,
		Low:           98,
		Close:         100,
		IsComplete:    true,
	})
	if len(sigs) != 1 || sigs[0].Direction != "sell" {
		t.Fatalf("want trailing sell, got %+v", sigs)
	}
	if !c.pendingExit {
		t.Fatal("trailing exit must set pendingExit")
	}
	c.OnExecution(strategy.ExecutionEvent{Status: "filled", AvgPrice: 99})
	if c.pos != 0 || c.pendingExit {
		t.Fatalf("filled trailing exit should flatten, pos=%d pendingExit=%t", c.pos, c.pendingExit)
	}
}

func TestCrossover_TrailingStop_ShortExitsFlat(t *testing.T) {
	c := New()
	if err := c.Configure(map[string]string{
		"fast_period":       "2",
		"slow_period":       "3",
		"quantity":          "1",
		"trailing_stop_pct": "0.10",
	}); err != nil {
		t.Fatal(err)
	}
	c.SyncBrokerPosition("uid-1", -1, 100, 100)
	c.closes = []float64{100, 100}

	if sigs := c.OnCandle(strategy.Candle{
		InstrumentUID: "uid-1",
		High:          91,
		Low:           90,
		Close:         91,
		IsComplete:    true,
	}); len(sigs) != 0 {
		t.Fatalf("unexpected signal before trail break: %+v", sigs)
	}
	sigs := c.OnCandle(strategy.Candle{
		InstrumentUID: "uid-1",
		High:          100,
		Low:           89,
		Close:         99,
		IsComplete:    true,
	})
	if len(sigs) != 1 || sigs[0].Direction != "buy" {
		t.Fatalf("want trailing buy, got %+v", sigs)
	}
	c.OnExecution(strategy.ExecutionEvent{Status: "filled", AvgPrice: 98})
	if c.pos != 0 || c.pendingExit {
		t.Fatalf("filled trailing exit should flatten, pos=%d pendingExit=%t", c.pos, c.pendingExit)
	}
}

func TestCrossover_LiveTrailingStop_ShortMovesBeforeClose(t *testing.T) {
	c := New()
	if err := c.Configure(map[string]string{
		"fast_period":       "2",
		"slow_period":       "3",
		"quantity":          "1",
		"trailing_stop_pct": "0.005",
	}); err != nil {
		t.Fatal(err)
	}
	c.SyncBrokerPosition("uid-1", -1, 110.36, 109.65)

	sigs := c.OnLiveCandle(strategy.Candle{
		InstrumentUID: "uid-1",
		High:          110.2,
		Low:           108.8,
		Close:         108.8,
		IsComplete:    false,
	})
	if len(sigs) != 0 {
		t.Fatalf("live trail move should not exit yet, got %+v", sigs)
	}
	if c.trailingBest != 108.8 {
		t.Fatalf("short live trailing best should move to low, got %.4f", c.trailingBest)
	}
	wantStop := 108.8 * 1.005
	if got := c.trailingStopPrice(); math.Abs(got-wantStop) > 1e-9 {
		t.Fatalf("live trailing stop not moved: want %.4f got %.4f", wantStop, got)
	}
}

func TestCrossover_LiveTrailingStop_ShortExitsOnCurrentPrice(t *testing.T) {
	c := New()
	if err := c.Configure(map[string]string{
		"fast_period":       "2",
		"slow_period":       "3",
		"quantity":          "1",
		"trailing_stop_pct": "0.005",
	}); err != nil {
		t.Fatal(err)
	}
	c.SyncBrokerPosition("uid-1", -1, 110.36, 108.8)

	sigs := c.OnLiveCandle(strategy.Candle{
		InstrumentUID: "uid-1",
		High:          109.4,
		Low:           108.8,
		Close:         109.35,
		IsComplete:    false,
	})
	if len(sigs) != 1 || sigs[0].Direction != "buy" {
		t.Fatalf("want live trailing buy, got %+v", sigs)
	}
	if sigs[0].Reason != "sma live trailing stop 0.50%" {
		t.Fatalf("unexpected reason: %q", sigs[0].Reason)
	}
	if !c.pendingExit {
		t.Fatal("live trailing exit must set pendingExit")
	}
}

func TestCrossover_SwingStructuralStopShort(t *testing.T) {
	c := New()
	if err := c.Configure(map[string]string{
		"fast_period":             "2",
		"slow_period":             "3",
		"quantity":                "1",
		"trailing_stop_pct":       "0.008",
		"initial_stop_swing_bars": "3",
	}); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	for i, h := range []float64{108.0, 107.5, 106.0} {
		c.history = append(c.history, CandlePoint{
			Time: base.Add(time.Duration(i) * time.Hour), High: h, Low: h - 1, Close: h - 0.5,
		})
	}
	c.pos = -1
	c.structuralStop = c.swingStructuralStop(-1)
	if math.Abs(c.structuralStop-108.0) > 1e-9 {
		t.Fatalf("structural stop want 108 (prev high), got %.4f", c.structuralStop)
	}
	stop, ok := c.ProtectiveStopPrice("uid", -1, 106.07)
	if !ok || math.Abs(stop-108.0) > 1e-9 {
		t.Fatalf("broker stop want 108, got %.4f ok=%v", stop, ok)
	}
	c.trailingBest = 106.0
	wantTrail := 106.0 * 1.008
	if got := c.trailingStopPrice(); math.Abs(got-wantTrail) > 1e-9 {
		t.Fatalf("trailing at 0.8%% want %.4f got %.4f", wantTrail, got)
	}
}
