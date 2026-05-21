package strategy

import (
	"fmt"
	"math"
	"sort"

	"github.com/24alert/trading-bot/internal/marketdata"
)

// detectMirrorLevels finds clustered swing zones where price flipped support↔resistance.
// Rules: wing≥5 on 1h, cluster nearby swings, require S≥2 and R≥2, max assistantMaxMirrorLevels.
func detectMirrorLevels(hourly, daily []marketdata.Candle, refPrice float64) []AssistantLevel {
	candles := hourly
	if len(candles) < 24 {
		candles = daily
	}
	swings := collectSwingLevels(candles, assistantSwingWingBars)
	if len(swings) == 0 {
		return nil
	}
	clusterTol := refPrice * clusterTolerancePct(refPrice)
	clustered := mergeNearLevels(swings, clusterTol)
	touchTol := refPrice * 0.0015
	if touchTol <= 0 {
		touchTol = 0.001
	}

	type cand struct {
		price            float64
		support, resist int
	}
	var candidates []cand
	for _, price := range clustered {
		s, r := countReactions(candles, price, touchTol)
		if s >= 2 && r >= 2 && s+r >= 5 {
			candidates = append(candidates, cand{price: price, support: s, resist: r})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		si := candidates[i].support + candidates[i].resist
		sj := candidates[j].support + candidates[j].resist
		return si > sj
	})
	if len(candidates) > assistantMaxMirrorLevels {
		candidates = candidates[:assistantMaxMirrorLevels]
	}
	out := make([]AssistantLevel, 0, len(candidates))
	for _, c := range candidates {
		strength := 3 + (c.support+c.resist)/4
		if strength > 5 {
			strength = 5
		}
		out = append(out, AssistantLevel{
			Price:    c.price,
			Kind:     "mirror",
			Source:   fmt.Sprintf("mirror S%d/R%d", c.support, c.resist),
			Strength: strength,
			Role:     "mirror",
		})
	}
	return out
}

func collectSwingLevels(candles []marketdata.Candle, wing int) []float64 {
	if len(candles) < wing*2+3 {
		return nil
	}
	seen := map[string]bool{}
	var prices []float64
	add := func(p float64) {
		key := fmt.Sprintf("%.5f", p)
		if !seen[key] {
			seen[key] = true
			prices = append(prices, p)
		}
	}
	for i := wing; i < len(candles)-wing; i++ {
		isHigh, isLow := true, true
		for j := i - wing; j <= i+wing; j++ {
			if j == i {
				continue
			}
			if candles[j].High >= candles[i].High {
				isHigh = false
			}
			if candles[j].Low <= candles[i].Low {
				isLow = false
			}
		}
		if isHigh {
			add(candles[i].High)
		}
		if isLow {
			add(candles[i].Low)
		}
	}
	return prices
}

func countReactions(candles []marketdata.Candle, level, tol float64) (support, resistance int) {
	for i := 1; i < len(candles); i++ {
		c := candles[i]
		prev := candles[i-1]
		if !candleTouchesLevel(c, level, tol) {
			continue
		}
		if prev.Close < level && c.Close > level {
			resistance++
		}
		if prev.Close > level && c.Close < level {
			support++
		}
		if c.Low <= level+tol && c.Close > level && c.Close > c.Open {
			support++
		}
		if c.High >= level-tol && c.Close < level && c.Close < c.Open {
			resistance++
		}
	}
	return support, resistance
}

func mergeNearLevels(levels []float64, tol float64) []float64 {
	if len(levels) == 0 {
		return nil
	}
	sortLevels(levels)
	var out []float64
	cluster := levels[0]
	count := 1
	for i := 1; i < len(levels); i++ {
		if math.Abs(levels[i]-cluster) <= tol {
			cluster = (cluster*float64(count) + levels[i]) / float64(count+1)
			count++
			continue
		}
		out = append(out, cluster)
		cluster = levels[i]
		count = 1
	}
	out = append(out, cluster)
	return out
}

func sortLevels(p []float64) {
	for i := 1; i < len(p); i++ {
		for j := i; j > 0 && p[j] < p[j-1]; j-- {
			p[j], p[j-1] = p[j-1], p[j]
		}
	}
}
