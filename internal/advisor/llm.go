package advisor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	openRouterURL       = "https://openrouter.ai/api/v1/chat/completions"
	defaultAdvisorModel = "nvidia/nemotron-3-super-120b-a12b:free"
	advisorPromptVer    = "advisor-v1"
	llmTimeout          = 90 * time.Second
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmAnalysisOutput struct {
	SummaryMD    string             `json:"summary_md"`
	Structured   AnalysisStructured `json:"structured"`
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
	if m := strings.TrimSpace(os.Getenv("ADVISOR_MODEL")); m != "" {
		add(m)
	} else {
		add(defaultAdvisorModel)
	}
	for _, part := range strings.Split(os.Getenv("ADVISOR_MODEL_FALLBACKS"), ",") {
		add(part)
	}
	if len(out) == 1 {
		add("google/gemma-4-31b-it:free")
	}
	return out
}

func callOpenRouter(ctx context.Context, model string, messages []chatMessage) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if apiKey == "" {
		return "", fmt.Errorf("OPENROUTER_API_KEY not set")
	}
	body := map[string]any{"model": model, "messages": messages}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterURL, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://24alert.ru")
	req.Header.Set("X-Title", "24alert Advisor")

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
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty choices")
	}
	return result.Choices[0].Message.Content, nil
}

func isRateLimit(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") || strings.Contains(msg, "rate-limit") || strings.Contains(msg, "rate limited")
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

	var lastErr error
	for _, model := range advisorModels() {
		raw, err := callOpenRouter(ctx, model, msgs)
		if err != nil {
			lastErr = err
			if isRateLimit(err) {
				continue
			}
			return AnalysisReport{}, "", err
		}
		out, err := parseAnalysisJSON(raw)
		if err != nil {
			lastErr = err
			continue
		}
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
	if lastErr == nil {
		lastErr = fmt.Errorf("no models")
	}
	return AnalysisReport{}, "", lastErr
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

	var lastErr error
	for _, model := range advisorModels() {
		raw, err := callOpenRouter(ctx, model, []chatMessage{{Role: "system", Content: sys}, {Role: "user", Content: user}})
		if err != nil {
			lastErr = err
			if isRateLimit(err) {
				continue
			}
			return StrategySynthesis{}, err
		}
		out, err := parseStrategyJSON(raw)
		if err != nil {
			lastErr = err
			continue
		}
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
	return StrategySynthesis{}, lastErr
}

func parseStrategyJSON(raw string) (*llmStrategyOutput, error) {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	var out llmStrategyOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
