package tradeanalyst

// TradeRound is one entry→exit cycle extracted from live/paper fills.
type TradeRound struct {
	ID            string   `json:"id"`
	SessionID     string   `json:"session_id"`
	Ticker        string   `json:"ticker"`
	InstrumentUID string   `json:"instrument_uid"`
	AccountID     string   `json:"account_id"`
	Side          string   `json:"side"` // long | short
	EntryTime     string   `json:"entry_time"`
	EntryPrice    float64  `json:"entry_price"`
	ExitTime      string   `json:"exit_time"`
	ExitPrice     float64  `json:"exit_price"`
	Quantity      int64    `json:"quantity"`
	RealizedRUB   float64  `json:"realized_rub,omitempty"`
	HoldMinutes   float64  `json:"hold_minutes"`
	EntrySource   string   `json:"entry_source,omitempty"`
	ExitReason    string   `json:"exit_reason,omitempty"`
	PlannedSL     float64  `json:"planned_sl,omitempty"`
	PlannedTP     float64  `json:"planned_tp,omitempty"`
	TPMultATR     float64  `json:"tp_mult_atr,omitempty"`
	SLMultATR     float64  `json:"sl_mult_atr,omitempty"`
	MoveATR       float64  `json:"move_atr,omitempty"`
	EntryTiming   string   `json:"entry_timing,omitempty"` // early | ok | late | unknown
	Outcome       string   `json:"outcome,omitempty"`      // win | loss | flat
	Tags          []string `json:"tags,omitempty"`
}

// DecisionLink ties an LLM/runner journal event to a trade action.
type DecisionLink struct {
	EventTime   string `json:"event_time"`
	Action      string `json:"action"`
	Intent      string `json:"intent,omitempty"`
	MarketBias  string `json:"market_bias,omitempty"`
	Summary     string `json:"summary,omitempty"`
	OffsetSec   int    `json:"offset_sec"` // seconds from nearest fill
	LinkKind    string `json:"link_kind"`  // entry | exit | observe
}

// HourlyVolStat aggregates range/vol for one clock hour (MSK trading day).
type HourlyVolStat struct {
	HourUTC      int     `json:"hour_utc"`
	BarCount     int     `json:"bar_count"`
	RangePct     float64 `json:"range_pct"`
	ATR          float64 `json:"atr"`
	AvgSpreadBPS float64 `json:"avg_spread_bps,omitempty"`
	SampleTicks  int     `json:"sample_ticks,omitempty"`
}

// FrequencyReview scores how often the bot tried to enter.
type FrequencyReview struct {
	LimitPlacements int     `json:"limit_placements"`
	RoundsClosed    int     `json:"rounds_closed"`
	PerHour         float64 `json:"per_hour"`
	Verdict         string  `json:"verdict"` // ok | too_often | too_rare
	Note            string  `json:"note,omitempty"`
}

// TargetReview scores whether TP/SL distances matched volatility.
type TargetReview struct {
	AvgTPMultATR   float64 `json:"avg_tp_mult_atr"`
	AvgSLMultATR   float64 `json:"avg_sl_mult_atr"`
	TPHitRate      float64 `json:"tp_hit_rate"`
	SLHitRate      float64 `json:"sl_hit_rate"`
	Verdict        string  `json:"verdict"` // ok | tp_too_far | sl_too_tight | mixed
	Note           string  `json:"note,omitempty"`
}

// StrategyFitReview compares level-intraday method vs observed behavior.
type StrategyFitReview struct {
	Score       float64  `json:"score"` // 0..1
	Verdict     string   `json:"verdict"` // aligned | partial | misaligned
	Strengths   []string `json:"strengths,omitempty"`
	Weaknesses  []string `json:"weaknesses,omitempty"`
}

// SessionReport is post-market output for one AI Trader session.
type SessionReport struct {
	SessionID       string            `json:"session_id"`
	Ticker          string            `json:"ticker"`
	InstrumentUID   string            `json:"instrument_uid"`
	AccountID       string            `json:"account_id"`
	ExecutionMode   string            `json:"execution_mode"`
	StartedAt       string            `json:"started_at"`
	StoppedAt       string            `json:"stopped_at"`
	GeneratedAt     string            `json:"generated_at"`
	TradeRounds     []TradeRound      `json:"trade_rounds"`
	DecisionLinks   []DecisionLink    `json:"decision_links"`
	HourlyVol       []HourlyVolStat   `json:"hourly_volatility"`
	Frequency       FrequencyReview   `json:"frequency"`
	Targets         TargetReview      `json:"targets"`
	StrategyFit     StrategyFitReview `json:"strategy_fit"`
	LLMEvents       int               `json:"llm_events"`
	JournalEvents   int               `json:"journal_events"`
	RealizedRUB     float64           `json:"realized_rub"`
	WinRate         float64           `json:"win_rate"`
	SummaryRU       string            `json:"summary_ru"`
	Recommendations []string          `json:"recommendations"`
	RawNotes        string            `json:"raw_notes,omitempty"`
}

// InstrumentStats is rolling stats for a ticker across sessions.
type InstrumentStats struct {
	Ticker           string   `json:"ticker"`
	SessionsAnalyzed int      `json:"sessions_analyzed"`
	TotalRounds      int      `json:"total_rounds"`
	WinRate          float64  `json:"win_rate"`
	AvgHoldMinutes   float64  `json:"avg_hold_minutes"`
	AvgRealizedRUB   float64  `json:"avg_realized_rub_per_session"`
	LastReportAt     string   `json:"last_report_at"`
	Lessons          []string `json:"lessons,omitempty"`
}

// TradingHints are fed into the next AI Trader session policy (soft gates).
type TradingHints struct {
	Ticker              string   `json:"ticker"`
	UpdatedAt           string   `json:"updated_at"`
	BlockNewEntry       bool     `json:"block_new_entry,omitempty"`
	EntryMinConfidence  float64  `json:"entry_min_confidence,omitempty"` // suggested floor
	TPMultScale         float64  `json:"tp_mult_scale,omitempty"`        // multiply default TP mult
	SLMultScale         float64  `json:"sl_mult_scale,omitempty"`
	AvoidHoursUTC       []int    `json:"avoid_hours_utc,omitempty"`
	Notes               []string `json:"notes,omitempty"`
}
