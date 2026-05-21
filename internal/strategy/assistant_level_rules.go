package strategy

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Level build limits (see docs/TECHNICAL_ASSISTANT.md).
const (
	assistantMaxLevelsTotal   = 18
	assistantMaxMirrorLevels  = 5
	assistantDailyHighsLows   = 2 // per side from daily horizon
	assistantHourlyHighsLows  = 2 // per side from ~5 trading days on 1h
	assistantSwingWingBars    = 5 // hourly swing sensitivity (higher = fewer pivots)
)

// clusterTolerancePct returns merge distance as fraction of reference price.
func clusterTolerancePct(refPrice float64) float64 {
	if refPrice <= 0 {
		return 0.002
	}
	// Shares ~0.25%; low-priced futures slightly wider.
	if refPrice < 50 {
		return 0.004
	}
	return 0.0025
}

// clusterAssistantLevels merges levels closer than clusterTolerance into one zone.
func clusterAssistantLevels(levels []AssistantLevel, refPrice float64) []AssistantLevel {
	if len(levels) == 0 {
		return nil
	}
	tol := refPrice * clusterTolerancePct(refPrice)
	if tol <= 0 {
		tol = 0.01
	}
	sorted := append([]AssistantLevel(nil), levels...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Price < sorted[j].Price })

	var out []AssistantLevel
	cluster := sorted[0]
	count := 1
	mergeInto := func(dst, src AssistantLevel) AssistantLevel {
		if src.Strength > dst.Strength {
			dst.Strength = src.Strength
			dst.Kind = src.Kind
			dst.Role = src.Role
		}
		if src.Touches > dst.Touches {
			dst.Touches = src.Touches
		}
		if src.VolumeInZone > dst.VolumeInZone {
			dst.VolumeInZone = src.VolumeInZone
		}
		dst.Source = mergeSourceLabel(dst.Source, src.Source)
		return dst
	}
	for i := 1; i < len(sorted); i++ {
		l := sorted[i]
		if math.Abs(l.Price-cluster.Price) <= tol {
			cluster.Price = (cluster.Price*float64(count) + l.Price) / float64(count+1)
			count++
			cluster = mergeInto(cluster, l)
			continue
		}
		out = append(out, cluster)
		cluster = l
		count = 1
	}
	out = append(out, cluster)
	return out
}

func mergeSourceLabel(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" || strings.Contains(a, b) {
		return a
	}
	return a + "; " + b
}

// capAssistantLevels keeps strongest levels near current price for operator UI.
func capAssistantLevels(levels []AssistantLevel, refPrice float64, max int) []AssistantLevel {
	if max <= 0 || len(levels) <= max {
		return levels
	}
	if refPrice <= 0 {
		refPrice = levels[0].Price
	}
	maxDist := 0.12 // ignore structural levels >12% away unless very strong daily
	scored := make([]AssistantLevel, 0, len(levels))
	for _, l := range levels {
		dist := math.Abs(l.Price-refPrice) / refPrice
		if dist > maxDist && !strings.HasPrefix(l.Source, "daily") && l.Strength < 5 {
			continue
		}
		scored = append(scored, l)
	}
	if len(scored) == 0 {
		scored = levels
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Strength != scored[j].Strength {
			return scored[i].Strength > scored[j].Strength
		}
		di := math.Abs(scored[i].Price - refPrice)
		dj := math.Abs(scored[j].Price - refPrice)
		return di < dj
	})
	if len(scored) > max {
		scored = scored[:max]
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].Price > scored[j].Price })
	return scored
}

// chartRefPrice is the anchor for distance filters; must match the chart's candle series.
func chartRefPrice(cs assistantCandleSet, tf string) float64 {
	switch tf {
	case "1d":
		if r := lastClose(cs.Daily1y); r > 0 {
			return r
		}
		return lastClose(cs.Daily90d)
	case "1h":
		if r := lastClose(cs.Hourly1m); r > 0 {
			return r
		}
		return lastClose(cs.Hourly1w)
	default:
		if r := lastClose(cs.FiveMin7d); r > 0 {
			return r
		}
		return lastClose(cs.Hourly1m)
	}
}

func filterLevelsForChart(levels []AssistantLevel, tf string, ref float64) []AssistantLevel {
	maxDist := map[string]float64{"1h": 0.06, "5m": 0.018}[tf]
	maxN := map[string]int{"1d": 6, "1h": 10, "5m": 8}[tf]
	var pool []AssistantLevel
	for _, l := range levels {
		switch tf {
		case "1d":
			// Year chart: only structural daily highs/lows (no intraday mirrors/POC).
			if strings.HasPrefix(l.Source, "daily") {
				pool = append(pool, l)
			}
		case "1h":
			if strings.HasPrefix(l.Source, "hourly") || strings.HasPrefix(l.Source, "volume_poc_1h") ||
				(l.Kind == "mirror" && chartDistance(l.Price, ref) <= maxDist) {
				pool = append(pool, l)
			}
		case "5m":
			if chartDistance(l.Price, ref) <= maxDist {
				pool = append(pool, l)
			}
		default:
			pool = append(pool, l)
		}
	}
	return capAssistantLevels(pool, ref, maxN)
}

func chartDistance(price, ref float64) float64 {
	if ref <= 0 {
		return 0
	}
	return math.Abs(price-ref) / ref
}

// assignLevelIDs sets L1..Ln after sort by strength.
func assignLevelIDs(levels []AssistantLevel) []AssistantLevel {
	for i := range levels {
		levels[i].ID = fmt.Sprintf("L%d", i+1)
	}
	return levels
}
