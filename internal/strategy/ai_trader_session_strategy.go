package strategy

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// AITraderSessionStrategy is the explicit plan for this session (not UI commentary).
type AITraderSessionStrategy struct {
	Status       string          `json:"status"` // forming | active | revising
	FormedAt     string          `json:"formed_at,omitempty"`
	UpdatedAt    string          `json:"updated_at,omitempty"`
	Source       string          `json:"source,omitempty"` // llm | rules | merge
	Hypothesis   string          `json:"hypothesis,omitempty"`   // who controls tape / MM behavior
	Participants string          `json:"participants,omitempty"` // aggressor, walls, absorption summary
	Regime       string          `json:"regime,omitempty"`
	Tactics      string          `json:"tactics,omitempty"` // bounce_support | fade_resistance | breakout | wait
	KeyLevels    []AITraderLevel `json:"key_levels,omitempty"`
	Rules        []string        `json:"rules,omitempty"`
	AllowLong    bool            `json:"allow_long"`
	AllowShort   bool            `json:"allow_short"`
	AllowNewEntry bool           `json:"allow_new_entry"`
	RevisionNote string          `json:"revision_note,omitempty"`
}

func sessionStrategyActive(st *AITraderSessionStrategy) bool {
	return st != nil && strings.EqualFold(st.Status, "active") && st.AllowNewEntry
}

func aiTraderAutoPlaybookEntry() bool {
	v := strings.TrimSpace(os.Getenv("AI_TRADER_AUTO_PLAYBOOK_ENTRY"))
	return v == "1" || strings.EqualFold(v, "true")
}

func (s *AITraderSession) sessionStrategyAllowsSide(side string) bool {
	st := s.SessionStrategy
	if !sessionStrategyActive(st) {
		return false
	}
	switch strings.ToLower(side) {
	case "buy":
		return st.AllowLong
	case "sell":
		return st.AllowShort
	default:
		return false
	}
}

func (s *AITraderSession) sessionStrategyAllowsEntry() bool {
	return sessionStrategyActive(s.SessionStrategy)
}

func playbookEntryEnabled(s *AITraderSession) bool {
	if s == nil || !sessionStrategyActive(s.SessionStrategy) {
		return false
	}
	if aiTraderAutoPlaybookEntry() {
		return true
	}
	t := strings.TrimSpace(s.SessionStrategy.Tactics)
	return t != "" && t != "wait"
}

func microstructureBlocksEntry(s *AITraderSession, side string) string {
	if s == nil || len(s.MicroSignals) == 0 {
		return ""
	}
	for i := len(s.MicroSignals) - 1; i >= 0 && i >= len(s.MicroSignals)-5; i-- {
		ms := s.MicroSignals[i]
		if ms.Kind == "iceberg" {
			if side == "sell" && ms.Side == "buy" {
				return "iceberg buy absorption — не шортить"
			}
			if side == "buy" && ms.Side == "sell" {
				return "iceberg sell — не лонгить"
			}
		}
		if ms.Kind != "spoof" && ms.Kind != "mm_flicker" {
			continue
		}
		if ms.Kind == "mm_flicker" {
			return "MM flicker — подождать стабилизации стакана"
		}
		if side == "buy" && ms.Side == "ask" {
			return "spoof on ask — не лонгить в стену"
		}
		if side == "sell" && ms.Side == "bid" {
			return "spoof on bid — не шортить в стену"
		}
	}
	return ""
}

func buildBaselineSessionStrategy(s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext) *AITraderSessionStrategy {
	now := time.Now().UTC().Format(time.RFC3339)
	st := &AITraderSessionStrategy{
		Status:        "forming",
		FormedAt:      now,
		UpdatedAt:     now,
		Source:        "rules",
		AllowLong:     true,
		AllowShort:    false,
		AllowNewEntry: false,
		Tactics:       "wait",
	}
	if s != nil {
		st.Regime = s.SessionRegime
		if st.Regime == "" {
			st.Regime = detectSessionRegime(mctx)
		}
	}
	if f != nil && f.Mid > 0 && s != nil {
		st.KeyLevels = tradeableLevelsForSession(s, effectivePolicy(s), f.Mid)
	}
	var parts []string
	if mctx != nil {
		ts := mctx.TapeStats
		parts = append(parts, formatTapeHypothesis(ts))
	}
	if len(s.MicroSignals) > 0 {
		ms := s.MicroSignals[len(s.MicroSignals)-1]
		parts = append(parts, ms.Kind+" "+ms.Side+" @ "+formatPriceKey(ms.Price)+": "+ms.Detail)
	}
	st.Participants = strings.Join(parts, "; ")
	st.Hypothesis = "Сбор завершён; требуется LLM-стратегия по графику и уровням."
	if len(st.KeyLevels) >= 2 {
		st.Hypothesis = "Торговать только у ключевых уровней из playbook; стакан — подтверждение."
	}
	st.Rules = []string{
		"не входить у bid/ask wall без привязки к daily/hourly",
		"вход только после session_strategy.active",
		"при spoof на стороне входа — ждать",
	}
	switch st.Regime {
	case RegimeTrend:
		td := trendDirection(mctx)
		if td == "up" {
			st.Tactics = "bounce_support"
			st.AllowShort = false
		} else if td == "down" {
			st.Tactics = "fade_resistance"
			st.AllowShort = true
			st.AllowLong = false
		}
	case RegimeLowVol:
		st.Tactics = "wait"
		st.AllowNewEntry = false
	}
	return st
}

func formatTapeHypothesis(ts AITraderTapeStats) string {
	if ts.TradeCount == 0 {
		return "лента: мало данных"
	}
	ag := ts.Aggressor
	if ag == "" {
		if ts.DeltaPct > 0.1 {
			ag = "buyers"
		} else if ts.DeltaPct < -0.1 {
			ag = "sellers"
		} else {
			ag = "balanced"
		}
	}
	return fmt.Sprintf("лента %ds: delta %.0f%%, aggressor %s", ts.WindowSec, ts.DeltaPct*100, ag)
}

// mergeSessionStrategyFromLLM applies LLM draft when present.
func mergeSessionStrategyFromLLM(base *AITraderSessionStrategy, draft *aiTraderSessionStrategyDraft) *AITraderSessionStrategy {
	if base == nil {
		base = &AITraderSessionStrategy{Status: "forming"}
	}
	if draft == nil {
		return base
	}
	now := time.Now().UTC().Format(time.RFC3339)
	out := *base
	out.UpdatedAt = now
	out.Source = "llm"
	if strings.TrimSpace(draft.Hypothesis) != "" {
		out.Hypothesis = strings.TrimSpace(draft.Hypothesis)
	}
	if strings.TrimSpace(draft.Participants) != "" {
		out.Participants = strings.TrimSpace(draft.Participants)
	}
	if strings.TrimSpace(draft.Regime) != "" {
		out.Regime = strings.TrimSpace(draft.Regime)
	}
	if strings.TrimSpace(draft.Tactics) != "" {
		out.Tactics = strings.TrimSpace(draft.Tactics)
	}
	if len(draft.Rules) > 0 {
		out.Rules = draft.Rules
	}
	if len(draft.KeyLevels) > 0 {
		out.KeyLevels = draft.KeyLevels
	}
	if draft.AllowLong != nil {
		out.AllowLong = *draft.AllowLong
	}
	if draft.AllowShort != nil {
		out.AllowShort = *draft.AllowShort
	}
	if draft.AllowNewEntry != nil {
		out.AllowNewEntry = *draft.AllowNewEntry
	}
	if strings.TrimSpace(draft.Status) != "" {
		out.Status = strings.TrimSpace(draft.Status)
	}
	if draft.RevisionNote != "" {
		out.RevisionNote = draft.RevisionNote
	}
	if strings.EqualFold(out.Status, "active") || (out.AllowNewEntry && out.Hypothesis != "") {
		out.Status = "active"
		out.AllowNewEntry = true
	}
	return &out
}

type aiTraderSessionStrategyDraft struct {
	Status        string          `json:"status,omitempty"`
	Hypothesis    string          `json:"hypothesis,omitempty"`
	Participants  string          `json:"participants,omitempty"`
	Regime        string          `json:"regime,omitempty"`
	Tactics       string          `json:"tactics,omitempty"`
	KeyLevels     []AITraderLevel `json:"key_levels,omitempty"`
	Rules         []string        `json:"rules,omitempty"`
	AllowLong     *bool           `json:"allow_long,omitempty"`
	AllowShort    *bool           `json:"allow_short,omitempty"`
	AllowNewEntry *bool           `json:"allow_new_entry,omitempty"`
	RevisionNote  string          `json:"revision_note,omitempty"`
}
