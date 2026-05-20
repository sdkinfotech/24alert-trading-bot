package strategy

import (
	"fmt"
	"strings"
	"time"
)

// AITraderExecutionLogEntry records an entry or exit with linked LLM / policy context.
type AITraderExecutionLogEntry struct {
	Time              string  `json:"time"`
	Kind              string  `json:"kind"` // entry | exit
	Side              string  `json:"side"`
	Price             float64 `json:"price"`
	Quantity          int64   `json:"quantity"`
	PositionAfter     int64   `json:"position_after"`
	Trigger           string  `json:"trigger"`
	FillNote          string  `json:"fill_note,omitempty"`
	LLMSummary        string  `json:"llm_summary,omitempty"`
	LLMReason         string  `json:"llm_reason,omitempty"`
	LLMIntent         string  `json:"llm_intent,omitempty"`
	LLMBias           string  `json:"llm_bias,omitempty"`
	LLMConfidence     float64 `json:"llm_confidence,omitempty"`
	LLMSource         string  `json:"llm_source,omitempty"`
	TradeSignalReason string  `json:"trade_signal_reason,omitempty"`
	PolicySummary     string  `json:"policy_summary,omitempty"`
	StopLoss          float64 `json:"stop_loss,omitempty"`
	TakeProfit        float64 `json:"take_profit,omitempty"`
	RealizedRUB       float64 `json:"realized_rub,omitempty"`
}

const maxExecutionLogEntries = 100

func (r *Runner) appendAITraderExecutionLog(s *AITraderSession, entry AITraderExecutionLogEntry) {
	if s == nil || entry.Kind == "" {
		return
	}
	if entry.Time == "" {
		entry.Time = time.Now().UTC().Format(time.RFC3339)
	}
	if r.aiTrader != nil {
		r.aiTrader.mu.Lock()
		cur := r.aiTrader.findLocked(s.ID)
		target := s
		if cur != nil {
			target = cur
		}
		target.ExecutionLog = append([]AITraderExecutionLogEntry{entry}, target.ExecutionLog...)
		if len(target.ExecutionLog) > maxExecutionLogEntries {
			target.ExecutionLog = target.ExecutionLog[:maxExecutionLogEntries]
		}
		r.aiTrader.mu.Unlock()
	} else {
		s.ExecutionLog = append([]AITraderExecutionLogEntry{entry}, s.ExecutionLog...)
		if len(s.ExecutionLog) > maxExecutionLogEntries {
			s.ExecutionLog = s.ExecutionLog[:maxExecutionLogEntries]
		}
	}

	ev := AITraderDecisionEvent{
		Time:           entry.Time,
		SessionID:      s.ID,
		Mode:           s.StrategyKind,
		Action:         "trade_" + entry.Kind,
		Intent:         entry.Trigger,
		Reason:         firstNonEmpty(entry.LLMReason, entry.TradeSignalReason, entry.FillNote),
		Summary:        executionLogSummary(entry),
		MarketBias:     entry.LLMBias,
		Confidence:     entry.LLMConfidence,
		RiskResult:     entry.Kind,
		AnalysisSource: "execution",
	}
	r.appendAITraderEvent(ev)
}

func (r *Runner) logLiveFillExecution(s *AITraderSession, fill LiveFill, prevPos, newPos int64) {
	kind, trigger := classifyFillExecution(prevPos, newPos, fill.Note)
	if kind == "" {
		return
	}
	entry := r.buildExecutionLogEntry(s, kind, trigger, fill.Side, fill.Price, fill.Quantity, newPos, fill.Note, true)
	if kind == "exit" && s.LiveState != nil {
		entry.RealizedRUB = s.LiveState.RealizedRUB
	}
	r.appendAITraderExecutionLog(s, entry)
}

func (r *Runner) logPaperFillExecution(s *AITraderSession, fill PaperFill, prevPos, newPos int64) {
	kind, trigger := classifyFillExecution(prevPos, newPos, fill.Note)
	if kind == "" {
		return
	}
	entry := r.buildExecutionLogEntry(s, kind, trigger, fill.Side, fill.Price, fill.Quantity, newPos, fill.Note, false)
	if kind == "exit" && s.PaperState != nil {
		entry.RealizedRUB = s.PaperState.RealizedRUB
	}
	r.appendAITraderExecutionLog(s, entry)
}

func (r *Runner) logLiveExitExecution(s *AITraderSession, side string, price float64, qty int64, trigger, note string) {
	entry := r.buildExecutionLogEntry(s, "exit", trigger, side, price, qty, 0, note, true)
	if s.LiveState != nil {
		entry.RealizedRUB = s.LiveState.RealizedRUB
	}
	r.appendAITraderExecutionLog(s, entry)
}

func classifyFillExecution(prevPos, newPos int64, note string) (kind, trigger string) {
	n := strings.ToLower(note)
	switch {
	case strings.Contains(n, "stop_loss"), strings.Contains(n, "stop"):
		trigger = "stop_loss"
	case strings.Contains(n, "take_profit"), strings.Contains(n, "profit"):
		trigger = "take_profit"
	case strings.Contains(n, "flatten"):
		trigger = "flatten"
	case strings.Contains(n, "llm"):
		trigger = "llm_signal"
	case strings.Contains(n, "confluence"), strings.Contains(n, "support"), strings.Contains(n, "resistance"):
		trigger = "confluence"
	case strings.Contains(n, "broker fill"):
		trigger = "playbook_level"
	default:
		trigger = "fill"
	}
	if prevPos == 0 && newPos != 0 {
		return "entry", trigger
	}
	if prevPos != 0 && newPos == 0 {
		return "exit", trigger
	}
	return "", trigger
}

func (r *Runner) buildExecutionLogEntry(s *AITraderSession, kind, trigger, side string, price float64, qty, posAfter int64, note string, live bool) AITraderExecutionLogEntry {
	entry := AITraderExecutionLogEntry{
		Time: time.Now().UTC().Format(time.RFC3339), Kind: kind, Side: side,
		Price: price, Quantity: qty, PositionAfter: posAfter, Trigger: trigger, FillNote: note,
	}
	if live && s.LiveState != nil {
		entry.StopLoss = s.LiveState.StopLoss
		entry.TakeProfit = s.LiveState.TakeProfit
	} else if s.PaperState != nil {
		entry.StopLoss = s.PaperState.StopLoss
		entry.TakeProfit = s.PaperState.TakeProfit
	}
	llm := nearestLLMContext(s)
	entry.LLMSummary = llm.Summary
	entry.LLMReason = llm.Reason
	entry.LLMIntent = llm.Intent
	entry.LLMBias = llm.MarketBias
	entry.LLMConfidence = llm.Confidence
	entry.LLMSource = llm.AnalysisSource
	if s.LastTradeSignal != nil {
		entry.TradeSignalReason = s.LastTradeSignal.Reason
	}
	if s.ActivePolicy != nil && s.ActivePolicy.Summary != "" {
		entry.PolicySummary = s.ActivePolicy.Summary
	} else if s.LevelPlaybook != nil {
		entry.PolicySummary = s.LevelPlaybook.Summary
	}
	return entry
}

func nearestLLMContext(s *AITraderSession) AITraderDecisionEvent {
	if s == nil {
		return AITraderDecisionEvent{}
	}
	for _, ev := range s.Events {
		if ev.AnalysisSource == "llm" {
			return ev
		}
	}
	if s.LastDecision != nil && s.LastDecision.AnalysisSource == "llm" {
		return *s.LastDecision
	}
	if s.LastDecision != nil {
		return *s.LastDecision
	}
	return AITraderDecisionEvent{}
}

func executionLogSummary(e AITraderExecutionLogEntry) string {
	dir := "LONG"
	if e.PositionAfter < 0 || (e.Kind == "entry" && e.Side == "sell") {
		dir = "SHORT"
	}
	if e.Kind == "exit" {
		dir = "закрытие"
	}
	var b strings.Builder
	b.WriteString(strings.ToUpper(e.Kind))
	b.WriteString(" ")
	b.WriteString(dir)
	b.WriteString(" @ ")
	b.WriteString(formatPrice4(e.Price))
	b.WriteString(" — ")
	b.WriteString(e.Trigger)
	if e.LLMSummary != "" {
		b.WriteString(". ")
		s := e.LLMSummary
		if len(s) > 280 {
			s = s[:280] + "…"
		}
		b.WriteString(s)
	}
	return b.String()
}

func formatPrice4(p float64) string {
	return fmt.Sprintf("%.4f", p)
}

func firstNonEmpty(parts ...string) string {
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			return strings.TrimSpace(p)
		}
	}
	return ""
}
