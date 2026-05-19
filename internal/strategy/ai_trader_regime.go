package strategy

import (
	"math"

	"github.com/24alert/trading-bot/pkg/metrics"
)

const (
	RegimeTrend    = "trend"
	RegimeRange    = "range"
	RegimeBreakout = "breakout"
	RegimeLowVol   = "low_volatility"
)

// detectSessionRegime uses 1m bars ATR ratio and directional move.
func detectSessionRegime(mctx *AITraderMarketContext) string {
	if mctx == nil || len(mctx.ChartBars) < 8 {
		return RegimeRange
	}
	bars := mctx.ChartBars
	n := len(bars)
	if n > 30 {
		bars = bars[n-30:]
		n = 30
	}
	atr := simpleATR(bars, n)
	if atr <= 0 {
		return RegimeRange
	}
	last := bars[n-1]
	move := math.Abs(last.Close - bars[0].Open)
	moveRatio := move / atr
	// consecutive direction
	up, down := 0, 0
	for i := 1; i < n; i++ {
		if bars[i].Close > bars[i-1].Close {
			up++
		} else if bars[i].Close < bars[i-1].Close {
			down++
		}
	}
	if atr/last.Close*10000 < 3 {
		return RegimeLowVol
	}
	if moveRatio > 2.5 && (up >= n/2 || down >= n/2) {
		return RegimeTrend
	}
	if moveRatio > 3.0 {
		return RegimeBreakout
	}
	return RegimeRange
}

func regimeGaugeValue(regime string) float64 {
	switch regime {
	case RegimeTrend:
		return 1
	case RegimeRange:
		return 2
	case RegimeBreakout:
		return 3
	case RegimeLowVol:
		return 4
	default:
		return 0
	}
}

func updateRegimeMetrics(sessionID, regime string) {
	metrics.AITraderRegime.WithLabelValues(sessionID).Set(regimeGaugeValue(regime))
}

func trendDirection(mctx *AITraderMarketContext) string {
	if mctx == nil || len(mctx.ChartBars) < 3 {
		return ""
	}
	b := mctx.ChartBars
	first := b[len(b)-10].Close
	if len(b) < 10 {
		first = b[0].Close
	}
	last := b[len(b)-1].Close
	if last > first*1.001 {
		return "up"
	}
	if last < first*0.999 {
		return "down"
	}
	return ""
}

func simpleATR(bars []AITraderCandleBar, n int) float64 {
	if n < 2 {
		return 0
	}
	var sum float64
	for i := 1; i < n; i++ {
		tr := math.Max(bars[i].High-bars[i].Low,
			math.Max(math.Abs(bars[i].High-bars[i-1].Close), math.Abs(bars[i].Low-bars[i-1].Close)))
		sum += tr
	}
	return sum / float64(n-1)
}

func playbookATR(s *AITraderSession, mctx *AITraderMarketContext) float64 {
	if mctx != nil && len(mctx.ChartBars) >= 5 {
		n := len(mctx.ChartBars)
		if n > 20 {
			n = 20
		}
		return simpleATR(mctx.ChartBars[len(mctx.ChartBars)-n:], n)
	}
	if s != nil && s.Features != nil && s.Features.Mid > 0 {
		return s.Features.Mid * 0.001
	}
	return 0.01
}
