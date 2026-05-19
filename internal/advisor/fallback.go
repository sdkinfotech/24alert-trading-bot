package advisor

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const factsFallbackModel = "facts-fallback"

func factsFallbackEnabled() bool {
	v := strings.TrimSpace(os.Getenv("ADVISOR_FACTS_FALLBACK"))
	if v == "" {
		return true
	}
	switch strings.ToLower(v) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func buildFactsFallbackReport(tf Timeframe, facts FactsBundle, lastErr error) AnalysisReport {
	summary := fmt.Sprintf(
		"**Автосводка по фактам** (%s, %s)\n\nLLM недоступен (%s). Ниже — детерминированный дайджест периода.\n\n%s",
		facts.Ticker, facts.PeriodLabel, FormatError(lastErr), facts.TextDigest,
	)

	conclusions := extractDigestBullets(facts.TextDigest, 7)
	if len(conclusions) == 0 {
		conclusions = []string{"Недостаточно данных для выводов — дождитесь следующего окна или проверьте ingest."}
	}

	nextWatch := []string{"Повторный LLM-отчёт при восстановлении OpenRouter"}
	if len(facts.ThoughtLines) > 0 {
		nextWatch = append(nextWatch, "Последний вывод runner: "+facts.ThoughtLines[0])
	}

	return AnalysisReport{
		Timeframe:     tf,
		Status:        ReportStatusOK,
		SummaryMD:     summary,
		Model:         factsFallbackModel,
		PromptVersion: advisorPromptVer,
		CreatedAt:     time.Now().UTC(),
		Structured: AnalysisStructured{
			MarketRegime:  "unclear",
			Conclusions:   conclusions,
			NextWatch:     nextWatch,
			FactsDigest:   facts.TextDigest,
			VolumeNotes:   nonEmptySlice(facts.TapeSummary),
			IcebergHints:  facts.IcebergHints,
			Repositioning: facts.WallChanges,
			Confidence:    0.15,
		},
	}
}

func buildStrategyFallbackSynthesis(ticker string, dayReport AnalysisReport, allReports []AnalysisReport, lastErr error) StrategySynthesis {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("**Сводка дня (без LLM)** — %s\n\n", ticker))
	b.WriteString("LLM недоступен: " + FormatError(lastErr) + "\n\n")
	b.WriteString(dayReport.SummaryMD)

	conclusions := []string{"Стратегический LLM-синтез недоступен — используйте отчёты по таймфреймам ниже."}
	for _, r := range allReports {
		if r.Status != ReportStatusOK || r.Timeframe == TFStrategy {
			continue
		}
		line := fmt.Sprintf("[%s] %s", r.Timeframe, truncateForLog(r.SummaryMD, 120))
		conclusions = append(conclusions, line)
	}

	return StrategySynthesis{
		SummaryMD:  b.String(),
		Model:      factsFallbackModel,
		CreatedAt:  time.Now().UTC(),
		Structured: AnalysisStructured{MarketRegime: "unclear", Conclusions: conclusions, Confidence: 0.15},
		Drafts: []StrategyDraft{{
			Kind:  "rule_hint",
			Title: "LLM fallback",
			Body:  "Перезапустите advisor-svc или проверьте OPENROUTER_API_KEY / лимиты free-моделей.",
		}},
	}
}

func extractDigestBullets(digest string, max int) []string {
	var out []string
	for _, line := range strings.Split(digest, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "==") {
			continue
		}
		out = append(out, line)
		if len(out) >= max {
			break
		}
	}
	return out
}

func nonEmptySlice(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return []string{s}
}
