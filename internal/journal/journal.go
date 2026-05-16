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

// EventRecord persists runner decisions that do not naturally become orders
// or executions, such as a signal cancelled by session/risk guards.
type EventRecord struct {
	Type          string
	InstanceID    string
	InstrumentUID string
	Direction     string
	Quantity      int64
	OrderType     string
	RefPrice      float64
	Reason        string
	Status        string
	Message       string
	CreatedAt     time.Time
}

// TimelineEvent is a unified event for the trade event log.
type TimelineEvent struct {
	Type          string  `json:"type"` // "signal", "order", "execution"
	Time          string  `json:"time"`
	InstanceID    string  `json:"instance_id"`
	InstrumentUID string  `json:"instrument_uid,omitempty"`
	Direction     string  `json:"direction,omitempty"`
	Quantity      int64   `json:"quantity,omitempty"`
	OrderType     string  `json:"order_type,omitempty"`
	RefPrice      float64 `json:"ref_price,omitempty"`
	Reason        string  `json:"reason,omitempty"`
	OrderID       string  `json:"order_id,omitempty"`
	Status        string  `json:"status,omitempty"`
	FilledQty     int64   `json:"filled_qty,omitempty"`
	AvgPrice      float64 `json:"avg_price,omitempty"`
	Message       string  `json:"message,omitempty"`
}

// Journal persists strategy signals, orders, and executions when enabled.
type Journal interface {
	RecordSignal(ctx context.Context, r SignalRecord) error
	RecordOrder(ctx context.Context, r OrderRecord) error
	RecordExecution(ctx context.Context, r ExecutionRecord) error
	RecordEvent(ctx context.Context, r EventRecord) error
	ListRecentExecutions(ctx context.Context, instanceID string, limit int) ([]ExecutionRecord, error)
	ListRecentSignals(ctx context.Context, instanceID string, limit int) ([]SignalRecord, error)
	ListRecentOrders(ctx context.Context, instanceID string, limit int) ([]OrderRecord, error)
	ListEvents(ctx context.Context, instanceID string, limit int) ([]TimelineEvent, error)
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
func (Noop) RecordEvent(_ context.Context, _ EventRecord) error         { return nil }
func (Noop) ListRecentExecutions(_ context.Context, _ string, _ int) ([]ExecutionRecord, error) {
	return nil, nil
}
func (Noop) ListRecentSignals(_ context.Context, _ string, _ int) ([]SignalRecord, error) {
	return nil, nil
}
func (Noop) ListRecentOrders(_ context.Context, _ string, _ int) ([]OrderRecord, error) {
	return nil, nil
}
func (Noop) ListEvents(_ context.Context, _ string, _ int) ([]TimelineEvent, error) {
	return nil, nil
}
func (Noop) DailySummary(_ context.Context, _ time.Time) (DailySummary, error) {
	return DailySummary{}, nil
}
func (Noop) SaveStrategyState(_ context.Context, _ string, _ []byte) error { return nil }
func (Noop) LoadStrategyState(_ context.Context, _ string) ([]byte, error) { return nil, nil }
func (Noop) Close() error                                                  { return nil }
