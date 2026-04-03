package risk

import (
	"context"
	"testing"
	"time"

	"github.com/24alert/trading-bot/internal/risk/checker"
	"github.com/24alert/trading-bot/pkg/config"
	"github.com/24alert/trading-bot/pkg/logging"
)

// --- mocks for checker interfaces ---

type mockMarketData struct {
	status *checker.TradingStatusInfo
	err    error
}

func (m *mockMarketData) GetTradingStatus(_ context.Context, _ string) (*checker.TradingStatusInfo, error) {
	return m.status, m.err
}

type mockPortfolio struct {
	positions []checker.PositionInfo
	limits    []checker.WithdrawLimitInfo
	posErr    error
	limErr    error
}

func (m *mockPortfolio) GetPositions(_ context.Context, _ string) ([]checker.PositionInfo, error) {
	return m.positions, m.posErr
}

func (m *mockPortfolio) GetWithdrawLimits(_ context.Context, _ string) ([]checker.WithdrawLimitInfo, error) {
	return m.limits, m.limErr
}

func newTestService(t *testing.T, tripped bool, sessionOK bool) *Service {
	t.Helper()

	md := &mockMarketData{status: &checker.TradingStatusInfo{APITradeAvailable: sessionOK}}
	pq := &mockPortfolio{
		limits:    []checker.WithdrawLimitInfo{{Currency: "RUB", WithdrawAmount: 1_000_000}},
		positions: []checker.PositionInfo{},
	}

	cb := NewCircuitBreaker(5, time.Hour)
	if tripped {
		for i := 0; i < 5; i++ {
			cb.RecordFailure()
		}
	}

	logger, _ := logging.NewLogger("error", "text", "stdout", "")

	return NewService(
		checker.NewSessionChecker(md),
		checker.NewBalanceChecker(pq),
		checker.NewPositionLimitChecker(pq, 100),
		cb,
		config.RiskConfig{CheckTradingSession: true},
		logger,
	)
}

func TestValidateOrderIntent_CircuitBreakerTripped(t *testing.T) {
	svc := newTestService(t, true, true)
	resp, err := svc.ValidateOrderIntent(context.Background(), OrderIntent{
		AccountID:     "acc1",
		InstrumentUID: "uid1",
		Direction:     "buy",
		Quantity:      1,
		EstimatedCost: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Allowed {
		t.Fatal("expected rejected when circuit breaker tripped")
	}
	if len(resp.Checks) == 0 || resp.Checks[0].Name != "circuit_breaker" {
		t.Fatalf("expected circuit_breaker check, got: %+v", resp.Checks)
	}
}

func TestValidateOrderIntent_AllChecksPassing(t *testing.T) {
	svc := newTestService(t, false, true)
	resp, err := svc.ValidateOrderIntent(context.Background(), OrderIntent{
		AccountID:     "acc1",
		InstrumentUID: "uid1",
		Direction:     "buy",
		Quantity:      1,
		EstimatedCost: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Allowed {
		t.Fatalf("expected allowed, checks: %+v", resp.Checks)
	}
}

func TestValidateOrderIntent_SessionClosed(t *testing.T) {
	svc := newTestService(t, false, false)
	resp, err := svc.ValidateOrderIntent(context.Background(), OrderIntent{
		AccountID:     "acc1",
		InstrumentUID: "uid1",
		Direction:     "buy",
		Quantity:      1,
		EstimatedCost: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Allowed {
		t.Fatal("expected rejected when session closed")
	}
}

func TestGetRiskStatus(t *testing.T) {
	svc := newTestService(t, false, true)
	st := svc.GetRiskStatus(context.Background())
	if st.CircuitBreakerTripped {
		t.Error("expected not tripped")
	}
	if st.Threshold != 5 {
		t.Errorf("threshold = %d, want 5", st.Threshold)
	}
}

func TestResetCircuitBreaker(t *testing.T) {
	svc := newTestService(t, true, true)
	if err := svc.ResetCircuitBreaker(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	st := svc.GetRiskStatus(context.Background())
	if st.CircuitBreakerTripped {
		t.Error("expected not tripped after reset")
	}
}
