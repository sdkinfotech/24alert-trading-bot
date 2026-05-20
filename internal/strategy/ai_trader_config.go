package strategy

import "strings"

// AITraderPublicConfig is exposed to the dashboard.
type AITraderPublicConfig struct {
	ArmedLiveEnabled bool   `json:"armed_live_enabled"`
	StreamBook       bool   `json:"stream_book"`
	KillSwitch       bool   `json:"kill_switch"`
	MinReportTF      string `json:"min_report_tf"`
	DefaultAccountID string `json:"default_account_id,omitempty"`
	ClassicAccountID string `json:"classic_account_id,omitempty"`
}

func (r *Runner) AITraderPublicConfig() AITraderPublicConfig {
	return AITraderPublicConfig{
		ArmedLiveEnabled: aiTraderArmedLiveEnabled(),
		StreamBook:       aiTraderStreamBookEnabled(),
		KillSwitch:       r.AITraderKillSwitch(),
		MinReportTF:      aiTraderTradingMinReportTF(),
		DefaultAccountID: strings.TrimSpace(r.strategiesCfg.AITraderAccountID),
		ClassicAccountID: strings.TrimSpace(r.strategiesCfg.ClassicAccountID),
	}
}

func aiTraderStartRiskResult(execMode string) string {
	if execMode == ExecutionModeArmedLive {
		return "live_armed_collecting"
	}
	return "live_orders_disabled"
}
