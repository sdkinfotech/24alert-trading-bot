package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/24alert/trading-bot/pkg/metrics"
)

const (
	defaultAITraderLLMInterval = 90 * time.Second
	aiTraderLLMTimeout         = 45 * time.Second
	// Free model; override with AI_TRADER_MODEL. Do not inherit AI_CHAT_MODEL (Sonnet is too costly at high polling).
	defaultAITraderModel = "nvidia/nemotron-3-super-120b-a12b:free"
	defaultAITraderModelFallbacks = "google/gemma-4-31b-it:free"
)

type aiTraderLLMOutput struct {
	Summary       string                    `json:"summary"`
	MarketBias    string                    `json:"market_bias"`
	Action        string                    `json:"action"`
	Intent        string                    `json:"intent"`
	Reason        string                    `json:"reason"`
	NextWatch     string                    `json:"next_watch"`
	Confidence    float64                   `json:"confidence"`
	TradingPolicy *aiTraderLLMTradingPolicy `json:"trading_policy,omitempty"`
	TradeSignal   *aiTraderLLMTradeSignal   `json:"trade_signal,omitempty"`
}

type aiTraderLLMTradeSignal struct {
	Side         string  `json:"side"`
	LevelPrice   float64 `json:"level_price"`
	Confidence   float64 `json:"confidence"`
	Reason       string  `json:"reason"`
	RiskOverride string  `json:"risk_override"`
	OrderAction  string  `json:"order_action"`
	StopLoss     float64 `json:"stop_loss"`
	TakeProfit   float64 `json:"take_profit"`
	Quantity     int64   `json:"quantity"`
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
	switch s.Phase {
	case AITraderPhaseCollecting:
		return false
	case AITraderPhaseTrading:
		if s.lastLLMAt.IsZero() {
			return true
		}
		return time.Since(s.lastLLMAt) >= aiTraderLLMIntervalTrading()
	}
	// analyzing / ready
	if s.lastLLMAt.IsZero() {
		return s.PhaseProgress.CollectSeconds >= s.PhaseProgress.MinCollectSec
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
	// Stale feed: no LLM; journal at most once per LLM interval (not every 2s observe tick).
	if base.RiskResult == "blocked_stale_feed" {
		base.AnalysisSource = "rules"
		if !r.shouldRunAITraderLLM(s) {
			r.refreshAITraderFeaturesOnly(s, f)
			r.attachAITraderMarketContext(s)
			return base, false
		}
		r.markAITraderLLMTick(s)
		return base, true
	}
	if !r.shouldRunAITraderLLM(s) {
		r.refreshAITraderFeaturesOnly(s, f)
		r.attachAITraderMarketContext(s)
		return base, false
	}
	// Reserve interval before the HTTP call so parallel observe ticks do not fan out LLM requests.
	r.markAITraderLLMTick(s)

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
		if isOpenRouterRateLimit(err) {
			// Retry sooner than full interval when provider throttles free tier.
			s.lastLLMAt = time.Now().UTC().Add(-aiTraderLLMInterval(s) + 45*time.Second)
		}
		base.AnalysisSource = "rules_fallback"
		base.LLMModel = "rules_fallback"
		base.Summary = "LLM временно недоступен, используется rule-engine: " + base.Summary
		base.Reason = formatAITraderLLMError(err) + " | " + base.Reason
		base.OperatorNote = strings.TrimSpace(base.OperatorNote + " " + formatAITraderLLMError(err))
		return base, true
	}
	ev.AnalysisSource = "llm"
	ev = applyAITraderRiskGate(ev, base, f)
	r.logger.Info("ai trader llm", "session", s.ID, "action", ev.Action, "bias", ev.MarketBias, "risk", ev.RiskResult)
	return ev, true
}

func (r *Runner) markAITraderLLMTick(s *AITraderSession) {
	if s != nil {
		s.lastLLMAt = time.Now().UTC()
	}
}

// applyAITraderRiskGate keeps rule-engine safety flags (wide spread, etc.) on top of LLM narrative.
func applyAITraderRiskGate(ev, base AITraderDecisionEvent, f *AITraderFeatures) AITraderDecisionEvent {
	switch base.RiskResult {
	case "", "allowed_observe_only":
		return ev
	}
	ev.RiskResult = base.RiskResult
	switch base.RiskResult {
	case "blocked_spread":
		ev.Action = "hold"
		if strings.TrimSpace(ev.Intent) == "" {
			ev.Intent = base.Intent
		}
		prefix := strings.TrimSpace(base.Reason)
		if f != nil && f.SpreadBPS > 0 && prefix == "" {
			prefix = fmt.Sprintf("spread %.1f bps exceeds limit", f.SpreadBPS)
		}
		if prefix != "" {
			ev.Summary = prefix + ". " + ev.Summary
		}
		if base.Reason != "" {
			ev.OperatorNote = strings.TrimSpace(ev.OperatorNote + " | " + base.Reason)
		}
	case "blocked_stale_feed":
		ev.Action = "block"
		ev.MarketBias = "blocked"
	}
	return ev
}

func aiTraderModelCandidates() []string {
	seen := make(map[string]bool)
	var out []string
	add := func(m string) {
		m = strings.TrimSpace(m)
		if m == "" || seen[m] {
			return
		}
		seen[m] = true
		out = append(out, m)
	}
	add(aiTraderModel())
	if fb := strings.TrimSpace(os.Getenv("AI_TRADER_MODEL_FALLBACKS")); fb != "" {
		for _, part := range strings.Split(fb, ",") {
			add(part)
		}
	} else {
		for _, part := range strings.Split(defaultAITraderModelFallbacks, ",") {
			add(part)
		}
	}
	return out
}

func isOpenRouterRateLimit(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") || strings.Contains(msg, "rate-limit") || strings.Contains(msg, "rate limited")
}

func formatAITraderLLMError(err error) string {
	if err == nil {
		return ""
	}
	if isOpenRouterRateLimit(err) {
		return "OpenRouter: лимит free-модели (429). Подождите 1–2 мин, увеличьте AI_TRADER_LLM_INTERVAL_SEC или смените AI_TRADER_MODEL / пополните OpenRouter."
	}
	msg := strings.TrimSpace(err.Error())
	if len(msg) > 240 {
		return msg[:240] + "…"
	}
	return msg
}

func (r *Runner) callAITraderLLM(ctx context.Context, apiKey string, s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext, base AITraderDecisionEvent) (AITraderDecisionEvent, error) {
	msgs := buildAITraderLLMMessages(s, f, mctx, base)
	var lastErr error
	lastModel := "unknown"
	for _, model := range aiTraderModelCandidates() {
		lastModel = model
		attemptStart := time.Now()
		raw, err := callOpenRouter(apiKey, model, msgs)
		if err != nil {
			lastErr = err
			result := metrics.ClassifyLLMError(err)
			metrics.RecordLLMRequest(metrics.LLMServiceAITrader, model, result, 0)
			if isOpenRouterRateLimit(err) {
				r.logger.Warn("ai trader llm rate limited", "session", s.ID, "model", model, "result", result)
				continue
			}
			r.logger.Warn("ai trader llm call failed", "session", s.ID, "model", model, "error", err, "result", result)
			continue
		}
		out, err := parseAITraderLLMReply(raw)
		if err != nil {
			lastErr = err
			metrics.RecordLLMRequest(metrics.LLMServiceAITrader, model, metrics.LLMResultParseError, 0)
			r.logger.Warn("ai trader llm parse", "session", s.ID, "model", model, "error", err)
			continue
		}
		metrics.RecordLLMRequest(metrics.LLMServiceAITrader, model, metrics.LLMResultSuccess, time.Since(attemptStart))
		ev := mergeAITraderLLMOutput(s, f, base, out)
		ev.LLMModel = model
		if pol, changed := mergeLLMPolicyIntoSession(s, out.TradingPolicy, s.LevelPlaybook); out.TradingPolicy != nil {
			r.persistAITraderPolicy(s.ID, pol, changed)
		}
		if sig := llmOutputToSignal(out); sig != nil {
			r.persistAITraderSignal(s.ID, sig)
			r.recordAITraderTick(s, "trade_signal", sig)
		}
		if model != aiTraderModel() {
			ev.OperatorNote = strings.TrimSpace(ev.OperatorNote + " | model=" + model)
		}
		return ev, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no ai trader models configured")
	}
	metrics.RecordLLMRequest(metrics.LLMServiceAITrader, lastModel, metrics.ClassifyLLMError(lastErr), 0)
	return AITraderDecisionEvent{}, lastErr
}

func buildAITraderLLMMessages(s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext, base AITraderDecisionEvent) []chatMessage {
	var b strings.Builder
	b.WriteString("Ты — AI Trader 24alert, intraday level strategy (стакан + лента + дневные S/R).\n")
	b.WriteString(fmt.Sprintf("Фаза: %s. strategy_kind=%s. Брокерские заявки ЗАПРЕЩЕНЫ до фазы trading.\n", s.Phase, s.StrategyKind))
	b.WriteString("Инструкция оператора: " + strings.TrimSpace(s.Instruction) + "\n")
	b.WriteString(fmt.Sprintf("Инструмент: %s (%s), счёт %s\n", s.Ticker, s.InstrumentID, s.AccountID))
	b.WriteString(fmt.Sprintf("Прогресс: сбор %d/%d с; отчёты advisor: %v; trading_ready=%v\n\n",
		s.PhaseProgress.CollectSeconds, s.PhaseProgress.MinCollectSec, s.PhaseProgress.ReportsReady, s.PhaseProgress.TradingReady))
	if s.collectBuf != nil {
		b.WriteString("== Агрегат окна наблюдения ==\n")
		b.WriteString(s.collectBuf.digestForLLM())
		b.WriteString("\n")
	}
	if s.LevelPlaybook != nil {
		b.WriteString("== Level playbook ==\n")
		b.WriteString(s.LevelPlaybook.Summary + "\n")
		for _, lv := range s.LevelPlaybook.Levels {
			b.WriteString(fmt.Sprintf("- %s %.4f (%s)\n", lv.Kind, lv.Price, lv.Source))
		}
		b.WriteString("\n")
	}
	pol := effectivePolicy(s)
	b.WriteString("== Active trading policy (ты можешь обновить через trading_policy) ==\n")
	b.WriteString(policySummaryLine(pol) + "\n")
	if s.Phase == AITraderPhaseTrading {
		b.WriteString("Фаза TRADING: обновляй trading_policy и trade_signal (stop_loss/take_profit) по контексту.\n")
		if s.LiveState != nil && s.LiveState.PositionLots != 0 {
			b.WriteString(fmt.Sprintf("Открытая LIVE позиция: %d лот @ %.4f SL=%.4f TP=%.4f\n",
				s.LiveState.PositionLots, s.LiveState.AvgPrice, s.LiveState.StopLoss, s.LiveState.TakeProfit))
		}
		if s.PaperState != nil && s.PaperState.PositionLots != 0 {
			b.WriteString(fmt.Sprintf("Открытая PAPER позиция: %d лот @ %.4f SL=%.4f TP=%.4f\n",
				s.PaperState.PositionLots, s.PaperState.AvgPrice, s.PaperState.StopLoss, s.PaperState.TakeProfit))
		}
	}
	b.WriteString("\n")

	writeAITraderContextSummary(&b, f, mctx)

	b.WriteString("\n== Подсказка rule-engine (safety, не копируй слепо) ==\n")
	b.WriteString(fmt.Sprintf("action=%s intent=%s bias=%s risk=%s reason=%s\n",
		base.Action, base.Intent, base.MarketBias, base.RiskResult, base.Reason))
	if base.RiskResult == "blocked_spread" {
		b.WriteString("Спред шире лимита: дай аналитический вывод по стакану/ленте, но action только hold/observe, без торговых планов.\n")
	}

	if fb := paperFeedbackForLLM(s.PaperState); fb != "" {
		b.WriteString("\n== Paper execution feedback ==\n")
		b.WriteString(fb + "\n")
	}
	if len(s.MicroSignals) > 0 {
		b.WriteString("\n== Microstructure (deterministic) ==\n")
		for _, ms := range s.MicroSignals {
			b.WriteString(fmt.Sprintf("- %s %s @ %.4f: %s\n", ms.Kind, ms.Side, ms.Price, ms.Detail))
		}
	}
	if s.SessionRegime != "" {
		b.WriteString("\n== Session regime ==\n" + s.SessionRegime + "\n")
	}

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
  "confidence": 0.0,
  "trading_policy": {
    "market_bias": "bullish|bearish|neutral|blocked",
    "entry_min_confidence": 0.55,
    "confluence_min_score": 2.5,
    "sl_mult_atr": 0.5,
    "tp_mult_atr": 1.5,
    "allow_new_entry": true,
    "allow_scale_in": false,
    "tactics": ["limit_at_level"],
    "preferred_levels": [{"price": 0.0, "kind": "support", "source": "level"}],
    "summary": "краткий план сессии"
  },
  "trade_signal": {
    "side": "buy|sell|none",
    "level_price": 0.0,
    "confidence": 0.0,
    "reason": "почему этот уровень",
    "risk_override": "hold|cancel_all|flatten",
    "order_action": "place_limit|replace_limit|cancel_all|flatten|adjust_stops|hold",
    "stop_loss": 0.0,
    "take_profit": 0.0,
    "quantity": 1
  }
}

Правила:
- Опирайся на book_timeline, recent_prints, tape_stats, chart_bars, levels и scene_notes.
- trading_policy: задай правила сессии (пороги входа, SL/TP множители ATR, allow_new_entry). entry_min_confidence не выше 0.55.
- В фазе trading: trade_signal управляет лимитками и мягкими SL/TP (stop_loss/take_profit).
- При открытой позиции: order_action adjust_stops + stop_loss/take_profit; flatten — закрыть; cancel_all — снять заявки.
- Без позиции, если mid ушёл от лимитки: order_action replace_limit (side + новый level_price), не дублируй заявки.
- Вне trading: trade_signal.side должен быть none; trading_policy всё равно можно обновить.
- action остаётся hold|observe_plan|paper_plan|observe_wall|block (нарратив).
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
	action := sanitizeAITraderLLMAction(out.Action, s)
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

func llmOutputToSignal(out *aiTraderLLMOutput) *AITraderTradeSignal {
	if out == nil || out.TradeSignal == nil {
		return nil
	}
	ts := out.TradeSignal
	sig := &AITraderTradeSignal{
		Side:         strings.TrimSpace(strings.ToLower(ts.Side)),
		LevelPrice:   ts.LevelPrice,
		Confidence:   ts.Confidence,
		Reason:       strings.TrimSpace(ts.Reason),
		RiskOverride: strings.TrimSpace(strings.ToLower(ts.RiskOverride)),
		OrderAction:  strings.TrimSpace(strings.ToLower(ts.OrderAction)),
		StopLoss:     ts.StopLoss,
		TakeProfit:   ts.TakeProfit,
		Quantity:     ts.Quantity,
		ReceivedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if sig.OrderAction == "" && sig.RiskOverride != "" && sig.RiskOverride != "hold" {
		sig.OrderAction = sig.RiskOverride
	}
	if sig.Confidence < 0 {
		sig.Confidence = 0
	}
	if sig.Confidence > 1 {
		sig.Confidence = 1
	}
	return sig
}

func (r *Runner) persistAITraderSignal(sessionID string, sig *AITraderTradeSignal) {
	if sig == nil {
		return
	}
	r.aiTrader.mu.Lock()
	defer r.aiTrader.mu.Unlock()
	cur := r.aiTrader.findLocked(sessionID)
	if cur == nil {
		return
	}
	cur.LastTradeSignal = sig
}

func sanitizeAITraderLLMAction(action string, s *AITraderSession) string {
	action = strings.TrimSpace(strings.ToLower(action))
	switch action {
	case "hold", "observe_plan", "paper_plan", "observe_wall", "block":
	default:
		action = "hold"
	}
	if s != nil && s.Phase != AITraderPhaseTrading && action == "paper_plan" {
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
		b.WriteString("\n== Уровни (относительно цены) ==\n")
		ref := f.Mid
		if mctx.TapeStats.LastPrice > 0 {
			ref = mctx.TapeStats.LastPrice
		}
		if ref > 0 {
			fmt.Fprintf(b, "reference price %.4f\n", ref)
		}
		for _, line := range formatLevelsByDistance(mctx.Levels, ref) {
			b.WriteString(line + "\n")
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
