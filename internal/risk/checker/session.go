package checker

import (
	"context"
	"fmt"
)

// SessionChecker verifies that trading is currently open for the instrument.
type SessionChecker struct {
	marketData MarketDataQuerier
}

func NewSessionChecker(md MarketDataQuerier) *SessionChecker {
	return &SessionChecker{marketData: md}
}

func (c *SessionChecker) Check(ctx context.Context, instrumentUID string) *RiskCheckResult {
	result := &RiskCheckResult{Name: "trading_session"}

	status, err := c.marketData.GetTradingStatus(ctx, instrumentUID)
	if err != nil {
		result.Passed = false
		result.Reason = fmt.Sprintf("failed to get trading status: %v", err)
		return result
	}

	if !status.APITradeAvailable {
		result.Passed = false
		result.Reason = fmt.Sprintf("API trading not available for %s (status: %s)", instrumentUID, status.TradingStatus)
		return result
	}

	result.Passed = true
	result.Reason = "trading session is open"
	return result
}
