package tradeanalyst

import (
	"testing"
)

func TestExtractRoundsLong(t *testing.T) {
	fills := []Fill{
		{Time: "2026-05-20T10:00:00Z", Side: "buy", Price: 100, Quantity: 1, Note: "broker fill support"},
		{Time: "2026-05-20T10:05:00Z", Side: "sell", Price: 101, Quantity: 1, Note: "ai_trader close: take_profit"},
	}
	rounds := ExtractRounds("sess-1", "BMM6", "uid", "acc", fills, 99, 102, 0.5, 1.5)
	if len(rounds) != 1 {
		t.Fatalf("rounds=%d", len(rounds))
	}
	if rounds[0].Side != "long" || rounds[0].Outcome != "win" {
		t.Fatalf("%+v", rounds[0])
	}
}

func TestAnalyzeSessionFrequency(t *testing.T) {
	in := SessionInput{
		SessionID: "ai-trader-bmm6-20260520-115123",
		Ticker:    "BMM6",
		StartedAt: "2026-05-20T11:00:00Z",
		StoppedAt: "2026-05-20T12:00:00Z",
		SLMultATR: 0.5,
		TPMultATR: 1.5,
		Fills: []Fill{
			{Time: "2026-05-20T11:10:00Z", Side: "buy", Price: 108, Quantity: 1},
			{Time: "2026-05-20T11:20:00Z", Side: "sell", Price: 107, Quantity: 1, Note: "stop_loss"},
		},
	}
	journal := []JournalEvent{
		{Time: "2026-05-20T11:09:50Z", Action: "paper_plan", AnalysisSource: "llm", MarketBias: "bullish", Summary: "buy support"},
	}
	rep, err := AnalyzeSession(in, journal)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.TradeRounds) != 1 {
		t.Fatalf("rounds=%d", len(rep.TradeRounds))
	}
	if len(rep.DecisionLinks) == 0 {
		t.Fatal("expected decision link")
	}
	if rep.SummaryRU == "" {
		t.Fatal("empty summary")
	}
}
