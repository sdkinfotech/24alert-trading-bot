package checker

import (
	"context"
	"fmt"
)

// BalanceChecker verifies that the account has enough withdrawable funds
// to cover the estimated cost of the order.
type BalanceChecker struct {
	portfolio PortfolioQuerier
}

func NewBalanceChecker(pq PortfolioQuerier) *BalanceChecker {
	return &BalanceChecker{portfolio: pq}
}

func (c *BalanceChecker) Check(ctx context.Context, accountID string, estimatedCost float64) *RiskCheckResult {
	result := &RiskCheckResult{Name: "balance"}

	limits, err := c.portfolio.GetWithdrawLimits(ctx, accountID)
	if err != nil {
		result.Passed = false
		result.Reason = fmt.Sprintf("failed to get withdraw limits: %v", err)
		return result
	}

	var totalAvailable float64
	for _, lim := range limits {
		totalAvailable += lim.WithdrawAmount
	}

	if totalAvailable < estimatedCost {
		result.Passed = false
		result.Reason = fmt.Sprintf("insufficient funds: available %.2f, required %.2f", totalAvailable, estimatedCost)
		return result
	}

	result.Passed = true
	result.Reason = fmt.Sprintf("sufficient funds: available %.2f, required %.2f", totalAvailable, estimatedCost)
	return result
}
