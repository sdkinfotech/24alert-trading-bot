package strategy

import (
	"fmt"
	"os"
	"strings"
)

func (r *Runner) aiTraderArchived() bool {
	if r != nil && r.strategiesCfg.AITraderArchived {
		return true
	}
	v := strings.TrimSpace(os.Getenv("AI_TRADER_ARCHIVED"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

func (r *Runner) aiTraderArchiveError() error {
	return fmt.Errorf("AI Trader archived: use classic strategy instances (SMA) and ai-scanner cron; see docs/archive/ai-trader/")
}
