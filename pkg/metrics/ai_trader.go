package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	AITraderOrdersTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "ai_trader",
		Name:      "orders_total",
		Help:      "AI Trader paper/live order events.",
	}, []string{"mode", "side", "status"})
)
