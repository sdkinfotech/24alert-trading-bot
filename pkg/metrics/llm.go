package metrics

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// LLM service names for metrics labels.
const (
	LLMServiceAdvisor    = "advisor"
	LLMServiceAITrader   = "ai_trader"
	LLMServiceAIChat     = "ai_chat"
	LLMServiceAssistant  = "assistant"
)

// LLM call outcomes (result label).
const (
	LLMResultSuccess    = "success"
	LLMResultHTTPError  = "http_error"
	LLMResultRateLimit  = "rate_limit"
	LLMResultParseError = "parse_error"
	LLMResultFallback   = "fallback"
	LLMResultError      = "error"
)

var (
	LLMRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "llm",
		Name:      "requests_total",
		Help:      "OpenRouter LLM calls by service, model, and result.",
	}, []string{"service", "model", "result"})

	LLMRequestDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "llm",
		Name:      "request_duration_seconds",
		Help:      "Successful LLM round-trip latency by service and model.",
		Buckets:   []float64{0.5, 1, 2, 5, 10, 20, 30, 45, 60, 90, 120},
	}, []string{"service", "model"})
)

// LLMModelLabel sanitizes OpenRouter model ids for Prometheus label values.
func LLMModelLabel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return "unknown"
	}
	r := strings.NewReplacer("/", "_", ":", "_", ",", "_", " ", "_")
	return r.Replace(model)
}

// RecordLLMRequest increments request counter and optionally observes latency.
func RecordLLMRequest(service, model, result string, duration time.Duration) {
	model = LLMModelLabel(model)
	LLMRequestsTotal.WithLabelValues(service, model, result).Inc()
	if result == LLMResultSuccess && duration > 0 {
		LLMRequestDurationSeconds.WithLabelValues(service, model).Observe(duration.Seconds())
	}
}

// ClassifyLLMError maps an error to a result label for metrics.
func ClassifyLLMError(err error) string {
	if err == nil {
		return LLMResultError
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "429"), strings.Contains(msg, "rate-limit"), strings.Contains(msg, "rate limited"):
		return LLMResultRateLimit
	case strings.Contains(msg, "parse"), strings.Contains(msg, "json"), strings.Contains(msg, "empty analysis"),
		strings.Contains(msg, "empty llm"), strings.Contains(msg, "empty choices"):
		return LLMResultParseError
	default:
		if strings.Contains(msg, "openrouter") {
			return LLMResultHTTPError
		}
		return LLMResultError
	}
}
