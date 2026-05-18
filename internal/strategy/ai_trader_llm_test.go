package strategy

import (
	"testing"
)

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
	if got := sanitizeAITraderLLMAction("paper_plan", AITraderModeObserve); got != "observe_plan" {
		t.Fatalf("expected observe_plan, got %s", got)
	}
	if got := sanitizeAITraderLLMAction("buy_now", AITraderModePaper); got != "hold" {
		t.Fatalf("expected hold, got %s", got)
	}
}

func TestMergeAITraderLLMOutput(t *testing.T) {
	s := &AITraderSession{ID: "s1", Mode: AITraderModePaper}
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
