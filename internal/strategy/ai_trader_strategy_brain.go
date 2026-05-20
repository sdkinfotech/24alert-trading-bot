package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

func (r *Runner) maybeFormSessionStrategy(ctx context.Context, s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext) {
	if s == nil || f == nil || f.Mid <= 0 {
		return
	}
	if s.Phase != AITraderPhaseAnalyzing && s.Phase != AITraderPhaseReady && s.Phase != AITraderPhaseTrading {
		return
	}
	interval := 90 * time.Second
	if s.Phase == AITraderPhaseTrading {
		interval = 120 * time.Second
	}
	if s.lastStrategyAt != nil && time.Since(*s.lastStrategyAt) < interval {
		return
	}

	baseline := buildBaselineSessionStrategy(s, f, mctx)
	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if apiKey == "" {
		r.applySessionStrategyLocked(s, baseline)
		return
	}
	msgs := buildSessionStrategyLLMMessages(s, f, mctx, baseline)
	_ = ctx
	raw, err := callOpenRouter(apiKey, aiTraderModel(), msgs)
	if err != nil {
		r.logger.Warn("ai trader session strategy llm", "session", s.ID, "error", err)
		r.applySessionStrategyLocked(s, baseline)
		return
	}
	draft, err := parseSessionStrategyLLMReply(raw)
	if err != nil {
		r.logger.Warn("ai trader session strategy parse", "session", s.ID, "error", err)
		r.applySessionStrategyLocked(s, baseline)
		return
	}
	merged := mergeSessionStrategyFromLLM(baseline, draft)
	r.applySessionStrategyLocked(s, merged)
}

func buildSessionStrategyLLMMessages(s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext, baseline *AITraderSessionStrategy) []chatMessage {
	var b strings.Builder
	b.WriteString("Ты — аналитик микроструктуры MOEX. Задача: сформировать ТОРГОВУЮ СТРАТЕГИЮ на текущий момент, не комментарий.\n")
	b.WriteString("Изучи: DOM-события (add/pull лимиток), ленту (агрессор), iceberg/spoof/mm_flicker, график 1m+5m, коррелятор lead, уровни daily/hourly.\n")
	b.WriteString("На BMM6 lead BRM6 — график и корреляция важнее сырого bid/ask wall.\n\n")
	if len(s.MicroSignals) > 0 {
		b.WriteString("== Micro signals ==\n")
		for _, ms := range s.MicroSignals {
			b.WriteString(fmt.Sprintf("- %s %s @ %.4f: %s\n", ms.Kind, ms.Side, ms.Price, ms.Detail))
		}
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf("Инструмент %s, фаза %s\n", s.Ticker, s.Phase))
	if s.collectBuf != nil {
		b.WriteString(s.collectBuf.digestForLLM() + "\n")
	}
	writeAITraderContextSummary(&b, f, mctx)
	if baseline != nil {
		b.WriteString("\n== Черновик rules ==\n")
		b.WriteString("hypothesis: " + baseline.Hypothesis + "\n")
		b.WriteString("participants: " + baseline.Participants + "\n")
	}
	b.WriteString(`
Ответь ТОЛЬКО JSON:
{
  "session_strategy": {
    "status": "active",
    "hypothesis": "кто контролирует цену и почему",
    "participants": "MM/агрессор/айсберг/спуф — что видно",
    "regime": "trend|range|breakout|low_vol",
    "tactics": "bounce_support|fade_resistance|breakout|wait",
    "key_levels": [{"price": 0.0, "kind": "support|resistance", "source": "daily_high|hourly_low|..."}],
    "rules": ["правило 1", "правило 2"],
    "allow_long": true,
    "allow_short": false,
    "allow_new_entry": true,
    "revision_note": "если пересмотр"
  }
}
allow_new_entry=true только если есть чёткий план у конкретных уровней. Иначе wait и allow_new_entry=false.
`)
	return []chatMessage{
		{Role: "system", Content: b.String()},
		{Role: "user", Content: "Сформируй session_strategy по собранным данным."},
	}
}

func parseSessionStrategyLLMReply(raw string) (*aiTraderSessionStrategyDraft, error) {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	var wrap struct {
		SessionStrategy *aiTraderSessionStrategyDraft `json:"session_strategy"`
	}
	if err := json.Unmarshal([]byte(raw), &wrap); err != nil {
		return nil, err
	}
	if wrap.SessionStrategy == nil {
		return nil, fmt.Errorf("session_strategy missing")
	}
	return wrap.SessionStrategy, nil
}

func (r *Runner) applySessionStrategyLocked(s *AITraderSession, st *AITraderSessionStrategy) {
	if s == nil || st == nil || r == nil {
		return
	}
	now := time.Now().UTC()
	r.aiTrader.mu.Lock()
	cur := r.aiTrader.findLocked(s.ID)
	if cur == nil {
		r.aiTrader.mu.Unlock()
		return
	}
	t := now
	cur.lastStrategyAt = &t
	cur.SessionStrategy = st
	cur.PhaseProgress.StrategyReady = sessionStrategyActive(st)
	if st.AllowNewEntry {
		p := effectivePolicy(cur)
		p.AllowNewEntry = true
		p.MarketBias = regimeToBias(st.Regime, st.Tactics)
		p.Summary = "Стратегия: " + st.Tactics + " — " + st.Hypothesis
		if len(st.KeyLevels) > 0 {
			p.PreferredLevels = append([]AITraderLevel(nil), st.KeyLevels...)
		}
		cur.ActivePolicy = &p
	}
	cur.appendCollectFeed("strategy", "Стратегия сессии",
		st.Hypothesis+" | "+st.Tactics+" | entry="+fmt.Sprintf("%v", st.AllowNewEntry))
	ev := AITraderDecisionEvent{
		Time: now.Format(time.RFC3339), SessionID: cur.ID, Mode: cur.StrategyKind,
		Action: "session_strategy", Intent: st.Tactics,
		Summary: st.Hypothesis,
		Reason: strings.Join(st.Rules, "; "),
		AnalysisSource: st.Source,
	}
	cur.Events = append([]AITraderDecisionEvent{ev}, cur.Events...)
	if len(cur.Events) > aiTraderMaxSessionEvents {
		cur.Events = cur.Events[:aiTraderMaxSessionEvents]
	}
	cur.UpdatedAt = now.Format(time.RFC3339)
	r.aiTrader.mu.Unlock()
	r.appendAITraderEvent(ev)
}

func regimeToBias(regime, tactics string) string {
	if strings.Contains(tactics, "fade") || strings.Contains(tactics, "short") {
		return "bearish"
	}
	if strings.Contains(tactics, "bounce") || strings.Contains(tactics, "long") {
		return "bullish"
	}
	switch regime {
	case RegimeTrend:
		return "bullish"
	default:
		return "neutral"
	}
}
