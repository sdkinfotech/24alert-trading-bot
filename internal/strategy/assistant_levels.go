package strategy

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/24alert/trading-bot/internal/marketdata"
)

// buildAssistantLevels builds zones top-down: daily → hourly → mirrors/5m POC.
func buildAssistantLevels(cs assistantCandleSet) []AssistantLevel {
	ref := refPrice(cs)
	zoneTol := ref * clusterTolerancePct(ref)

	// --- Tier 0: daily structure (year + quarter swings + bracket + POC) ---
	dailyRows := trimDailyLevels(computeDailyLevels(cs.Daily1y, 365), assistantDailyHighsLows)
	dailyRows = append(dailyRows, dailySwingLevels(cs.Daily90d, 90, assistantDailySwingWing, assistantDailySwingMax)...)
	dailyRows = append(dailyRows, nearestDailyRangeLevels(cs.Daily90d, 90, ref)...)
	daily := levelsFromAITrader(dailyRows)
	if poc := pocLevel(cs.Daily90d, "1d"); poc != nil {
		daily = mergeAssistantLevels(daily, []AssistantLevel{*poc})
	}
	daily = clusterAssistantLevels(daily, ref)

	// --- Tier 1: hourly (skip if already inside daily zone) ---
	hourlyRows := trimHourlyLevels(computeHourlyLevels(cs.Hourly1m, 5*16), assistantHourlyHighsLows)
	hourly := levelsFromAITrader(hourlyRows)
	if poc := pocLevel(cs.Hourly1m, "1h"); poc != nil {
		hourly = mergeAssistantLevels(hourly, []AssistantLevel{*poc})
	}
	hourly = rejectLevelsNearZones(hourly, daily, zoneTol)

	merged := mergeAssistantLevels(daily, hourly)
	merged = clusterAssistantLevels(merged, ref)

	// --- Tier 2: mirrors + 5m POC (only new zones) ---
	mirrors := detectMirrorLevels(cs.Hourly1m, cs.Daily90d, ref)
	mirrors = rejectLevelsNearZones(mirrors, merged, zoneTol*0.85)
	var intraday []AssistantLevel
	if poc := pocLevel(cs.FiveMin7d, "5m"); poc != nil {
		intraday = append(intraday, *poc)
	}
	intraday = rejectLevelsNearZones(intraday, merged, zoneTol*0.85)
	merged = mergeAssistantLevels(merged, mirrors, intraday)
	merged = clusterAssistantLevels(merged, ref)

	for i := range merged {
		enrichLevelVolumeStats(&merged[i], cs)
		merged[i].Strength = strengthFromLevel(merged[i])
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

// refPrice — единая опорная цена для отчёта, кластеров и фильтра всех графиков.
func refPrice(cs assistantCandleSet) float64 {
	if r := lastClose(cs.FiveMin7d); r > 0 {
		return r
	}
	if r := lastClose(cs.Hourly1m); r > 0 {
		return r
	}
	return lastClose(cs.Daily1y)
}

func nearestDailyRangeLevels(daily []marketdata.Candle, days int, ref float64) []AITraderLevel {
	if ref <= 0 || len(daily) == 0 {
		return nil
	}
	start := 0
	if len(daily) > days {
		start = len(daily) - days
	}
	var bestHigh, bestLow *AITraderLevel
	highDist, lowDist := math.MaxFloat64, math.MaxFloat64
	for i := start; i < len(daily); i++ {
		c := daily[i]
		date := c.Time.UTC().Format("2006-01-02")
		if c.High >= ref {
			if d := c.High - ref; d < highDist {
				highDist = d
				bestHigh = &AITraderLevel{Price: c.High, Kind: "resistance", Source: "daily90_high " + date, Rank: 1}
			}
		}
		if c.Low <= ref {
			if d := ref - c.Low; d < lowDist {
				lowDist = d
				bestLow = &AITraderLevel{Price: c.Low, Kind: "support", Source: "daily90_low " + date, Rank: 1}
			}
		}
	}
	var out []AITraderLevel
	if bestHigh != nil {
		out = append(out, *bestHigh)
	}
	if bestLow != nil {
		out = append(out, *bestLow)
	}
	return out
}

// dailySwingLevels adds significant D1 swings (not only absolute year extremes).
func dailySwingLevels(daily []marketdata.Candle, days, wing, maxN int) []AITraderLevel {
	if len(daily) < wing*2+3 {
		return nil
	}
	start := 0
	if len(daily) > days {
		start = len(daily) - days
	}
	slice := daily[start:]
	ref := lastClose(slice)
	swings := collectSwingLevels(slice, wing)
	if len(swings) == 0 {
		return nil
	}
	tol := ref * clusterTolerancePct(ref)
	clustered := mergeNearLevels(swings, tol)
	sort.Slice(clustered, func(i, j int) bool {
		return math.Abs(clustered[i]-ref) > math.Abs(clustered[j]-ref)
	})
	if len(clustered) > maxN {
		clustered = clustered[:maxN]
	}
	out := make([]AITraderLevel, 0, len(clustered))
	for rank, price := range clustered {
		kind := "resistance"
		if price < ref {
			kind = "support"
		}
		out = append(out, AITraderLevel{
			Price:  price,
			Kind:   kind,
			Source: fmt.Sprintf("daily_swing_%s rank%d", kind, rank+1),
			Rank:   rank + 1,
		})
	}
	return out
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

func candlesForLevelSource(source string, cs assistantCandleSet) []marketdata.Candle {
	switch {
	case isDailyStructuralSource(source):
		if len(cs.Daily90d) > 0 {
			return cs.Daily90d
		}
		return cs.Daily1y
	case strings.HasPrefix(source, "hourly"), strings.HasPrefix(source, "mirror"), strings.HasPrefix(source, "volume_poc_1h"):
		return cs.Hourly1m
	case strings.HasPrefix(source, "volume_poc_5m"):
		return cs.FiveMin7d
	default:
		return cs.Hourly1m
	}
}

func enrichLevelVolumeStats(l *AssistantLevel, cs assistantCandleSet) {
	if l == nil {
		return
	}
	candles := candlesForLevelSource(l.Source, cs)
	tol := levelTolerance(l.Price)
	touches, vol := countTouchesDeduped(candles, l.Price, tol)
	var reactions []float64
	for _, c := range candles {
		if !candleTouchesLevel(c, l.Price, tol) {
			continue
		}
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
	tf := levelSourceTF(l.Source)
	l.VolumeNote = fmt.Sprintf("%s: касаний %d, объём в зоне %d, реакция %.0f bps", tf, touches, vol, l.AvgReactionBPS)
}

func levelSourceTF(source string) string {
	switch {
	case isDailyStructuralSource(source):
		return "1d"
	case strings.HasPrefix(source, "hourly"), strings.HasPrefix(source, "mirror"), strings.HasPrefix(source, "volume_poc_1h"):
		return "1h"
	case strings.HasPrefix(source, "volume_poc_5m"):
		return "5m"
	default:
		return "1h"
	}
}

func levelTolerance(price float64) float64 {
	if price <= 0 {
		return 0.001
	}
	return math.Max(price*0.0015, 0.0001)
}

// countTouchesDeduped counts entries into the zone (not every bar inside the zone).
func countTouchesDeduped(candles []marketdata.Candle, price, tol float64) (touches int, vol int64) {
	inZone := false
	for _, c := range candles {
		touch := candleTouchesLevel(c, price, tol)
		if touch {
			vol += c.Volume
			if !inZone {
				touches++
				inZone = true
			}
			continue
		}
		inZone = false
	}
	return touches, vol
}

func candleTouchesLevel(c marketdata.Candle, level, tol float64) bool {
	lo := level - tol
	hi := level + tol
	return c.High >= lo && c.Low <= hi
}

func candleReactionBPS(c marketdata.Candle, kind string) float64 {
	if c.Open <= 0 {
		return 0
	}
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
	ref := refPrice(cs)
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
