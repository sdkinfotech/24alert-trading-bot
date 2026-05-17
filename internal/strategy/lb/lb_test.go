package lb

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/24alert/trading-bot/internal/strategy"
)

func TestBounce_OnSignalDispatchFailed_clearsPendingEntry(t *testing.T) {
	b := New()
	if err := b.Configure(map[string]string{
		"quantity": "1", "timezone": "Europe/Moscow",
	}); err != nil {
		t.Fatal(err)
	}
	b.support = []float64{100, 99, 98}
	b.resistance = []float64{120, 121, 122}
	b.atr = 2
	b.pendingEntry = 1
	b.entryPrice = 101
	b.stopLoss = 90
	b.takeProfit = 110

	b.OnSignalDispatchFailed(strategy.Signal{Reason: "bounce S=100.0"}, "risk_rejected")

	if b.pendingEntry != 0 {
		t.Fatalf("pendingEntry want 0 got %d", b.pendingEntry)
	}
	if b.entryPrice != 0 || b.stopLoss != 0 || b.takeProfit != 0 {
		t.Fatalf("entry levels should clear, got entry=%v sl=%v tp=%v", b.entryPrice, b.stopLoss, b.takeProfit)
	}
	if b.pos != 0 {
		t.Fatalf("pos want 0 got %d", b.pos)
	}
}

func TestBounce_OnExecution_filled_promotesPendingEntry(t *testing.T) {
	b := New()
	b.pendingEntry = -1
	b.pos = 0
	b.OnExecution(strategy.ExecutionEvent{Status: "filled"})
	if b.pos != -1 || b.pendingEntry != 0 {
		t.Fatalf("want pos=-1 pending=0, got pos=%d pending=%d", b.pos, b.pendingEntry)
	}
}

func TestBounce_OnExecution_filled_clearsPendingExit(t *testing.T) {
	b := New()
	b.pos = 1
	b.pendingExit = true
	b.entryPrice = 100
	b.stopLoss = 95
	b.takeProfit = 105
	b.OnExecution(strategy.ExecutionEvent{Status: "filled"})
	if b.pos != 0 || b.pendingExit {
		t.Fatalf("want flat after exit fill, pos=%d pendingExit=%v", b.pos, b.pendingExit)
	}
	if b.entryPrice != 0 || b.stopLoss != 0 || b.takeProfit != 0 {
		t.Fatalf("levels should clear after exit, entry=%v sl=%v tp=%v", b.entryPrice, b.stopLoss, b.takeProfit)
	}
}

func TestBounce_ResetTradingStateAfterWarmup(t *testing.T) {
	b := New()
	b.pos = 1
	b.pendingEntry = 1
	b.pendingExit = true
	b.entryPrice = 10
	b.eodSent = true
	b.currentDay = "2026-05-01"
	b.signals = []SignalPoint{{Direction: "buy"}}
	b.ResetTradingStateAfterWarmup()
	if b.pos != 0 || b.pendingEntry != 0 || b.pendingExit || b.entryPrice != 0 || b.eodSent || b.currentDay != "" {
		t.Fatalf("unexpected state after reset: %+v %+v %+v", b.pos, b.pendingEntry, b.currentDay)
	}
	if len(b.signals) != 0 {
		t.Fatalf("warmup signals should be cleared, got %d", len(b.signals))
	}
}

func TestBounce_OnCandle_entryUsesPendingNotPos(t *testing.T) {
	b := New()
	if err := b.Configure(map[string]string{
		"quantity":    "1",
		"timezone":    "Europe/Moscow",
		"cutoff_hour": "23",
		"cutoff_min":  "59",
	}); err != nil {
		t.Fatal(err)
	}
	b.support = []float64{100, 99, 98}
	b.resistance = []float64{200, 201, 202}
	b.atr = 10
	// MSK session day; before cutoff 23:59
	ts := time.Date(2026, 5, 15, 12, 0, 0, 0, b.tz)
	k := strategy.Candle{
		InstrumentUID: "uid",
		Open:          105, High: 105, Low: 99, Close: 101,
		Time:       ts,
		IsComplete: true,
	}
	sigs := b.OnCandle(k)
	if len(sigs) != 1 || sigs[0].Direction != "buy" {
		t.Fatalf("want one buy signal, got %#v", sigs)
	}
	if b.pos != 0 {
		t.Fatalf("pos should stay 0 until fill, got %d", b.pos)
	}
	if b.pendingEntry != 1 {
		t.Fatalf("pendingEntry want 1 got %d", b.pendingEntry)
	}
}

func TestBounce_OnDailyCandle_recordsLevelSources(t *testing.T) {
	b := New()
	base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		b.OnDailyCandle(strategy.Candle{
			High:       100 + float64(i),
			Low:        90 - float64(i),
			Close:      95 + float64(i),
			Time:       base.AddDate(0, 0, i),
			IsComplete: true,
		})
	}

	snap := b.IndicatorData().(IndicatorSnapshot)
	if len(snap.ResistanceSources) != 3 || len(snap.SupportSources) != 3 {
		t.Fatalf("want 3 support/resistance sources, got %d/%d", len(snap.SupportSources), len(snap.ResistanceSources))
	}
	if snap.ResistanceSources[0].Price != 104 || snap.ResistanceSources[0].Date != "2026-05-05" {
		t.Fatalf("unexpected top resistance source: %+v", snap.ResistanceSources[0])
	}
	if snap.SupportSources[0].Price != 86 || snap.SupportSources[0].Date != "2026-05-05" {
		t.Fatalf("unexpected top support source: %+v", snap.SupportSources[0])
	}
}

func TestBounce_RestoreOldSnapshotClearsUndatedDailyBuffers(t *testing.T) {
	b := New()
	blob, err := (&Bounce{
		dailyHighs: []float64{101, 102, 103},
		dailyLows:  []float64{91, 92, 93},
		dailyTRs:   []float64{10, 10},
	}).Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	var st lbState
	if err := json.Unmarshal(blob, &st); err != nil {
		t.Fatal(err)
	}
	st.DailyDates = nil
	legacyBlob, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}

	if err := b.Restore(legacyBlob); err != nil {
		t.Fatal(err)
	}
	if len(b.dailyHighs) != 0 || len(b.dailyLows) != 0 || len(b.dailyTRs) != 0 {
		t.Fatalf("legacy undated daily buffers should be cleared, highs=%v lows=%v trs=%v", b.dailyHighs, b.dailyLows, b.dailyTRs)
	}
}
