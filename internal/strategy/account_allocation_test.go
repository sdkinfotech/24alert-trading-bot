package strategy

import (
	"testing"

	"github.com/24alert/trading-bot/pkg/config"
)

func TestValidateClassicInstanceAccount(t *testing.T) {
	r := &Runner{
		strategiesCfg: config.StrategiesRunnerConfig{
			AITraderAccountID: "2001673385",
			ClassicAccountID:  "2239786114",
		},
	}
	if err := r.validateClassicInstanceAccount(config.StrategyInstanceConfig{
		ID: "fut-mechel-lb", Enabled: true, AccountID: "2239786114",
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.validateClassicInstanceAccount(config.StrategyInstanceConfig{
		ID: "fut-brent-mini-lb", Enabled: true, AccountID: "2001673385",
	}); err == nil {
		t.Fatal("expected error for enabled classic on IIS while AI Trader active")
	}
	r.strategiesCfg.AITraderArchived = true
	if err := r.validateClassicInstanceAccount(config.StrategyInstanceConfig{
		ID: "fut-brent-mini-lb", Enabled: true, AccountID: "2001673385",
	}); err != nil {
		t.Fatalf("archived: classic on IIS should be allowed: %v", err)
	}
}

func TestResolveAITraderAccountID(t *testing.T) {
	r := &Runner{strategiesCfg: config.StrategiesRunnerConfig{AITraderAccountID: "2001673385"}}
	acc, err := r.resolveAITraderAccountID("")
	if err != nil || acc != "2001673385" {
		t.Fatalf("default: got %q %v", acc, err)
	}
	_, err = r.resolveAITraderAccountID("2239786114")
	if err == nil {
		t.Fatal("expected reject autosledovanie for AI")
	}
}
