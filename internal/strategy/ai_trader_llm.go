package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultAITraderLLMInterval = 15 * time.Second
	aiTraderLLMTimeout         = 45 * time.Second
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
	if m := strings.TrimSpace(os.Getenv("AI_CHAT_MODEL")); m != "" {
		return m
	}
	return defaultAIModel
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

	ev, err := r.callAITraderLLM(llmCtx, apiKey, s, f, base)
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

func (r *Runner) callAITraderLLM(ctx context.Context, apiKey string, s *AITraderSession, f *AITraderFeatures, base AITraderDecisionEvent) (AITraderDecisionEvent, error) {
	msgs := buildAITraderLLMMessages(s, f, base)
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

func buildAITraderLLMMessages(s *AITraderSession, f *AITraderFeatures, base AITraderDecisionEvent) []chatMessage {
	var b strings.Builder
	b.WriteString("Ты — AI Trader 24alert. Это отдельная prompt-driven сессия, НЕ стандартная SMA/Level/ORB стратегия.\n")
	b.WriteString("Режим: " + s.Mode + ". Real broker orders ЗАПРЕЩЕНЫ. Только анализ и рекомендации observe/paper.\n")
	b.WriteString("Инструкция оператора: " + strings.TrimSpace(s.Instruction) + "\n")
	b.WriteString(fmt.Sprintf("Инструмент: %s (%s), счёт %s\n\n", s.Ticker, s.InstrumentID, s.AccountID))

	b.WriteString("== Снимок стакана (JSON) ==\n")
	if raw, err := json.MarshalIndent(f, "", "  "); err == nil {
		b.Write(raw)
		b.WriteByte('\n')
	}

	b.WriteString("\n== Подсказка rule-engine (не копируй слепо) ==\n")
	b.WriteString(fmt.Sprintf("action=%s intent=%s bias=%s reason=%s\n", base.Action, base.Intent, base.MarketBias, base.Reason))

	if len(s.Events) > 0 {
		b.WriteString("\n== Предыдущие выводы ==\n")
		max := 3
		if len(s.Events) < max {
			max = len(s.Events)
		}
		for i := 0; i < max; i++ {
			ev := s.Events[i]
			line := ev.Summary
			if line == "" {
				line = ev.Reason
			}
			b.WriteString(fmt.Sprintf("- [%s] %s\n", ev.Time, line))
		}
	}

	b.WriteString(`
Ответь ТОЛЬКО валидным JSON без markdown:
{
  "summary": "краткий вывод на русском для трейдера, 2-4 предложения",
  "market_bias": "bullish|bearish|neutral|blocked",
  "action": "hold|observe_plan|paper_plan|observe_wall|block",
  "intent": "короткий код намерения",
  "reason": "почему так, со ссылкой на стакан",
  "next_watch": "что смотреть в ближайшие минуты",
  "confidence": 0.0
}

Правила:
- Не предлагай market/limit/stop ордера и не пиши buy/sell/enter/exit/flatten.
- В режиме observe action paper_plan замени на observe_plan.
- Если данных мало или риск высокий — action=block или hold.
- confidence от 0 до 1.
`)

	sys := b.String()
	user := "Сделай новый вывод по текущему стакану с учётом инструкции оператора."
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

func sanitizeAITraderLLMBias(bias string) string {
	bias = strings.TrimSpace(strings.ToLower(bias))
	switch bias {
	case "bullish", "bearish", "neutral", "blocked":
		return bias
	default:
		return "neutral"
	}
}
