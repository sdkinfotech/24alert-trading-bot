package strategy

// StatefulStrategy is optional persistence for built-in strategies across restarts
// when order journal / SQLite is enabled.
type StatefulStrategy interface {
	Strategy
	Snapshot() ([]byte, error)
	Restore([]byte) error
}
