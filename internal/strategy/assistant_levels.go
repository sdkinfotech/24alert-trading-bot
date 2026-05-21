package strategy

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/24alert/trading-bot/internal/marketdata"
)

func buildAssistantLevels(cs assistantCandleSet) []AssistantLevel {
	// Structural: year/quarter on daily; intraday focus on last ~5 sessions on 1h.
	dailyLv := levelsFromAITrader(trimDailyLevels(computeDailyLevels(cs.Daily1y, 365), assistantDailyHighsLows))
	hourlyLv := levelsFromAITrader(trimHourlyLevels(computeHourlyLevels(cs.Hourly1m, 5*16), assistantHourlyHighsLows))
	poc5m := pocLevel(cs.FiveMin7d, "5m")
	poc1h := pocLevel(cs.Hourly1m, "1h")

	merged := mergeAssistantLevels(dailyLv, hourlyLv)
	if poc5m != nil {
		merged = append(merged, *poc5m)
	}
	if poc1h != nil {
		merged = append(merged, *poc1h)
	}

	ref := lastClose(cs.FiveMin7d)
	if ref <= 0 {
		ref = lastClose(cs.Hourly1m)
	}
	mirrors := detectMirrorLevels(cs.Hourly1m, cs.Daily90d, ref)
	merged = mergeAssistantLevels(merged, mirrors)
	merged = clusterAssistantLevels(merged, ref)

	for i := range merged {
		enrichLevelVolumeStats(&merged[i], cs)
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Strength != merged[j].Strength {
			return merged[i].Strength > merged[j].Strength
		}
		return merged[i].Price > merged[j].Price
	})
	merged = capAssistantLevels(merged, ref, assistantMaxLevelsTotal)
	return assignLevelIDs(merged)
}

// trimDailyLevels keeps top N highs and N lows from computeDailyLevels output.
func trimDailyLevels(rows []AITraderLevel, n int) []AITraderLevel {
	var highs, lows []AITraderLevel
	for _, l := range rows {
		if l.Kind == "resistance" {
			highs = append(highs, l)
		} else {
			lows = append(lows, l)
		}
	}
	if len(highs) > n {
		highs = highs[:n]
	}
	if len(lows) > n {
		lows = lows[:n]
	}
	return append(highs, lows...)
}

func trimHourlyLevels(rows []AITraderLevel, n int) []AITraderLevel {
	return trimDailyLevels(rows, n)
}

func levelsFromAITrader(rows []AITraderLevel) []AssistantLevel {
	out := make([]AssistantLevel, 0, len(rows))
	for _, l := range rows {
		kind := strings.TrimSpace(l.Kind)
		if kind == "" {
			kind = "pivot"
		}
		strength := 3
		if strings.HasPrefix(l.Source, "daily") {
			strength = 4
		}
		if l.Rank == 1 {
			strength++
		}
		if strength > 5 {
			strength = 5
		}
		out = append(out, AssistantLevel{
			Price:    l.Price,
			Kind:     kind,
			Source:   l.Source,
			Strength: strength,
			Role:     kind,
		})
	}
	return out
}

func mergeAssistantLevels(parts ...[]AssistantLevel) []AssistantLevel {
	seen := map[string]bool{}
	var out []AssistantLevel
	for _, part := range parts {
		for _, l := range part {
			key := fmt.Sprintf("%.4f-%s", l.Price, l.Kind)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, l)
		}
	}
	return out
}

func pocLevel(candles []marketdata.Candle, tf string) *AssistantLevel {
	if len(candles) == 0 {
		return nil
	}
	type bucket struct {
		lo, hi float64
		vol    int64
	}
	if len(candles) == 0 {
		return nil
	}
	minP, maxP := candles[0].Low, candles[0].High
	for _, c := range candles {
		if c.Low < minP {
			minP = c.Low
		}
		if c.High > maxP {
			maxP = c.High
		}
	}
	if maxP <= minP {
		return nil
	}
	const bins = 40
	step := (maxP - minP) / bins
	if step <= 0 {
		return nil
	}
	buckets := make([]bucket, bins)
	for _, c := range candles {
		mid := (c.High + c.Low) / 2
		idx := int((mid - minP) / step)
		if idx < 0 {
			idx = 0
		}
		if idx >= bins {
			idx = bins - 1
		}
		buckets[idx].vol += c.Volume
		buckets[idx].lo = minP + float64(idx)*step
		buckets[idx].hi = buckets[idx].lo + step
	}
	var best int
	for i := 1; i < bins; i++ {
		if buckets[i].vol > buckets[best].vol {
			best = i
		}
	}
	if buckets[best].vol <= 0 {
		return nil
	}
	price := (buckets[best].lo + buckets[best].hi) / 2
	return &AssistantLevel{
		Price:    price,
		Kind:     "poc",
		Source:   "volume_poc_" + tf,
		Strength: 4,
		Role:     "poc",
	}
}

func enrichLevelVolumeStats(l *AssistantLevel, cs assistantCandleSet) {
	if l == nil {
		return
	}
	tol := levelTolerance(l.Price, cs)
	candles := cs.Hourly1m
	if len(candles) == 0 {
		candles = cs.FiveMin7d
	}
	var vol int64
	var touches int
	var reactions []float64
	for _, c := range candles {
		if !candleTouchesLevel(c, l.Price, tol) {
			continue
		}
		touches++
		vol += c.Volume
		reactions = append(reactions, candleReactionBPS(c, l.Kind))
	}
	l.Touches = touches
	l.VolumeInZone = vol
	if len(reactions) > 0 {
		var sum float64
		for _, r := range reactions {
			sum += r
		}
		l.AvgReactionBPS = sum / float64(len(reactions))
	}
	l.VolumeNote = fmt.Sprintf("касаний %d, объём в зоне %d, средняя реакция %.0f bps", touches, vol, l.AvgReactionBPS)
}

func levelTolerance(price float64, cs assistantCandleSet) float64 {
	if price <= 0 {
		return 0.001
	}
	// ~0.15% or min tick proxy
	return math.Max(price*0.0015, 0.0001)
}

func candleTouchesLevel(c marketdata.Candle, level, tol float64) bool {
	lo := level - tol
	hi := level + tol
	return c.High >= lo && c.Low <= hi
}

func candleReactionBPS(c marketdata.Candle, kind string) float64 {
	body := (c.Close - c.Open) / c.Open * 10000
	if strings.Contains(kind, "support") && body > 0 {
		return body
	}
	if strings.Contains(kind, "resistance") && body < 0 {
		return -body
	}
	return math.Abs(body)
}

func buildAssistantCharts(cs assistantCandleSet, levels []AssistantLevel) map[string]AssistantChartPayload {
	ref := lastClose(cs.FiveMin7d)
	if ref <= 0 {
		ref = lastClose(cs.Hourly1m)
	}
	return map[string]AssistantChartPayload{
		"1d": {
			Timeframe: "1d",
			Horizon:   "year",
			Candles:   candlesToAssistantBars(tailCandles(cs.Daily1y, 400)),
			Levels:    filterLevelsForChart(levels, "1d", ref),
		},
		"1h": {
			Timeframe: "1h",
			Horizon:   "month",
			Candles:   candlesToAssistantBars(tailCandles(cs.Hourly1m, 600)),
			Levels:    filterLevelsForChart(levels, "1h", ref),
		},
		"5m": {
			Timeframe: "5m",
			Horizon:   "week",
			Candles:   candlesToAssistantBars(cs.FiveMin7d),
			Levels:    filterLevelsForChart(levels, "5m", ref),
		},
	}
}

func recentTrendLabel(candles []marketdata.Candle, n int) string {
	if len(candles) < 2 {
		return "unknown"
	}
	if n <= 0 || n > len(candles) {
		n = len(candles)
	}
	slice := candles[len(candles)-n:]
	start := slice[0].Close
	end := slice[len(slice)-1].Close
	if start <= 0 {
		return "unknown"
	}
	ch := (end - start) / start * 100
	switch {
	case ch > 1.5:
		return fmt.Sprintf("up %.1f%%", ch)
	case ch < -1.5:
		return fmt.Sprintf("down %.1f%%", ch)
	default:
		return fmt.Sprintf("flat %.1f%%", ch)
	}
}
