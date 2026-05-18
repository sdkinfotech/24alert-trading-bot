package strategy

import (
	"bytes"
	"context"
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
	openRouterURL    = "https://openrouter.ai/api/v1/chat/completions"
	defaultAIModel   = "anthropic/claude-sonnet-4"
	maxHistoryLen    = 20
	aiRequestTimeout = 120 * time.Second
)

func shortUID(uid string) string {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return "unknown"
	}
	if len(uid) > 8 {
		return uid[:8]
	}
	return uid
}

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

func systemPrompt(fullContext string) string {
	return fmt.Sprintf(`Ты — AI-ассистент торговой системы 24alert. Ты встроен в дашборд strategy-runner и имеешь полный доступ к текущему состоянию системы.

Твои возможности:
- Портфель: баланс счёта, позиции брокера (количество, средняя цена, текущая цена, доходность)
- Стратегии: тип, параметры (fast/slow, ATR, SL/TP, cutoff, interval, quantity), статус
- P&L: для futures unrealized/total должны опираться на broker ExpectedYield, а не на разницу цены из runner ledger
- Позиции (ledger): количество, средняя цена по каждой стратегии
- Индикаторы: SMA линии, S/R уровни, ATR, ORB range — текущие значения
- Рыночные цены: последние цены инструментов стратегий (из кеша)
- Сигналы: последние 5 сигналов каждой стратегии (направление, цена, причина)
- Сделки: последние 5 исполнений (статус, объём, цена)
- Дневная статистика: количество сигналов, ордеров, исполнений за сегодня
- Глобальное расписание FORTS session guard и причины блокировки сигналов
- Журнал Trade Events: сигналы, отменённые сигналы, ордера, исполнения

Правила ответа:
- Отвечай на русском, кратко и по существу
- Используй конкретные цифры из контекста — не додумывай данные
- Если данных нет (портфель недоступен, стратегия остановлена) — скажи об этом явно
- Перед ответом о том, торгуется ли сейчас система, всегда смотри блоки "СИСТЕМНЫЕ ПРАВИЛА", "РАСПИСАНИЕ", "СТРАТЕГИИ" и "ЖУРНАЛ/TRADE EVENTS".
- Не делай вывод о разрешённых торгах только по cutoff стратегии: cutoff — это EOD-ограничение конкретной стратегии, а не глобальный session guard.
- Если спрашивают про выходные, отвечай явно: автоторговля по выходным запрещена намеренно из-за низкой ликвидности/тонкого рынка.
- Если спрашивают про логи, используй Trade Events из контекста как журнал стратегии; если нужны raw Docker/system logs — скажи, что они проверяются операционным skill через docker logs на VPS.
- Форматируй числа: цены с копейками (123.45₽), количество целым числом
- Для PnL используй +/- знак и ₽ символ

%s`, fullContext)
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

func (r *Runner) buildFullContext(ctx context.Context) string {
	var b strings.Builder

	r.writeSystemFacts(&b)

	// --- Portfolio per unique account ---
	accounts := make(map[string]bool)
	for _, inst := range r.strategiesCfg.Instances {
		if inst.AccountID != "" && !accounts[inst.AccountID] {
			accounts[inst.AccountID] = true
		}
	}
	if len(accounts) > 0 {
		b.WriteString("== ПОРТФЕЛЬ ==\n")
		for acc := range accounts {
			b.WriteString(r.PortfolioSnapshot(ctx, acc))
			b.WriteString("\n")
		}
	}

	// --- Strategies ---
	b.WriteString("== СТРАТЕГИИ ==\n\n")
	for _, inst := range r.strategiesCfg.Instances {
		running := r.InstanceRunning(inst.ID)
		tickers := r.InstanceTickers(inst)
		status := "stopped"
		if running {
			status = "running"
		}
		fmt.Fprintf(&b, "[%s] %s | %s | %s\n", inst.ID, inst.Type, tickers, status)

		// Params from config
		if len(inst.Params) > 0 {
			parts := make([]string, 0, len(inst.Params))
			for k, v := range inst.Params {
				parts = append(parts, k+"="+v)
			}
			fmt.Fprintf(&b, "  Параметры: %s\n", strings.Join(parts, ", "))
		}

		if !running {
			b.WriteString("\n")
			continue
		}

		// PnL
		if rl, un, tot, source, ok := r.InstancePNLBrokerAware(ctx, inst.ID); ok {
			fmt.Fprintf(&b, "  PnL: realized=%.2f₽ unrealized=%.2f₽ total=%.2f₽ source=%s\n", rl, un, tot, source)
		}

		// Ledger positions
		if qty, avg, _, ok := r.InstanceLedgerPositions(inst.ID); ok && len(qty) > 0 {
			for uid, q := range qty {
				if q == 0 {
					continue
				}
				ticker := shortUID(uid)
				if inf, ok := r.instrCache.GetInstrument(uid); ok && inf.Ticker != "" {
					ticker = inf.Ticker
				}
				avgP := avg[uid]
				fmt.Fprintf(&b, "  Позиция (ledger): %s %.0f шт, avg=%.2f₽\n", ticker, q, avgP)
			}
		} else {
			b.WriteString("  Позиция: flat\n")
		}

		// Current prices for instruments
		for _, uid := range inst.Instruments {
			uid = strings.TrimSpace(uid)
			if uid == "" {
				continue
			}
			if px, ok := r.priceCache.GetLastPrice(uid); ok {
				ticker := shortUID(uid)
				if inf, ok := r.instrCache.GetInstrument(uid); ok && inf.Ticker != "" {
					ticker = inf.Ticker
				}
				fmt.Fprintf(&b, "  Цена %s: %.2f₽ (обновлено %s)\n",
					ticker, px.Price, px.UpdatedAt.Format("15:04:05"))
			}
		}

		// Indicator data (key numbers only, not candle arrays)
		if data, ok := r.InstanceIndicatorData(inst.ID); ok {
			b.WriteString(formatIndicatorSummary(data))
		}

		// Recent signals (last 5)
		if sigs, err := r.InstanceRecentSignals(ctx, inst.ID, 5); err == nil && len(sigs) > 0 {
			b.WriteString("  Последние сигналы:\n")
			for _, s := range sigs {
				ticker := shortUID(s.InstrumentUID)
				if inf, ok := r.instrCache.GetInstrument(s.InstrumentUID); ok && inf.Ticker != "" {
					ticker = inf.Ticker
				}
				fmt.Fprintf(&b, "    %s %s %s ref=%.2f₽ %q\n",
					s.CreatedAt.Format("02.01 15:04"), s.Direction, ticker, s.RefPrice, s.Reason)
			}
		}

		// Recent executions (last 5)
		if execs, err := r.InstanceRecentExecutions(ctx, inst.ID, 5); err == nil && len(execs) > 0 {
			b.WriteString("  Последние сделки:\n")
			for _, e := range execs {
				ticker := shortUID(e.InstrumentUID)
				if inf, ok := r.instrCache.GetInstrument(e.InstrumentUID); ok && inf.Ticker != "" {
					ticker = inf.Ticker
				}
				fmt.Fprintf(&b, "    %s %s %s %d лот @%.2f₽ [%s]\n",
					e.CreatedAt.Format("02.01 15:04"), e.Status, ticker,
					e.FilledQty, e.AvgPrice, e.Message)
			}
		}

		// Unified journal timeline (signals, cancellations, orders, executions).
		if events, err := r.InstanceEvents(ctx, inst.ID, 10); err == nil && len(events) > 0 {
			b.WriteString("  Журнал/Trade Events (последние 10):\n")
			for _, ev := range events {
				ticker := shortUID(ev.InstrumentUID)
				if inf, ok := r.instrCache.GetInstrument(ev.InstrumentUID); ok && inf.Ticker != "" {
					ticker = inf.Ticker
				}
				fmt.Fprintf(&b, "    %s %s %s %s qty=%d ref=%.2f status=%s reason=%q msg=%q\n",
					ev.Time, ev.Type, ev.Direction, ticker, ev.Quantity, ev.RefPrice, ev.Status, ev.Reason, ev.Message)
			}
		} else {
			b.WriteString("  Журнал/Trade Events: нет недавних событий\n")
		}

		b.WriteString("\n")
	}

	// --- Daily summary ---
	if sum, err := r.DailyJournalSummary(ctx, time.Now().UTC()); err == nil {
		fmt.Fprintf(&b, "== СЕГОДНЯ (UTC %s) ==\n", sum.DayUTC.Format("2006-01-02"))
		fmt.Fprintf(&b, "Signals: %d | Orders: %d | Executions: %d\n",
			sum.SignalsCount, sum.OrdersCount, sum.ExecutionsCount)
	}

	if b.Len() == 0 {
		b.WriteString("Нет настроенных стратегий\n")
	}
	return b.String()
}

func (r *Runner) writeSystemFacts(b *strings.Builder) {
	b.WriteString("== СИСТЕМНЫЕ ПРАВИЛА ==\n")
	b.WriteString("- Production режим: только MOEX FORTS futures; акции/ETF/валюта/облигации в стратегии не добавлять.\n")
	b.WriteString("- Ручные стратегии без префикса auto- не менять без явной команды пользователя.\n")
	b.WriteString("- Weekend trading запрещён намеренно: низкая ликвидность и тонкий рынок важнее broker trading-status.\n")
	b.WriteString("- cutoff_hour/cutoff_min у level_bounce — это EOD-ограничение стратегии, не глобальное расписание торгов.\n\n")

	b.WriteString("== РАСПИСАНИЕ / SESSION GUARD ==\n")
	nowUTC := time.Now().UTC()
	fmt.Fprintf(b, "UTC now: %s\n", nowUTC.Format("2006-01-02 15:04:05"))
	if r.schedule == nil {
		b.WriteString("TradingSchedule: не настроен; это аварийное состояние, нельзя делать выводы по cutoff.\n\n")
		return
	}
	nowLocal := nowUTC.In(r.schedule.tz)
	allowed := r.schedule.IsMainSession(nowUTC)
	fmt.Fprintf(b, "Local now: %s %s (%s)\n", nowLocal.Format("2006-01-02 15:04:05"), r.schedule.TimezoneName(), nowLocal.Weekday())
	fmt.Fprintf(b, "Allowed sessions: Mon-Fri %s %s\n", r.schedule.WindowString(), r.schedule.TimezoneName())
	fmt.Fprintf(b, "Trading allowed now: %v\n", allowed)
	if !allowed {
		fmt.Fprintf(b, "Next allowed open: %s %s\n", r.schedule.NextSessionOpen(nowUTC).Format("2006-01-02 15:04:05"), r.schedule.TimezoneName())
	}
	b.WriteString("Blocked intervals: weekends, clearing break 14:00-14:05 MSK, and gaps outside configured FORTS sessions.\n\n")
}

func formatIndicatorSummary(data interface{}) string {
	raw, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}

	var b strings.Builder
	stype, _ := m["strategy_type"].(string)
	pos, _ := m["position"].(float64)

	posStr := "flat"
	switch {
	case pos > 0:
		posStr = "long"
	case pos < 0:
		posStr = "short"
	}

	switch stype {
	case "sma_crossover", "":
		if fn, ok := m["fast_period"].(float64); ok {
			if sn, ok := m["slow_period"].(float64); ok {
				fmt.Fprintf(&b, "  SMA: fast_period=%.0f, slow_period=%.0f, position=%s\n", fn, sn, posStr)
			}
		}
	case "level_bounce":
		if atr, ok := m["atr"].(float64); ok {
			fmt.Fprintf(&b, "  ATR=%.2f₽, position=%s\n", atr, posStr)
		}
		if sup, ok := m["support"].([]interface{}); ok && len(sup) > 0 {
			vals := make([]string, 0, len(sup))
			for _, v := range sup {
				if f, ok := v.(float64); ok {
					vals = append(vals, fmt.Sprintf("%.2f", f))
				}
			}
			fmt.Fprintf(&b, "  Support: [%s]\n", strings.Join(vals, ", "))
		}
		if res, ok := m["resistance"].([]interface{}); ok && len(res) > 0 {
			vals := make([]string, 0, len(res))
			for _, v := range res {
				if f, ok := v.(float64); ok {
					vals = append(vals, fmt.Sprintf("%.2f", f))
				}
			}
			fmt.Fprintf(&b, "  Resistance: [%s]\n", strings.Join(vals, ", "))
		}
	case "orb_breakout":
		if rh, ok := m["range_high"].(float64); ok {
			rl, _ := m["range_low"].(float64)
			formed, _ := m["range_formed"].(bool)
			fmt.Fprintf(&b, "  Range: %.2f-%.2f (formed=%v), position=%s\n", rl, rh, formed, posStr)
		}
	}
	return b.String()
}

func registerAIChatHandlers(mux *http.ServeMux, r *Runner, parentCtx context.Context) {
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(aiChatResponse{Error: "bad request: " + err.Error()})
			return
		}
		if strings.TrimSpace(body.Message) == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(aiChatResponse{Error: "empty message"})
			return
		}

		model := os.Getenv("AI_CHAT_MODEL")
		if model == "" {
			model = defaultAIModel
		}

		chat.addUser(body.Message)

		ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
		fullCtx := r.buildFullContext(ctx)
		cancel()
		msgs := chat.messages(systemPrompt(fullCtx))

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
