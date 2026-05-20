package strategy

import "context"

// tryStructuralPlaybookEntry places limits only at filtered structural levels (chart-first).
func (r *Runner) tryStructuralPlaybookEntry(ctx context.Context, s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext, live bool) bool {
	if s == nil || f == nil || f.Mid <= 0 {
		return false
	}
	regime := s.SessionRegime
	if regime == "" {
		regime = detectSessionRegime(mctx)
	}
	if !allowNewEntry(s, regime) || !s.sessionStrategyAllowsEntry() {
		return false
	}
	if !playbookEntryEnabled(s) {
		return false
	}
	pol := effectivePolicy(s)
	lvls := tradeableLevelsForSession(s, pol, f.Mid)
	if len(lvls) == 0 {
		return false
	}
	scored := scoreLevels(lvls, f, mctx, nil)
	minScore := minStructuralConfluenceScore(pol)
	mid := f.Mid
	allowBuy := regime != RegimeTrend || trendDirection(mctx) != "down"
	allowSell := regime != RegimeTrend || trendDirection(mctx) != "up"
	if pol.MarketBias == "bearish" {
		allowBuy = false
	} else if pol.MarketBias == "bullish" {
		allowSell = false
	}
	placed := false
	if sup, ok := bestSupportLevel(scored, mid, minScore); ok && allowBuy {
		if live {
			r.placeLiveLimit(ctx, s, "buy", sup.Price, 1, sup.Source, "structural support")
		} else {
			r.placePaperOrder(s, "buy", sup.Price, 1, sup.Source, "structural support")
		}
		placed = true
	}
	if res, ok := bestResistanceLevel(scored, mid, minScore); ok && allowSell {
		if live {
			r.placeLiveLimit(ctx, s, "sell", res.Price, 1, res.Source, "structural resistance")
		} else {
			r.placePaperOrder(s, "sell", res.Price, 1, res.Source, "structural resistance")
		}
		placed = true
	}
	return placed
}

// tryValidatedLLMEntry places an LLM signal only when snapped to a structural level.
func (r *Runner) tryValidatedLLMEntry(ctx context.Context, s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext, sig *AITraderTradeSignal, live bool) bool {
	if sig == nil {
		return false
	}
	pol := effectivePolicy(s)
	if !sig.actionableWith(pol.EntryMinConfidence) || !s.sessionStrategyAllowsEntry() {
		return false
	}
	tradeable := tradeableLevelsForSession(s, pol, f.Mid)
	if !s.sessionStrategyAllowsSide(sig.Side) {
		return false
	}
	if reason := microstructureBlocksEntry(s, sig.Side); reason != "" {
		return false
	}
	if !entrySignalAllowed(sig, tradeable, f, mctx) {
		return false
	}
	px, src, ok := snapSignalToTradeableLevel(sig, tradeable, f)
	if !ok {
		return false
	}
	if live {
		r.placeLiveLimit(ctx, s, sig.Side, px, 1, src, "llm@"+src+": "+sig.Reason)
	} else {
		r.placePaperOrder(s, sig.Side, px, 1, src, "llm@"+src+": "+sig.Reason)
	}
	return true
}
