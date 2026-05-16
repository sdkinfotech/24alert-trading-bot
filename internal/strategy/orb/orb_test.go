package orb

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/24alert/trading-bot/internal/strategy"
)

var msk = time.FixedZone("MSK", 3*3600)

func candle(t time.Time, o, h, l, c float64) strategy.Candle {
	return strategy.Candle{
		InstrumentUID: "sber-uid",
		Open:          o, High: h, Low: l, Close: c,
		Time:       t,
		IsComplete: true,
	}
}

func newORB() *Breakout {
	b := New()
	_ = b.Configure(map[string]string{
		"range_candles": "2",
		"quantity":      "1",
		"cutoff_hour":   "18",
		"cutoff_min":    "30",
		"timezone":      "Etc/GMT-3", // +03:00
	})
	return b
}

func TestORBFormRange(t *testing.T) {
	b := newORB()
	day := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	// Candle 1: 10:00-10:15 — no signal, range forming
	sigs := b.OnCandle(candle(day, 320, 325, 318, 322))
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals during range formation, got %d", len(sigs))
	}
	if b.rangeFormed {
		t.Fatal("range should not be formed after 1 candle")
	}

	// Candle 2: 10:15-10:30 — range completes
	sigs = b.OnCandle(candle(day.Add(15*time.Minute), 322, 328, 319, 326))
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals when range just formed, got %d", len(sigs))
	}
	if !b.rangeFormed {
		t.Fatal("range should be formed after 2 candles")
	}
	if b.rangeHigh != 328 {
		t.Errorf("rangeHigh = %v, want 328", b.rangeHigh)
	}
	if b.rangeLow != 318 {
		t.Errorf("rangeLow = %v, want 318", b.rangeLow)
	}
}

func TestORBBuyBreakout(t *testing.T) {
	b := newORB()
	day := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	b.OnCandle(candle(day, 320, 325, 318, 322))
	b.OnCandle(candle(day.Add(15*time.Minute), 322, 328, 319, 326))

	// Candle 3: close above rangeHigh (328) → BUY
	sigs := b.OnCandle(candle(day.Add(30*time.Minute), 326, 330, 325, 329))
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Direction != "buy" {
		t.Errorf("direction = %q, want buy", sigs[0].Direction)
	}
	if sigs[0].Quantity != 1 {
		t.Errorf("quantity = %d, want 1", sigs[0].Quantity)
	}
	if b.pendingEntry != 1 {
		t.Errorf("pendingEntry = %d, want 1", b.pendingEntry)
	}
	// Confirm fill
	b.OnExecution(strategy.ExecutionEvent{Status: "filled"})
	if b.pos != 1 {
		t.Errorf("pos = %d after fill, want 1", b.pos)
	}
}

func TestORBSellBreakout(t *testing.T) {
	b := newORB()
	day := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	b.OnCandle(candle(day, 320, 325, 318, 322))
	b.OnCandle(candle(day.Add(15*time.Minute), 322, 328, 319, 326))

	// Candle 3: close below rangeLow (318) → SELL
	sigs := b.OnCandle(candle(day.Add(30*time.Minute), 320, 321, 316, 317))
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Direction != "sell" {
		t.Errorf("direction = %q, want sell", sigs[0].Direction)
	}
	if b.pendingEntry != -1 {
		t.Errorf("pendingEntry = %d, want -1", b.pendingEntry)
	}
	b.OnExecution(strategy.ExecutionEvent{Status: "filled"})
	if b.pos != -1 {
		t.Errorf("pos = %d after fill, want -1", b.pos)
	}
}

func TestORBReversePosition(t *testing.T) {
	b := newORB()
	day := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	b.OnCandle(candle(day, 320, 325, 318, 322))
	b.OnCandle(candle(day.Add(15*time.Minute), 322, 328, 319, 326))

	// Breakout long + confirm fill
	b.OnCandle(candle(day.Add(30*time.Minute), 326, 330, 325, 329))
	b.OnExecution(strategy.ExecutionEvent{Status: "filled"})

	// Now close below rangeLow — should reverse with 2x quantity
	sigs := b.OnCandle(candle(day.Add(45*time.Minute), 325, 325, 315, 316))
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].Direction != "sell" {
		t.Errorf("direction = %q, want sell", sigs[0].Direction)
	}
	if sigs[0].Quantity != 2 {
		t.Errorf("quantity = %d, want 2 (reverse)", sigs[0].Quantity)
	}
	b.OnExecution(strategy.ExecutionEvent{Status: "filled"})
	if b.pos != -1 {
		t.Errorf("pos = %d after fill, want -1", b.pos)
	}
}

func TestORBEndOfDay(t *testing.T) {
	b := newORB()
	day := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	b.OnCandle(candle(day, 320, 325, 318, 322))
	b.OnCandle(candle(day.Add(15*time.Minute), 322, 328, 319, 326))
	b.OnCandle(candle(day.Add(30*time.Minute), 326, 330, 325, 329)) // long pending
	b.OnExecution(strategy.ExecutionEvent{Status: "filled"})        // confirm long

	// EOD candle at 18:30 MSK — should close long position
	eod := time.Date(2026, 5, 15, 18, 30, 0, 0, msk)
	sigs := b.OnCandle(candle(eod, 332, 333, 330, 331))
	if len(sigs) != 1 {
		t.Fatalf("expected 1 eod signal, got %d", len(sigs))
	}
	if sigs[0].Direction != "sell" {
		t.Errorf("eod direction = %q, want sell (closing long)", sigs[0].Direction)
	}
	if !b.pendingExit {
		t.Error("pendingExit should be true after EOD signal")
	}
	b.OnExecution(strategy.ExecutionEvent{Status: "filled"})
	if b.pos != 0 {
		t.Errorf("pos = %d after EOD fill, want 0", b.pos)
	}

	// Next candle at 18:45 — should NOT produce another signal
	sigs = b.OnCandle(candle(eod.Add(15*time.Minute), 331, 332, 330, 331))
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals after EOD close, got %d", len(sigs))
	}
}

func TestORBEndOfDayShort(t *testing.T) {
	b := newORB()
	day := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	b.OnCandle(candle(day, 320, 325, 318, 322))
	b.OnCandle(candle(day.Add(15*time.Minute), 322, 328, 319, 326))
	b.OnCandle(candle(day.Add(30*time.Minute), 320, 321, 316, 317)) // short pending
	b.OnExecution(strategy.ExecutionEvent{Status: "filled"})        // confirm short

	eod := time.Date(2026, 5, 15, 18, 30, 0, 0, msk)
	sigs := b.OnCandle(candle(eod, 315, 316, 314, 315))
	if len(sigs) != 1 {
		t.Fatalf("expected 1 eod signal, got %d", len(sigs))
	}
	if sigs[0].Direction != "buy" {
		t.Errorf("eod direction = %q, want buy (closing short)", sigs[0].Direction)
	}
}

func TestORBNewDayReset(t *testing.T) {
	b := newORB()
	day1 := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	b.OnCandle(candle(day1, 320, 325, 318, 322))
	b.OnCandle(candle(day1.Add(15*time.Minute), 322, 328, 319, 326))
	b.OnCandle(candle(day1.Add(30*time.Minute), 326, 330, 325, 329)) // long pending
	b.OnExecution(strategy.ExecutionEvent{Status: "filled"})         // confirm

	// New day — range/counter resets but position state carries over until explicitly closed
	day2 := time.Date(2026, 5, 16, 10, 0, 0, 0, msk)
	sigs := b.OnCandle(candle(day2, 330, 335, 328, 332))
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals on new day first candle, got %d", len(sigs))
	}
	if b.rangeFormed {
		t.Error("range should not be formed after day reset first candle")
	}
	if b.currentDay != "2026-05-16" {
		t.Errorf("currentDay = %q, want 2026-05-16", b.currentDay)
	}
}

func TestORBNoDuplicateSignals(t *testing.T) {
	b := newORB()
	day := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	b.OnCandle(candle(day, 320, 325, 318, 322))
	b.OnCandle(candle(day.Add(15*time.Minute), 322, 328, 319, 326))

	// First breakout long
	sigs := b.OnCandle(candle(day.Add(30*time.Minute), 326, 330, 325, 329))
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	b.OnExecution(strategy.ExecutionEvent{Status: "filled"})

	// Second candle also above rangeHigh — no duplicate (pos already 1)
	sigs = b.OnCandle(candle(day.Add(45*time.Minute), 329, 332, 328, 331))
	if len(sigs) != 0 {
		t.Fatalf("expected 0 duplicate signals, got %d", len(sigs))
	}
}

func TestORBNoSignalWithinRange(t *testing.T) {
	b := newORB()
	day := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	b.OnCandle(candle(day, 320, 325, 318, 322))
	b.OnCandle(candle(day.Add(15*time.Minute), 322, 328, 319, 326))

	// Close within range — no signal
	sigs := b.OnCandle(candle(day.Add(30*time.Minute), 320, 327, 319, 325))
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals within range, got %d", len(sigs))
	}
}

func TestORBIncompleteCandle(t *testing.T) {
	b := newORB()
	day := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	incomplete := candle(day, 320, 325, 318, 322)
	incomplete.IsComplete = false
	sigs := b.OnCandle(incomplete)
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals for incomplete candle, got %d", len(sigs))
	}
}

func TestORBSnapshotRestore(t *testing.T) {
	b := newORB()
	day := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	b.OnCandle(candle(day, 320, 325, 318, 322))
	b.OnCandle(candle(day.Add(15*time.Minute), 322, 328, 319, 326))
	b.OnCandle(candle(day.Add(30*time.Minute), 326, 330, 325, 329)) // long pending
	b.OnExecution(strategy.ExecutionEvent{Status: "filled"})        // confirm

	blob, err := b.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	b2 := New()
	_ = b2.Configure(map[string]string{
		"timezone": "Etc/GMT-3",
	})
	if err := b2.Restore(blob); err != nil {
		t.Fatal(err)
	}
	if b2.pos != 1 {
		t.Errorf("restored pos = %d, want 1", b2.pos)
	}
	if !b2.rangeFormed {
		t.Error("restored rangeFormed should be true")
	}
	if b2.rangeHigh != 328 {
		t.Errorf("restored rangeHigh = %v, want 328", b2.rangeHigh)
	}

	// Verify it still produces correct signals after restore
	sigs := b2.OnCandle(candle(day.Add(45*time.Minute), 325, 325, 315, 316))
	if len(sigs) != 1 || sigs[0].Direction != "sell" {
		t.Fatalf("expected sell reverse after restore, got %v", sigs)
	}
}

func TestORBIndicatorData(t *testing.T) {
	b := newORB()
	day := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	b.OnCandle(candle(day, 320, 325, 318, 322))
	b.OnCandle(candle(day.Add(15*time.Minute), 322, 328, 319, 326))
	b.OnCandle(candle(day.Add(30*time.Minute), 326, 330, 325, 329)) // long pending
	b.OnExecution(strategy.ExecutionEvent{Status: "filled"})

	data := b.IndicatorData()
	snap, ok := data.(IndicatorSnapshot)
	if !ok {
		t.Fatalf("IndicatorData returned %T, want IndicatorSnapshot", data)
	}
	if snap.StrategyType != "orb_breakout" {
		t.Errorf("strategy_type = %q, want orb_breakout", snap.StrategyType)
	}
	if len(snap.Candles) != 3 {
		t.Errorf("len(candles) = %d, want 3", len(snap.Candles))
	}
	if len(snap.Signals) != 1 {
		t.Errorf("len(signals) = %d, want 1", len(snap.Signals))
	}
	if snap.RangeHigh != 328 {
		t.Errorf("range_high = %v, want 328", snap.RangeHigh)
	}

	if snap.Candles[0].RangeHigh != 0 || snap.Candles[0].RangeLow != 0 {
		t.Error("first candle should have zero range lines (range not yet formed)")
	}
	if snap.Candles[2].RangeHigh != 328 {
		t.Errorf("third candle RangeHigh = %v, want 328", snap.Candles[2].RangeHigh)
	}

	j, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(j) < 10 {
		t.Error("JSON too short")
	}
}

func TestORB_pendingEntry_blocksCandle(t *testing.T) {
	b := newORB()
	day := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	b.OnCandle(candle(day, 320, 325, 318, 322))
	b.OnCandle(candle(day.Add(15*time.Minute), 322, 328, 319, 326))
	b.OnCandle(candle(day.Add(30*time.Minute), 326, 330, 325, 329)) // sets pendingEntry=1

	if b.pendingEntry != 1 {
		t.Fatalf("pendingEntry = %d, want 1", b.pendingEntry)
	}

	// While pending, no new signals should be emitted
	sigs := b.OnCandle(candle(day.Add(45*time.Minute), 320, 320, 310, 311))
	if sigs != nil {
		t.Fatalf("should block signals while pendingEntry, got %+v", sigs)
	}
}

func TestORB_OnExecution_rejected_clearsPending(t *testing.T) {
	b := newORB()
	b.pendingEntry = 1
	b.OnExecution(strategy.ExecutionEvent{Status: "rejected", Message: "order reject: insufficient"})
	if b.pendingEntry != 0 {
		t.Fatalf("pendingEntry should clear on reject, got %d", b.pendingEntry)
	}
}

func TestORB_OnSignalDispatchFailed_clearsEntry(t *testing.T) {
	b := newORB()
	b.pendingEntry = -1
	b.OnSignalDispatchFailed(strategy.Signal{Reason: "orb breakout short"}, "risk_rejected")
	if b.pendingEntry != 0 {
		t.Fatalf("pendingEntry should be 0, got %d", b.pendingEntry)
	}
}

func TestORB_OnSignalDispatchFailed_eodRetry(t *testing.T) {
	b := newORB()
	b.pendingExit = true
	b.eodSent = true
	b.OnSignalDispatchFailed(strategy.Signal{Reason: "eod close long"}, "post_error")
	if b.pendingExit {
		t.Fatal("pendingExit should be cleared")
	}
	if b.eodSent {
		t.Fatal("eodSent should be reset for retry")
	}
}

func TestORB_ResetTradingState(t *testing.T) {
	b := newORB()
	b.pos = 1
	b.pendingEntry = 1
	b.pendingExit = true
	b.eodSent = true
	b.signals = []SignalPoint{{Direction: "sell"}}
	b.ResetTradingStateAfterWarmup()
	if b.pos != 0 || b.pendingEntry != 0 || b.pendingExit || b.eodSent {
		t.Fatalf("state not fully reset: pos=%d pe=%d px=%v eod=%v",
			b.pos, b.pendingEntry, b.pendingExit, b.eodSent)
	}
	if len(b.signals) != 0 {
		t.Fatalf("warmup signals should be cleared, got %d", len(b.signals))
	}
}

func TestORB_Snapshot_includesPendingFields(t *testing.T) {
	b := newORB()
	b.pendingEntry = -1
	b.pendingExit = true
	blob, err := b.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	b2 := New()
	_ = b2.Configure(map[string]string{"timezone": "Etc/GMT-3"})
	if err := b2.Restore(blob); err != nil {
		t.Fatal(err)
	}
	if b2.pendingEntry != -1 {
		t.Fatalf("pendingEntry not restored: want -1 got %d", b2.pendingEntry)
	}
	if !b2.pendingExit {
		t.Fatal("pendingExit not restored: want true")
	}
}
