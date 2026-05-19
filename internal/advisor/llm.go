package advisor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/24alert/trading-bot/pkg/metrics"
)

const (
	openRouterURL            = "https://openrouter.ai/api/v1/chat/completions"
	defaultAdvisorModel      = "nvidia/nemotron-3-super-120b-a12b:free"
	defaultAdvisorModelFB    = "google/gemma-4-31b-it:free"
	defaultAdvisorPaidModel  = "google/gemini-2.5-flash"
	advisorPromptVer         = "advisor-v1"
	llmTimeout               = 90 * time.Second
	defaultAdvisorMaxTokens  = 4096
	defaultAdvisorLLMRetries = 2
)

// Overridable in tests.
var (
	openRouterEndpoint = openRouterURL
	openRouterDo       = defaultOpenRouterDo
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmAnalysisOutput struct {
	SummaryMD  string             `json:"summary_md"`
	Structured AnalysisStructured `json:"structured"`
}

type llmStrategyOutput struct {
	SummaryMD  string             `json:"summary_md"`
	Structured AnalysisStructured `json:"structured"`
	Drafts     []struct {
		Kind  string `json:"kind"`
		Title string `json:"title"`
		Body  string `json:"body"`
	} `json:"drafts"`
}

func advisorLLMRetries() int {
	v := strings.TrimSpace(os.Getenv("ADVISOR_LLM_RETRIES"))
	if v == "" {
		return defaultAdvisorLLMRetries
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return defaultAdvisorLLMRetries
	}
	if n > 5 {
		n = 5
	}
	return n
}

func advisorLLMMaxTokens() int {
	v := strings.TrimSpace(os.Getenv("ADVISOR_LLM_MAX_TOKENS"))
	if v == "" {
		return defaultAdvisorMaxTokens
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 512 {
		return defaultAdvisorMaxTokens
	}
	if n > 16384 {
		n = 16384
	}
	return n
}

func advisorModels() []string {
	seen := map[string]bool{}
	var out []string
	add := func(m string) {
		m = strings.TrimSpace(m)
		if m != "" && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}

	advisorPrimary := strings.TrimSpace(os.Getenv("ADVISOR_MODEL"))
	advisorFB := strings.TrimSpace(os.Getenv("ADVISOR_MODEL_FALLBACKS"))

	if advisorPrimary != "" {
		add(advisorPrimary)
	} else if advisorFB == "" {
		// No explicit advisor config: inherit AI Trader free models.
		if m := strings.TrimSpace(os.Getenv("AI_TRADER_MODEL")); m != "" {
			add(m)
		} else {
			add(defaultAdvisorModel)
		}
		traderFB := strings.TrimSpace(os.Getenv("AI_TRADER_MODEL_FALLBACKS"))
		if traderFB != "" {
			for _, part := range strings.Split(traderFB, ",") {
				add(part)
			}
		}
	} else {
		add(defaultAdvisorModel)
	}

	for _, part := range strings.Split(advisorFB, ",") {
		add(part)
	}

	if len(out) == 0 {
		add(defaultAdvisorModel)
	}
	if len(out) == 1 {
		add(defaultAdvisorModelFB)
	}

	// Paid model is always last — used only after free tier is exhausted.
	if paid := strings.TrimSpace(os.Getenv("ADVISOR_PAID_MODEL")); paid != "" {
		add(paid)
	} else {
		add(defaultAdvisorPaidModel)
	}

	return out
}

func callOpenRouter(ctx context.Context, model string, messages []chatMessage) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if apiKey == "" {
		return "", fmt.Errorf("OPENROUTER_API_KEY not set")
	}
	body := map[string]any{
		"model":    model,
		"messages": messages,
		"response_format": map[string]string{
			"type": "json_object",
		},
		"max_tokens":  advisorLLMMaxTokens(),
		"temperature": 0.2,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterEndpoint, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://24alert.ru")
	req.Header.Set("X-Title", "24alert Advisor")
	return openRouterDo(req)
}

func defaultOpenRouterDo(req *http.Request) (string, error) {
	client := &http.Client{Timeout: llmTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal openrouter response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("openrouter api: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty choices")
	}
	content := strings.TrimSpace(result.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("empty LLM response")
	}
	return content, nil
}

func isRateLimit(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") || strings.Contains(msg, "rate-limit") || strings.Contains(msg, "rate limited")
}

func isRetryableHTTP(err error) bool {
	if err == nil {
		return false
	}
	if isRateLimit(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, code := range []string{"500", "502", "503", "504", "529", "408"} {
		if strings.Contains(msg, "openrouter "+code) {
			return true
		}
	}
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "eof")
}

func isRetryableLLM(err error) bool {
	if err == nil {
		return false
	}
	if isRetryableHTTP(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "empty choices") ||
		strings.Contains(msg, "empty llm response") ||
		strings.Contains(msg, "unexpected end of json") ||
		strings.Contains(msg, "invalid character") ||
		strings.Contains(msg, "empty analysis")
}

func rateLimitBackoff(attempt int) time.Duration {
	switch attempt {
	case 0:
		return 2 * time.Second
	default:
		return 5 * time.Second
	}
}

func truncateForLog(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

type llmParseFunc[T any] func(raw string) (*T, error)

func runLLMWithFallback[T any](ctx context.Context, messages []chatMessage, parse llmParseFunc[T]) (T, string, error) {
	var zero T
	retries := advisorLLMRetries()
	models := advisorModels()
	var lastErr error
	lastModel := "unknown"
	log := slog.Default()

	for _, model := range models {
		lastModel = model
		for attempt := 0; attempt < retries; attempt++ {
			attemptStart := time.Now()
			raw, err := callOpenRouter(ctx, model, messages)
			if err != nil {
				lastErr = err
				result := metrics.ClassifyLLMError(err)
				metrics.RecordLLMRequest(metrics.LLMServiceAdvisor, model, result, 0)
				AdvisorLLMErrorsTotal.WithLabelValues(metrics.LLMModelLabel(model)).Inc()
				log.Warn("advisor llm call",
					"model", model, "attempt", attempt+1, "error", err, "result", result)
				if isRetryableHTTP(err) {
					if isRateLimit(err) {
						select {
						case <-ctx.Done():
							return zero, "", ctx.Err()
						case <-time.After(rateLimitBackoff(attempt)):
						}
					}
					continue
				}
				break // non-retryable HTTP for this model → next model
			}

			out, err := parse(raw)
			if err != nil {
				lastErr = err
				metrics.RecordLLMRequest(metrics.LLMServiceAdvisor, model, metrics.LLMResultParseError, 0)
				AdvisorLLMErrorsTotal.WithLabelValues(metrics.LLMModelLabel(model)).Inc()
				log.Warn("advisor llm parse",
					"model", model, "attempt", attempt+1, "error", err,
					"raw_preview", truncateForLog(raw, 200))
				if isRetryableLLM(err) {
					continue
				}
				break
			}
			metrics.RecordLLMRequest(metrics.LLMServiceAdvisor, model, metrics.LLMResultSuccess, time.Since(attemptStart))
			log.Info("advisor llm ok", "model", model, "attempt", attempt+1, "duration_ms", time.Since(attemptStart).Milliseconds())
			return *out, model, nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all advisor models exhausted")
	}
	metrics.RecordLLMRequest(metrics.LLMServiceAdvisor, lastModel, metrics.ClassifyLLMError(lastErr), 0)
	return zero, "", lastErr
}

func runAnalysisLLM(ctx context.Context, tf Timeframe, ticker, instruction string, facts FactsBundle, childSummaries []string) (AnalysisReport, string, error) {
	sys := buildAnalysisSystemPrompt(tf, ticker, instruction)
	user := facts.TextDigest
	if len(childSummaries) > 0 {
		user += "\n\n== Child timeframe reports ==\n"
		for _, s := range childSummaries {
			user += "---\n" + s + "\n"
		}
	}
	msgs := []chatMessage{{Role: "system", Content: sys}, {Role: "user", Content: user}}

	out, model, err := runLLMWithFallback(ctx, msgs, parseAnalysisJSON)
	if err == nil {
		out.Structured.FactsDigest = facts.TextDigest
		rep := AnalysisReport{
			Timeframe:     tf,
			Status:        ReportStatusOK,
			SummaryMD:     out.SummaryMD,
			Structured:    out.Structured,
			Model:         model,
			PromptVersion: advisorPromptVer,
			CreatedAt:     time.Now().UTC(),
		}
		return rep, model, nil
	}

	if factsFallbackEnabled() && strings.TrimSpace(facts.TextDigest) != "" {
		metrics.RecordLLMRequest(metrics.LLMServiceAdvisor, factsFallbackModel, metrics.LLMResultFallback, 0)
		rep := buildFactsFallbackReport(tf, facts, err)
		return rep, factsFallbackModel, nil
	}
	return AnalysisReport{}, "", err
}

func buildAnalysisSystemPrompt(tf Timeframe, ticker, instruction string) string {
	return fmt.Sprintf(`You are an MOEX microstructure analyst for AI Trader (timeframe %s) on %s.
Operator instruction: %s
Analyze order book, prints, walls, spoofing vs real density, iceberg hints, market maker clouds.
Output ONLY valid JSON:
{
  "summary_md": "Russian markdown summary with clear market conclusions",
  "structured": {
    "market_regime": "trend|balance|impulse|unclear",
    "key_levels": ["..."],
    "participants": [{"role":"buyers|sellers|mm|unknown","notes":"..."}],
    "volume_notes": ["..."],
    "large_limits": [{"side":"bid|ask","price":0,"quantity":0,"event":"placed|pulled|eaten|repositioned"}],
    "repositioning": ["..."],
    "mm_clouds": ["..."],
    "densities": [{"price":0,"side":"bid|ask","assessment":"real|spoof|unclear","reason":"..."}],
    "iceberg_hints": ["..."],
    "conclusions": ["3-7 bullets in Russian"],
    "next_watch": ["..."],
    "trading_ideas": ["observe-only ideas, no live orders"],
    "confidence": 0.0
  }
}
No broker orders. Be specific to the instrument.`, tf, ticker, instruction)
}

func parseAnalysisJSON(raw string) (*llmAnalysisOutput, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty LLM response")
	}
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	var out llmAnalysisOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.SummaryMD) == "" && len(out.Structured.Conclusions) == 0 {
		return nil, fmt.Errorf("empty analysis")
	}
	if out.SummaryMD == "" {
		out.SummaryMD = strings.Join(out.Structured.Conclusions, "\n")
	}
	return &out, nil
}

func runStrategyLLM(ctx context.Context, ticker, instruction string, dayReport AnalysisReport, allReports []AnalysisReport) (StrategySynthesis, error) {
	var b strings.Builder
	b.WriteString("Day report:\n" + dayReport.SummaryMD + "\n")
	for _, r := range allReports {
		if r.Timeframe == TFStrategy {
			continue
		}
		fmt.Fprintf(&b, "\n[%s %s] %s\n", r.Timeframe, r.PeriodEnd.Format(time.RFC3339), r.SummaryMD)
	}
	sys := `You are the top strategic AI Trader advisor for MOEX. Synthesize the full day bottom-up.
Output ONLY JSON:
{
  "summary_md": "Russian trading day synthesis",
  "structured": { same schema as timeframe reports },
  "drafts": [
    {"kind":"micro_prompt|rule_hint|strategy_idea","title":"...","body":"..."}
  ]
}
Drafts are for operator review only — not auto-trading.`
	user := fmt.Sprintf("Ticker %s. Instruction: %s\n\n%s", ticker, instruction, b.String())
	msgs := []chatMessage{{Role: "system", Content: sys}, {Role: "user", Content: user}}

	out, model, err := runLLMWithFallback(ctx, msgs, parseStrategyJSON)
	if err == nil {
		syn := StrategySynthesis{
			SummaryMD:  out.SummaryMD,
			Structured: out.Structured,
			Model:      model,
			CreatedAt:  time.Now().UTC(),
		}
		for _, d := range out.Drafts {
			syn.Drafts = append(syn.Drafts, StrategyDraft{
				Kind: d.Kind, Title: d.Title, Body: d.Body,
			})
		}
		return syn, nil
	}

	if factsFallbackEnabled() {
		metrics.RecordLLMRequest(metrics.LLMServiceAdvisor, factsFallbackModel, metrics.LLMResultFallback, 0)
		return buildStrategyFallbackSynthesis(ticker, dayReport, allReports, err), nil
	}
	return StrategySynthesis{}, err
}

func parseStrategyJSON(raw string) (*llmStrategyOutput, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty LLM response")
	}
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	var out llmStrategyOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.SummaryMD) == "" {
		return nil, fmt.Errorf("empty analysis")
	}
	return &out, nil
}
