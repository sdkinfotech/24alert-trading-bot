package risk

import "time"

// OrderIntent describes an order that the strategy wants to place.
// The risk service validates it before it reaches the order service.
type OrderIntent struct {
	AccountID     string
	InstrumentUID string
	Direction     string // "buy" or "sell"
	Quantity      int64
	EstimatedCost float64
}

// RiskCheckResult is the outcome of a single risk check.
type RiskCheckResult struct {
	Name   string
	Passed bool
	Reason string
}

// RiskResponse aggregates the results of all checks for one order intent.
type RiskResponse struct {
	Allowed bool
	Checks  []RiskCheckResult
}

// RiskStatus exposes the current health of the risk service.
type RiskStatus struct {
	CircuitBreakerTripped bool
	FailureCount          int
	LastFailure           time.Time
	Threshold             int
	Cooldown              time.Duration
}
