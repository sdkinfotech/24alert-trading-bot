package tradeanalyst

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

// JournalEvent is a subset of strategy.AITraderDecisionEvent for correlation.
type JournalEvent struct {
	Time           string  `json:"time"`
	SessionID      string  `json:"session_id"`
	Action         string  `json:"action"`
	Intent         string  `json:"intent"`
	Reason         string  `json:"reason"`
	Summary        string  `json:"summary"`
	MarketBias     string  `json:"market_bias"`
	Confidence     float64 `json:"confidence"`
	RiskResult     string  `json:"risk_result"`
	AnalysisSource string  `json:"analysis_source"`
	Mid            float64 `json:"-"`
}

type journalFeatures struct {
	Mid float64 `json:"mid"`
}

type journalEventRaw struct {
	Time           string           `json:"time"`
	SessionID      string           `json:"session_id"`
	Action         string           `json:"action"`
	Intent         string           `json:"intent"`
	Reason         string           `json:"reason"`
	Summary        string           `json:"summary"`
	MarketBias     string           `json:"market_bias"`
	Confidence     float64          `json:"confidence"`
	RiskResult     string           `json:"risk_result"`
	AnalysisSource string           `json:"analysis_source"`
	Features       *journalFeatures `json:"features"`
}

// LoadJournalEvents reads JSONL journal lines for one session.
func LoadJournalEvents(path, sessionID string) ([]JournalEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []JournalEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 256*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.Contains(line, sessionID) {
			continue
		}
		var raw journalEventRaw
		if json.Unmarshal([]byte(line), &raw) != nil || raw.SessionID != sessionID {
			continue
		}
		ev := JournalEvent{
			Time: raw.Time, SessionID: raw.SessionID, Action: raw.Action,
			Intent: raw.Intent, Reason: raw.Reason, Summary: raw.Summary,
			MarketBias: raw.MarketBias, Confidence: raw.Confidence,
			RiskResult: raw.RiskResult, AnalysisSource: raw.AnalysisSource,
		}
		if raw.Features != nil {
			ev.Mid = raw.Features.Mid
		}
		out = append(out, ev)
	}
	return out, sc.Err()
}
