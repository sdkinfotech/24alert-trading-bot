package sma

import (
	"encoding/json"
	"fmt"
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
	InstanceID    string        `json:"instance_id,omitempty"`
	InstrumentUID string        `json:"instrument_uid,omitempty"`
	FastN         int           `json:"fast_period"`
	SlowN         int           `json:"slow_period"`
	Position      int64         `json:"position"`
	Candles       []CandlePoint `json:"candles"`
	Signals       []SignalPoint `json:"signals"`
}

const maxHistoryLen = 500

// Crossover implements a minimal SMA crossover on completed candles.
type Crossover struct {
	fastN   int
	slowN   int
	qty     int64
	closes  []float64
	pos     int64 // +1 long, -1 short, 0 flat
	stopped bool

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
	c.closes = c.closes[:0]
	c.pos = 0
	return nil
}

func (c *Crossover) OnCandle(k strategy.Candle) []strategy.Signal {
	if c.stopped || !k.IsComplete {
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
		}
		out = append(out, sig)
		c.recordSignal(k.Time, "buy", "sma golden cross", k.Close)
		c.pos = 1
	} else if prevFast >= prevSlow && fast < slow && c.pos >= 0 {
		sig := strategy.Signal{
			InstrumentUID: k.InstrumentUID,
			Direction:     "sell",
			Quantity:      c.qty,
			OrderType:     "market",
			Reason:        "sma death cross",
		}
		out = append(out, sig)
		c.recordSignal(k.Time, "sell", "sma death cross", k.Close)
		c.pos = -1
	}
	return out
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
		FastN:    c.fastN,
		SlowN:    c.slowN,
		Position: c.pos,
		Candles:  candles,
		Signals:  sigs,
	}
}

func (c *Crossover) OnOrderbook(_ strategy.Orderbook) []strategy.Signal { return nil }

func (c *Crossover) OnExecution(ev strategy.ExecutionEvent) {
	switch strings.ToLower(ev.Status) {
	case "filled", "partially_filled":
		// Position is tracked by crossover direction; refine with fills if needed.
	case "cancelled", "rejected":
		// reset flat on hard failure
		if strings.Contains(strings.ToLower(ev.Message), "reject") {
			c.pos = 0
		}
	}
}

func (c *Crossover) Stop() {
	c.stopped = true
}

type smaState struct {
	FastN   int           `json:"fast_n"`
	SlowN   int           `json:"slow_n"`
	Qty     int64         `json:"qty"`
	Closes  []float64     `json:"closes"`
	Pos     int64         `json:"pos"`
	History []CandlePoint `json:"history,omitempty"`
	Signals []SignalPoint `json:"signals,omitempty"`
}

// Snapshot persists internal buffers for StatefulStrategy.
func (c *Crossover) Snapshot() ([]byte, error) {
	st := smaState{
		FastN:   c.fastN,
		SlowN:   c.slowN,
		Qty:     c.qty,
		Closes:  append([]float64(nil), c.closes...),
		Pos:     c.pos,
		History: c.history,
		Signals: c.signals,
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
	if st.FastN > 0 {
		c.fastN = st.FastN
	}
	if st.SlowN > 0 {
		c.slowN = st.SlowN
	}
	if st.Qty > 0 {
		c.qty = st.Qty
	}
	c.closes = append([]float64(nil), st.Closes...)
	c.pos = st.Pos
	c.history = st.History
	c.signals = st.Signals
	return nil
}

var _ strategy.StatefulStrategy = (*Crossover)(nil)

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
