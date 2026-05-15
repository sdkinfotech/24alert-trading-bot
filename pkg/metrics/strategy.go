package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Strategy runner metrics.
var (
	StrategySignalsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "strategy",
		Name:      "signals_total",
		Help:      "Signals emitted by strategy instances.",
	}, []string{"instance", "direction"})

	StrategyOrdersTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "strategy",
		Name:      "orders_total",
		Help:      "Orders placed from strategy runner.",
	}, []string{"instance", "status"})

	StrategyEvaluationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "strategy",
		Name:      "evaluation_duration_seconds",
		Help:      "Time spent in strategy evaluation (OnCandle / gRPC Evaluate).",
		Buckets:   []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
	}, []string{"instance"})

	StrategyRealizedPnLRub = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "strategy",
		Name:      "realized_pnl_rub",
		Help:      "Cumulative realized P&L in RUB from fills (best-effort, runner ledger).",
	}, []string{"instance"})

	StrategyUnrealizedPnLRub = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "strategy",
		Name:      "unrealized_pnl_rub",
		Help:      "Mark-to-market unrealized P&L in RUB (last price vs average entry).",
	}, []string{"instance"})

	StrategyTotalPnLRub = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "strategy",
		Name:      "total_pnl_rub",
		Help:      "Realized + unrealized P&L in RUB.",
	}, []string{"instance"})

	StrategyWinRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "strategy",
		Name:      "win_rate",
		Help:      "Rolling win rate from incremental fill P&L (0..1); coarse proxy.",
	}, []string{"instance"})

	StrategyTradesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "strategy",
		Name:      "trades_total",
		Help:      "Fill events that increased realized P&L positively (win) or negatively (loss).",
	}, []string{"instance", "result"})

	StrategyDrawdownPercent = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "strategy",
		Name:      "drawdown_percent",
		Help:      "Drawdown vs peak total P&L for this instance (percent).",
	}, []string{"instance"})

	StrategySlippageBps = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "strategy",
		Name:      "slippage_bps",
		Help:      "Signed slippage vs reference price at signal time (basis points).",
		Buckets:   []float64{-500, -200, -100, -50, -20, -10, -5, 0, 5, 10, 20, 50, 100, 200, 500},
	}, []string{"instance"})

	StrategyPositionQty = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "strategy",
		Name:      "position_qty_shares",
		Help:      "Net position size in instrument units (shares) from runner ledger.",
	}, []string{"instance", "instrument"})

	StrategyReconcileMismatch = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "strategy",
		Name:      "reconcile_mismatch_total",
		Help:      "Times broker position drift exceeded tolerance vs runner ledger.",
	}, []string{"instance"})
)
