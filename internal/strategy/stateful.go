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
