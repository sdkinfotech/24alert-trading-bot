package strategy

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

func (r *Runner) startAssistantAnalysis(parent context.Context, ticker string) (*AssistantAnalysis, error) {
	ticker = strings.TrimSpace(strings.ToUpper(ticker))
	if ticker == "" {
		return nil, fmt.Errorf("ticker is required")
	}
	if !assistantEnabled() {
		return nil, fmt.Errorf("assistant is disabled (ASSISTANT_ENABLED=false)")
	}
	rows, err := SearchInstruments(parent, ticker, "all", 10)
	if err != nil {
		return nil, err
	}
	var pick *CatalogInstrument
	for i := range rows {
		if strings.EqualFold(rows[i].Ticker, ticker) {
			pick = &rows[i]
			break
		}
	}
	if pick == nil && len(rows) > 0 {
		pick = &rows[0]
	}
	if pick == nil {
		return nil, fmt.Errorf("instrument not found: %s", ticker)
	}

	id := fmt.Sprintf("asst-%s-%d", strings.ToLower(pick.Ticker), time.Now().Unix())
	a := &AssistantAnalysis{
		ID:             id,
		Ticker:         pick.Ticker,
		InstrumentUID:  pick.UID,
		InstrumentName: pick.Name,
		Status:         "running",
		ProgressPct:    5,
		CreatedAt:      time.Now().UTC(),
		Charts:         map[string]AssistantChartPayload{},
	}
	r.assistant.put(a)

	go r.runAssistantAnalysisJob(parent, id, pick)
	return a, nil
}

func (r *Runner) runAssistantAnalysisJob(parent context.Context, id string, pick *CatalogInstrument) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Minute)
	defer cancel()

	update := func(pct int, status, errMsg string) {
		r.assistant.update(id, func(a *AssistantAnalysis) {
			a.ProgressPct = pct
			if status != "" {
				a.Status = status
			}
			if errMsg != "" {
				a.Error = errMsg
			}
		})
	}

	update(10, "running", "")
	cs, err := r.fetchAssistantCandles(ctx, pick.UID)
	if err != nil {
		update(100, "error", err.Error())
		return
	}
	update(35, "running", "")

	levels := buildAssistantLevels(cs)
	update(55, "running", "")

	lastPx := refPrice(cs)
	facts := AssistantFacts{
		Ticker:        pick.Ticker,
		InstrumentUID: pick.UID,
		LastPrice:     lastPx,
		Horizons: map[string]string{
			"year":    fmt.Sprintf("1d candles: %d bars", len(cs.Daily1y)),
			"quarter": fmt.Sprintf("1d candles: %d bars", len(cs.Daily90d)),
			"month":   fmt.Sprintf("1h candles: %d bars", len(cs.Hourly1m)),
			"week":    fmt.Sprintf("1h candles: %d bars", len(cs.Hourly1w)),
			"hour":    fmt.Sprintf("5m candles: %d bars", len(cs.FiveMin7d)),
		},
		Levels:        levels,
		RecentTrend1h: recentTrendLabel(cs.Hourly1w, 40),
		RecentTrend1d: recentTrendLabel(cs.Daily90d, 20),
		VolumeSummary: fmt.Sprintf("Опорная цена %.4f (последний 5m/1h); касания на TF источника уровня; зоны авто, не сигнал к сделке.", lastPx),
	}
	update(70, "running", "")

	llmOut, model, fallback, _ := r.runAssistantLLM(ctx, facts)
	levels = mergeLLMIntoLevels(levels, llmOut)
	charts := buildAssistantCharts(cs, levels)

	now := time.Now().UTC()
	r.assistant.update(id, func(a *AssistantAnalysis) {
		a.Status = "done"
		a.ProgressPct = 100
		a.CompletedAt = now
		a.SummaryMD = llmOut.SummaryMD
		a.Levels = levels
		a.Scenarios = llmOut.Scenarios
		a.Charts = charts
		a.LLMModel = model
		a.LLMFallback = fallback
		if a.LLMModel == "" && os.Getenv("OPENROUTER_API_KEY") != "" {
			a.LLMModel = assistantModel()
		}
	})
}

func (r *Runner) getAssistantAnalysis(id string) (*AssistantAnalysis, bool) {
	if r.assistant == nil {
		return nil, false
	}
	return r.assistant.get(id)
}

func (r *Runner) deleteAssistantAnalysis(id string) {
	if r.assistant == nil {
		return
	}
	r.assistant.delete(id)
}

func (r *Runner) assistantChartPayload(id, tf string) (AssistantChartPayload, bool) {
	a, ok := r.getAssistantAnalysis(id)
	if !ok || a.Charts == nil {
		return AssistantChartPayload{}, false
	}
	p, ok := a.Charts[tf]
	return p, ok
}
