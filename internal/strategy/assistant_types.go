package strategy

import "time"

// AssistantCandleBar is OHLCV for dashboard charts.
type AssistantCandleBar struct {
	Time   string  `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}

// AssistantLevel is a tradeable structure level with stats and optional LLM narrative.
type AssistantLevel struct {
	ID             string  `json:"id"`
	Price          float64 `json:"price"`
	Kind           string  `json:"kind"`   // support, resistance, mirror, poc, pivot
	Source         string  `json:"source"` // daily_high, hourly_low, mirror, volume_poc, ...
	Strength       int     `json:"strength"`
	Role           string  `json:"role,omitempty"`
	Touches        int     `json:"touches"`
	VolumeInZone   int64   `json:"volume_in_zone"`
	AvgReactionBPS float64 `json:"avg_reaction_bps"`
	VolumeNote     string  `json:"volume_note,omitempty"`
	ReportMD       string  `json:"report_md,omitempty"`
}

// AssistantScenario is a variant intraday plan (bounce / breakout / range).
type AssistantScenario struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Bias          string `json:"bias"` // bounce, breakout, range
	Probability   string `json:"probability"`
	Trigger       string `json:"trigger"`
	Invalidation  string `json:"invalidation"`
	PlaybookMD    string `json:"playbook_md"`
}

// AssistantChartPayload is candles + level lines for one timeframe tab.
type AssistantChartPayload struct {
	Timeframe string               `json:"timeframe"`
	Horizon   string               `json:"horizon,omitempty"`
	Candles   []AssistantCandleBar `json:"candles"`
	Levels    []AssistantLevel     `json:"levels"`
}

// AssistantAnalysis is the full stored analysis job.
type AssistantAnalysis struct {
	ID            string                         `json:"analysis_id"`
	Ticker        string                         `json:"ticker"`
	InstrumentUID string                         `json:"instrument_uid"`
	InstrumentName string                        `json:"instrument_name,omitempty"`
	Status        string                         `json:"status"` // running, done, error
	ProgressPct   int                            `json:"progress_pct"`
	Error         string                         `json:"error,omitempty"`
	CreatedAt     time.Time                      `json:"created_at"`
	CompletedAt   time.Time                      `json:"completed_at,omitempty"`
	SummaryMD     string                         `json:"summary_md,omitempty"`
	Levels        []AssistantLevel               `json:"levels,omitempty"`
	Scenarios     []AssistantScenario            `json:"scenarios,omitempty"`
	Charts        map[string]AssistantChartPayload `json:"charts,omitempty"`
	LLMModel      string                         `json:"llm_model,omitempty"`
	LLMFallback   bool                           `json:"llm_fallback,omitempty"`
}

// AssistantFacts is deterministic input for LLM (JSON-serialized).
type AssistantFacts struct {
	Ticker         string             `json:"ticker"`
	InstrumentUID  string             `json:"instrument_uid"`
	LastPrice      float64            `json:"last_price"`
	Horizons       map[string]string  `json:"horizons"`
	Levels         []AssistantLevel   `json:"levels"`
	RecentTrend1h  string             `json:"recent_trend_1h"`
	RecentTrend1d  string             `json:"recent_trend_1d"`
	VolumeSummary  string             `json:"volume_summary"`
}

// assistantLLMOutput is expected JSON from the model.
type assistantLLMOutput struct {
	SummaryMD string `json:"summary_md"`
	Levels    []struct {
		ID         string  `json:"id"`
		ReportMD   string  `json:"report_md"`
		VolumeNote string  `json:"volume_note"`
		Strength   int     `json:"strength"`
	} `json:"levels"`
	Scenarios []AssistantScenario `json:"scenarios"`
}
