package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type advisorReportLite struct {
	Status string `json:"status"`
}

func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

func httpNewRequestGET(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	return req, err
}

func decodeAdvisorReports(req *http.Request) ([]advisorReportLite, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("advisor status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out []advisorReportLite
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// AdvisorReadiness is returned by advisor-svc GET .../readiness.
type AdvisorReadiness struct {
	TradingReady  bool                `json:"trading_ready"`
	ReadyReason   string              `json:"ready_reason,omitempty"`
	PlaybookDraft *advisorPlaybookDTO `json:"playbook_draft,omitempty"`
	ReportsReady  []string            `json:"reports_ready,omitempty"`
}

type advisorPlaybookDTO struct {
	Summary      string              `json:"summary,omitempty"`
	MarketBias   string              `json:"market_bias,omitempty"`
	Levels       []advisorLevelDTO   `json:"levels,omitempty"`
	EntryRules   []string            `json:"entry_rules,omitempty"`
	ReadyToTrade bool                `json:"ready_to_trade,omitempty"`
}

type advisorLevelDTO struct {
	Price  float64 `json:"price"`
	Kind   string  `json:"kind"`
	Source string  `json:"source"`
	Rank   int     `json:"rank"`
}

func playbookFromAdvisorDTO(d *advisorPlaybookDTO) *LevelPlaybook {
	if d == nil {
		return nil
	}
	pb := &LevelPlaybook{
		Summary: d.Summary, MarketBias: d.MarketBias, EntryRules: d.EntryRules, ReadyToTrade: d.ReadyToTrade,
	}
	for _, l := range d.Levels {
		if l.Price <= 0 {
			continue
		}
		pb.Levels = append(pb.Levels, AITraderLevel{Price: l.Price, Kind: l.Kind, Source: l.Source, Rank: l.Rank})
	}
	return pb
}

func fetchAdvisorReadiness(sessionID string) (*AdvisorReadiness, error) {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("ADVISOR_URL")), "/")
	if base == "" {
		base = "http://advisor-svc:9030"
	}
	url := base + "/advisor/sessions/" + sessionID + "/readiness"
	ctx, cancel := contextWithTimeout(5 * time.Second)
	defer cancel()
	req, err := httpNewRequestGET(ctx, url)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return &AdvisorReadiness{}, nil
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("advisor readiness %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out AdvisorReadiness
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *Runner) evaluateTradingReadinessLocked(s *AITraderSession, f *AITraderFeatures) {
	if s == nil || s.Phase == AITraderPhaseTrading {
		return
	}
	minTF := aiTraderTradingMinReportTF()
	hasMinReport := advisorReportGateOK(s.PhaseProgress.ReportsReady, minTF)
	collectOK := s.PhaseProgress.CollectSeconds >= s.PhaseProgress.MinCollectSec
	if f != nil && f.Stale {
		s.PhaseProgress.TradingReady = false
		s.PhaseProgress.ReadyReason = "данные устарели, ждём свежий стакан"
		if s.Phase == AITraderPhaseReady {
			s.Phase = AITraderPhaseAnalyzing
		}
		return
	}
	if f != nil && f.SpreadBPS > s.Limits.MaxSpreadBPS {
		s.PhaseProgress.TradingReady = false
		s.PhaseProgress.ReadyReason = fmt.Sprintf("спред %.1f bps выше лимита", f.SpreadBPS)
		return
	}

	playbook := r.buildLevelPlaybook(s, f)
	if playbook != nil {
		s.LevelPlaybook = playbook
	}

	readiness, _ := fetchAdvisorReadiness(s.ID)
	if readiness != nil {
		if len(readiness.ReportsReady) > 0 {
			s.PhaseProgress.ReportsReady = mergeStringSets(s.PhaseProgress.ReportsReady, readiness.ReportsReady)
		}
		if draft := playbookFromAdvisorDTO(readiness.PlaybookDraft); draft != nil && len(draft.Levels) > 0 {
			s.LevelPlaybook = mergePlaybooks(s.LevelPlaybook, draft)
		}
	}

	levelsOK := s.LevelPlaybook != nil && len(s.LevelPlaybook.Levels) >= 2
	advisorReady := readiness != nil && readiness.TradingReady

	rulesFallback := r.rulesReadyForTrading(s, f)
	if !hasMinReport && rulesFallback {
		hasMinReport = true
	}
	if collectOK && hasMinReport && levelsOK && (advisorReady || rulesFallback) {
		s.PhaseProgress.TradingReady = true
		reason := "уровни и отчёт " + minTF + " готовы"
		if readiness != nil && readiness.ReadyReason != "" {
			reason = readiness.ReadyReason
		} else if s.LevelPlaybook != nil && s.LevelPlaybook.Summary != "" {
			reason = s.LevelPlaybook.Summary
		}
		s.PhaseProgress.ReadyReason = reason
		s.Phase = AITraderPhaseReady
		return
	}

	s.PhaseProgress.TradingReady = false
	switch {
	case !collectOK:
		s.PhaseProgress.ReadyReason = fmt.Sprintf("сбор данных %d/%d с", s.PhaseProgress.CollectSeconds, s.PhaseProgress.MinCollectSec)
	case !hasMinReport:
		s.PhaseProgress.ReadyReason = "ожидаем отчёт advisor " + minTF
	case !levelsOK:
		s.PhaseProgress.ReadyReason = "строим уровни S/R"
	default:
		s.PhaseProgress.ReadyReason = "агент оценивает микроструктуру и стакан"
	}
	if s.Phase == AITraderPhaseReady {
		s.Phase = AITraderPhaseAnalyzing
	}
}

func mergeStringSets(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range append(a, b...) {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func advisorReportGateOK(reports []string, minTF string) bool {
	for _, tf := range reports {
		if tf == minTF {
			return true
		}
	}
	// Allow 5m rollup while waiting for 15m when min gate is 15m.
	if minTF == "15m" {
		for _, tf := range reports {
			if tf == "5m" {
				return true
			}
		}
	}
	return false
}

func (r *Runner) rulesReadyForTrading(s *AITraderSession, f *AITraderFeatures) bool {
	if s == nil || f == nil || s.LevelPlaybook == nil {
		return false
	}
	// Deterministic fallback when advisor LLM has not set trading_ready yet.
	if len(s.LevelPlaybook.Levels) < 4 {
		return false
	}
	if s.PhaseProgress.CollectSeconds < s.PhaseProgress.MinCollectSec {
		return false
	}
	if f.Stale || f.SpreadBPS > s.Limits.MaxSpreadBPS {
		return false
	}
	return true
}
