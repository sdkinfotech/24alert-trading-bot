package strategy

import (
	"context"
	"strings"
	"testing"
)

func TestLabVerdictKeepProdSmallDelta(t *testing.T) {
	prod := &StrategyLabRunRow{
		Strategy: "sma_crossover", Mode: "prod",
		Params: map[string]string{"fast_period": "12", "slow_period": "16", "trailing_stop_pct": "0.003"},
		PnL: 8, MaxDrawdown: 2, Trades: 30, LiveEligible: true,
	}
	cand := &StrategyLabRunRow{
		Strategy: "sma_crossover", Mode: "sma_trailing",
		Params: map[string]string{"fast_period": "18", "slow_period": "25", "trailing_stop_pct": "0.003"},
		PnL: 9, MaxDrawdown: 2.5, Trades: 33, LiveEligible: true,
	}
	cfg := &StrategyLabConfigProd{
		InstanceID: "fut-brent-mini-lb",
		Type:       "sma_crossover",
		Params:     map[string]string{"fast_period": "12", "slow_period": "16", "trailing_stop_pct": "0.003"},
	}
	v, _ := labVerdict(cand, prod, cfg, "ru")
	if v != labVerdictKeepProd {
		t.Fatalf("expected keep_prod for delta 1pt, got %s", v)
	}
}

func TestLabVerdictDeployCandidate(t *testing.T) {
	prod := &StrategyLabRunRow{
		Strategy: "sma_crossover", Mode: "prod",
		Params: map[string]string{"fast_period": "12", "slow_period": "16", "trailing_stop_pct": "0.003"},
		PnL: 5, MaxDrawdown: 3, Trades: 25, LiveEligible: true,
	}
	cand := &StrategyLabRunRow{
		Strategy: "sma_crossover", Mode: "sma_trailing",
		Params: map[string]string{"fast_period": "10", "slow_period": "19", "trailing_stop_pct": "0.005"},
		PnL: 12, MaxDrawdown: 3.5, Trades: 28, LiveEligible: true,
	}
	cfg := &StrategyLabConfigProd{
		InstanceID: "fut-gas-mini-sma",
		Params:     map[string]string{"fast_period": "10", "slow_period": "19", "trailing_stop_pct": "0.003"},
	}
	v, _ := labVerdict(cand, prod, cfg, "ru")
	if v != labVerdictDeployCandidate {
		t.Fatalf("expected deploy_candidate, got %s", v)
	}
}

func TestBuildLabAnalysisSummary(t *testing.T) {
	inst := &labMatrixInstrument{
		Ticker: "BMM6",
		Production: &StrategyLabRunRow{
			Strategy: "sma_crossover", Mode: "prod_baseline",
			PnL: 6, Trades: 20, MaxDrawdown: 2, LiveEligible: true,
			Params: map[string]string{"fast_period": "12", "slow_period": "16", "trailing_stop_pct": "0.003"},
		},
		BestDeployable: &StrategyLabRunRow{
			Strategy: "sma_crossover", Mode: "sma_trailing",
			PnL: 10, Trades: 22, MaxDrawdown: 2.5, LiveEligible: true,
			Params: map[string]string{"fast_period": "10", "slow_period": "19", "trailing_stop_pct": "0.005"},
		},
	}
	cfg := &StrategyLabConfigProd{InstanceID: "fut-brent-mini-lb", Type: "sma_crossover",
		Params: map[string]string{"fast_period": "12", "slow_period": "16", "trailing_stop_pct": "0.003"}}
	out := buildLabAnalysis("BMM6", "uid", 90, "ru", inst, cfg)
	if out.Verdict == "" {
		t.Fatal("empty verdict")
	}
	if out.SummaryMD == "" || !strings.Contains(out.SummaryMD, "BMM6") {
		t.Fatal("expected summary")
	}
	if len(out.Rollout.Steps) < 5 {
		t.Fatalf("expected rollout steps, got %d", len(out.Rollout.Steps))
	}
	if out.Candidate == nil {
		t.Fatal("expected candidate")
	}
}

func TestLabApplyEnableLiveBlocked(t *testing.T) {
	t.Setenv("STRATEGY_LAB_ALLOW_LIVE_START", "")
	r := &Runner{}

	res, err := r.labApplyEnableLive(context.Background(), StrategyLabApplyRequest{
		ConfirmLive: false, AnalysisVerdict: labVerdictDeployCandidate,
	}, "test-id", "sma_crossover", "2239786114", "uid", map[string]string{
		"interval": "1h", "fast_period": "10", "slow_period": "19", "trailing_stop_pct": "0.003", "quantity": "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "blocked" || len(res.BlockedReasons) == 0 {
		t.Fatalf("expected blocked, got %+v", res)
	}
}
