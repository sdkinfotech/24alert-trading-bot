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
