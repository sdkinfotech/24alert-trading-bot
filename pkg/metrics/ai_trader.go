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

	AITraderPnLRUB = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "ai_trader",
		Name:      "pnl_rub",
		Help:      "AI Trader session PnL in RUB (realized + unrealized).",
	}, []string{"session_id", "ticker"})

	AITraderTradesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "ai_trader",
		Name:      "trades_total",
		Help:      "AI Trader closed round-trips.",
	}, []string{"session_id", "side", "result"})

	AITraderWinRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "ai_trader",
		Name:      "win_rate",
		Help:      "AI Trader win rate (0-1) per session.",
	}, []string{"session_id"})

	AITraderDrawdownRUB = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "ai_trader",
		Name:      "drawdown_rub",
		Help:      "AI Trader peak-to-trough drawdown in RUB.",
	}, []string{"session_id"})

	AITraderFillQualityBPS = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "ai_trader",
		Name:      "fill_quality_bps",
		Help:      "Limit price vs fill price in bps (signed: worse fills positive).",
		Buckets:   []float64{-20, -10, -5, -2, -1, 0, 1, 2, 5, 10, 20, 50},
	}, []string{"side"})

	AITraderMicroSignalsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "ai_trader",
		Name:      "micro_signals_total",
		Help:      "Deterministic microstructure signals.",
	}, []string{"kind"})

	AITraderRegime = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "ai_trader",
		Name:      "regime",
		Help:      "Session regime encoded: trend=1 range=2 breakout=3 low_vol=4.",
	}, []string{"session_id"})

	AITraderKillSwitch = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "ai_trader",
		Name:      "kill_switch_active",
		Help:      "1 when global AI Trader kill switch is on.",
	})

	AITraderPolicyUpdatesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "ai_trader",
		Name:      "policy_updates_total",
		Help:      "Dynamic trading policy updates from LLM.",
	})

	AITraderStopAdjustmentsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "ai_trader",
		Name:      "stop_adjustments_total",
		Help:      "Soft SL/TP adjustments from LLM trade_signal.",
	})
)
