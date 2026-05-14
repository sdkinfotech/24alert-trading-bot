package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const namespace = "alert24"

// HTTP metrics (gateway).
var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "http",
		Name:      "requests_total",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "path"})

	HTTPResponseSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "http",
		Name:      "response_size_bytes",
		Buckets:   prometheus.ExponentialBuckets(100, 10, 6),
	}, []string{"method", "path"})
)

// Trading / order metrics.
var (
	OrdersTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "trading",
		Name:      "orders_total",
	}, []string{"direction", "order_type", "status"})

	OrderErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "trading",
		Name:      "order_errors_total",
	}, []string{"direction", "order_type"})

	OrderLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "trading",
		Name:      "order_latency_seconds",
		Buckets:   []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
	}, []string{"direction", "order_type"})
)

// T-Invest upstream API metrics.
var (
	TInvestRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "tinvest",
		Name:      "requests_total",
	}, []string{"service", "method", "status"})

	TInvestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "tinvest",
		Name:      "request_duration_seconds",
		Buckets:   []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
	}, []string{"service", "method"})

	TInvestErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "tinvest",
		Name:      "errors_total",
	}, []string{"service", "method", "error_type"})
)

// Market data freshness metrics.
var (
	MarketDataUpdatesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "marketdata",
		Name:      "updates_total",
	}, []string{"type"})

	MarketDataStaleness = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "marketdata",
		Name:      "staleness_seconds",
		Help:      "Seconds since last update for an instrument.",
	}, []string{"instrument"})
)

// Risk / circuit breaker metrics.
var (
	CircuitBreakerState = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "risk",
		Name:      "circuit_breaker_state",
		Help:      "0 = closed, 1 = open.",
	})

	RiskChecksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "risk",
		Name:      "checks_total",
	}, []string{"result"})
)
