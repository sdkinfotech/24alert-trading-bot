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
	if b.pos != 1 {
		t.Errorf("pos = %d, want 1", b.pos)
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
	if b.pos != -1 {
		t.Errorf("pos = %d, want -1", b.pos)
	}
}

func TestORBReversePosition(t *testing.T) {
	b := newORB()
	day := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	b.OnCandle(candle(day, 320, 325, 318, 322))
	b.OnCandle(candle(day.Add(15*time.Minute), 322, 328, 319, 326))

	// Breakout long
	b.OnCandle(candle(day.Add(30*time.Minute), 326, 330, 325, 329))

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
	if b.pos != -1 {
		t.Errorf("pos = %d, want -1", b.pos)
	}
}

func TestORBEndOfDay(t *testing.T) {
	b := newORB()
	day := time.Date(2026, 5, 15, 10, 0, 0, 0, msk)

	b.OnCandle(candle(day, 320, 325, 318, 322))
	b.OnCandle(candle(day.Add(15*time.Minute), 322, 328, 319, 326))
	b.OnCandle(candle(day.Add(30*time.Minute), 326, 330, 325, 329)) // long

	// EOD candle at 18:30 MSK — should close long position
	eod := time.Date(2026, 5, 15, 18, 30, 0, 0, msk)
	sigs := b.OnCandle(candle(eod, 332, 333, 330, 331))
	if len(sigs) != 1 {
		t.Fatalf("expected 1 eod signal, got %d", len(sigs))
	}
	if sigs[0].Direction != "sell" {
		t.Errorf("eod direction = %q, want sell (closing long)", sigs[0].Direction)
	}
	if b.pos != 0 {
		t.Errorf("pos = %d after EOD, want 0", b.pos)
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
	b.OnCandle(candle(day.Add(30*time.Minute), 320, 321, 316, 317)) // short

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
	b.OnCandle(candle(day1.Add(30*time.Minute), 326, 330, 325, 329)) // long

	// New day — all state should reset
	day2 := time.Date(2026, 5, 16, 10, 0, 0, 0, msk)
	sigs := b.OnCandle(candle(day2, 330, 335, 328, 332))
	if len(sigs) != 0 {
		t.Fatalf("expected 0 signals on new day first candle, got %d", len(sigs))
	}
	if b.pos != 0 {
		t.Errorf("pos = %d after day reset, want 0", b.pos)
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

	// Second candle also above rangeHigh — no duplicate
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
	b.OnCandle(candle(day.Add(30*time.Minute), 326, 330, 325, 329)) // long

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
	b.OnCandle(candle(day.Add(30*time.Minute), 326, 330, 325, 329)) // long

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

	// Verify candles during range formation have zero range lines
	if snap.Candles[0].RangeHigh != 0 || snap.Candles[0].RangeLow != 0 {
		t.Error("first candle should have zero range lines (range not yet formed)")
	}
	if snap.Candles[2].RangeHigh != 328 {
		t.Errorf("third candle RangeHigh = %v, want 328", snap.Candles[2].RangeHigh)
	}

	// Verify JSON serialization works
	j, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(j) < 10 {
		t.Error("JSON too short")
	}
}
