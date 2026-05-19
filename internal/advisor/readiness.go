package advisor

import (
	"context"
	"fmt"
	"strings"
)

// ReadinessResponse is returned to strategy-runner for the Start Trading gate.
type ReadinessResponse struct {
	TradingReady  bool              `json:"trading_ready"`
	ReadyReason   string            `json:"ready_reason,omitempty"`
	PlaybookDraft *PlaybookDraft    `json:"playbook_draft,omitempty"`
	ReportsReady  []string          `json:"reports_ready,omitempty"`
}

// PlaybookDraft mirrors runner LevelPlaybook for advisor → runner handoff.
type PlaybookDraft struct {
	Summary      string   `json:"summary,omitempty"`
	MarketBias   string   `json:"market_bias,omitempty"`
	Levels       []LevelDraft `json:"levels,omitempty"`
	EntryRules   []string `json:"entry_rules,omitempty"`
	ReadyToTrade bool     `json:"ready_to_trade,omitempty"`
}

type LevelDraft struct {
	Price  float64 `json:"price"`
	Kind   string  `json:"kind"`
	Source string  `json:"source"`
	Rank   int     `json:"rank"`
}

func (svc *Service) SessionReadiness(ctx context.Context, sessionID string) (ReadinessResponse, error) {
	var out ReadinessResponse
	_, _, ticker, _, err := svc.store.GetSession(ctx, sessionID)
	if err != nil {
		return out, err
	}
	for _, tf := range []Timeframe{TF5m, TF15m, TF30m, TF1h} {
		reps, err := svc.store.ListReports(ctx, sessionID, tf, 3)
		if err != nil {
			continue
		}
		for _, r := range reps {
			if r.Status == ReportStatusOK {
				out.ReportsReady = appendUniqueStr(out.ReportsReady, string(tf))
				break
			}
		}
	}
	minTF := TF15m
	has15m := false
	for _, tf := range out.ReportsReady {
		if tf == string(minTF) {
			has15m = true
			break
		}
	}
	syn, _ := svc.store.GetSynthesis(ctx, sessionID)
	draft := playbookFromReports(ctx, svc, sessionID, ticker)
	if syn != nil && len(syn.Structured.KeyLevels) > 0 {
		draft = mergeDraftFromSynthesis(draft, syn)
	}
	out.PlaybookDraft = draft

	confOK := draft != nil && draft.ReadyToTrade && len(draft.Levels) >= 2
	if has15m && confOK {
		out.TradingReady = true
		out.ReadyReason = fmt.Sprintf("%s: отчёт 15m и playbook готовы", ticker)
		if draft.Summary != "" {
			out.ReadyReason = draft.Summary
		}
	} else if !has15m {
		out.ReadyReason = "ожидаем advisor-отчёт 15m"
	} else {
		out.ReadyReason = "агент формирует уровни и сценарий"
	}
	return out, nil
}

func appendUniqueStr(slice []string, v string) []string {
	for _, s := range slice {
		if s == v {
			return slice
		}
	}
	return append(slice, v)
}

func playbookFromReports(ctx context.Context, svc *Service, sessionID, ticker string) *PlaybookDraft {
	reps, err := svc.store.ListReports(ctx, sessionID, TF15m, 5)
	if err != nil || len(reps) == 0 {
		return &PlaybookDraft{Summary: "Сбор уровней для " + ticker}
	}
	var latest *AnalysisReport
	for i := range reps {
		if reps[i].Status == ReportStatusOK {
			latest = &reps[i]
			break
		}
	}
	if latest == nil {
		return nil
	}
	d := &PlaybookDraft{
		Summary:    latest.SummaryMD,
		MarketBias: latest.Structured.MarketRegime,
		EntryRules: append([]string(nil), latest.Structured.TradingIdeas...),
	}
	for _, kl := range latest.Structured.KeyLevels {
		d.Levels = append(d.Levels, LevelDraft{Price: 0, Kind: "level", Source: kl})
	}
	for _, ln := range latest.Structured.LargeLimits {
		kind := "support"
		if strings.EqualFold(ln.Side, "ask") || strings.EqualFold(ln.Side, "sell") {
			kind = "resistance"
		}
		if ln.Price > 0 {
			d.Levels = append(d.Levels, LevelDraft{Price: ln.Price, Kind: kind, Source: "orderbook_limit"})
		}
	}
	d.ReadyToTrade = latest.Structured.Confidence >= 0.55 && len(d.Levels) >= 2
	return d
}

func mergeDraftFromSynthesis(d *PlaybookDraft, syn *StrategySynthesis) *PlaybookDraft {
	if syn == nil {
		return d
	}
	if d == nil {
		d = &PlaybookDraft{}
	}
	if syn.SummaryMD != "" {
		d.Summary = syn.SummaryMD
	}
	if syn.Structured.MarketRegime != "" {
		d.MarketBias = syn.Structured.MarketRegime
	}
	if syn.Structured.Confidence >= 0.5 {
		d.ReadyToTrade = d.ReadyToTrade || len(d.Levels) >= 2
	}
	return d
}
