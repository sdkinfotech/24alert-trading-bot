package advisor

import "time"

// Timeframe identifies rollup analysis level.
type Timeframe string

const (
	TF5m       Timeframe = "5m"
	TF15m      Timeframe = "15m"
	TF30m      Timeframe = "30m"
	TF1h       Timeframe = "1h"
	TF4h       Timeframe = "4h"
	TF1d       Timeframe = "1d"
	TFStrategy Timeframe = "strategy"
)

func (tf Timeframe) Duration() time.Duration {
	switch tf {
	case TF5m:
		return 5 * time.Minute
	case TF15m:
		return 15 * time.Minute
	case TF30m:
		return 30 * time.Minute
	case TF1h:
		return time.Hour
	case TF4h:
		return 4 * time.Hour
	case TF1d, TFStrategy:
		return 24 * time.Hour
	default:
		return 5 * time.Minute
	}
}

// ChildTimeframes returns report TFs included when building this rollup.
func (tf Timeframe) ChildTimeframes() []Timeframe {
	switch tf {
	case TF15m:
		return []Timeframe{TF5m}
	case TF30m:
		return []Timeframe{TF5m, TF15m}
	case TF1h:
		return []Timeframe{TF5m, TF15m, TF30m}
	case TF4h:
		return []Timeframe{TF5m, TF15m, TF30m, TF1h}
	case TF1d:
		return []Timeframe{TF5m, TF15m, TF30m, TF1h, TF4h}
	case TFStrategy:
		return []Timeframe{TF1d}
	default:
		return nil
	}
}

const ReportStatusOK = "ok"
const ReportStatusFailed = "failed"
const ReportStatusPending = "pending"

// Session mirrors strategy-runner AI trader session (JSON API).
type Session struct {
	ID            string                 `json:"id"`
	AccountID     string                 `json:"account_id"`
	InstrumentUID string                 `json:"instrument_uid"`
	Ticker        string                 `json:"ticker,omitempty"`
	Mode          string                 `json:"mode"`
	Instruction   string                 `json:"instruction"`
	Status        string                 `json:"status"`
	StartedAt     string                 `json:"started_at"`
	UpdatedAt     string                 `json:"updated_at"`
	StoppedAt     string                 `json:"stopped_at,omitempty"`
	Features      map[string]any         `json:"features,omitempty"`
	MarketContext map[string]any         `json:"market_context,omitempty"`
	Events        []DecisionEvent        `json:"events,omitempty"`
	LastDecision  *DecisionEvent         `json:"last_decision,omitempty"`
}

type DecisionEvent struct {
	Time           string  `json:"time"`
	Action         string  `json:"action"`
	Intent         string  `json:"intent"`
	Reason         string  `json:"reason"`
	Summary        string  `json:"summary"`
	MarketBias     string  `json:"market_bias"`
	NextWatch      string  `json:"next_watch"`
	AnalysisSource string  `json:"analysis_source,omitempty"`
	Confidence     float64 `json:"confidence"`
}

// MicroSnapshot is persisted ingest payload.
type MicroSnapshot struct {
	ID        int64
	SessionID string
	CapturedAt time.Time
	PayloadJSON string
}

// AnalysisStructured is LLM output schema for UI sections.
type AnalysisStructured struct {
	MarketRegime   string              `json:"market_regime"`
	KeyLevels      []string            `json:"key_levels,omitempty"`
	Participants   []ParticipantNote   `json:"participants,omitempty"`
	VolumeNotes    []string            `json:"volume_notes,omitempty"`
	LargeLimits    []LimitNote         `json:"large_limits,omitempty"`
	Repositioning  []string            `json:"repositioning,omitempty"`
	MMClouds       []string            `json:"mm_clouds,omitempty"`
	Densities      []DensityNote       `json:"densities,omitempty"`
	IcebergHints   []string            `json:"iceberg_hints,omitempty"`
	Conclusions    []string            `json:"conclusions,omitempty"`
	NextWatch      []string            `json:"next_watch,omitempty"`
	TradingIdeas   []string            `json:"trading_ideas,omitempty"`
	Confidence     float64             `json:"confidence"`
	FactsDigest    string              `json:"facts_digest,omitempty"`
}

type ParticipantNote struct {
	Role    string `json:"role"`
	Notes   string `json:"notes"`
}

type LimitNote struct {
	Side     string `json:"side"`
	Price    float64 `json:"price"`
	Quantity int64  `json:"quantity"`
	Event    string `json:"event"`
}

type DensityNote struct {
	Price    float64 `json:"price"`
	Side     string  `json:"side"`
	Assessment string `json:"assessment"`
	Reason   string  `json:"reason"`
}

// AnalysisReport is stored analysis row.
type AnalysisReport struct {
	ID              string
	SessionID       string
	Timeframe       Timeframe
	PeriodStart     time.Time
	PeriodEnd       time.Time
	Status          string
	SummaryMD       string
	Structured      AnalysisStructured
	SourceReportIDs []string
	Model           string
	PromptVersion   string
	ErrorMessage    string
	CreatedAt       time.Time
}

// StrategyDraft is a prompt/rule draft from the top agent.
type StrategyDraft struct {
	ID          string
	SessionID   string
	Kind        string
	Title       string
	Body        string
	Ticker      string
	InstrumentUID string
	CreatedAt   time.Time
}

// StrategySynthesis is day-level top report.
type StrategySynthesis struct {
	SessionID   string
	SummaryMD   string
	Structured  AnalysisStructured
	Drafts      []StrategyDraft
	Model       string
	CreatedAt   time.Time
}

// RegisterRequest from runner or UI.
type RegisterRequest struct {
	SessionID     string `json:"session_id"`
	AccountID     string `json:"account_id"`
	InstrumentUID string `json:"instrument_uid"`
	Ticker        string `json:"ticker,omitempty"`
	Mode          string `json:"mode,omitempty"`
	Instruction   string `json:"instruction,omitempty"`
	StartedAt     string `json:"started_at,omitempty"`
}
