package checker

import (
	"context"
	"errors"
	"testing"
)

// --- mocks ---

type mockMarketData struct {
	status *TradingStatusInfo
	err    error
}

func (m *mockMarketData) GetTradingStatus(_ context.Context, _ string) (*TradingStatusInfo, error) {
	return m.status, m.err
}

type mockPortfolio struct {
	positions []PositionInfo
	limits    []WithdrawLimitInfo
	posErr    error
	limErr    error
}

func (m *mockPortfolio) GetPositions(_ context.Context, _ string) ([]PositionInfo, error) {
	return m.positions, m.posErr
}

func (m *mockPortfolio) GetWithdrawLimits(_ context.Context, _ string) ([]WithdrawLimitInfo, error) {
	return m.limits, m.limErr
}

// --- SessionChecker ---

func TestSessionChecker_TradingAvailable(t *testing.T) {
	md := &mockMarketData{status: &TradingStatusInfo{APITradeAvailable: true}}
	c := NewSessionChecker(md)
	r := c.Check(context.Background(), "uid1")
	if !r.Passed {
		t.Fatalf("expected passed, got reason: %s", r.Reason)
	}
}

func TestSessionChecker_TradingNotAvailable(t *testing.T) {
	md := &mockMarketData{status: &TradingStatusInfo{APITradeAvailable: false, TradingStatus: "closed"}}
	c := NewSessionChecker(md)
	r := c.Check(context.Background(), "uid1")
	if r.Passed {
		t.Fatal("expected failed when trading not available")
	}
}

func TestSessionChecker_Error(t *testing.T) {
	md := &mockMarketData{err: errors.New("rpc fail")}
	c := NewSessionChecker(md)
	r := c.Check(context.Background(), "uid1")
	if r.Passed {
		t.Fatal("expected failed on error")
	}
	if r.Name != "trading_session" {
		t.Errorf("unexpected name: %s", r.Name)
	}
}

// --- BalanceChecker ---

func TestBalanceChecker_Sufficient(t *testing.T) {
	pq := &mockPortfolio{limits: []WithdrawLimitInfo{
		{Currency: "RUB", WithdrawAmount: 100_000},
	}}
	c := NewBalanceChecker(pq)
	r := c.Check(context.Background(), "acc1", 50_000)
	if !r.Passed {
		t.Fatalf("expected passed, reason: %s", r.Reason)
	}
}

func TestBalanceChecker_Insufficient(t *testing.T) {
	pq := &mockPortfolio{limits: []WithdrawLimitInfo{
		{Currency: "RUB", WithdrawAmount: 1_000},
	}}
	c := NewBalanceChecker(pq)
	r := c.Check(context.Background(), "acc1", 50_000)
	if r.Passed {
		t.Fatal("expected failed for insufficient funds")
	}
}

func TestBalanceChecker_Error(t *testing.T) {
	pq := &mockPortfolio{limErr: errors.New("connection lost")}
	c := NewBalanceChecker(pq)
	r := c.Check(context.Background(), "acc1", 1)
	if r.Passed {
		t.Fatal("expected failed on error")
	}
}

func TestBalanceChecker_MultipleCurrencies(t *testing.T) {
	pq := &mockPortfolio{limits: []WithdrawLimitInfo{
		{Currency: "RUB", WithdrawAmount: 30_000},
		{Currency: "USD", WithdrawAmount: 25_000},
	}}
	c := NewBalanceChecker(pq)
	r := c.Check(context.Background(), "acc1", 50_000)
	if !r.Passed {
		t.Fatalf("expected passed (30k+25k=55k >= 50k), reason: %s", r.Reason)
	}
}

// --- PositionLimitChecker ---

func TestPositionLimitChecker_WithinLimit(t *testing.T) {
	pq := &mockPortfolio{positions: []PositionInfo{
		{InstrumentUID: "uid1", Quantity: 5},
	}}
	c := NewPositionLimitChecker(pq, 10)
	r := c.Check(context.Background(), "acc1", "uid1", "buy", 3)
	if !r.Passed {
		t.Fatalf("expected passed, reason: %s", r.Reason)
	}
}

func TestPositionLimitChecker_ExceedsLimit(t *testing.T) {
	pq := &mockPortfolio{positions: []PositionInfo{
		{InstrumentUID: "uid1", Quantity: 8},
	}}
	c := NewPositionLimitChecker(pq, 10)
	r := c.Check(context.Background(), "acc1", "uid1", "buy", 5)
	if r.Passed {
		t.Fatal("expected failed: 8+5=13 > 10")
	}
}

func TestPositionLimitChecker_SellReduces(t *testing.T) {
	pq := &mockPortfolio{positions: []PositionInfo{
		{InstrumentUID: "uid1", Quantity: 8},
	}}
	c := NewPositionLimitChecker(pq, 10)
	// Old buggy logic would treat sell as +5 lots → 13 > 10 and reject.
	r := c.Check(context.Background(), "acc1", "uid1", "sell", 5)
	if !r.Passed {
		t.Fatalf("expected passed after sell reduces position, reason: %s", r.Reason)
	}
}

func TestPositionLimitChecker_NoExistingPosition(t *testing.T) {
	pq := &mockPortfolio{positions: []PositionInfo{}}
	c := NewPositionLimitChecker(pq, 10)
	r := c.Check(context.Background(), "acc1", "uid1", "buy", 5)
	if !r.Passed {
		t.Fatalf("expected passed for new position, reason: %s", r.Reason)
	}
}

func TestPositionLimitChecker_Error(t *testing.T) {
	pq := &mockPortfolio{posErr: errors.New("timeout")}
	c := NewPositionLimitChecker(pq, 10)
	r := c.Check(context.Background(), "acc1", "uid1", "buy", 1)
	if r.Passed {
		t.Fatal("expected failed on error")
	}
}
