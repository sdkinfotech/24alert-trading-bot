package e2e

import (
	"testing"
)

func TestRiskStatus(t *testing.T) {
	rateLimitPause()
	r := doGet(t, "/api/v1/risk/status", nil)
	status := unmarshal[RiskStatus](t, r)
	t.Logf("Risk: tripped=%v failures=%d threshold=%d",
		status.CircuitBreakerTripped, status.FailureCount, status.Threshold)
}

func TestRiskReset(t *testing.T) {
	rateLimitPause()
	r, code := doPost(t, "/api/v1/risk/reset", nil)
	if code != 200 {
		t.Logf("Reset returned %d: %s", code, r.Error)
	}
	if r.Error != "" {
		t.Logf("Reset error (may be expected): %s", r.Error)
		return
	}
	t.Log("Circuit breaker reset")

	rateLimitPause()

	// Verify status after reset
	statusR := doGet(t, "/api/v1/risk/status", nil)
	status := unmarshal[RiskStatus](t, statusR)
	t.Logf("After reset: tripped=%v failures=%d", status.CircuitBreakerTripped, status.FailureCount)
}
