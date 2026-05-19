package strategy

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// LevelPlaybook is the AI + deterministic intraday plan for level trading.
type LevelPlaybook struct {
	Summary     string              `json:"summary"`
	MarketBias  string              `json:"market_bias,omitempty"`
	Levels      []AITraderLevel     `json:"levels"`
	EntryRules  []string            `json:"entry_rules,omitempty"`
	RiskNotes   []string            `json:"risk_notes,omitempty"`
	SLMultATR   float64             `json:"sl_mult_atr,omitempty"`
	TPMultATR   float64             `json:"tp_mult_atr,omitempty"`
	ReadyToTrade bool               `json:"ready_to_trade,omitempty"`
}

func mergePlaybooks(base, overlay *LevelPlaybook) *LevelPlaybook {
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}
	out := *base
	if overlay.Summary != "" {
		out.Summary = overlay.Summary
	}
	if overlay.MarketBias != "" {
		out.MarketBias = overlay.MarketBias
	}
	seen := map[string]bool{}
	for _, l := range out.Levels {
		seen[fmt.Sprintf("%.6f-%s", l.Price, l.Kind)] = true
	}
	for _, l := range overlay.Levels {
		k := fmt.Sprintf("%.6f-%s", l.Price, l.Kind)
		if !seen[k] {
			out.Levels = append(out.Levels, l)
			seen[k] = true
		}
	}
	out.EntryRules = append(out.EntryRules, overlay.EntryRules...)
	out.RiskNotes = append(out.RiskNotes, overlay.RiskNotes...)
	out.ReadyToTrade = out.ReadyToTrade || overlay.ReadyToTrade
	return &out
}

func (r *Runner) buildLevelPlaybook(s *AITraderSession, f *AITraderFeatures) *LevelPlaybook {
	if s == nil {
		return nil
	}
	levels := make([]AITraderLevel, 0, 12)
	if s.ctxState != nil {
		levels = append(levels, s.ctxState.levels...)
	}
	levels = mergeIntradayLevels(levels, f, s.ctxState)

	pb := &LevelPlaybook{
		Levels:     levels,
		SLMultATR:  0.5,
		TPMultATR:  1.5,
		MarketBias: "neutral",
		EntryRules: []string{
			"лимитка у поддержки при отскоке в ленте (дельта покупателей)",
			"лимитка у сопротивления при отказе и росте sell-принтов",
			"не входить при широком спреде или stale feed",
		},
		RiskNotes: []string{
			"paper-only: виртуальные лимитки, без брокера",
			"макс 1 лот, 2 активные заявки",
		},
	}
	if f != nil && !f.Stale {
		pb.Summary = fmt.Sprintf("%s: %d уровней, mid %.4f, spread %.1f bps",
			s.Ticker, len(levels), f.Mid, f.SpreadBPS)
	} else {
		pb.Summary = fmt.Sprintf("%s: уровни загружены, ожидаем свежий стакан", s.Ticker)
	}
	return pb
}

func mergeIntradayLevels(base []AITraderLevel, f *AITraderFeatures, ctx *aiTraderContextState) []AITraderLevel {
	out := append([]AITraderLevel(nil), base...)
	seen := map[string]bool{}
	for _, l := range out {
		seen[fmt.Sprintf("%.6f-%s", l.Price, l.Kind)] = true
	}
	if f != nil {
		if f.LargestBidWall.Quantity > 0 {
			k := fmt.Sprintf("%.6f-support", f.LargestBidWall.Price)
			if !seen[k] {
				out = append(out, AITraderLevel{
					Price: f.LargestBidWall.Price, Kind: "support", Source: "bid_wall", Rank: f.LargestBidWall.Rank,
				})
				seen[k] = true
			}
		}
		if f.LargestAskWall.Quantity > 0 {
			k := fmt.Sprintf("%.6f-resistance", f.LargestAskWall.Price)
			if !seen[k] {
				out = append(out, AITraderLevel{
					Price: f.LargestAskWall.Price, Kind: "resistance", Source: "ask_wall", Rank: f.LargestAskWall.Rank,
				})
				seen[k] = true
			}
		}
	}
	if ctx != nil && len(ctx.footprint) > 0 {
		poc := footprintPOCFromState(ctx.footprint)
		if poc > 0 {
			k := fmt.Sprintf("%.6f-poc", poc)
			if !seen[k] {
				out = append(out, AITraderLevel{Price: poc, Kind: "poc", Source: "footprint_1m", Rank: 1})
			}
		}
	}
	return out
}

func footprintPOCFromState(cols []*footprintMinute) float64 {
	var bestPrice float64
	var bestVol int64
	for _, col := range cols {
		if col == nil {
			continue
		}
		for price, cell := range col.cells {
			if cell == nil {
				continue
			}
			total := cell.buyVol + cell.sellVol
			if total > bestVol {
				bestVol = total
				bestPrice = price
			}
		}
	}
	return bestPrice
}

// Nearest support/resistance for paper orders.
func nearestSupport(levels []AITraderLevel, mid float64) (AITraderLevel, bool) {
	var best AITraderLevel
	var ok bool
	bestDist := math.MaxFloat64
	for _, l := range levels {
		if l.Kind != "support" && l.Kind != "poc" {
			continue
		}
		if l.Price > mid {
			continue
		}
		d := mid - l.Price
		if d < bestDist {
			bestDist = d
			best = l
			ok = true
		}
	}
	return best, ok
}

func nearestResistance(levels []AITraderLevel, mid float64) (AITraderLevel, bool) {
	var best AITraderLevel
	var ok bool
	bestDist := math.MaxFloat64
	for _, l := range levels {
		if l.Kind != "resistance" {
			continue
		}
		if l.Price < mid {
			continue
		}
		d := l.Price - mid
		if d < bestDist {
			bestDist = d
			best = l
			ok = true
		}
	}
	return best, ok
}

// formatLevelsByDistance returns human/LLM-readable lines sorted by proximity to ref price.
func formatLevelsByDistance(levels []AITraderLevel, ref float64) []string {
	if len(levels) == 0 {
		return nil
	}
	type item struct {
		lv   AITraderLevel
		dist float64
		bps  float64
	}
	items := make([]item, 0, len(levels))
	for _, lv := range levels {
		if lv.Price <= 0 {
			continue
		}
		dist := ref - lv.Price
		bps := 0.0
		if ref > 0 {
			bps = (dist / ref) * 10000
		}
		items = append(items, item{lv: lv, dist: dist, bps: bps})
	}
	sort.Slice(items, func(i, j int) bool {
		return math.Abs(items[i].bps) < math.Abs(items[j].bps)
	})
	out := make([]string, 0, len(items))
	for _, it := range items {
		dir := "at price"
		if it.bps > 2 {
			dir = fmt.Sprintf("%.0f bps below", it.bps)
		} else if it.bps < -2 {
			dir = fmt.Sprintf("%.0f bps above", -it.bps)
		}
		out = append(out, fmt.Sprintf("- %s %.4f | %s | %s", it.lv.Kind, it.lv.Price, it.lv.Source, dir))
	}
	return out
}

func playbookEntrySummary(pb *LevelPlaybook) string {
	if pb == nil {
		return ""
	}
	var parts []string
	for _, r := range pb.EntryRules {
		r = strings.TrimSpace(r)
		if r != "" {
			parts = append(parts, r)
		}
	}
	return strings.Join(parts, "; ")
}
