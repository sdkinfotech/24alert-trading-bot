package strategy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	openRouterURL   = "https://openrouter.ai/api/v1/chat/completions"
	defaultAIModel  = "anthropic/claude-sonnet-4"
	maxHistoryLen   = 20
	aiRequestTimeout = 120 * time.Second
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type aiChatRequest struct {
	Message string `json:"message"`
}

type aiChatResponse struct {
	Reply string `json:"reply"`
	Model string `json:"model"`
	Error string `json:"error,omitempty"`
}

type aiChatStatusResponse struct {
	Available    bool   `json:"available"`
	Model        string `json:"model"`
	ScannerCron  bool   `json:"scanner_cron"`
	CursorKeySet bool   `json:"cursor_key_set"`
}

func systemPrompt(instancesSummary string) string {
	return fmt.Sprintf(`Ты — AI-ассистент торговой системы 24alert. Ты встроен в дашборд и помогаешь трейдеру:
- Объясняешь текущее состояние стратегий, позиций и PnL
- Даёшь рекомендации по параметрам стратегий
- Анализируешь торговые результаты
- Отвечаешь на вопросы о рынке и стратегиях

Текущее состояние системы:
%s

Отвечай кратко и по существу на русском языке. Используй цифры и факты из контекста.`, instancesSummary)
}

type aiChat struct {
	mu      sync.Mutex
	history []chatMessage
}

func (c *aiChat) addUser(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = append(c.history, chatMessage{Role: "user", Content: msg})
	if len(c.history) > maxHistoryLen {
		c.history = c.history[len(c.history)-maxHistoryLen:]
	}
}

func (c *aiChat) addAssistant(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = append(c.history, chatMessage{Role: "assistant", Content: msg})
	if len(c.history) > maxHistoryLen {
		c.history = c.history[len(c.history)-maxHistoryLen:]
	}
}

func (c *aiChat) messages(sysPrompt string) []chatMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	msgs := make([]chatMessage, 0, len(c.history)+1)
	msgs = append(msgs, chatMessage{Role: "system", Content: sysPrompt})
	msgs = append(msgs, c.history...)
	return msgs
}

func (c *aiChat) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = nil
}

func callOpenRouter(apiKey, model string, messages []chatMessage) (string, error) {
	body := map[string]any{
		"model":    model,
		"messages": messages,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, openRouterURL, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://24alert.ru")
	req.Header.Set("X-Title", "24alert Trading Bot")

	client := &http.Client{Timeout: aiRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter %d: %s", resp.StatusCode, string(respBody))
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
		return "", fmt.Errorf("unmarshal: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("api error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty choices")
	}
	return result.Choices[0].Message.Content, nil
}

func (r *Runner) buildInstancesSummary() string {
	var b strings.Builder
	for _, inst := range r.strategiesCfg.Instances {
		running := r.InstanceRunning(inst.ID)
		tickers := r.InstanceTickers(inst)
		status := "stopped"
		if running {
			status = "running"
		}
		fmt.Fprintf(&b, "- %s (%s) [%s] tickers=%s", inst.ID, inst.Type, status, tickers)
		if running {
			rl, un, tot, ok := r.InstancePNL(inst.ID)
			if ok {
				fmt.Fprintf(&b, " PnL: realized=%.2f unrealized=%.2f total=%.2f₽", rl, un, tot)
			}
		}
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		b.WriteString("Нет настроенных стратегий\n")
	}
	return b.String()
}

func registerAIChatHandlers(mux *http.ServeMux, r *Runner) {
	chat := &aiChat{}

	mux.HandleFunc("POST /ai-chat", func(w http.ResponseWriter, req *http.Request) {
		apiKey := os.Getenv("OPENROUTER_API_KEY")
		if apiKey == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(aiChatResponse{
				Error: "OPENROUTER_API_KEY not configured",
			})
			return
		}

		var body aiChatRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Message) == "" {
			http.Error(w, "empty message", http.StatusBadRequest)
			return
		}

		model := os.Getenv("AI_CHAT_MODEL")
		if model == "" {
			model = defaultAIModel
		}

		chat.addUser(body.Message)

		summary := r.buildInstancesSummary()
		msgs := chat.messages(systemPrompt(summary))

		reply, err := callOpenRouter(apiKey, model, msgs)
		if err != nil {
			chat.addAssistant("[error: " + err.Error() + "]")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(aiChatResponse{
				Error: err.Error(),
				Model: model,
			})
			return
		}

		chat.addAssistant(reply)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiChatResponse{
			Reply: reply,
			Model: model,
		})
	})

	mux.HandleFunc("POST /ai-chat/reset", func(w http.ResponseWriter, _ *http.Request) {
		chat.reset()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("GET /ai-chat/status", func(w http.ResponseWriter, _ *http.Request) {
		orKey := os.Getenv("OPENROUTER_API_KEY")
		cursorKey := os.Getenv("CURSOR_API_KEY")
		model := os.Getenv("AI_CHAT_MODEL")
		if model == "" {
			model = defaultAIModel
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiChatStatusResponse{
			Available:    orKey != "",
			Model:        model,
			ScannerCron:  cursorKey != "" && cursorKey != "stub",
			CursorKeySet: cursorKey != "" && cursorKey != "stub",
		})
	})
}
