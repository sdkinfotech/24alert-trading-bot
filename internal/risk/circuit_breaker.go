package risk

import (
	"sync"
	"time"
)

// CircuitBreakerState is a read-only snapshot of the breaker.
type CircuitBreakerState struct {
	Tripped      bool
	FailureCount int
	LastFailure  time.Time
	Threshold    int
	Cooldown     time.Duration
}

// CircuitBreaker tracks consecutive failures and trips after a threshold.
// Once tripped it stays open until the cooldown expires or Reset is called.
type CircuitBreaker struct {
	mu           sync.Mutex
	threshold    int
	cooldown     time.Duration
	failureCount int
	lastFailure  time.Time
	tripped      bool
}

func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		cooldown:  cooldown,
	}
}

// RecordFailure increments the failure counter and trips the breaker
// once the threshold is reached.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailure = time.Now()
	if cb.failureCount >= cb.threshold {
		cb.tripped = true
	}
}

// RecordSuccess resets the failure counter (but does not un-trip).
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount = 0
}

// IsTripped returns true when the breaker is open and cooldown has not
// elapsed yet.  It automatically resets after the cooldown period.
func (cb *CircuitBreaker) IsTripped() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !cb.tripped {
		return false
	}
	if time.Since(cb.lastFailure) >= cb.cooldown {
		cb.tripped = false
		cb.failureCount = 0
		return false
	}
	return true
}

// Reset manually resets the breaker regardless of cooldown.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.tripped = false
	cb.failureCount = 0
	cb.lastFailure = time.Time{}
}

// State returns a point-in-time snapshot.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return CircuitBreakerState{
		Tripped:      cb.tripped,
		FailureCount: cb.failureCount,
		LastFailure:  cb.lastFailure,
		Threshold:    cb.threshold,
		Cooldown:     cb.cooldown,
	}
}
