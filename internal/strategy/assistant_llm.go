package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/24alert/trading-bot/pkg/llm"
	"github.com/24alert/trading-bot/pkg/metrics"
)

func assistantModel() string {
	if m := strings.TrimSpace(os.Getenv("ASSISTANT_MODEL")); m != "" {
		return m
	}
	if m := strings.TrimSpace(os.Getenv("AI_CHAT_MODEL")); m != "" {
		return m
	}
	return "anthropic/claude-sonnet-4"
}

func assistantModelFallbacks() []string {
	if v := strings.TrimSpace(os.Getenv("ASSISTANT_MODEL_FALLBACKS")); v != "" {
		return strings.Split(v, ",")
	}
	return nil
}

func assistantEnabled() bool {
	v := strings.TrimSpace(os.Getenv("ASSISTANT_ENABLED"))
	return v == "" || strings.EqualFold(v, "true") || v == "1"
}

func (r *Runner) runAssistantLLM(ctx context.Context, facts AssistantFacts) (assistantLLMOutput, string, bool, error) {
	if !assistantEnabled() {
		return assistantFallbackOutput(facts), "", true, nil
	}
	factsJSON, _ := json.MarshalIndent(facts, "", "  ")
	system := `Ты технический ассистент для внутридневной торговли на MOEX (фьючерсы/акции).
Только анализ уровней и сценариев. НЕ давай прямых команд "купи/продай", не упоминай автоторговлю.
Ответ строго JSON:
{
  "summary_md": "краткий обзор рынка",
  "levels": [{"id":"L1","report_md":"...","volume_note":"...","strength":4}],
  "scenarios": [{"id":"S1","title":"...","bias":"bounce|breakout|range","probability":"low|medium|high","trigger":"...","invalidation":"...","playbook_md":"..."}]
}
Минимум 2 сценария: один отскок, один пробой. Используй id уровней из входных данных.`
	user := fmt.Sprintf("Тикер %s. Факты:\n%s", facts.Ticker, string(factsJSON))

	res, err := llm.CompleteJSON(ctx, llm.JSONCompletionRequest{
		Service:   metrics.LLMServiceAssistant,
		Model:     assistantModel(),
		Fallbacks: assistantModelFallbacks(),
		System:    system,
		User:      user,
		MaxTokens: 4096,
	})
	if err != nil {
		r.logger.Warn("assistant llm failed, using fallback", "error", err)
		return assistantFallbackOutput(facts), "", true, nil
	}
	var out assistantLLMOutput
	content := strings.TrimSpace(res.Content)
	if i := strings.Index(content, "{"); i > 0 {
		content = content[i:]
	}
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		r.logger.Warn("assistant llm parse failed", "error", err)
		return assistantFallbackOutput(facts), res.Model, true, nil
	}
	if len(out.Scenarios) < 2 {
		fb := assistantFallbackOutput(facts)
		if len(out.Scenarios) == 0 {
			out.Scenarios = fb.Scenarios
		} else {
			out.Scenarios = append(out.Scenarios, fb.Scenarios...)
		}
	}
	return out, res.Model, false, nil
}

func assistantFallbackOutput(facts AssistantFacts) assistantLLMOutput {
	var scenarios []AssistantScenario
	for i, l := range facts.Levels {
		if i >= 3 {
			break
		}
		if strings.Contains(l.Kind, "support") || l.Kind == "poc" {
			scenarios = append(scenarios, AssistantScenario{
				ID:           fmt.Sprintf("S%d", len(scenarios)+1),
				Title:        fmt.Sprintf("Отскок от %.4f (%s)", l.Price, l.Source),
				Bias:         "bounce",
				Probability:  "medium",
				Trigger:      fmt.Sprintf("Цена подходит к %.4f, объём снижается, нет пробоя", l.Price),
				Invalidation: fmt.Sprintf("Закрытие 1h ниже %.4f", l.Price*0.998),
				PlaybookMD:   "Лимитный вход у уровня, стоп за зоной, цель — следующий уровень сопротивления.",
			})
		}
	}
	for _, l := range facts.Levels {
		if strings.Contains(l.Kind, "resistance") || l.Kind == "mirror" {
			scenarios = append(scenarios, AssistantScenario{
				ID:           fmt.Sprintf("S%d", len(scenarios)+1),
				Title:        fmt.Sprintf("Пробой %.4f (%s)", l.Price, l.Source),
				Bias:         "breakout",
				Probability:  "medium",
				Trigger:      fmt.Sprintf("Закрытие 1h выше %.4f на повышенном объёме", l.Price),
				Invalidation: fmt.Sprintf("Возврат под %.4f", l.Price),
				PlaybookMD:   "Вход на ретесте зеркала/уровня после пробоя, стоп под уровнем.",
			})
			break
		}
	}
	if len(scenarios) < 2 {
		scenarios = append(scenarios, AssistantScenario{
			ID: "S1", Title: "Боковик между уровнями", Bias: "range", Probability: "medium",
			Trigger: "Цена между двумя сильными уровнями", Invalidation: "Выход за диапазон на объёме",
			PlaybookMD: "Торговать от границ диапазона, не усреднять в середине.",
		})
	}
	summary := fmt.Sprintf("**%s** — анализ по истории (LLM недоступен). Тренд 1h: %s. %s",
		facts.Ticker, facts.RecentTrend1h, facts.VolumeSummary)
	return assistantLLMOutput{
		SummaryMD: summary,
		Scenarios: scenarios,
	}
}

func mergeLLMIntoLevels(levels []AssistantLevel, llmOut assistantLLMOutput) []AssistantLevel {
	byID := map[string]int{}
	for i := range levels {
		byID[levels[i].ID] = i
	}
	for _, row := range llmOut.Levels {
		idx, ok := byID[row.ID]
		if !ok {
			continue
		}
		if row.ReportMD != "" {
			levels[idx].ReportMD = row.ReportMD
		}
		if row.VolumeNote != "" {
			levels[idx].VolumeNote = row.VolumeNote
		}
		if row.Strength >= 1 && row.Strength <= 5 {
			levels[idx].Strength = row.Strength
		}
	}
	return levels
}
