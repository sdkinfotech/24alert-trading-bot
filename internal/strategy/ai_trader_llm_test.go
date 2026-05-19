package strategy

import (
	"fmt"
	"strings"
	"testing"
)

func TestFormatAITraderLLMErrorRateLimit(t *testing.T) {
	msg := formatAITraderLLMError(fmt.Errorf("openrouter 429: rate-limited"))
	if !strings.Contains(msg, "429") || !strings.Contains(msg, "OpenRouter") {
		t.Fatalf("unexpected: %q", msg)
	}
}

func TestParseAITraderLLMReply(t *testing.T) {
	raw := "```json\n{\"summary\":\"Давление продавцов\",\"market_bias\":\"bearish\",\"action\":\"observe_plan\",\"intent\":\"watch_short\",\"reason\":\"ask wall\",\"next_watch\":\"prints\",\"confidence\":0.7}\n```"
	out, err := parseAITraderLLMReply(raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.Summary == "" || out.MarketBias != "bearish" {
		t.Fatalf("unexpected parse: %+v", out)
	}
}

func TestSanitizeAITraderLLMActionObserveMode(t *testing.T) {
	collecting := &AITraderSession{Phase: AITraderPhaseAnalyzing}
	if got := sanitizeAITraderLLMAction("paper_plan", collecting); got != "observe_plan" {
		t.Fatalf("expected observe_plan, got %s", got)
	}
	trading := &AITraderSession{Phase: AITraderPhaseTrading}
	if got := sanitizeAITraderLLMAction("paper_plan", trading); got != "paper_plan" {
		t.Fatalf("expected paper_plan, got %s", got)
	}
	if got := sanitizeAITraderLLMAction("buy_now", trading); got != "hold" {
		t.Fatalf("expected hold, got %s", got)
	}
}

func TestApplyAITraderRiskGateWideSpread(t *testing.T) {
	base := AITraderDecisionEvent{
		RiskResult: "blocked_spread",
		Action:     "hold",
		Intent:     "avoid_wide_spread",
		Reason:     "spread 34.80bps exceeds limit 15.00bps",
	}
	ev := AITraderDecisionEvent{
		Action:  "observe_plan",
		Summary: "Продавцы у стены",
	}
	out := applyAITraderRiskGate(ev, base, &AITraderFeatures{SpreadBPS: 34.8})
	if out.Action != "hold" || out.RiskResult != "blocked_spread" {
		t.Fatalf("unexpected gated event: %+v", out)
	}
	if !strings.Contains(out.Summary, "spread") || !strings.Contains(out.Summary, "Продавцы") {
		t.Fatalf("expected spread prefix and llm summary, got %q", out.Summary)
	}
}

func TestMergeAITraderLLMOutput(t *testing.T) {
	s := &AITraderSession{ID: "s1", Phase: AITraderPhaseTrading, StrategyKind: AITraderStrategyLevelIntraday}
	base := decideAITraderRules(s, &AITraderFeatures{SpreadBPS: 1})
	out := &aiTraderLLMOutput{
		Summary:    "Стакан перекошен в сторону продавцов",
		MarketBias: "bearish",
		Action:     "paper_plan",
		Intent:     "watch_short_setup",
		Reason:     "ask volume dominates",
		NextWatch:  "wall at ask",
		Confidence: 0.72,
	}
	ev := mergeAITraderLLMOutput(s, &AITraderFeatures{BestBid: 1, BestAsk: 2}, base, out)
	if ev.AnalysisSource != "llm" || ev.Summary == "" || ev.Action != "paper_plan" {
		t.Fatalf("unexpected merged event: %+v", ev)
	}
}
