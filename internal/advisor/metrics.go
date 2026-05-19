package advisor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	AdvisorReportsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "advisor",
		Name:      "reports_total",
		Help:      "Analysis reports produced by timeframe and status",
	}, []string{"timeframe", "status"})

	// AdvisorLLMErrorsTotal is deprecated; use alert24_llm_requests_total{service="advisor",result!="success"}.
	AdvisorLLMErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "advisor",
		Name:      "llm_errors_total",
		Help:      "Deprecated: LLM failures (use alert24_llm_requests_total). Kept for existing dashboards.",
	}, []string{"model"})

	AdvisorIngestSnapshotsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "advisor",
		Name:      "ingest_snapshots_total",
		Help:      "Micro snapshots persisted from strategy-runner",
	})
)
