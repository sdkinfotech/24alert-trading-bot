package strategy

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/24alert/trading-bot/pkg/metrics"
)

func aiTraderPlaybookRefreshSec() int {
	v := strings.TrimSpace(os.Getenv("AI_TRADER_PLAYBOOK_REFRESH_SEC"))
	if v == "" {
		return 60
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 15 {
		return 60
	}
	if n > 300 {
		return 300
	}
	return n
}

// refreshPlaybookWhileTrading rebuilds level playbook from live book/tape during trading phase.
func (r *Runner) refreshPlaybookWhileTrading(s *AITraderSession, f *AITraderFeatures) {
	if s == nil || f == nil || s.Phase != AITraderPhaseTrading {
		return
	}
	interval := time.Duration(aiTraderPlaybookRefreshSec()) * time.Second
	if !s.lastPlaybookRefreshAt.IsZero() && time.Since(s.lastPlaybookRefreshAt) < interval {
		return
	}
	r.aiTrader.mu.Lock()
	cur := r.aiTrader.findLocked(s.ID)
	if cur == nil {
		r.aiTrader.mu.Unlock()
		return
	}
	oldN := 0
	if cur.LevelPlaybook != nil {
		oldN = len(cur.LevelPlaybook.Levels)
	}
	newPB := r.buildLevelPlaybook(cur, f)
	if newPB == nil {
		r.aiTrader.mu.Unlock()
		return
	}
	if cur.LevelPlaybook != nil {
		cur.LevelPlaybook = mergePlaybooks(cur.LevelPlaybook, newPB)
	} else {
		cur.LevelPlaybook = newPB
	}
	r.refreshAdvisorPlaybookLiteLocked(cur)
	if cur.LevelPlaybook != nil && f.Mid > 0 {
		cur.LevelPlaybook.Levels = selectTradeableLevels(cur.LevelPlaybook.Levels, f.Mid, cur.Ticker)
	}
	if cur.ActivePolicy != nil {
		cur.ActivePolicy.PreferredLevels = nil
		if cur.LevelPlaybook != nil {
			cur.ActivePolicy.PreferredLevels = append([]AITraderLevel(nil), cur.LevelPlaybook.Levels...)
		}
	}
	cur.lastPlaybookRefreshAt = time.Now().UTC()
	cur.LastPlaybookRefreshAt = cur.lastPlaybookRefreshAt.Format(time.RFC3339)
	newN := len(cur.LevelPlaybook.Levels)
	mid := f.Mid
	cur.appendCollectFeed("phase", "Playbook обновлён",
		fmt.Sprintf("%d→%d уровней, mid %.4f", oldN, newN, mid))
	ev := AITraderDecisionEvent{
		Time:           cur.lastPlaybookRefreshAt.Format(time.RFC3339),
		SessionID:      cur.ID,
		Mode:           cur.StrategyKind,
		Action:         "playbook_refreshed",
		Intent:         "live_levels",
		Summary:        fmt.Sprintf("Обновление уровней: %d шт., mid %.4f", newN, mid),
		AnalysisSource: "runner",
	}
	cur.Events = append([]AITraderDecisionEvent{ev}, cur.Events...)
	if len(cur.Events) > aiTraderMaxSessionEvents {
		cur.Events = cur.Events[:aiTraderMaxSessionEvents]
	}
	cur.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	r.aiTrader.mu.Unlock()
	metrics.AITraderPlaybookRefreshTotal.Inc()
	r.appendAITraderEvent(ev)
}

// refreshAdvisorPlaybookLiteLocked merges advisor 5m draft without changing phase.
func (r *Runner) refreshAdvisorPlaybookLiteLocked(s *AITraderSession) {
	if s == nil {
		return
	}
	reports := fetchAdvisorReportsReady(s.ID)
	for _, tf := range reports {
		if tf == "5m" {
			s.PhaseProgress.ReportsReady = mergeStringSets(s.PhaseProgress.ReportsReady, []string{"5m"})
			break
		}
	}
	readiness, err := fetchAdvisorReadiness(s.ID)
	if err != nil || readiness == nil || readiness.PlaybookDraft == nil {
		return
	}
	if draft := playbookFromAdvisorDTO(readiness.PlaybookDraft); draft != nil && len(draft.Levels) > 0 {
		s.LevelPlaybook = mergePlaybooks(s.LevelPlaybook, draft)
	}
}
