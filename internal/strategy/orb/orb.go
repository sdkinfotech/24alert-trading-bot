package orb

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/24alert/trading-bot/internal/strategy"
)

// CandlePoint holds OHLC + range levels for the indicator chart.
type CandlePoint struct {
	Time      time.Time `json:"time"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	RangeHigh float64   `json:"range_high"`
	RangeLow  float64   `json:"range_low"`
}

// SignalPoint records when a signal was generated.
type SignalPoint struct {
	Time      time.Time `json:"time"`
	Direction string    `json:"direction"`
	Reason    string    `json:"reason"`
	RefPrice  float64   `json:"ref_price"`
}

// IndicatorSnapshot is returned by IndicatorData() for web dashboard visualization.
type IndicatorSnapshot struct {
	StrategyType string        `json:"strategy_type"`
	RangeHigh    float64       `json:"range_high"`
	RangeLow     float64       `json:"range_low"`
	RangeFormed  bool          `json:"range_formed"`
	Position     int64         `json:"position"`
	CurrentDay   string        `json:"current_day"`
	Candles      []CandlePoint `json:"candles"`
	Signals      []SignalPoint `json:"signals"`
}

const maxHistoryLen = 1000

// Breakout implements an Opening Range Breakout intraday strategy.
//
// Phase 1 (observation): Collect the first N candles after market open,
// record the high and low of that range.
// Phase 2 (trading): Enter long on close above range high, short on close
// below range low. Reverse on opposite breakout.
// Phase 3 (EOD close): Flatten all positions before cutoff time.
type Breakout struct {
	rangeCandles int
	qty          int64
	cutoffHour   int
	cutoffMin    int
	tz           *time.Location

	rangeHigh    float64
	rangeLow     float64
	rangeFormed  bool
	candleCount  int
	pos          int64  // +1 long, -1 short, 0 flat — confirmed by broker fill
	pendingEntry int64  // +1 or -1 while entry/reverse order is in flight
	pendingExit  bool   // true while EOD close order is in flight
	currentDay   string // "2006-01-02" — resets on new trading day
	eodSent      bool   // true if EOD close signal already sent today
	stopped      bool

	history []CandlePoint
	signals []SignalPoint
}

func New() *Breakout {
	loc, _ := time.LoadLocation("Europe/Moscow")
	if loc == nil {
		loc = time.FixedZone("MSK", 3*3600)
	}
	return &Breakout{
		rangeCandles: 2,
		qty:          1,
		cutoffHour:   18,
		cutoffMin:    30,
		tz:           loc,
	}
}

func RegisterBuiltins(reg *strategy.Registry) {
	reg.Register("orb_breakout", func() (strategy.Strategy, error) {
		return New(), nil
	})
}

func (b *Breakout) Info() strategy.StrategyInfo {
	return strategy.StrategyInfo{
		ID:          "orb_breakout",
		Name:        "Opening Range Breakout",
		Version:     "1.0.0",
		Description: "Intraday ORB: trade breakouts of the opening range, flatten before close",
		Author:      "24alert",
	}
}

func (b *Breakout) Configure(params map[string]string) error {
	b.rangeCandles = intFrom(params, "range_candles", 2)
	if b.rangeCandles < 1 || b.rangeCandles > 20 {
		return fmt.Errorf("range_candles must be 1..20, got %d", b.rangeCandles)
	}
	b.qty = int64From(params, "quantity", 1)
	if b.qty < 1 {
		return fmt.Errorf("quantity must be >= 1")
	}
	b.cutoffHour = intFrom(params, "cutoff_hour", 18)
	b.cutoffMin = intFrom(params, "cutoff_min", 30)

	if tz := params["timezone"]; tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return fmt.Errorf("invalid timezone %q: %w", tz, err)
		}
		b.tz = loc
	}

	b.rangeHigh = 0
	b.rangeLow = 0
	b.rangeFormed = false
	b.candleCount = 0
	b.pos = 0
	b.pendingEntry = 0
	b.pendingExit = false
	b.currentDay = ""
	b.eodSent = false
	return nil
}

func (b *Breakout) OnCandle(k strategy.Candle) []strategy.Signal {
	if b.stopped || !k.IsComplete {
		return nil
	}
	if b.pendingEntry != 0 || b.pendingExit {
		return nil
	}

	t := k.Time.In(b.tz)
	day := t.Format("2006-01-02")

	// New trading day — reset range and counters; pos stays (broker may still hold).
	if day != b.currentDay {
		b.currentDay = day
		b.rangeHigh = 0
		b.rangeLow = math.MaxFloat64
		b.rangeFormed = false
		b.candleCount = 0
		b.eodSent = false
	}

	b.candleCount++

	// Phase 1: accumulate opening range
	if b.candleCount <= b.rangeCandles {
		if k.High > b.rangeHigh {
			b.rangeHigh = k.High
		}
		if k.Low < b.rangeLow {
			b.rangeLow = k.Low
		}
		if b.candleCount == b.rangeCandles {
			b.rangeFormed = true
		}
		b.recordCandle(k)
		return nil
	}

	b.recordCandle(k)

	// Phase 3: EOD close — flatten before cutoff
	cutoff := time.Date(t.Year(), t.Month(), t.Day(), b.cutoffHour, b.cutoffMin, 0, 0, b.tz)
	if !t.Before(cutoff) {
		if b.pos != 0 && !b.eodSent {
			dir := "sell"
			reason := "eod close long"
			if b.pos < 0 {
				dir = "buy"
				reason = "eod close short"
			}
			b.eodSent = true
			b.pendingExit = true
			sig := strategy.Signal{
				InstrumentUID: k.InstrumentUID,
				Direction:     dir,
				Quantity:      b.qty,
				OrderType:     "market",
				Reason:        reason,
				CandleTime:    k.Time,
			}
			b.recordSignal(k.Time, dir, reason, k.Close)
			return []strategy.Signal{sig}
		}
		return nil
	}

	if !b.rangeFormed {
		return nil
	}

	// Phase 2: trade breakouts
	var out []strategy.Signal

	if k.Close > b.rangeHigh && b.pos <= 0 {
		qty := b.qty
		reason := "orb breakout long"
		if b.pos < 0 {
			qty = b.qty * 2
			reason = "orb reverse to long"
		}
		out = append(out, strategy.Signal{
			InstrumentUID: k.InstrumentUID,
			Direction:     "buy",
			Quantity:      qty,
			OrderType:     "market",
			Reason:        reason,
			CandleTime:    k.Time,
		})
		b.recordSignal(k.Time, "buy", reason, k.Close)
		b.pendingEntry = 1
	} else if k.Close < b.rangeLow && b.pos >= 0 {
		qty := b.qty
		reason := "orb breakout short"
		if b.pos > 0 {
			qty = b.qty * 2
			reason = "orb reverse to short"
		}
		out = append(out, strategy.Signal{
			InstrumentUID: k.InstrumentUID,
			Direction:     "sell",
			Quantity:      qty,
			OrderType:     "market",
			Reason:        reason,
			CandleTime:    k.Time,
		})
		b.recordSignal(k.Time, "sell", reason, k.Close)
		b.pendingEntry = -1
	}

	return out
}

func (b *Breakout) OnOrderbook(_ strategy.Orderbook) []strategy.Signal { return nil }

func (b *Breakout) OnExecution(ev strategy.ExecutionEvent) {
	switch strings.ToLower(ev.Status) {
	case "filled", "partially_filled":
		if b.pendingEntry != 0 {
			b.pos = b.pendingEntry
			b.pendingEntry = 0
		}
		if b.pendingExit {
			b.pos = 0
			b.pendingExit = false
		}
	case "cancelled", "rejected":
		if strings.Contains(strings.ToLower(ev.Message), "reject") {
			if b.pendingEntry != 0 {
				b.pendingEntry = 0
			}
			if b.pendingExit {
				b.pendingExit = false
				b.eodSent = false
			}
		}
	}
}

// OnSignalDispatchFailed implements strategy.SignalDispatchFailureHandler.
func (b *Breakout) OnSignalDispatchFailed(sig strategy.Signal, _ string) {
	if b.pendingEntry != 0 {
		b.pendingEntry = 0
	}
	if b.pendingExit {
		b.pendingExit = false
		if strings.Contains(strings.ToLower(sig.Reason), "eod") {
			b.eodSent = false
		}
	}
}

func (b *Breakout) Stop() { b.stopped = true }

func (b *Breakout) recordCandle(k strategy.Candle) {
	rh, rl := 0.0, 0.0
	if b.rangeFormed {
		rh = b.rangeHigh
		rl = b.rangeLow
	}
	b.history = append(b.history, CandlePoint{
		Time: k.Time, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close,
		RangeHigh: rh, RangeLow: rl,
	})
	if len(b.history) > maxHistoryLen {
		b.history = b.history[len(b.history)-maxHistoryLen:]
	}
}

func (b *Breakout) recordSignal(t time.Time, dir, reason string, price float64) {
	b.signals = append(b.signals, SignalPoint{Time: t, Direction: dir, Reason: reason, RefPrice: price})
	if len(b.signals) > maxHistoryLen {
		b.signals = b.signals[len(b.signals)-maxHistoryLen:]
	}
}

// IndicatorData returns the full indicator snapshot for visualization.
func (b *Breakout) IndicatorData() interface{} {
	candles := make([]CandlePoint, len(b.history))
	copy(candles, b.history)
	sigs := make([]SignalPoint, len(b.signals))
	copy(sigs, b.signals)
	return IndicatorSnapshot{
		StrategyType: "orb_breakout",
		RangeHigh:    b.rangeHigh,
		RangeLow:     b.rangeLow,
		RangeFormed:  b.rangeFormed,
		Position:     b.pos,
		CurrentDay:   b.currentDay,
		Candles:      candles,
		Signals:      sigs,
	}
}

// --- StatefulStrategy ---

type orbState struct {
	RangeCandles int           `json:"range_candles"`
	Qty          int64         `json:"qty"`
	CutoffHour   int           `json:"cutoff_hour"`
	CutoffMin    int           `json:"cutoff_min"`
	RangeHigh    float64       `json:"range_high"`
	RangeLow     float64       `json:"range_low"`
	RangeFormed  bool          `json:"range_formed"`
	CandleCount  int           `json:"candle_count"`
	Pos          int64         `json:"pos"`
	PendingEntry int64         `json:"pending_entry"`
	PendingExit  bool          `json:"pending_exit"`
	CurrentDay   string        `json:"current_day"`
	EodSent      bool          `json:"eod_sent"`
	History      []CandlePoint `json:"history,omitempty"`
	Signals      []SignalPoint `json:"signals,omitempty"`
}

func (b *Breakout) Snapshot() ([]byte, error) {
	return json.Marshal(orbState{
		RangeCandles: b.rangeCandles,
		Qty:          b.qty,
		CutoffHour:   b.cutoffHour,
		CutoffMin:    b.cutoffMin,
		RangeHigh:    b.rangeHigh,
		RangeLow:     b.rangeLow,
		RangeFormed:  b.rangeFormed,
		CandleCount:  b.candleCount,
		Pos:          b.pos,
		PendingEntry: b.pendingEntry,
		PendingExit:  b.pendingExit,
		CurrentDay:   b.currentDay,
		EodSent:      b.eodSent,
		History:      b.history,
		Signals:      b.signals,
	})
}

func (b *Breakout) Restore(blob []byte) error {
	if len(blob) == 0 {
		return nil
	}
	var st orbState
	if err := json.Unmarshal(blob, &st); err != nil {
		return err
	}
	if st.RangeCandles > 0 {
		b.rangeCandles = st.RangeCandles
	}
	if st.Qty > 0 {
		b.qty = st.Qty
	}
	b.cutoffHour = st.CutoffHour
	b.cutoffMin = st.CutoffMin
	b.rangeHigh = st.RangeHigh
	b.rangeLow = st.RangeLow
	b.rangeFormed = st.RangeFormed
	b.candleCount = st.CandleCount
	b.pos = st.Pos
	b.pendingEntry = st.PendingEntry
	b.pendingExit = st.PendingExit
	b.currentDay = st.CurrentDay
	b.eodSent = st.EodSent
	b.history = st.History
	b.signals = st.Signals
	return nil
}

// WarmupCandles returns the number of historical bars needed to form the
// opening range so the strategy is ready to trade after a restart.
func (b *Breakout) WarmupCandles() int { return b.rangeCandles }

// ChartCandles returns the number of historical bars for dashboard visualization.
// 200 fifteen-minute bars covers ~3.5 trading days with full range context.
func (b *Breakout) ChartCandles() int { return 200 }

// ResetTradingStateAfterWarmup implements strategy.PostWarmupCleanup.
func (b *Breakout) ResetTradingStateAfterWarmup() {
	b.pos = 0
	b.pendingEntry = 0
	b.pendingExit = false
	b.eodSent = false
}

var (
	_ strategy.StatefulStrategy            = (*Breakout)(nil)
	_ strategy.IndicatorProvider           = (*Breakout)(nil)
	_ strategy.WarmupHint                  = (*Breakout)(nil)
	_ strategy.ChartHint                   = (*Breakout)(nil)
	_ strategy.PostWarmupCleanup           = (*Breakout)(nil)
	_ strategy.SignalDispatchFailureHandler = (*Breakout)(nil)
)

func intFrom(m map[string]string, key string, def int) int {
	if v, ok := m[key]; ok {
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return i
		}
	}
	return def
}

func int64From(m map[string]string, key string, def int64) int64 {
	if v, ok := m[key]; ok {
		if i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return i
		}
	}
	return def
}
