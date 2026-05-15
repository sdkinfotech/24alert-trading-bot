package strategy

// StatefulStrategy is optional persistence for built-in strategies across restarts
// when order journal / SQLite is enabled.
type StatefulStrategy interface {
	Strategy
	Snapshot() ([]byte, error)
	Restore([]byte) error
}

// IndicatorProvider is optionally implemented by strategies that expose
// indicator data for visualization (charts, SMA lines, signal markers).
type IndicatorProvider interface {
	IndicatorData() interface{}
}

// WarmupHint is optionally implemented by strategies that benefit from
// historical candle prefetch on startup. The runner calls GetCandles to
// load this many completed bars before subscribing to the live stream,
// so the strategy can produce signals immediately without a cold start.
type WarmupHint interface {
	WarmupCandles() int
}
