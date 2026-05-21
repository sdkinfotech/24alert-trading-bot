package strategy

import (
	"fmt"
	"strings"

	"github.com/24alert/trading-bot/pkg/config"
)

// validateClassicInstanceAccount ensures enabled classic instances use allowed brokerage accounts.
func (r *Runner) validateClassicInstanceAccount(inst config.StrategyInstanceConfig) error {
	if !inst.Enabled {
		return nil
	}
	acc := strings.TrimSpace(inst.AccountID)
	aiAcc := strings.TrimSpace(r.strategiesCfg.AITraderAccountID)
	classicAcc := strings.TrimSpace(r.strategiesCfg.ClassicAccountID)
	if r.aiTraderArchived() {
		if acc == aiAcc || acc == classicAcc {
			return nil
		}
		return fmt.Errorf("instance %q: enabled classic must use ИИС (%s) or Автоследование (%s), got %s",
			inst.ID, aiAcc, classicAcc, acc)
	}
	if aiAcc != "" && acc == aiAcc {
		return fmt.Errorf("instance %q: classic strategy cannot be enabled on AI Trader account %s (ИИС); set strategies.ai_trader_archived=true to use SMA on ИИС", inst.ID, aiAcc)
	}
	if classicAcc != "" && acc != classicAcc {
		return fmt.Errorf("instance %q: enabled classic must use classic_account_id %s (Автоследование), got %s", inst.ID, classicAcc, acc)
	}
	return nil
}

func (r *Runner) resolveAITraderAccountID(requested string) (string, error) {
	acc := strings.TrimSpace(requested)
	want := strings.TrimSpace(r.strategiesCfg.AITraderAccountID)
	if acc == "" {
		if want == "" {
			return "", fmt.Errorf("account_id is required")
		}
		return want, nil
	}
	if want != "" && acc != want {
		return "", fmt.Errorf("AI Trader must use account %s (ИИС), got %s", want, acc)
	}
	return acc, nil
}
