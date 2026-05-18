package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	defaultAITraderLLMInterval = 15 * time.Second
	aiTraderLLMTimeout         = 45 * time.Second
	// Free high-frequency model; override with AI_TRADER_MODEL. Do not inherit AI_CHAT_MODEL (Sonnet is too costly at ~15s polling).
	defaultAITraderModel = "nvidia/nemotron-3-super-120b-a12b:free"
)

type aiTraderLLMOutput struct {
	Summary    string  `json:"summary"`
	MarketBias string  `json:"market_bias"`
	Action     string  `json:"action"`
	Intent     string  `json:"intent"`
	Reason     string  `json:"reason"`
	NextWatch  string  `json:"next_watch"`
	Confidence float64 `json:"confidence"`
}

func aiTraderLLMInterval(s *AITraderSession) time.Duration {
	if s == nil {
		return defaultAITraderLLMInterval
	}
	sec := os.Getenv("AI_TRADER_LLM_INTERVAL_SEC")
	if sec == "" {
		return defaultAITraderLLMInterval
	}
	var n int
	if _, err := fmt.Sscanf(sec, "%d", &n); err != nil || n < 10 {
		return defaultAITraderLLMInterval
	}
	if n > 120 {
		n = 120
	}
	return time.Duration(n) * time.Second
}

func aiTraderModel() string {
	if m := strings.TrimSpace(os.Getenv("AI_TRADER_MODEL")); m != "" {
		return m
	}
	return defaultAITraderModel
}

func aiTraderLLMEnabled() bool {
	if v := strings.TrimSpace(os.Getenv("AI_TRADER_LLM_ENABLED")); v != "" {
		switch strings.ToLower(v) {
		case "0", "false", "no", "off":
			return false
		default:
			return true
		}
	}
	return strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")) != ""
}

func (r *Runner) shouldRunAITraderLLM(s *AITraderSession) bool {
	if s == nil || !aiTraderLLMEnabled() {
		return false
	}
	if s.lastLLMAt.IsZero() {
		return true
	}
	return time.Since(s.lastLLMAt) >= aiTraderLLMInterval(s)
}

func (r *Runner) refreshAITraderFeaturesOnly(s *AITraderSession, f *AITraderFeatures) {
	r.aiTrader.mu.Lock()
	defer r.aiTrader.mu.Unlock()
	cur := r.aiTrader.findLocked(s.ID)
	if cur == nil || cur.ID != s.ID {
		return
	}
	cur.Features = f
	cur.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if cur.LastDecision != nil && f != nil {
		ev := *cur.LastDecision
		ev.Features = f
		cur.LastDecision = &ev
	}
}

func (r *Runner) decideAITraderWithBrain(ctx context.Context, s *AITraderSession, f *AITraderFeatures) (AITraderDecisionEvent, bool) {
	base := decideAITraderRules(s, f)
	if base.RiskResult == "blocked_stale_feed" || base.RiskResult == "blocked_spread" {
		base.AnalysisSource = "rules"
		return base, true
	}
	if !r.shouldRunAITraderLLM(s) {
		r.refreshAITraderFeaturesOnly(s, f)
		r.attachAITraderMarketContext(s)
		return base, false
	}

	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if apiKey == "" {
		base.AnalysisSource = "rules"
		base.OperatorNote = strings.TrimSpace(base.OperatorNote + " LLM недоступен: OPENROUTER_API_KEY не задан.")
		return base, true
	}

	llmCtx, cancel := context.WithTimeout(ctx, aiTraderLLMTimeout)
	defer cancel()

	mctx := r.snapshotAITraderContext(s)
	ev, err := r.callAITraderLLM(llmCtx, apiKey, s, f, mctx, base)
	if err != nil {
		r.logger.Warn("ai trader llm", "session", s.ID, "error", err)
		base.AnalysisSource = "rules_fallback"
		base.Summary = "LLM временно недоступен, используется rule-engine: " + base.Summary
		base.Reason = err.Error() + "; " + base.Reason
		return base, true
	}
	ev.AnalysisSource = "llm"
	s.lastLLMAt = time.Now().UTC()
	return ev, true
}

func (r *Runner) callAITraderLLM(ctx context.Context, apiKey string, s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext, base AITraderDecisionEvent) (AITraderDecisionEvent, error) {
	msgs := buildAITraderLLMMessages(s, f, mctx, base)
	raw, err := callOpenRouter(apiKey, aiTraderModel(), msgs)
	if err != nil {
		return AITraderDecisionEvent{}, err
	}
	out, err := parseAITraderLLMReply(raw)
	if err != nil {
		return AITraderDecisionEvent{}, err
	}
	return mergeAITraderLLMOutput(s, f, base, out), nil
}

func buildAITraderLLMMessages(s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext, base AITraderDecisionEvent) []chatMessage {
	var b strings.Builder
	b.WriteString("Ты — AI Trader 24alert. Отдельная prompt-driven сессия, НЕ SMA/Level/ORB.\n")
	b.WriteString("Режим: " + s.Mode + ". Real broker orders ЗАПРЕЩЕНЫ.\n")
	b.WriteString("Инструкция оператора: " + strings.TrimSpace(s.Instruction) + "\n")
	b.WriteString(fmt.Sprintf("Инструмент: %s (%s), счёт %s\n\n", s.Ticker, s.InstrumentID, s.AccountID))

	writeAITraderContextSummary(&b, f, mctx)

	b.WriteString("\n== Подсказка rule-engine (safety, не копируй слепо) ==\n")
	b.WriteString(fmt.Sprintf("action=%s intent=%s bias=%s reason=%s\n", base.Action, base.Intent, base.MarketBias, base.Reason))

	if len(s.Events) > 0 {
		b.WriteString("\n== Хронология твоих прошлых выводов (смотри развитие ситуации) ==\n")
		max := 5
		if len(s.Events) < max {
			max = len(s.Events)
		}
		for i := 0; i < max; i++ {
			ev := s.Events[i]
			line := ev.Summary
			if line == "" {
				line = ev.Reason
			}
			b.WriteString(fmt.Sprintf("- [%s] bias=%s action=%s | %s\n", ev.Time, ev.MarketBias, ev.Action, line))
		}
	}

	b.WriteString(`
Ответь ТОЛЬКО валидным JSON без markdown:
{
  "summary": "вывод на русском: что происходит СЕЙЧАС и как изменилось за последние минуты (стакан + лента + график + уровни)",
  "market_bias": "bullish|bearish|neutral|blocked",
  "action": "hold|observe_plan|paper_plan|observe_wall|block",
  "intent": "короткий код намерения",
  "reason": "почему так — сошлись принты, стенки, уровни, свечи",
  "next_watch": "что наблюдать дальше",
  "confidence": 0.0
}

Правила:
- Опирайся на book_timeline, recent_prints, tape_stats, chart_bars, levels и scene_notes.
- Если лента агрессивно бьёт в одну сторону — отрази это; если стенку сняли — отрази.
- Сравни цену с ближайшими support/resistance из levels.
- Не предлагай market/limit/stop, buy/sell/enter/exit/flatten.
- В observe режиме paper_plan -> observe_plan.
- confidence 0..1.
`)

	sys := b.String()
	user := "Обнови вывод: как развивается ситуация по стакану, ленте и графику?"
	return []chatMessage{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	}
}

func parseAITraderLLMReply(raw string) (*aiTraderLLMOutput, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty llm reply")
	}
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	var out aiTraderLLMOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse llm json: %w", err)
	}
	if strings.TrimSpace(out.Summary) == "" {
		return nil, fmt.Errorf("llm summary is empty")
	}
	return &out, nil
}

func mergeAITraderLLMOutput(s *AITraderSession, f *AITraderFeatures, base AITraderDecisionEvent, out *aiTraderLLMOutput) AITraderDecisionEvent {
	action := sanitizeAITraderLLMAction(out.Action, s.Mode)
	bias := sanitizeAITraderLLMBias(out.MarketBias)
	conf := out.Confidence
	if conf < 0 {
		conf = 0
	}
	if conf > 1 {
		conf = 1
	}
	note := "Real broker orders disabled."
	if s.Mode == AITraderModePaper {
		note = "Paper mode: analysis only, no broker orders."
	}
	return AITraderDecisionEvent{
		Time:           time.Now().UTC().Format(time.RFC3339),
		SessionID:      s.ID,
		Mode:           s.Mode,
		Action:         action,
		Intent:         strings.TrimSpace(out.Intent),
		Reason:         strings.TrimSpace(out.Reason),
		Summary:        strings.TrimSpace(out.Summary),
		MarketBias:     bias,
		NextWatch:      strings.TrimSpace(out.NextWatch),
		OperatorNote:   note,
		Confidence:     conf,
		RiskResult:     base.RiskResult,
		AnalysisSource: "llm",
		Features:       f,
	}
}

func sanitizeAITraderLLMAction(action, mode string) string {
	action = strings.TrimSpace(strings.ToLower(action))
	switch action {
	case "hold", "observe_plan", "paper_plan", "observe_wall", "block":
	default:
		action = "hold"
	}
	if mode == AITraderModeObserve && action == "paper_plan" {
		action = "observe_plan"
	}
	return action
}

func writeAITraderContextSummary(b *strings.Builder, f *AITraderFeatures, mctx *AITraderMarketContext) {
	if f != nil {
		b.WriteString("== Стакан ==\n")
		fmt.Fprintf(b, "best bid %.4f (top vol %d) | best ask %.4f (top vol %d) | mid %.4f | spread %.2f bps | imbalance %.3f | skew %.3f\n",
			f.BestBid, f.TopBidVolume, f.BestAsk, f.TopAskVolume, f.Mid, f.SpreadBPS, f.Imbalance, f.DepthSkew)
		if f.LargestBidWall.Quantity > 0 {
			fmt.Fprintf(b, "largest bid wall: %.4f x%d (rank %d)\n", f.LargestBidWall.Price, f.LargestBidWall.Quantity, f.LargestBidWall.Rank)
		}
		if f.LargestAskWall.Quantity > 0 {
			fmt.Fprintf(b, "largest ask wall: %.4f x%d (rank %d)\n", f.LargestAskWall.Price, f.LargestAskWall.Quantity, f.LargestAskWall.Rank)
		}
		b.WriteString("top bid walls: " + formatBookLevels(aiTraderTopBidLevels(f, mctx, 5)) + "\n")
		b.WriteString("top ask walls: " + formatBookLevels(aiTraderTopAskLevels(f, mctx, 5)) + "\n")
		if f.Stale {
			b.WriteString(fmt.Sprintf("WARNING: stale feed (%d ms)\n", f.DataFreshnessMS))
		}
	}

	if mctx == nil {
		return
	}

	b.WriteString("\n== Лента")
	if mctx.TapeStats.WindowSec > 0 {
		fmt.Fprintf(b, " (%ds)", mctx.TapeStats.WindowSec)
	}
	b.WriteString(" ==\n")
	ts := mctx.TapeStats
	fmt.Fprintf(b, "trades %d | buy vol %d | sell vol %d | last %.4f | vwap %.4f | delta %.1f%%",
		ts.TradeCount, ts.BuyVolume, ts.SellVolume, ts.LastPrice, ts.VWAP, ts.DeltaPct*100)
	if ts.Aggressor != "" {
		fmt.Fprintf(b, " | aggressor %s", ts.Aggressor)
	}
	b.WriteByte('\n')
	b.WriteString("large prints: " + formatAITraderPrints(aiTraderTopPrints(mctx.RecentPrints, 5)) + "\n")

	if len(mctx.ChartBars) > 0 {
		b.WriteString("\n== График (последние 1m свечи) ==\n")
		start := len(mctx.ChartBars) - 5
		if start < 0 {
			start = 0
		}
		for _, bar := range mctx.ChartBars[start:] {
			dir := "—"
			if bar.Close > bar.Open {
				dir = "▲"
			} else if bar.Close < bar.Open {
				dir = "▼"
			}
			label := bar.Time
			if len(label) > 16 {
				label = label[len(label)-5:]
			}
			fmt.Fprintf(b, "%s O=%.4f H=%.4f L=%.4f C=%.4f V=%s %s\n",
				label, bar.Open, bar.High, bar.Low, bar.Close, formatAITraderVol(bar.Volume), dir)
		}
	}

	if len(mctx.Levels) > 0 {
		b.WriteString("\n== Уровни ==\n")
		var support, resistance []string
		for _, lv := range mctx.Levels {
			line := fmt.Sprintf("%.4f (%s/%s)", lv.Price, lv.Kind, lv.Source)
			switch strings.ToLower(lv.Kind) {
			case "support", "support_zone":
				support = append(support, line)
			case "resistance", "resistance_zone":
				resistance = append(resistance, line)
			default:
				support = append(support, line)
			}
		}
		if len(support) > 0 {
			b.WriteString("Support: " + strings.Join(support, ", ") + "\n")
		}
		if len(resistance) > 0 {
			b.WriteString("Resistance: " + strings.Join(resistance, ", ") + "\n")
		}
	}

	if len(mctx.BookTimeline) > 0 {
		b.WriteString("\n== Эволюция стакана (timeline) ==\n")
		start := len(mctx.BookTimeline) - 5
		if start < 0 {
			start = 0
		}
		for _, d := range mctx.BookTimeline[start:] {
			fmt.Fprintf(b, "[%s] mid=%.4f spread=%.1f bps imb=%.3f bid_wall=%s ask_wall=%s\n",
				d.Time, d.Mid, d.SpreadBPS, d.Imbalance, d.BidWall, d.AskWall)
		}
	}

	if len(mctx.SceneNotes) > 0 {
		b.WriteString("\n== Заметки сцены ==\n")
		max := 5
		if len(mctx.SceneNotes) < max {
			max = len(mctx.SceneNotes)
		}
		for i := 0; i < max; i++ {
			b.WriteString("- " + mctx.SceneNotes[i] + "\n")
		}
	}

	if len(mctx.Footprint) > 0 {
		b.WriteString("\n== Кластеры (последние минуты) ==\n")
		start := len(mctx.Footprint) - 3
		if start < 0 {
			start = 0
		}
		for _, col := range mctx.Footprint[start:] {
			fmt.Fprintf(b, "%s vol=%d delta=%+d\n", col.Label, col.TotalVol, col.Delta)
		}
	}
}

func aiTraderTopBidLevels(f *AITraderFeatures, mctx *AITraderMarketContext, n int) []AITraderBookLevel {
	if mctx != nil && mctx.DOMBook != nil && len(mctx.DOMBook.Bids) > 0 {
		return topNBookLevels(mctx.DOMBook.Bids, n)
	}
	if f != nil {
		return topNBookLevels(f.OrderBookSnapshot.Bids, n)
	}
	return nil
}

func aiTraderTopAskLevels(f *AITraderFeatures, mctx *AITraderMarketContext, n int) []AITraderBookLevel {
	if mctx != nil && mctx.DOMBook != nil && len(mctx.DOMBook.Asks) > 0 {
		return topNBookLevels(mctx.DOMBook.Asks, n)
	}
	if f != nil {
		return topNBookLevels(f.OrderBookSnapshot.Asks, n)
	}
	return nil
}

func topNBookLevels(levels []AITraderBookLevel, n int) []AITraderBookLevel {
	if len(levels) == 0 {
		return nil
	}
	cp := append([]AITraderBookLevel(nil), levels...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Quantity > cp[j].Quantity })
	if len(cp) > n {
		cp = cp[:n]
	}
	return cp
}

func formatBookLevels(levels []AITraderBookLevel) string {
	if len(levels) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(levels))
	for _, lv := range levels {
		parts = append(parts, fmt.Sprintf("%.4f (%d)", lv.Price, lv.Quantity))
	}
	return strings.Join(parts, ", ")
}

func aiTraderTopPrints(prints []AITraderPrint, n int) []AITraderPrint {
	if len(prints) == 0 {
		return nil
	}
	cp := append([]AITraderPrint(nil), prints...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Quantity > cp[j].Quantity })
	if len(cp) > n {
		cp = cp[:n]
	}
	return cp
}

func formatAITraderPrints(prints []AITraderPrint) string {
	if len(prints) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(prints))
	for _, p := range prints {
		side := "SELL"
		if strings.Contains(strings.ToLower(p.Direction), "buy") {
			side = "BUY"
		}
		parts = append(parts, fmt.Sprintf("%s %d@%.4f", side, p.Quantity, p.Price))
	}
	return strings.Join(parts, ", ")
}

func formatAITraderVol(v int64) string {
	if v >= 1000 {
		return fmt.Sprintf("%.1fk", float64(v)/1000)
	}
	return fmt.Sprintf("%d", v)
}

func sanitizeAITraderLLMBias(bias string) string {
	bias = strings.TrimSpace(strings.ToLower(bias))
	switch bias {
	case "bullish", "bearish", "neutral", "blocked":
		return bias
	default:
		return "neutral"
	}
}
