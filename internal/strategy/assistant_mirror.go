package strategy

import (
	"fmt"
	"math"

	"github.com/24alert/trading-bot/internal/marketdata"
)

// detectMirrorLevels finds prices that acted as both support and resistance after a break.
func detectMirrorLevels(hourly, daily []marketdata.Candle, refPrice float64) []AssistantLevel {
	swings := collectSwingLevels(hourly, 3)
	if len(swings) == 0 {
		swings = collectSwingLevels(daily, 2)
	}
	tol := refPrice * 0.002
	if tol <= 0 {
		tol = 0.001
	}
	candles := hourly
	if len(candles) < 10 {
		candles = daily
	}
	var out []AssistantLevel
	for _, price := range swings {
		support, resist := countReactions(candles, price, tol)
		if support >= 1 && resist >= 1 {
			kind := "mirror"
			role := "mirror"
			strength := 3 + support + resist
			if strength > 5 {
				strength = 5
			}
			out = append(out, AssistantLevel{
				Price:    price,
				Kind:     kind,
				Source:   fmt.Sprintf("mirror S%d/R%d", support, resist),
				Strength: strength,
				Role:     role,
			})
		}
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
