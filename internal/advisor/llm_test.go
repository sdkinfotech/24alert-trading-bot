package advisor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseAnalysisJSON(t *testing.T) {
	raw := `{"summary_md":"Рынок в балансе","structured":{"market_regime":"balance","conclusions":["цена у VWAP"],"next_watch":["пробой"],"confidence":0.7}}`
	out, err := parseAnalysisJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.SummaryMD == "" {
		t.Fatal("empty summary")
	}
	if len(out.Structured.Conclusions) != 1 {
		t.Fatalf("conclusions=%v", out.Structured.Conclusions)
	}
}

func TestParseAnalysisJSONWithFence(t *testing.T) {
	raw := "Here is JSON:\n```json\n{\"summary_md\":\"ok\",\"structured\":{\"conclusions\":[\"a\"]}}\n```"
	out, err := parseAnalysisJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.SummaryMD != "ok" {
		t.Fatalf("summary=%q", out.SummaryMD)
	}
}

func TestRunLLMWithFallback429ThenOK(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		writeMockCompletion(w, `{"summary_md":"ok","structured":{"conclusions":["a"],"confidence":0.5}}`)
	}))
	defer srv.Close()

	restore := installLLMTestEnv(t, srv.URL, map[string]string{
		"ADVISOR_MODEL":           "test-primary",
		"ADVISOR_MODEL_FALLBACKS": "",
		"ADVISOR_PAID_MODEL":      "test-paid-unused",
		"ADVISOR_LLM_RETRIES":     "2",
		"OPENROUTER_API_KEY":      "test-key",
	})
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	out, model, err := runLLMWithFallback(ctx, []chatMessage{{Role: "user", Content: "x"}}, parseAnalysisJSON)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if model != "test-primary" {
		t.Fatalf("model=%q", model)
	}
	if out.SummaryMD != "ok" {
		t.Fatalf("summary=%q", out.SummaryMD)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected >=2 calls, got %d", calls.Load())
	}
}

func TestRunLLMWithFallbackModelChain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req.Model {
		case "test-primary":
			writeMockCompletion(w, `not json at all`)
		case "test-fallback":
			writeMockCompletion(w, `{"summary_md":"from fallback","structured":{"conclusions":["b"],"confidence":0.6}}`)
		default:
			http.Error(w, "unexpected model", http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	restore := installLLMTestEnv(t, srv.URL, map[string]string{
		"ADVISOR_MODEL":           "test-primary",
		"ADVISOR_MODEL_FALLBACKS": "test-fallback",
		"ADVISOR_PAID_MODEL":      "test-paid",
		"ADVISOR_LLM_RETRIES":     "1",
		"OPENROUTER_API_KEY":      "test-key",
	})
	defer restore()

	ctx := context.Background()
	out, model, err := runLLMWithFallback(ctx, []chatMessage{{Role: "user", Content: "x"}}, parseAnalysisJSON)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if model != "test-fallback" {
		t.Fatalf("model=%q", model)
	}
	if out.SummaryMD != "from fallback" {
		t.Fatalf("summary=%q", out.SummaryMD)
	}
}

func TestRunAnalysisLLMFactsFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	restore := installLLMTestEnv(t, srv.URL, map[string]string{
		"ADVISOR_MODEL":           "test-primary",
		"ADVISOR_MODEL_FALLBACKS": "",
		"ADVISOR_PAID_MODEL":      "test-paid",
		"ADVISOR_LLM_RETRIES":     "1",
		"ADVISOR_FACTS_FALLBACK":  "true",
		"OPENROUTER_API_KEY":      "test-key",
	})
	defer restore()

	facts := FactsBundle{
		Ticker:      "SBER",
		PeriodLabel: "10:00–10:05",
		TextDigest:  "spread 1.2 bps\nimbalance 0.1",
	}
	rep, model, err := runAnalysisLLM(context.Background(), TF5m, "SBER", "watch", facts, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if model != factsFallbackModel {
		t.Fatalf("model=%q", model)
	}
	if rep.Status != ReportStatusOK {
		t.Fatalf("status=%q", rep.Status)
	}
	if !strings.Contains(rep.SummaryMD, "Автосводка") {
		t.Fatalf("summary=%q", rep.SummaryMD)
	}
}

func TestBuildFactsFallbackReport(t *testing.T) {
	facts := FactsBundle{Ticker: "GAZP", PeriodLabel: "p", TextDigest: "line1\nline2"}
	rep := buildFactsFallbackReport(TF5m, facts, fmt.Errorf("openrouter 429"))
	if rep.Structured.Confidence != 0.15 {
		t.Fatalf("confidence=%v", rep.Structured.Confidence)
	}
	if len(rep.Structured.Conclusions) == 0 {
		t.Fatal("expected conclusions")
	}
}

func writeMockCompletion(w http.ResponseWriter, content string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]string{"content": content}},
		},
	})
}

func installLLMTestEnv(t *testing.T, endpoint string, env map[string]string) func() {
	t.Helper()
	prevEndpoint := openRouterEndpoint
	prevDo := openRouterDo
	openRouterEndpoint = endpoint

	keys := []string{
		"ADVISOR_MODEL", "ADVISOR_MODEL_FALLBACKS", "ADVISOR_PAID_MODEL",
		"ADVISOR_LLM_RETRIES", "ADVISOR_FACTS_FALLBACK", "OPENROUTER_API_KEY",
		"AI_TRADER_MODEL", "AI_TRADER_MODEL_FALLBACKS",
	}
	prev := make(map[string]string, len(keys))
	for _, k := range keys {
		prev[k] = os.Getenv(k)
		_ = os.Unsetenv(k)
	}
	for k, v := range env {
		_ = os.Setenv(k, v)
	}

	return func() {
		openRouterEndpoint = prevEndpoint
		openRouterDo = prevDo
		for _, k := range keys {
			if v, ok := prev[k]; ok && v != "" {
				_ = os.Setenv(k, v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}
}
