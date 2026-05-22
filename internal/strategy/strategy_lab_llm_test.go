package strategy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLabParamMapUnmarshalNumbers(t *testing.T) {
	var row StrategyLabRunRow
	err := json.Unmarshal([]byte(`{
		"strategy":"sma_crossover",
		"params":{"fast_period":18,"slow_period":25,"trailing_stop_pct":0.003},
		"pnl":1,"trades":10,"max_drawdown":1,"sharpe":1
	}`), &row)
	if err != nil {
		t.Fatal(err)
	}
	if row.Params["fast_period"] != "18" {
		t.Fatalf("fast_period=%q want 18", row.Params["fast_period"])
	}
	if row.Params["trailing_stop_pct"] != "0.003" {
		t.Fatalf("trailing=%q", row.Params["trailing_stop_pct"])
	}
}

func TestStrategyLabFallbackTrailingSweep(t *testing.T) {
	rows := make([]StrategyLabRunRow, 0, 13)
	for i, trail := range []string{"0.003", "0.004", "0.005", "0.006", "0.007", "0.008", "0.009", "0.01", "0.011", "0.012", "0.013", "0.014", "0.015"} {
		pnl := 9.0 - float64(i)*0.3
		rows = append(rows, StrategyLabRunRow{
			Strategy: "sma_crossover", Mode: "sma_trailing",
			Params: map[string]string{"fast_period": "18", "slow_period": "25", "trailing_stop_pct": trail},
			PnL: pnl, Sharpe: 2, MaxDrawdown: 1 + float64(i)*0.2, Trades: 33, LiveEligible: true,
			RiskScore: pnl,
		})
	}
	prod := StrategyLabRunRow{
		Strategy: "sma_crossover", Mode: "prod",
		Params: map[string]string{"fast_period": "12", "slow_period": "16", "trailing_stop_pct": "0.003"},
		PnL: 8.5, Sharpe: 1, MaxDrawdown: 2, Trades: 33, LiveEligible: true,
	}
	opt := &StrategyLabOptimization{Kind: "sma_two_step", FixedFast: 18, FixedSlow: 25}
	out := strategyLabFallbackInterpret(StrategyLabInterpretRequest{
		Ticker: "BMM6", Days: 90, Rows: rows, Selected: &rows[0], Production: &prod, Optimization: opt,
	}, "ru")
	if out.Recommendation != "keep_prod" {
		t.Fatalf("expected keep_prod for new fast/slow with modest edge, got %s", out.Recommendation)
	}
	if !strings.Contains(out.SummaryMD, "Чувствительность") {
		t.Fatalf("expected trailing table in summary: %s", out.SummaryMD[:80])
	}
	if !strings.Contains(out.VsProductionMD, "fast=12") {
		t.Fatalf("expected prod comparison")
	}
}

func TestStrategyLabFallbackInterpretApply(t *testing.T) {
	rows := []StrategyLabRunRow{
		{Strategy: "sma_crossover", Mode: "prod", PnL: 5, Sharpe: 0.8, MaxDrawdown: 3, Trades: 20, LiveEligible: true},
		{Strategy: "sma_crossover", Mode: "trail", Params: map[string]string{"fast_period": "12"}, PnL: 12, Sharpe: 1.2, MaxDrawdown: 4, Trades: 18, LiveEligible: true, RiskScore: 2.5},
	}
	out := strategyLabFallbackInterpret(StrategyLabInterpretRequest{
		Ticker: "BMM6", Days: 90, Rows: rows, Selected: &rows[1],
	}, "ru")
	if out.Recommendation != "apply" {
		t.Fatalf("expected apply, got %s", out.Recommendation)
	}
	if out.SummaryMD == "" {
		t.Fatal("expected summary")
	}
}

func TestSanitizeLabRecommendation(t *testing.T) {
	if sanitizeLabRecommendation("APPLY") != "apply" {
		t.Fatal("expected apply")
	}
	if sanitizeLabRecommendation("nope") != "wait" {
		t.Fatal("expected wait")
	}
}
