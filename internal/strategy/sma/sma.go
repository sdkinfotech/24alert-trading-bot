package sma

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/24alert/trading-bot/internal/strategy"
)

// Crossover implements a minimal SMA crossover on completed candles.
type Crossover struct {
	fastN   int
	slowN   int
	qty     int64
	closes  []float64
	pos     int64 // +1 long, -1 short, 0 flat
	stopped bool
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
		return nil
	}
	if len(c.closes) > c.slowN+5 {
		// keep a small tail
		c.closes = c.closes[len(c.closes)-c.slowN-5:]
	}
	fast := smaTail(c.closes, c.fastN)
	slow := smaTail(c.closes, c.slowN)
	prev := c.closes[:len(c.closes)-1]
	if len(prev) < c.slowN {
		return nil
	}
	prevFast := smaTail(prev, c.fastN)
	prevSlow := smaTail(prev, c.slowN)
	var out []strategy.Signal
	if prevFast <= prevSlow && fast > slow && c.pos <= 0 {
		out = append(out, strategy.Signal{
			InstrumentUID: k.InstrumentUID,
			Direction:     "buy",
			Quantity:      c.qty,
			OrderType:     "market",
			Reason:        "sma golden cross",
		})
		c.pos = 1
	} else if prevFast >= prevSlow && fast < slow && c.pos >= 0 {
		out = append(out, strategy.Signal{
			InstrumentUID: k.InstrumentUID,
			Direction:     "sell",
			Quantity:      c.qty,
			OrderType:     "market",
			Reason:        "sma death cross",
		})
		c.pos = -1
	}
	return out
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
	FastN  int       `json:"fast_n"`
	SlowN  int       `json:"slow_n"`
	Qty    int64     `json:"qty"`
	Closes []float64 `json:"closes"`
	Pos    int64     `json:"pos"`
}

// Snapshot persists internal buffers for StatefulStrategy.
func (c *Crossover) Snapshot() ([]byte, error) {
	st := smaState{
		FastN:  c.fastN,
		SlowN:  c.slowN,
		Qty:    c.qty,
		Closes: append([]float64(nil), c.closes...),
		Pos:    c.pos,
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
