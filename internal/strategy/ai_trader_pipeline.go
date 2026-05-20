package strategy

import (
	"fmt"
	"strings"
	"time"
)

const aiTraderMaxSessionEvents = 200

func (r *Runner) appendAITraderPipelineEventLocked(s *AITraderSession, action, summary, reason string) {
	if s == nil || r == nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	ev := AITraderDecisionEvent{
		Time:           now,
		SessionID:      s.ID,
		Mode:           s.StrategyKind,
		Action:         action,
		Intent:         "pipeline",
		Summary:        summary,
		Reason:         reason,
		Confidence:     1,
		RiskResult:     "live_orders_disabled",
		AnalysisSource: "session",
	}
	s.Events = append([]AITraderDecisionEvent{ev}, s.Events...)
	if len(s.Events) > aiTraderMaxSessionEvents {
		s.Events = s.Events[:aiTraderMaxSessionEvents]
	}
	s.LastDecision = &s.Events[0]
	r.appendAITraderEvent(ev)
}

func (r *Runner) noteAdvisorReportsLocked(s *AITraderSession, reports []string) {
	if s == nil {
		return
	}
	prev := map[string]bool{}
	for _, tf := range s.lastReportsReady {
		prev[tf] = true
	}
	for _, tf := range reports {
		if tf == "" || prev[tf] {
			continue
		}
		s.appendCollectFeed("advisor", fmt.Sprintf("Advisor: отчёт %s готов", tf),
			"rollup для следующего таймфрейма")
		r.appendAITraderPipelineEventLocked(s, "advisor_report_ready",
			fmt.Sprintf("Отчёт advisor %s готов", tf),
			"данные переданы в иерархию 5m→15m→1h")
	}
	s.lastReportsReady = append([]string(nil), reports...)
}

func (r *Runner) onPhaseCollectingToAnalyzingLocked(s *AITraderSession, f *AITraderFeatures) {
	if s == nil {
		return
	}
	detail := fmt.Sprintf("book=%d prints=%d", len(s.collectBuf.bookSnaps), len(s.collectBuf.printSnaps))
	if s.PhaseProgress.BufferStats.ChartBars > 0 {
		detail += fmt.Sprintf("; chart_bars=%d", s.PhaseProgress.BufferStats.ChartBars)
	}
	if s.PhaseProgress.BufferStats.LevelCount > 0 {
		detail += fmt.Sprintf("; levels=%d (daily %d, hourly %d)",
			s.PhaseProgress.BufferStats.LevelCount,
			s.PhaseProgress.BufferStats.DailyLevels,
			s.PhaseProgress.BufferStats.HourlyLevels)
	}
	s.appendCollectFeed("phase", "Минутное окно завершено — запуск micro-LLM и advisor", detail)
	r.appendAITraderPipelineEventLocked(s, "analyze_start",
		"Анализ собранных данных: micro-LLM + advisor rollup",
		"буфер стакана/ленты передан в модели; ожидаем отчёт "+aiTraderTradingMinReportTF())
}

func (r *Runner) onTradingReadyLocked(s *AITraderSession) {
	if s == nil || s.LevelPlaybook == nil {
		return
	}
	pb := s.LevelPlaybook
	summary := strings.TrimSpace(pb.Summary)
	if summary == "" {
		summary = fmt.Sprintf("Level intraday: %d уровней, bias %s", len(pb.Levels), pb.MarketBias)
	}
	s.appendCollectFeed("playbook", "Playbook готов — ожидаем session_strategy", summary)
	r.appendAITraderPipelineEventLocked(s, "playbook_ready",
		"Уровни готовы; следующий шаг — стратегия сессии (LLM)",
		summary)
}
