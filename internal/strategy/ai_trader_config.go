package strategy

// AITraderPublicConfig is exposed to the dashboard.
type AITraderPublicConfig struct {
	ArmedLiveEnabled bool `json:"armed_live_enabled"`
	StreamBook       bool `json:"stream_book"`
	KillSwitch       bool `json:"kill_switch"`
	MinReportTF      string `json:"min_report_tf"`
}

func (r *Runner) AITraderPublicConfig() AITraderPublicConfig {
	return AITraderPublicConfig{
		ArmedLiveEnabled: aiTraderArmedLiveEnabled(),
		StreamBook:       aiTraderStreamBookEnabled(),
		KillSwitch:       r.AITraderKillSwitch(),
		MinReportTF:      aiTraderTradingMinReportTF(),
	}
}

func aiTraderStartRiskResult(execMode string) string {
	if execMode == ExecutionModeArmedLive {
		return "live_armed_collecting"
	}
	return "live_orders_disabled"
}
