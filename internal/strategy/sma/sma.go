package sma

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/24alert/trading-bot/internal/strategy"
)

// CandlePoint holds OHLC + computed SMA values for the indicator chart.
type CandlePoint struct {
	Time    time.Time `json:"time"`
	Open    float64   `json:"open"`
	High    float64   `json:"high"`
	Low     float64   `json:"low"`
	Close   float64   `json:"close"`
	FastSMA float64   `json:"fast_sma"`
	SlowSMA float64   `json:"slow_sma"`
}

// SignalPoint records when a signal was generated.
type SignalPoint struct {
	Time      time.Time `json:"time"`
	Direction string    `json:"direction"`
	Reason    string    `json:"reason"`
	RefPrice  float64   `json:"ref_price"`
}

// IndicatorSnapshot is the full indicator state returned by IndicatorData().
type IndicatorSnapshot struct {
	InstanceID         string        `json:"instance_id,omitempty"`
	InstrumentUID      string        `json:"instrument_uid,omitempty"`
	FastN              int           `json:"fast_period"`
	SlowN              int           `json:"slow_period"`
	Position           int64         `json:"position"`
	TrailingStopPct      float64 `json:"trailing_stop_pct,omitempty"`
	TrailingBestPrice    float64 `json:"trailing_best_price,omitempty"`
	TrailingStopPrice    float64 `json:"trailing_stop_price,omitempty"`
	TrailingStopActive   bool    `json:"trailing_stop_active,omitempty"`
	StructuralStopPrice  float64 `json:"structural_stop_price,omitempty"`
	InitialStopSwingBars int     `json:"initial_stop_swing_bars,omitempty"`
	Candles            []CandlePoint `json:"candles"`
	Signals            []SignalPoint `json:"signals"`
}

const maxHistoryLen = 1000

// Crossover implements a minimal SMA crossover on completed candles.
type Crossover struct {
	fastN           int
	slowN           int
	qty             int64
	trailingStopPct       float64
	initialStopSwingBars  int
	structuralStop        float64 // swing high/low at entry for broker stop
	closes                []float64
	pos                   int64 // +1 long, -1 short, 0 flat — confirmed by broker fill
	pendingEntry          int64 // +1 or -1 while entry/reverse order is in flight, 0 when idle
	pendingExit           bool  // true while protective exit order is in flight
	trailingBest          float64
	stopped               bool

	history []CandlePoint
	signals []SignalPoint
}

// New creates an SMA crossover strategy with defaults (5 / 20).
func New() *Crossover {
	return &Crossover{fastN: 5, slowN: 20, qty: 1}
}

// RegisterBuiltins registers this strategy in the global registry.
func RegisterBuiltins(reg *strategy.Registry) {
	reg.Register("sma_crossover", func() (strategy.Strategy, error) {
		return New(), nil
	})
}

func (c *Crossover) Info() strategy.StrategyInfo {
	return strategy.StrategyInfo{
		ID:          "sma_crossover",
		Name:        "SMA Crossover",
		Version:     "1.0.0",
		Description: "Classic fast/slow SMA crossover on completed candles",
		Author:      "24alert",
	}
}

func (c *Crossover) Configure(params map[string]string) error {
	c.fastN = intFrom(params, "fast_period", 5)
	c.slowN = intFrom(params, "slow_period", 20)
	if c.fastN < 1 || c.slowN < 1 || c.fastN >= c.slowN {
		return fmt.Errorf("invalid periods: fast=%d slow=%d", c.fastN, c.slowN)
	}
	c.qty = int64From(params, "quantity", 1)
	if c.qty < 1 {
		return fmt.Errorf("quantity must be >= 1")
	}
	c.trailingStopPct = floatFrom(params, "trailing_stop_pct", 0)
	if c.trailingStopPct < 0 || c.trailingStopPct >= 0.5 {
		return fmt.Errorf("trailing_stop_pct must be >= 0 and < 0.5")
	}
	c.initialStopSwingBars = intFrom(params, "initial_stop_swing_bars", 5)
	if c.initialStopSwingBars < 0 {
		c.initialStopSwingBars = 0
	}
	c.closes = c.closes[:0]
	c.pos = 0
	c.pendingEntry = 0
	c.pendingExit = false
	c.trailingBest = 0
	c.structuralStop = 0
	return nil
}

func (c *Crossover) OnCandle(k strategy.Candle) []strategy.Signal {
	if c.stopped || !k.IsComplete {
		return nil
	}
	if c.pendingEntry != 0 || c.pendingExit {
		return nil
	}
	c.closes = append(c.closes, k.Close)
	if len(c.closes) < c.slowN {
		c.recordCandle(k, 0, 0)
		return nil
	}
	if len(c.closes) > c.slowN+5 {
		c.closes = c.closes[len(c.closes)-c.slowN-5:]
	}
	fast := smaTail(c.closes, c.fastN)
	slow := smaTail(c.closes, c.slowN)

	c.recordCandle(k, fast, slow)

	if sig, ok := c.trailingStopSignal(k); ok {
		return []strategy.Signal{sig}
	}

	prev := c.closes[:len(c.closes)-1]
	if len(prev) < c.slowN {
		return nil
	}
	prevFast := smaTail(prev, c.fastN)
	prevSlow := smaTail(prev, c.slowN)
	var out []strategy.Signal
	if prevFast <= prevSlow && fast > slow && c.pos <= 0 {
		sig := strategy.Signal{
			InstrumentUID: k.InstrumentUID,
			Direction:     "buy",
			Quantity:      c.qty,
			OrderType:     "market",
			Reason:        "sma golden cross",
			CandleTime:    k.Time,
		}
		out = append(out, sig)
		c.recordSignal(k.Time, "buy", "sma golden cross", k.Close)
		c.pendingEntry = 1
		c.trailingBest = 0
		c.structuralStop = 0
	} else if prevFast >= prevSlow && fast < slow && c.pos >= 0 {
		sig := strategy.Signal{
			InstrumentUID: k.InstrumentUID,
			Direction:     "sell",
			Quantity:      c.qty,
			OrderType:     "market",
			Reason:        "sma death cross",
			CandleTime:    k.Time,
		}
		out = append(out, sig)
		c.recordSignal(k.Time, "sell", "sma death cross", k.Close)
		c.pendingEntry = -1
		c.trailingBest = 0
		c.structuralStop = 0
	}
	return out
}

// swingStructuralStop returns stop at max high (short) or min low (long) over last N completed bars.
func (c *Crossover) swingStructuralStop(pos int64) float64 {
	if c.initialStopSwingBars <= 0 || len(c.history) == 0 || pos == 0 {
		return 0
	}
	start := len(c.history) - c.initialStopSwingBars
	if start < 0 {
		start = 0
	}
	if pos < 0 {
		maxH := 0.0
		for i := start; i < len(c.history); i++ {
			if c.history[i].High > maxH {
				maxH = c.history[i].High
			}
		}
		return maxH
	}
	minL := math.MaxFloat64
	for i := start; i < len(c.history); i++ {
		if c.history[i].Low < minL {
			minL = c.history[i].Low
		}
	}
	if minL == math.MaxFloat64 {
		return 0
	}
	return minL
}

// ProtectiveStopPrice implements strategy.ProtectiveStopProvider.
func (c *Crossover) ProtectiveStopPrice(_ string, quantity, avgPrice float64) (float64, bool) {
	if c.structuralStop <= 0 {
		return 0, false
	}
	if quantity < 0 && c.structuralStop > avgPrice {
		return c.structuralStop, true
	}
	if quantity > 0 && avgPrice > 0 && c.structuralStop < avgPrice {
		return c.structuralStop, true
	}
	return 0, false
}

// OnLiveCandle updates and triggers only the protective trailing stop on an
// in-progress candle. SMA crosses still wait for completed bars in OnCandle.
func (c *Crossover) OnLiveCandle(k strategy.Candle) []strategy.Signal {
	if c.stopped || k.IsComplete || c.pendingEntry != 0 || c.pendingExit {
		return nil
	}
	if sig, ok := c.liveTrailingStopSignal(k); ok {
		return []strategy.Signal{sig}
	}
	return nil
}

func (c *Crossover) trailingStopSignal(k strategy.Candle) (strategy.Signal, bool) {
	if c.trailingStopPct <= 0 || c.pos == 0 {
		return strategy.Signal{}, false
	}
	if c.trailingBest <= 0 {
		c.trailingBest = k.Close
	}
	if c.pos > 0 {
		c.trailingBest = max(c.trailingBest, k.High)
		stop := c.trailingBest * (1 - c.trailingStopPct)
		if k.Low <= stop {
			c.pendingExit = true
			c.recordSignal(k.Time, "sell", "sma trailing stop", stop)
			return strategy.Signal{
				InstrumentUID: k.InstrumentUID,
				Direction:     "sell",
				Quantity:      c.qty,
				OrderType:     "market",
				Reason:        fmt.Sprintf("sma trailing stop %.2f%%", c.trailingStopPct*100),
				CandleTime:    k.Time,
			}, true
		}
		return strategy.Signal{}, false
	}
	c.trailingBest = min(c.trailingBest, k.Low)
	stop := c.trailingBest * (1 + c.trailingStopPct)
	if k.High >= stop {
		c.pendingExit = true
		c.recordSignal(k.Time, "buy", "sma trailing stop", stop)
		return strategy.Signal{
			InstrumentUID: k.InstrumentUID,
			Direction:     "buy",
			Quantity:      c.qty,
			OrderType:     "market",
			Reason:        fmt.Sprintf("sma trailing stop %.2f%%", c.trailingStopPct*100),
			CandleTime:    k.Time,
		}, true
	}
	return strategy.Signal{}, false
}

func (c *Crossover) liveTrailingStopSignal(k strategy.Candle) (strategy.Signal, bool) {
	if c.trailingStopPct <= 0 || c.pos == 0 {
		return strategy.Signal{}, false
	}
	if c.trailingBest <= 0 {
		c.trailingBest = k.Close
	}
	if c.pos > 0 {
		c.trailingBest = max(c.trailingBest, maxPositive(k.High, k.Close))
		stop := c.trailingBest * (1 - c.trailingStopPct)
		if k.Close > 0 && k.Close <= stop {
			c.pendingExit = true
			c.recordSignal(k.Time, "sell", "sma live trailing stop", stop)
			return strategy.Signal{
				InstrumentUID: k.InstrumentUID,
				Direction:     "sell",
				Quantity:      c.qty,
				OrderType:     "market",
				Reason:        fmt.Sprintf("sma live trailing stop %.2f%%", c.trailingStopPct*100),
				CandleTime:    k.Time,
			}, true
		}
		return strategy.Signal{}, false
	}
	c.trailingBest = min(c.trailingBest, minPositive(k.Low, k.Close))
	stop := c.trailingBest * (1 + c.trailingStopPct)
	if k.Close > 0 && k.Close >= stop {
		c.pendingExit = true
		c.recordSignal(k.Time, "buy", "sma live trailing stop", stop)
		return strategy.Signal{
			InstrumentUID: k.InstrumentUID,
			Direction:     "buy",
			Quantity:      c.qty,
			OrderType:     "market",
			Reason:        fmt.Sprintf("sma live trailing stop %.2f%%", c.trailingStopPct*100),
			CandleTime:    k.Time,
		}, true
	}
	return strategy.Signal{}, false
}

func (c *Crossover) recordCandle(k strategy.Candle, fast, slow float64) {
	c.history = append(c.history, CandlePoint{
		Time: k.Time, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close,
		FastSMA: fast, SlowSMA: slow,
	})
	if len(c.history) > maxHistoryLen {
		c.history = c.history[len(c.history)-maxHistoryLen:]
	}
}

func (c *Crossover) recordSignal(t time.Time, dir, reason string, price float64) {
	c.signals = append(c.signals, SignalPoint{Time: t, Direction: dir, Reason: reason, RefPrice: price})
	if len(c.signals) > maxHistoryLen {
		c.signals = c.signals[len(c.signals)-maxHistoryLen:]
	}
}

// IndicatorData returns the full indicator snapshot for visualization.
func (c *Crossover) IndicatorData() interface{} {
	candles := make([]CandlePoint, len(c.history))
	copy(candles, c.history)
	sigs := make([]SignalPoint, len(c.signals))
	copy(sigs, c.signals)
	return IndicatorSnapshot{
		FastN:                c.fastN,
		SlowN:                c.slowN,
		Position:             c.pos,
		TrailingStopPct:      c.trailingStopPct,
		TrailingBestPrice:    c.trailingBest,
		TrailingStopPrice:    c.trailingStopPrice(),
		TrailingStopActive:   c.trailingStopPct > 0 && c.pos != 0 && c.trailingBest > 0,
		StructuralStopPrice:    c.structuralStop,
		InitialStopSwingBars: c.initialStopSwingBars,
		Candles:            candles,
		Signals:            sigs,
	}
}

func (c *Crossover) trailingStopPrice() float64 {
	if c.trailingStopPct <= 0 || c.pos == 0 || c.trailingBest <= 0 {
		return 0
	}
	if c.pos > 0 {
		return c.trailingBest * (1 - c.trailingStopPct)
	}
	return c.trailingBest * (1 + c.trailingStopPct)
}

func (c *Crossover) OnOrderbook(_ strategy.Orderbook) []strategy.Signal { return nil }

func (c *Crossover) OnExecution(ev strategy.ExecutionEvent) {
	switch strings.ToLower(ev.Status) {
	case "filled", "partially_filled":
		if c.pendingExit {
			c.pos = 0
			c.pendingExit = false
			c.trailingBest = 0
			c.structuralStop = 0
			return
		}
		if c.pendingEntry != 0 {
			c.pos = c.pendingEntry
			c.pendingEntry = 0
			c.structuralStop = c.swingStructuralStop(c.pos)
			if ev.AvgPrice > 0 {
				c.trailingBest = ev.AvgPrice
			} else {
				c.trailingBest = 0
			}
		}
	case "cancelled", "rejected":
		c.pendingEntry = 0
		c.pendingExit = false
	}
}

// OnSignalDispatchFailed implements strategy.SignalDispatchFailureHandler.
func (c *Crossover) OnSignalDispatchFailed(_ strategy.Signal, _ string) {
	c.pendingEntry = 0
	c.pendingExit = false
}

func (c *Crossover) Stop() {
	c.stopped = true
}

// ResetTradingStateAfterWarmup implements strategy.PostWarmupCleanup.
func (c *Crossover) ResetTradingStateAfterWarmup() {
	c.pos = 0
	c.pendingEntry = 0
	c.pendingExit = false
	c.trailingBest = 0
	c.structuralStop = 0
	// Warmup-replayed crossover signals are not real dispatched signals.
	c.signals = nil
}

// SyncBrokerPosition aligns strategy state with broker truth before live trading starts.
func (c *Crossover) SyncBrokerPosition(_ string, quantity float64, averagePrice float64, currentPrice float64) {
	c.pendingEntry = 0
	c.pendingExit = false
	switch {
	case quantity > 0:
		c.pos = 1
		c.trailingBest = maxPositive(averagePrice, currentPrice)
	case quantity < 0:
		c.pos = -1
		c.trailingBest = minPositive(averagePrice, currentPrice)
	default:
		c.pos = 0
		c.trailingBest = 0
		c.structuralStop = 0
	}
	if c.pos != 0 && c.structuralStop == 0 {
		c.structuralStop = c.swingStructuralStop(c.pos)
	}
}

type smaState struct {
	FastN           int           `json:"fast_n"`
	SlowN           int           `json:"slow_n"`
	Qty             int64         `json:"qty"`
	TrailingStopPct float64       `json:"trailing_stop_pct"`
	Closes          []float64     `json:"closes"`
	Pos             int64         `json:"pos"`
	PendingEntry    int64         `json:"pending_entry"`
	PendingExit     bool          `json:"pending_exit"`
	TrailingBest    float64       `json:"trailing_best"`
	History         []CandlePoint `json:"history,omitempty"`
	Signals         []SignalPoint `json:"signals,omitempty"`
}

// Snapshot persists internal buffers for StatefulStrategy.
func (c *Crossover) Snapshot() ([]byte, error) {
	st := smaState{
		FastN:           c.fastN,
		SlowN:           c.slowN,
		Qty:             c.qty,
		TrailingStopPct: c.trailingStopPct,
		Closes:          append([]float64(nil), c.closes...),
		Pos:             c.pos,
		PendingEntry:    c.pendingEntry,
		PendingExit:     c.pendingExit,
		TrailingBest:    c.trailingBest,
		History:         c.history,
		Signals:         c.signals,
	}
	return json.Marshal(st)
}

// Restore loads Snapshot output (called after Configure).
func (c *Crossover) Restore(blob []byte) error {
	if len(blob) == 0 {
		return nil
	}
	var st smaState
	if err := json.Unmarshal(blob, &st); err != nil {
		return err
	}
	// Configuration parameters are applied by Configure before Restore.
	// Do not overwrite them from an old snapshot, otherwise a config reload that
	// changes SMA periods can silently keep trading with stale values.
	c.closes = append([]float64(nil), st.Closes...)
	c.pos = st.Pos
	c.pendingEntry = st.PendingEntry
	c.pendingExit = st.PendingExit
	c.trailingBest = st.TrailingBest
	c.history = st.History
	c.signals = st.Signals
	return nil
}

// WarmupCandles returns the number of historical bars needed for the slow SMA.
func (c *Crossover) WarmupCandles() int { return c.slowN }

// ChartCandles returns the number of historical bars for dashboard visualization.
// We want enough history for a full month of hourly candles with SMA lines drawn
// across most of the chart (not just the last point).
func (c *Crossover) ChartCandles() int {
	n := c.slowN * 3
	if n < 500 {
		n = 500
	}
	return n
}

var _ strategy.StatefulStrategy = (*Crossover)(nil)
var _ strategy.WarmupHint = (*Crossover)(nil)
var _ strategy.ChartHint = (*Crossover)(nil)
var _ strategy.PostWarmupCleanup = (*Crossover)(nil)
var _ strategy.SignalDispatchFailureHandler = (*Crossover)(nil)
var _ strategy.BrokerPositionSyncer = (*Crossover)(nil)
var _ strategy.LiveCandleHandler = (*Crossover)(nil)

func smaTail(xs []float64, n int) float64 {
	if len(xs) < n || n <= 0 {
		return 0
	}
	tail := xs[len(xs)-n:]
	var s float64
	for _, v := range tail {
		s += v
	}
	return s / float64(n)
}

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

func floatFrom(m map[string]string, key string, def float64) float64 {
	if v, ok := m[key]; ok {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f
		}
	}
	return def
}

func maxPositive(a, b float64) float64 {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	return max(a, b)
}

func minPositive(a, b float64) float64 {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	return min(a, b)
}
