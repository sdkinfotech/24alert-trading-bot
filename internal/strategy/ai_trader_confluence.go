package strategy

import (
	"math"
	"strings"
)

// ScoredLevel is a playbook level with confluence score.
type ScoredLevel struct {
	Level AITraderLevel `json:"level"`
	Score float64       `json:"score"`
}

// scoreLevels ranks levels by multi-source confluence.
func scoreLevels(levels []AITraderLevel, f *AITraderFeatures, mctx *AITraderMarketContext, advisorMentioned map[float64]bool) []ScoredLevel {
	if len(levels) == 0 {
		return nil
	}
	poc := footprintPOC(mctx)
	out := make([]ScoredLevel, 0, len(levels))
	for _, lv := range levels {
		sc := scoreOneLevel(lv, f, mctx, poc, advisorMentioned)
		out = append(out, ScoredLevel{Level: lv, Score: sc})
	}
	sortScoredLevels(out)
	return out
}

func scoreOneLevel(lv AITraderLevel, f *AITraderFeatures, mctx *AITraderMarketContext, poc float64, advisor map[float64]bool) float64 {
	var sc float64
	src := lv.Source
	switch {
	case strings.Contains(src, "daily"):
		sc += 3.5
	case strings.Contains(src, "hourly"):
		sc += 2.8
	case strings.Contains(src, "advisor"):
		sc += 2.5
	}
	if strings.Contains(src, "footprint") || strings.Contains(src, "poc") {
		sc += 2.0
	}
	if isBookWallSource(src) {
		sc += 0.1
	}
	if poc > 0 && math.Abs(lv.Price-poc) < tickEpsilon(lv.Price) {
		sc += 1.5
	}
	if advisor != nil && advisor[roundPrice(lv.Price)] {
		sc += 1.0
	}
	if f != nil {
		if wallNear(f.LargestBidWall, lv.Price) || wallNear(f.LargestAskWall, lv.Price) {
			sc += 0.8
		}
		distBps := math.Abs(f.Mid-lv.Price) / f.Mid * 10000
		if distBps < 30 {
			sc += 0.5
		}
	}
	if lv.Rank > 0 && lv.Rank <= 2 {
		sc += 0.3
	}
	return sc
}

func footprintPOC(mctx *AITraderMarketContext) float64 {
	if mctx == nil || len(mctx.Footprint) == 0 {
		return 0
	}
	col := mctx.Footprint[len(mctx.Footprint)-1]
	var best float64
	var bestVol int64
	for _, c := range col.Cells {
		if c.Total > bestVol {
			bestVol = c.Total
			best = c.Price
		}
	}
	return best
}

func wallNear(w AITraderWall, price float64) bool {
	return w.Quantity > 0 && math.Abs(w.Price-price) < tickEpsilon(price)
}

func roundPrice(p float64) float64 {
	return math.Round(p*10000) / 10000
}

func sortScoredLevels(sl []ScoredLevel) {
	for i := 0; i < len(sl); i++ {
		for j := i + 1; j < len(sl); j++ {
			if sl[j].Score > sl[i].Score {
				sl[i], sl[j] = sl[j], sl[i]
			}
		}
	}
}

func bestSupportLevel(scored []ScoredLevel, mid float64, minScore float64) (AITraderLevel, bool) {
	var best AITraderLevel
	var bestSc float64
	for _, s := range scored {
		if s.Level.Price >= mid {
			continue
		}
		if s.Score < minScore {
			continue
		}
		if s.Score > bestSc {
			bestSc = s.Score
			best = s.Level
		}
	}
	return best, bestSc >= minScore
}

func bestResistanceLevel(scored []ScoredLevel, mid float64, minScore float64) (AITraderLevel, bool) {
	var best AITraderLevel
	var bestSc float64
	for _, s := range scored {
		if s.Level.Price <= mid {
			continue
		}
		if s.Score < minScore {
			continue
		}
		if s.Score > bestSc {
			bestSc = s.Score
			best = s.Level
		}
	}
	return best, bestSc >= minScore
}
