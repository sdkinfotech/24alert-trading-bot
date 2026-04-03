package checker

import (
	"context"
	"fmt"
)

// PositionLimitChecker ensures that the new order would not push the total
// position past the configured maximum number of lots.
type PositionLimitChecker struct {
	portfolio PortfolioQuerier
	maxLots   int
}

func NewPositionLimitChecker(pq PortfolioQuerier, maxLots int) *PositionLimitChecker {
	return &PositionLimitChecker{
		portfolio: pq,
		maxLots:   maxLots,
	}
}

func (c *PositionLimitChecker) Check(ctx context.Context, accountID, instrumentUID string, quantity int64) *RiskCheckResult {
	result := &RiskCheckResult{Name: "position_limit"}

	positions, err := c.portfolio.GetPositions(ctx, accountID)
	if err != nil {
		result.Passed = false
		result.Reason = fmt.Sprintf("failed to get positions: %v", err)
		return result
	}

	var currentQty float64
	for _, p := range positions {
		if p.InstrumentUID == instrumentUID {
			currentQty = p.Quantity
			break
		}
	}

	newTotal := currentQty + float64(quantity)
	if newTotal > float64(c.maxLots) {
		result.Passed = false
		result.Reason = fmt.Sprintf("position limit exceeded: current %.0f + order %d = %.0f, max %d",
			currentQty, quantity, newTotal, c.maxLots)
		return result
	}

	result.Passed = true
	result.Reason = fmt.Sprintf("within limit: current %.0f + order %d = %.0f, max %d",
		currentQty, quantity, newTotal, c.maxLots)
	return result
}
