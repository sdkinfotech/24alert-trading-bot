package llm

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

	"github.com/24alert/trading-bot/pkg/metrics"
)

const defaultOpenRouterURL = "https://openrouter.ai/api/v1/chat/completions"

// ChatMessage is one OpenRouter chat message.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// JSONCompletionRequest asks for a JSON object response.
type JSONCompletionRequest struct {
	Service     string
	Model       string
	Fallbacks   []string
	System      string
	User        string
	MaxTokens   int
	Timeout     time.Duration
	Temperature float64
}

// JSONCompletionResult is parsed assistant content.
type JSONCompletionResult struct {
	Raw     string
	Model   string
	Content string
}

var httpDo = func(req *http.Request) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

// CompleteJSON calls OpenRouter with response_format json_object.
func CompleteJSON(ctx context.Context, req JSONCompletionRequest) (JSONCompletionResult, error) {
	key := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if key == "" {
		return JSONCompletionResult{}, fmt.Errorf("OPENROUTER_API_KEY is not set")
	}
	models := modelChain(req.Model, req.Fallbacks)
	if len(models) == 0 {
		models = []string{"anthropic/claude-sonnet-4"}
	}
	maxTok := req.MaxTokens
	if maxTok <= 0 {
		maxTok = 4096
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	temp := req.Temperature
	if temp <= 0 {
		temp = 0.2
	}
	service := strings.TrimSpace(req.Service)
	if service == "" {
		service = "llm"
	}

	var lastErr error
	for _, model := range models {
		start := time.Now()
		body := map[string]any{
			"model": model,
			"messages": []ChatMessage{
				{Role: "system", Content: req.System},
				{Role: "user", Content: req.User},
			},
			"max_tokens":      maxTok,
			"temperature":     temp,
			"response_format": map[string]string{"type": "json_object"},
		}
		raw, err := json.Marshal(body)
		if err != nil {
			return JSONCompletionResult{}, err
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterURL(), bytes.NewReader(raw))
		if err != nil {
			return JSONCompletionResult{}, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+key)
		httpReq.Header.Set("HTTP-Referer", "https://24alert.local")
		httpReq.Header.Set("X-Title", "24alert")

		resp, err := httpDo(httpReq)
		if err != nil {
			lastErr = err
			metrics.RecordLLMRequest(service, model, metrics.ClassifyLLMError(err), time.Since(start))
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("openrouter %d: %s", resp.StatusCode, truncate(string(b), 400))
			metrics.RecordLLMRequest(service, model, metrics.ClassifyLLMError(lastErr), time.Since(start))
			continue
		}
		var parsed struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(b, &parsed); err != nil {
			lastErr = err
			metrics.RecordLLMRequest(service, model, metrics.LLMResultParseError, time.Since(start))
			continue
		}
		if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
			lastErr = fmt.Errorf("empty llm response")
			metrics.RecordLLMRequest(service, model, metrics.LLMResultParseError, time.Since(start))
			continue
		}
		content := strings.TrimSpace(parsed.Choices[0].Message.Content)
		metrics.RecordLLMRequest(service, model, metrics.LLMResultSuccess, time.Since(start))
		return JSONCompletionResult{Raw: string(b), Model: model, Content: content}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all models failed")
	}
	return JSONCompletionResult{}, lastErr
}

func openRouterURL() string {
	if u := strings.TrimSpace(os.Getenv("OPENROUTER_BASE_URL")); u != "" {
		return strings.TrimRight(u, "/") + "/chat/completions"
	}
	return defaultOpenRouterURL
}

func modelChain(primary string, fallbacks []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(m string) {
		m = strings.TrimSpace(m)
		if m != "" && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	add(primary)
	for _, f := range fallbacks {
		for _, part := range strings.Split(f, ",") {
			add(part)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
