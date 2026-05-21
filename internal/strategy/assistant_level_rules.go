package strategy

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Level build limits (see docs/TECHNICAL_ASSISTANT.md).
const (
	assistantMaxLevelsTotal  = 18
	assistantMaxMirrorLevels = 5
	assistantDailyHighsLows  = 2
	assistantHourlyHighsLows = 2
	assistantDailySwingWing  = 3
	assistantDailySwingMax   = 4
	assistantSwingWingBars   = 5
)

func clusterTolerancePct(refPrice float64) float64 {
	if refPrice <= 0 {
		return 0.002
	}
	if refPrice < 50 {
		return 0.004
	}
	return 0.0025
}

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

// rejectLevelsNearZones drops intraday candidates already covered by higher-TF zones.
func rejectLevelsNearZones(candidates, zones []AssistantLevel, tol float64) []AssistantLevel {
	if len(candidates) == 0 {
		return nil
	}
	var out []AssistantLevel
	for _, c := range candidates {
		if levelNearAnyZone(c.Price, zones, tol) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func levelNearAnyZone(price float64, zones []AssistantLevel, tol float64) bool {
	for _, z := range zones {
		if math.Abs(z.Price-price) <= tol {
			return true
		}
	}
	return false
}

func capAssistantLevels(levels []AssistantLevel, refPrice float64, max int) []AssistantLevel {
	if max <= 0 || len(levels) <= max {
		return levels
	}
	if refPrice <= 0 {
		refPrice = levels[0].Price
	}
	nearBand := 0.025
	var near, far []AssistantLevel
	for _, l := range levels {
		dist := math.Abs(l.Price-refPrice) / refPrice
		if dist <= nearBand || strings.HasPrefix(l.Source, "volume_poc") || l.Kind == "mirror" {
			near = append(near, l)
		} else if strings.HasPrefix(l.Source, "daily") || l.Strength >= 4 {
			far = append(far, l)
		}
	}
	sortByStrengthDist := func(a []AssistantLevel) {
		sort.Slice(a, func(i, j int) bool {
			if a[i].Strength != a[j].Strength {
				return a[i].Strength > a[j].Strength
			}
			return math.Abs(a[i].Price-refPrice) < math.Abs(a[j].Price-refPrice)
		})
	}
	sortByStrengthDist(near)
	sortByStrengthDist(far)
	nearCap := max / 2
	if nearCap < 3 {
		nearCap = 3
	}
	farCap := max - nearCap
	if farCap < 2 {
		farCap = 2
		nearCap = max - farCap
	}
	out := append(takeAssistantLevels(near, nearCap), takeAssistantLevels(far, farCap)...)
	if len(out) < max {
		rest := append([]AssistantLevel(nil), levels...)
		sortByStrengthDist(rest)
		seen := map[string]bool{}
		for _, l := range out {
			seen[fmt.Sprintf("%.4f", l.Price)] = true
		}
		for _, l := range rest {
			k := fmt.Sprintf("%.4f", l.Price)
			if seen[k] {
				continue
			}
			out = append(out, l)
			seen[k] = true
			if len(out) >= max {
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Price > out[j].Price })
	return out
}

func takeAssistantLevels(in []AssistantLevel, n int) []AssistantLevel {
	if n <= 0 || len(in) == 0 {
		return nil
	}
	if len(in) <= n {
		return append([]AssistantLevel(nil), in...)
	}
	return append([]AssistantLevel(nil), in[:n]...)
}

func filterLevelsForChart(levels []AssistantLevel, tf string, ref float64) []AssistantLevel {
	maxDist := map[string]float64{"1h": 0.08, "5m": 0.02}[tf]
	maxN := map[string]int{"1d": 8, "1h": 10, "5m": 8}[tf]
	var pool []AssistantLevel
	for _, l := range levels {
		switch tf {
		case "1d":
			if isDailyStructuralSource(l.Source) {
				pool = append(pool, l)
			}
		case "1h":
			if strings.HasPrefix(l.Source, "hourly") || strings.HasPrefix(l.Source, "volume_poc_1h") ||
				strings.HasPrefix(l.Source, "mirror") {
				if chartDistance(l.Price, ref) <= maxDist {
					pool = append(pool, l)
				}
			}
			// Show nearest daily bracket on 1h for context.
			if isDailyStructuralSource(l.Source) && chartDistance(l.Price, ref) <= 0.12 {
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
	pool = dedupeLevelsByPrice(pool)
	return capAssistantLevels(pool, ref, maxN)
}

func isDailyStructuralSource(source string) bool {
	return strings.HasPrefix(source, "daily") || strings.HasPrefix(source, "volume_poc_1d")
}

func dedupeLevelsByPrice(levels []AssistantLevel) []AssistantLevel {
	seen := map[string]bool{}
	var out []AssistantLevel
	for _, l := range levels {
		k := fmt.Sprintf("%.4f", l.Price)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, l)
	}
	return out
}

func chartDistance(price, ref float64) float64 {
	if ref <= 0 {
		return 0
	}
	return math.Abs(price-ref) / ref
}

func assignLevelIDs(levels []AssistantLevel) []AssistantLevel {
	for i := range levels {
		levels[i].ID = fmt.Sprintf("L%d", i+1)
	}
	return levels
}

func strengthFromLevel(l AssistantLevel) int {
	s := 2
	switch {
	case isDailyStructuralSource(l.Source):
		s = 4
	case strings.HasPrefix(l.Source, "hourly"):
		s = 3
	case l.Kind == "mirror":
		s = 3
	case strings.HasPrefix(l.Source, "volume_poc"):
		s = 4
	}
	if l.Touches >= 3 {
		s++
	}
	if l.Touches >= 10 {
		s++
	}
	if s > 5 {
		return 5
	}
	if s < 1 {
		return 1
	}
	return s
}
