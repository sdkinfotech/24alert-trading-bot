package strategy

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/24alert/trading-bot/internal/tradeanalyst"
	"github.com/24alert/trading-bot/pkg/metrics"
)

// tradeAnalystHintsLookup is wired from runner when analyst store is ready.
var tradeAnalystHintsLookup func(ticker string) (*tradeanalyst.TradingHints, bool)

const (
	defaultPolicyEntryMinConfidence = 0.55
	defaultPolicyConfluenceMinScore = 2.5
	defaultPolicySLMultATR          = 0.5
	defaultPolicyTPMultATR          = 1.5
)

// DynamicTradingPolicy is the session-scoped plan: LLM and advisor set rules; code executes with clamps.
type DynamicTradingPolicy struct {
	UpdatedAt          string           `json:"updated_at"`
	Source             string           `json:"source,omitempty"` // advisor | llm | default
	MarketBias         string           `json:"market_bias,omitempty"`
	EntryMinConfidence float64          `json:"entry_min_confidence"`
	ConfluenceMinScore float64          `json:"confluence_min_score"`
	SLMultATR          float64          `json:"sl_mult_atr"`
	TPMultATR          float64          `json:"tp_mult_atr"`
	AllowScaleIn       bool             `json:"allow_scale_in"`
	AllowNewEntry      bool             `json:"allow_new_entry"`
	Tactics            []string         `json:"tactics,omitempty"`
	PreferredLevels    []AITraderLevel  `json:"preferred_levels,omitempty"`
	Summary            string           `json:"summary,omitempty"`
}

type aiTraderLLMTradingPolicy struct {
	MarketBias         string                `json:"market_bias,omitempty"`
	EntryMinConfidence float64               `json:"entry_min_confidence,omitempty"`
	ConfluenceMinScore float64               `json:"confluence_min_score,omitempty"`
	SLMultATR          float64               `json:"sl_mult_atr,omitempty"`
	TPMultATR          float64               `json:"tp_mult_atr,omitempty"`
	AllowScaleIn       *bool                 `json:"allow_scale_in,omitempty"`
	AllowNewEntry      *bool                 `json:"allow_new_entry,omitempty"`
	Tactics            []string              `json:"tactics,omitempty"`
	PreferredLevels    []aiTraderLLMLevelRef `json:"preferred_levels,omitempty"`
	Summary            string                `json:"summary,omitempty"`
}

type aiTraderLLMLevelRef struct {
	Price  float64 `json:"price"`
	Kind   string  `json:"kind,omitempty"`
	Source string  `json:"source,omitempty"`
}

func defaultDynamicTradingPolicy() DynamicTradingPolicy {
	return DynamicTradingPolicy{
		UpdatedAt:          time.Now().UTC().Format(time.RFC3339),
		Source:             "default",
		MarketBias:         "neutral",
		EntryMinConfidence: defaultPolicyEntryMinConfidence,
		ConfluenceMinScore: defaultPolicyConfluenceMinScore,
		SLMultATR:          defaultPolicySLMultATR,
		TPMultATR:          defaultPolicyTPMultATR,
		AllowScaleIn:       false,
		AllowNewEntry:      true,
	}
}

func policyFromPlaybook(pb *LevelPlaybook) DynamicTradingPolicy {
	p := defaultDynamicTradingPolicy()
	p.Source = "playbook"
	if pb == nil {
		return p
	}
	if pb.MarketBias != "" {
		p.MarketBias = pb.MarketBias
	}
	if pb.Summary != "" {
		p.Summary = pb.Summary
	}
	if pb.SLMultATR > 0 {
		p.SLMultATR = pb.SLMultATR
	}
	if pb.TPMultATR > 0 {
		p.TPMultATR = pb.TPMultATR
	}
	if len(pb.Levels) > 0 {
		p.PreferredLevels = append([]AITraderLevel(nil), pb.Levels...)
	}
	return clampDynamicTradingPolicy(p)
}

func policyFromAdvisorDraft(summary, bias string, slMult, tpMult float64, allowEntry *bool, levels []AITraderLevel) DynamicTradingPolicy {
	p := defaultDynamicTradingPolicy()
	p.Source = "advisor"
	p.Summary = summary
	if bias != "" {
		p.MarketBias = bias
	}
	if slMult > 0 {
		p.SLMultATR = slMult
	}
	if tpMult > 0 {
		p.TPMultATR = tpMult
	}
	if allowEntry != nil {
		p.AllowNewEntry = *allowEntry
	}
	if len(levels) > 0 {
		p.PreferredLevels = levels
	}
	return clampDynamicTradingPolicy(p)
}

func llmPolicyToDynamic(in *aiTraderLLMTradingPolicy) DynamicTradingPolicy {
	p := defaultDynamicTradingPolicy()
	p.Source = "llm"
	if in == nil {
		return p
	}
	if in.MarketBias != "" {
		p.MarketBias = strings.TrimSpace(strings.ToLower(in.MarketBias))
	}
	if in.EntryMinConfidence > 0 {
		p.EntryMinConfidence = in.EntryMinConfidence
	}
	if in.ConfluenceMinScore > 0 {
		p.ConfluenceMinScore = in.ConfluenceMinScore
	}
	if in.SLMultATR > 0 {
		p.SLMultATR = in.SLMultATR
	}
	if in.TPMultATR > 0 {
		p.TPMultATR = in.TPMultATR
	}
	if in.AllowScaleIn != nil {
		p.AllowScaleIn = *in.AllowScaleIn
	}
	if in.AllowNewEntry != nil {
		p.AllowNewEntry = *in.AllowNewEntry
	}
	p.Tactics = append([]string(nil), in.Tactics...)
	p.Summary = strings.TrimSpace(in.Summary)
	for _, lv := range in.PreferredLevels {
		if lv.Price <= 0 {
			continue
		}
		p.PreferredLevels = append(p.PreferredLevels, AITraderLevel{
			Price: lv.Price, Kind: lv.Kind, Source: lv.Source,
		})
	}
	return clampDynamicTradingPolicy(p)
}

func clampDynamicTradingPolicy(p DynamicTradingPolicy) DynamicTradingPolicy {
	// LLM may lower entry threshold (more aggressive) but not above platform default.
	if p.EntryMinConfidence <= 0 {
		p.EntryMinConfidence = defaultPolicyEntryMinConfidence
	}
	if p.EntryMinConfidence > defaultPolicyEntryMinConfidence {
		p.EntryMinConfidence = defaultPolicyEntryMinConfidence
	}
	if p.EntryMinConfidence < 0.35 {
		p.EntryMinConfidence = 0.35
	}
	p.ConfluenceMinScore = clampF(p.ConfluenceMinScore, defaultPolicyConfluenceMinScore, 1.0, 5.0)
	p.SLMultATR = clampF(p.SLMultATR, defaultPolicySLMultATR, 0.2, 2.0)
	p.TPMultATR = clampF(p.TPMultATR, defaultPolicyTPMultATR, 0.5, 3.0)
	switch p.MarketBias {
	case "bullish", "bearish", "neutral", "blocked":
	default:
		if p.MarketBias != "" {
			p.MarketBias = "neutral"
		}
	}
	return p
}

func clampF(v, def, min, max float64) float64 {
	if v <= 0 {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// effectivePolicy returns merged playbook + active policy for execution.
func effectivePolicy(s *AITraderSession) DynamicTradingPolicy {
	if s == nil {
		return defaultDynamicTradingPolicy()
	}
	base := policyFromPlaybook(s.LevelPlaybook)
	if s.ActivePolicy != nil {
		ap := *s.ActivePolicy
		if ap.MarketBias != "" {
			base.MarketBias = ap.MarketBias
		}
		if ap.EntryMinConfidence > 0 {
			base.EntryMinConfidence = ap.EntryMinConfidence
		}
		if ap.ConfluenceMinScore > 0 {
			base.ConfluenceMinScore = ap.ConfluenceMinScore
		}
		if ap.SLMultATR > 0 {
			base.SLMultATR = ap.SLMultATR
		}
		if ap.TPMultATR > 0 {
			base.TPMultATR = ap.TPMultATR
		}
		base.AllowScaleIn = ap.AllowScaleIn
		base.AllowNewEntry = ap.AllowNewEntry
		if len(ap.Tactics) > 0 {
			base.Tactics = ap.Tactics
		}
		if len(ap.PreferredLevels) > 0 {
			base.PreferredLevels = ap.PreferredLevels
		}
		if ap.Summary != "" {
			base.Summary = ap.Summary
		}
		if ap.UpdatedAt != "" {
			base.UpdatedAt = ap.UpdatedAt
		}
		if ap.Source != "" {
			base.Source = ap.Source
		}
	}
	base = applyTradeAnalystHints(s.Ticker, base)
	return clampDynamicTradingPolicy(base)
}

func applyTradeAnalystHints(ticker string, base DynamicTradingPolicy) DynamicTradingPolicy {
	if tradeAnalystHintsLookup == nil || ticker == "" {
		return base
	}
	h, ok := tradeAnalystHintsLookup(ticker)
	if !ok || h == nil {
		return base
	}
	if h.BlockNewEntry {
		base.AllowNewEntry = false
	}
	if h.EntryMinConfidence > base.EntryMinConfidence {
		base.EntryMinConfidence = h.EntryMinConfidence
	}
	if h.TPMultScale > 0 && h.TPMultScale != 1 {
		base.TPMultATR *= h.TPMultScale
	}
	if h.SLMultScale > 0 && h.SLMultScale != 1 {
		base.SLMultATR *= h.SLMultScale
	}
	if len(h.Notes) > 0 && base.Summary == "" {
		base.Summary = h.Notes[0]
	}
	base.Source = "trade_analyst_hints"
	return base
}

func tradeAnalystHourBlocked(ticker string) bool {
	if tradeAnalystHintsLookup == nil || ticker == "" {
		return false
	}
	h, ok := tradeAnalystHintsLookup(ticker)
	if !ok || h == nil || len(h.AvoidHoursUTC) == 0 {
		return false
	}
	hour := time.Now().UTC().Hour()
	for _, ah := range h.AvoidHoursUTC {
		if ah == hour {
			return true
		}
	}
	return false
}

func mergeLLMPolicyIntoSession(s *AITraderSession, llm *aiTraderLLMTradingPolicy, pb *LevelPlaybook) (DynamicTradingPolicy, bool) {
	if s == nil {
		return defaultDynamicTradingPolicy(), false
	}
	prev := effectivePolicy(s)
	next := llmPolicyToDynamic(llm)
	if pb != nil {
		seed := policyFromPlaybook(pb)
		if next.Summary == "" {
			next.Summary = seed.Summary
		}
		if len(next.PreferredLevels) == 0 && len(seed.PreferredLevels) > 0 {
			next.PreferredLevels = seed.PreferredLevels
		}
	}
	next.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	next.Source = "llm"
	changed := policyChanged(prev, next)
	if changed {
		metrics.AITraderPolicyUpdatesTotal.Inc()
	}
	return next, changed
}

func policyChanged(a, b DynamicTradingPolicy) bool {
	return a.EntryMinConfidence != b.EntryMinConfidence ||
		a.ConfluenceMinScore != b.ConfluenceMinScore ||
		a.SLMultATR != b.SLMultATR ||
		a.TPMultATR != b.TPMultATR ||
		a.AllowNewEntry != b.AllowNewEntry ||
		a.MarketBias != b.MarketBias
}

func (r *Runner) persistAITraderPolicy(sessionID string, pol DynamicTradingPolicy, changed bool) {
	r.aiTrader.mu.Lock()
	defer r.aiTrader.mu.Unlock()
	cur := r.aiTrader.findLocked(sessionID)
	if cur == nil {
		return
	}
	p := pol
	cur.ActivePolicy = &p
	if changed {
		ev := AITraderDecisionEvent{
			Time:           pol.UpdatedAt,
			SessionID:      sessionID,
			Mode:           cur.StrategyKind,
			Action:         "policy_updated",
			Intent:         "dynamic_policy",
			Summary:        policySummaryLine(pol),
			Reason:         fmt.Sprintf("entry_conf=%.2f confluence=%.1f sl_atr=%.2f tp_atr=%.2f allow_entry=%v",
				pol.EntryMinConfidence, pol.ConfluenceMinScore, pol.SLMultATR, pol.TPMultATR, pol.AllowNewEntry),
			AnalysisSource: pol.Source,
		}
		cur.Events = append([]AITraderDecisionEvent{ev}, cur.Events...)
		if len(cur.Events) > aiTraderMaxSessionEvents {
			cur.Events = cur.Events[:aiTraderMaxSessionEvents]
		}
	}
}

func policySummaryLine(p DynamicTradingPolicy) string {
	return fmt.Sprintf("План: bias=%s entry≥%.2f conf≥%.1f SL=%.2fxATR TP=%.2fxATR new_entry=%v",
		p.MarketBias, p.EntryMinConfidence, p.ConfluenceMinScore, p.SLMultATR, p.TPMultATR, p.AllowNewEntry)
}

func levelsForConfluence(s *AITraderSession, pol DynamicTradingPolicy) []AITraderLevel {
	if len(pol.PreferredLevels) > 0 {
		return pol.PreferredLevels
	}
	if s != nil && s.LevelPlaybook != nil {
		return s.LevelPlaybook.Levels
	}
	return nil
}

func aiTraderLLMIntervalTrading() time.Duration {
	v := strings.TrimSpace(os.Getenv("AI_TRADER_LLM_INTERVAL_TRADING_SEC"))
	if v == "" {
		return 45 * time.Second
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 15 {
		return 45 * time.Second
	}
	if n > 120 {
		n = 120
	}
	return time.Duration(n) * time.Second
}

// applyLLMPositionManagement updates soft SL/TP from the latest trade signal when in a position.
func applyLLMPositionManagement(s *AITraderSession, sig *AITraderTradeSignal, mid float64, isLive bool) bool {
	if s == nil || sig == nil || mid <= 0 {
		return false
	}
	action := strings.TrimSpace(strings.ToLower(sig.OrderAction))
	if action == "" {
		action = strings.TrimSpace(strings.ToLower(sig.RiskOverride))
	}
	switch action {
	case "adjust_stops", "hold":
		// apply stops below if set
	case "cancel_all", "flatten", "place_limit":
		return false
	default:
		if sig.StopLoss <= 0 && sig.TakeProfit <= 0 {
			return false
		}
	}

	var pos int64
	var sl, tp *float64
	if isLive && s.LiveState != nil {
		pos = s.LiveState.PositionLots
		sl = &s.LiveState.StopLoss
		tp = &s.LiveState.TakeProfit
	} else if !isLive && s.PaperState != nil {
		pos = s.PaperState.PositionLots
		sl = &s.PaperState.StopLoss
		tp = &s.PaperState.TakeProfit
	}
	if pos == 0 {
		return false
	}

	adjusted := false
	if sig.StopLoss > 0 {
		if v := validateSoftStop(pos, mid, sig.StopLoss); v > 0 && (action == "adjust_stops" || *sl != v) {
			*sl = v
			adjusted = true
		}
	}
	if sig.TakeProfit > 0 {
		if v := validateSoftTakeProfit(pos, mid, sig.TakeProfit); v > 0 && (action == "adjust_stops" || *tp != v) {
			*tp = v
			adjusted = true
		}
	}
	if adjusted {
		metrics.AITraderStopAdjustmentsTotal.Inc()
	}
	return adjusted
}

func validateSoftStop(pos int64, mid, stop float64) float64 {
	if pos == 0 || stop <= 0 || mid <= 0 {
		return 0
	}
	maxDist := mid * 0.03
	if pos > 0 {
		if stop >= mid {
			return 0
		}
		if mid-stop > maxDist {
			return mid - maxDist
		}
		return stop
	}
	if stop <= mid {
		return 0
	}
	if stop-mid > maxDist {
		return mid + maxDist
	}
	return stop
}

func validateSoftTakeProfit(pos int64, mid, tp float64) float64 {
	if pos == 0 || tp <= 0 || mid <= 0 {
		return 0
	}
	maxDist := mid * 0.06
	if pos > 0 {
		if tp <= mid {
			return 0
		}
		if tp-mid > maxDist {
			return mid + maxDist
		}
		return tp
	}
	if tp >= mid {
		return 0
	}
	if mid-tp > maxDist {
		return mid - maxDist
	}
	return tp
}

func setStopsFromPolicy(s *AITraderSession, side string, entry float64, isLive bool) {
	pol := effectivePolicy(s)
	atr := playbookATR(s, s.MarketContext)
	slMult, tpMult := pol.SLMultATR, pol.TPMultATR
	if slMult <= 0 {
		slMult = defaultPolicySLMultATR
	}
	if tpMult <= 0 {
		tpMult = defaultPolicyTPMultATR
	}
	var sl, tp float64
	switch side {
	case "buy":
		sl = entry - atr*slMult
		tp = entry + atr*tpMult
	case "sell":
		sl = entry + atr*slMult
		tp = entry - atr*tpMult
	default:
		return
	}
	if isLive && s.LiveState != nil {
		s.LiveState.StopLoss = sl
		s.LiveState.TakeProfit = tp
	} else if s.PaperState != nil {
		s.PaperState.StopLoss = sl
		s.PaperState.TakeProfit = tp
	}
}

func allowNewEntry(s *AITraderSession, regime string) bool {
	if s != nil && tradeAnalystHourBlocked(s.Ticker) {
		return false
	}
	pol := effectivePolicy(s)
	if !pol.AllowNewEntry {
		return false
	}
	if regime == RegimeLowVol {
		return false
	}
	if pol.MarketBias == "blocked" {
		return false
	}
	return true
}
