package strategy

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

const (
	defaultTradeMaxDistBPS     = 80
	defaultTradeMaxLevels      = 6
	defaultStructuralNearBPS   = 45
	defaultMinStructuralScore  = 3.0
)

// tradeableLevelsForSession returns chart/SR levels near mid for entries (no raw book walls).
func tradeableLevelsForSession(s *AITraderSession, pol DynamicTradingPolicy, mid float64) []AITraderLevel {
	lvls := levelsForConfluence(s, pol)
	return selectTradeableLevels(lvls, mid, tickerForSession(s))
}

func tickerForSession(s *AITraderSession) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(s.Ticker)
}

func aiTraderTradeMaxDistBPS() float64 {
	v := strings.TrimSpace(os.Getenv("AI_TRADER_TRADE_MAX_DIST_BPS"))
	if v == "" {
		return defaultTradeMaxDistBPS
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || n < 20 {
		return defaultTradeMaxDistBPS
	}
	if n > 250 {
		return 250
	}
	return n
}

func aiTraderTradeMaxLevels() int {
	v := strings.TrimSpace(os.Getenv("AI_TRADER_TRADE_MAX_LEVELS"))
	if v == "" {
		return defaultTradeMaxLevels
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 2 {
		return defaultTradeMaxLevels
	}
	if n > 12 {
		return 12
	}
	return n
}

func levelSourceTier(source string) int {
	src := strings.ToLower(strings.TrimSpace(source))
	switch {
	case strings.Contains(src, "daily"):
		return 4
	case strings.Contains(src, "hourly"):
		return 3
	case strings.Contains(src, "advisor"):
		return 3
	case strings.Contains(src, "footprint"), strings.Contains(src, "poc"):
		return 2
	case strings.Contains(src, "bid_wall"), strings.Contains(src, "ask_wall"), strings.Contains(src, "orderbook"):
		return 0
	default:
		return 1
	}
}

func isStructuralLevelSource(source string) bool {
	return levelSourceTier(source) >= 2
}

func isBookWallSource(source string) bool {
	src := strings.ToLower(source)
	return strings.Contains(src, "bid_wall") || strings.Contains(src, "ask_wall")
}

func distanceBPS(mid, price float64) float64 {
	if mid <= 0 || price <= 0 {
		return math.MaxFloat64
	}
	return math.Abs(mid-price) / mid * 10000
}

// normalizeQuotationPrice maps broker RUB quotation to points when mid is in points.
func normalizeQuotationPrice(price, mid float64) float64 {
	if price <= 0 {
		return 0
	}
	if mid <= 0 || mid >= 500 {
		return price
	}
	if price < 300 {
		return price
	}
	// Broker fill often in RUB (e.g. 7564) while mid ~106 pts.
	rubPerPoint := price / mid
	if rubPerPoint < 10 {
		return price
	}
	return price / rubPerPoint
}

func selectTradeableLevels(levels []AITraderLevel, mid float64, ticker string) []AITraderLevel {
	if mid <= 0 || len(levels) == 0 {
		return nil
	}
	maxDist := aiTraderTradeMaxDistBPS()
	maxN := aiTraderTradeMaxLevels()
	type cand struct {
		lv    AITraderLevel
		tier  int
		dist  float64
		score float64
	}
	cands := make([]cand, 0, len(levels))
	for _, lv := range levels {
		px := normalizeQuotationPrice(lv.Price, mid)
		if px <= 0 {
			continue
		}
		lv.Price = px
		tier := levelSourceTier(lv.Source)
		if isBookWallSource(lv.Source) {
			continue
		}
		if tier < 2 {
			continue
		}
		dist := distanceBPS(mid, px)
		if dist > maxDist {
			continue
		}
		sc := float64(tier)*10 - dist*0.05
		if strings.Contains(strings.ToLower(lv.Source), "daily") {
			sc += 5
		}
		_ = ticker
		cands = append(cands, cand{lv: lv, tier: tier, dist: dist, score: sc})
	}
	if len(cands) == 0 {
		return nil
	}
	for i := 0; i < len(cands); i++ {
		for j := i + 1; j < len(cands); j++ {
			if cands[j].score > cands[i].score {
				cands[i], cands[j] = cands[j], cands[i]
			}
		}
	}
	out := make([]AITraderLevel, 0, maxN)
	seen := map[string]bool{}
	for _, c := range cands {
		if len(out) >= maxN {
			break
		}
		k := fmt.Sprintf("%.6f-%s", c.lv.Price, c.lv.Kind)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, c.lv)
	}
	return out
}

func nearestTradeableLevel(levels []AITraderLevel, price, mid float64, maxBPS float64) *AITraderLevel {
	if len(levels) == 0 || mid <= 0 {
		return nil
	}
	px := normalizeQuotationPrice(price, mid)
	var best *AITraderLevel
	bestGap := maxBPS + 1
	for i := range levels {
		lv := &levels[i]
		if distanceBPS(mid, lv.Price) > maxBPS {
			continue
		}
		gap := distanceBPS(px, lv.Price)
		if gap < bestGap {
			bestGap = gap
			best = lv
		}
	}
	if best != nil && bestGap <= maxBPS {
		return best
	}
	return nil
}

func entrySideMatchesLevel(side string, lv *AITraderLevel, mid float64) bool {
	if lv == nil || mid <= 0 {
		return false
	}
	tol := mid * 0.003 // ~30 bps: limit at support/resistance near touch
	kind := strings.ToLower(lv.Kind)
	switch strings.ToLower(side) {
	case "buy":
		if !strings.Contains(kind, "support") && !strings.Contains(kind, "poc") {
			return false
		}
		return lv.Price <= mid+tol
	case "sell":
		if !strings.Contains(kind, "resist") {
			return false
		}
		return lv.Price >= mid-tol
	default:
		return false
	}
}

// entrySignalAllowed gates LLM entries to structural levels aligned with side and chart.
func entrySignalAllowed(sig *AITraderTradeSignal, tradeable []AITraderLevel, f *AITraderFeatures, mctx *AITraderMarketContext) bool {
	if sig == nil || f == nil || f.Mid <= 0 || len(tradeable) == 0 {
		return false
	}
	lv := nearestTradeableLevel(tradeable, sig.LevelPrice, f.Mid, defaultStructuralNearBPS)
	if lv == nil {
		return false
	}
	if !entrySideMatchesLevel(sig.Side, lv, f.Mid) {
		return false
	}
	if isBookWallSource(lv.Source) {
		return false
	}
	if mctx != nil {
		td := trendDirection(mctx)
		if sig.Side == "buy" && td == "down" && levelSourceTier(lv.Source) < 3 {
			return false
		}
		if sig.Side == "sell" && td == "up" && levelSourceTier(lv.Source) < 3 {
			return false
		}
	}
	return true
}

func snapSignalToTradeableLevel(sig *AITraderTradeSignal, tradeable []AITraderLevel, f *AITraderFeatures) (price float64, source string, ok bool) {
	if sig == nil || f == nil || f.Mid <= 0 {
		return 0, "", false
	}
	lv := nearestTradeableLevel(tradeable, sig.LevelPrice, f.Mid, defaultStructuralNearBPS)
	if lv == nil || !entrySideMatchesLevel(sig.Side, lv, f.Mid) {
		return 0, "", false
	}
	return lv.Price, lv.Source, true
}

func formatPositionForLLM(pos int64, avgPrice, mid, sl, tp float64) string {
	avgPts := normalizeQuotationPrice(avgPrice, mid)
	line := fmt.Sprintf("%d лот @ %.4f пт", pos, avgPts)
	if mid > 0 && avgPrice > mid*3 {
		line += fmt.Sprintf(" (брокер %.2f)", avgPrice)
	}
	if sl > 0 {
		line += fmt.Sprintf(" SL=%.4f", normalizeQuotationPrice(sl, mid))
	}
	if tp > 0 {
		line += fmt.Sprintf(" TP=%.4f", normalizeQuotationPrice(tp, mid))
	}
	if mid > 0 && pos != 0 {
		pnlBps := (mid - avgPts) / mid * 10000 * float64(pos)
		line += fmt.Sprintf(" | mid=%.4f uPnL~%.1f bps", mid, pnlBps)
	}
	return line
}

func playbookEntryRulesChartFirst() []string {
	return []string{
		"вход только у сильных уровней (daily/hourly/POC) в пределах ~80 bps от mid",
		"стены стакана (bid/ask wall) — контекст, не цель входа",
		"лонг у support при подтверждении графиком; шорт у resistance — только при явном медвежьем контексте",
		"обязательно stop_loss/take_profit в пунктах рядом с mid",
	}
}

func minStructuralConfluenceScore(pol DynamicTradingPolicy) float64 {
	min := pol.ConfluenceMinScore
	if min < defaultMinStructuralScore {
		min = defaultMinStructuralScore
	}
	return min
}
