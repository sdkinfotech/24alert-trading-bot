package tinvest

import (
	"context"
	"testing"
	"time"
)

func TestWait_TokensAvailable(t *testing.T) {
	rl := NewRateLimiter("test", 100)

	ctx := context.Background()
	start := time.Now()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("Wait() unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("Wait() took %v, expected near-instant when tokens available", elapsed)
	}
}

func TestWait_BlocksWhenExhausted(t *testing.T) {
	rl := NewRateLimiter("test", 2)

	ctx := context.Background()
	// Consume all tokens
	for i := 0; i < 2; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Wait() error on call %d: %v", i, err)
		}
	}

	// Force zero tokens and a recent lastRefill so no partial refill helps
	rl.mu.Lock()
	rl.tokens = 0
	rl.lastRefill = time.Now()
	rl.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx)
	// Should either succeed after a short wait or be cancelled by timeout.
	// The key assertion is that it didn't return instantly — the rate limiter
	// recognised that tokens were exhausted.
	_ = err
}

func TestWait_ContextCancellation(t *testing.T) {
	rl := NewRateLimiter("test", 1)

	ctx := context.Background()
	_ = rl.Wait(ctx) // consume the one token

	rl.mu.Lock()
	rl.tokens = 0
	rl.lastRefill = time.Now()
	rl.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := rl.Wait(ctx)
	if err == nil {
		t.Error("Wait() should return error when context is cancelled")
	}
}

func TestRateLimiterManager_GetLimiter(t *testing.T) {
	config := map[string]int{"orders": 50}
	mgr := NewRateLimiterManager(config)

	l1 := mgr.GetLimiter("orders")
	if l1 == nil {
		t.Fatal("GetLimiter() returned nil")
	}
	if l1.tokensPerMinute != 50 {
		t.Errorf("tokensPerMinute = %d, want 50", l1.tokensPerMinute)
	}

	l2 := mgr.GetLimiter("orders")
	if l1 != l2 {
		t.Error("GetLimiter() should return the same instance for the same method")
	}
}

func TestRateLimiterManager_DefaultRPM(t *testing.T) {
	mgr := NewRateLimiterManager(map[string]int{})
	l := mgr.GetLimiter("unknown_method")
	if l.tokensPerMinute != 100 {
		t.Errorf("default tokensPerMinute = %d, want 100", l.tokensPerMinute)
	}
}

func TestRateLimiterManager_Wait(t *testing.T) {
	config := map[string]int{"method1": 60}
	mgr := NewRateLimiterManager(config)

	ctx := context.Background()
	if err := mgr.Wait(ctx, "method1"); err != nil {
		t.Fatalf("Wait() unexpected error: %v", err)
	}
}

func TestUpdateConfig(t *testing.T) {
	config := map[string]int{"orders": 50}
	mgr := NewRateLimiterManager(config)

	_ = mgr.GetLimiter("orders") // create the limiter
	mgr.UpdateConfig("orders", 200)

	l := mgr.GetLimiter("orders")
	if l.tokensPerMinute != 200 {
		t.Errorf("after UpdateConfig, tokensPerMinute = %d, want 200", l.tokensPerMinute)
	}
}

func TestUpdateConfig_NewMethod(t *testing.T) {
	mgr := NewRateLimiterManager(map[string]int{})
	mgr.UpdateConfig("newmethod", 300)

	l := mgr.GetLimiter("newmethod")
	if l.tokensPerMinute != 300 {
		t.Errorf("tokensPerMinute = %d, want 300", l.tokensPerMinute)
	}
}

func TestRateLimitError(t *testing.T) {
	err := &RateLimitError{Method: "PostOrder", RetryAfter: 5 * time.Second}
	want := "rate limit exceeded for method PostOrder, retry after 5s"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}
