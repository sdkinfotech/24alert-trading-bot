package tinvest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/russianinvestments/invest-api-go-sdk/investgo"
)

// --- client.go tests ---

func TestNewTInvestClient_EmptyToken(t *testing.T) {
	_, err := NewTInvestClient(context.Background(), "sandbox-invest-public-api.tbank.ru:443", "", nil)
	if err == nil {
		t.Fatal("NewTInvestClient() should return error for empty token")
	}
}

func TestSimpleLogger_Methods(t *testing.T) {
	l := NewSimpleLogger()
	if l == nil {
		t.Fatal("NewSimpleLogger() returned nil")
	}
	// These just exercise the code paths without crashing.
	l.Errorf("error: %s", "test")
	l.Infof("info: %s", "test")
	l.Debugf("debug: %s", "test")
}

func TestIsProductionEndpoint(t *testing.T) {
	c := &Client{config: investgo.Config{EndPoint: "invest-public-api.tbank.ru:443"}}
	if !c.IsProductionEndpoint() {
		t.Error("IsProductionEndpoint() should return true for production endpoint")
	}
	if c.IsSandboxEndpoint() {
		t.Error("IsSandboxEndpoint() should return false for production endpoint")
	}
}

func TestIsSandboxEndpoint(t *testing.T) {
	c := &Client{config: investgo.Config{EndPoint: "sandbox-invest-public-api.tbank.ru:443"}}
	if !c.IsSandboxEndpoint() {
		t.Error("IsSandboxEndpoint() should return true for sandbox endpoint")
	}
	if c.IsProductionEndpoint() {
		t.Error("IsProductionEndpoint() should return false for sandbox endpoint")
	}
}

func TestIsEndpoint_Unknown(t *testing.T) {
	c := &Client{config: investgo.Config{EndPoint: "custom-endpoint:443"}}
	if c.IsProductionEndpoint() {
		t.Error("IsProductionEndpoint() should return false for unknown endpoint")
	}
	if c.IsSandboxEndpoint() {
		t.Error("IsSandboxEndpoint() should return false for unknown endpoint")
	}
}

func TestClientStop_NilUnderlying(t *testing.T) {
	c := &Client{}
	// Should not panic
	c.Stop()
}

// --- ratelimiter.go additional coverage ---

func TestRefillTokens_FullRefillAfterInterval(t *testing.T) {
	rl := NewRateLimiter("test", 60)
	// Force a stale lastRefill so a full refill triggers
	rl.mu.Lock()
	rl.tokens = 0
	rl.lastRefill = time.Now().Add(-2 * time.Minute)
	rl.mu.Unlock()

	ctx := context.Background()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("Wait() after full refill should succeed: %v", err)
	}
}

func TestRateLimiterManager_ConcurrentGetLimiter(t *testing.T) {
	mgr := NewRateLimiterManager(map[string]int{"method": 100})
	var wg sync.WaitGroup
	results := make([]*RateLimiter, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = mgr.GetLimiter("method")
		}(i)
	}
	wg.Wait()

	// All goroutines should get the same instance
	for i, r := range results {
		if r != results[0] {
			t.Errorf("results[%d] is a different instance than results[0]", i)
		}
	}
}

func TestUpdateConfig_UpdatesExistingLimiter(t *testing.T) {
	mgr := NewRateLimiterManager(map[string]int{"method": 50})
	l := mgr.GetLimiter("method")
	if l.tokensPerMinute != 50 {
		t.Fatalf("initial tokensPerMinute = %d, want 50", l.tokensPerMinute)
	}

	mgr.UpdateConfig("method", 150)
	if l.tokensPerMinute != 150 {
		t.Errorf("after UpdateConfig, tokensPerMinute = %d, want 150", l.tokensPerMinute)
	}
}
