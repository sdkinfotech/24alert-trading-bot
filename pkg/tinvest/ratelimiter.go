package tinvest

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu              sync.Mutex
	method          string
	tokensPerMinute int
	tokens          float64
	lastRefill      time.Time
	refillInterval  time.Duration
}

// NewRateLimiter creates a new rate limiter for a specific method
func NewRateLimiter(method string, requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		method:          method,
		tokensPerMinute: requestsPerMinute,
		tokens:          float64(requestsPerMinute),
		lastRefill:      time.Now(),
		refillInterval:  time.Minute,
	}
}

// refillTokens updates rl.tokens and rl.lastRefill from elapsed time. Caller must hold rl.mu.
func (rl *RateLimiter) refillTokens() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	if elapsed >= rl.refillInterval {
		rl.tokens = float64(rl.tokensPerMinute)
		rl.lastRefill = now
		return
	}
	refillRate := float64(rl.tokensPerMinute) / 60.0 // tokens per second
	tokensToAdd := refillRate * elapsed.Seconds()
	rl.tokens = min(rl.tokens+tokensToAdd, float64(rl.tokensPerMinute))
	rl.lastRefill = now
}

// Wait blocks until a token is available, respecting rate limits
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		rl.mu.Lock()
		rl.refillTokens()

		if rl.tokens >= 1.0 {
			rl.tokens--
			rl.mu.Unlock()
			return nil
		}

		tokensNeeded := 1.0 - rl.tokens
		waitSeconds := tokensNeeded / (float64(rl.tokensPerMinute) / 60.0)
		waitTime := time.Duration(waitSeconds*1000) * time.Millisecond
		rl.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
		}
	}
}

// RateLimiterManager manages rate limiters for multiple methods
type RateLimiterManager struct {
	mu        sync.RWMutex
	limiters  map[string]*RateLimiter
	configMap map[string]int // method -> rpm
}

// NewRateLimiterManager creates a new manager with configuration
func NewRateLimiterManager(config map[string]int) *RateLimiterManager {
	return &RateLimiterManager{
		limiters:  make(map[string]*RateLimiter),
		configMap: config,
	}
}

// GetLimiter returns or creates a rate limiter for the given method
func (m *RateLimiterManager) GetLimiter(method string) *RateLimiter {
	m.mu.RLock()
	limiter, exists := m.limiters[method]
	m.mu.RUnlock()

	if exists {
		return limiter
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check again in case of race condition
	limiter, exists = m.limiters[method]
	if exists {
		return limiter
	}

	// Create new limiter
	rpm := m.configMap[method]
	if rpm == 0 {
		rpm = 100 // conservative default
	}

	limiter = NewRateLimiter(method, rpm)
	m.limiters[method] = limiter
	return limiter
}

// Wait waits for the rate limiter of the given method
func (m *RateLimiterManager) Wait(ctx context.Context, method string) error {
	limiter := m.GetLimiter(method)
	return limiter.Wait(ctx)
}

// UpdateConfig updates rate limit configuration for a method
func (m *RateLimiterManager) UpdateConfig(method string, requestsPerMinute int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.configMap[method] = requestsPerMinute

	if limiter, exists := m.limiters[method]; exists {
		limiter.tokensPerMinute = requestsPerMinute
	}
}

// RateLimitError wraps rate limit errors
type RateLimitError struct {
	Method     string
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded for method %s, retry after %v", e.Method, e.RetryAfter)
}
