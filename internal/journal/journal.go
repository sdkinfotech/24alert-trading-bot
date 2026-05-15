package journal

import (
	"context"
	"time"
)

// SignalRecord is a strategy-emitted signal persisted for audit / replay.
type SignalRecord struct {
	InstanceID    string
	InstrumentUID string
	Direction     string
	Quantity      int64
	OrderType     string
	RefPrice      float64
	Reason        string
	CreatedAt     time.Time
}

// OrderRecord links a broker order to a strategy instance.
type OrderRecord struct {
	InstanceID    string
	OrderID       string
	InstrumentUID string
	Direction     string
	Quantity      int64
	OrderType     string
	RefPrice      float64
	CreatedAt     time.Time
}

// ExecutionRecord is persisted order execution feedback (aligned with strategy.ExecutionEvent).
type ExecutionRecord struct {
	InstanceID    string
	OrderID       string
	InstrumentUID string
	Status        string
	FilledQty     int64
	AvgPrice      float64
	Message       string
	CreatedAt     time.Time
}

// Journal persists strategy signals, orders, and executions when enabled.
type Journal interface {
	RecordSignal(ctx context.Context, r SignalRecord) error
	RecordOrder(ctx context.Context, r OrderRecord) error
	RecordExecution(ctx context.Context, r ExecutionRecord) error
	ListRecentExecutions(ctx context.Context, instanceID string, limit int) ([]ExecutionRecord, error)
	DailySummary(ctx context.Context, day time.Time) (DailySummary, error)
	SaveStrategyState(ctx context.Context, instanceID string, state []byte) error
	LoadStrategyState(ctx context.Context, instanceID string) ([]byte, error)
	Close() error
}

// DailySummary aggregates persisted activity for a UTC calendar day.
type DailySummary struct {
	DayUTC          time.Time
	SignalsCount    int64
	OrdersCount     int64
	ExecutionsCount int64
}

// Noop is a no-op journal (feature disabled).
type Noop struct{}

func (Noop) RecordSignal(_ context.Context, _ SignalRecord) error       { return nil }
func (Noop) RecordOrder(_ context.Context, _ OrderRecord) error         { return nil }
func (Noop) RecordExecution(_ context.Context, _ ExecutionRecord) error { return nil }
func (Noop) ListRecentExecutions(_ context.Context, _ string, _ int) ([]ExecutionRecord, error) {
	return nil, nil
}
func (Noop) DailySummary(_ context.Context, _ time.Time) (DailySummary, error) {
	return DailySummary{}, nil
}
func (Noop) SaveStrategyState(_ context.Context, _ string, _ []byte) error { return nil }
func (Noop) LoadStrategyState(_ context.Context, _ string) ([]byte, error) { return nil, nil }
func (Noop) Close() error                                                  { return nil }
