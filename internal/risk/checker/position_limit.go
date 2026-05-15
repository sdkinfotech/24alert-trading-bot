package checker

import (
	"context"
	"fmt"
	"strings"
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

func (c *PositionLimitChecker) Check(ctx context.Context, accountID, instrumentUID, direction string, quantity int64) *RiskCheckResult {
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

	delta := float64(quantity)
	if strings.EqualFold(strings.TrimSpace(direction), "sell") {
		delta = -float64(quantity)
	}
	newTotal := currentQty + delta
	maxF := float64(c.maxLots)
	if newTotal > maxF || newTotal < -maxF {
		result.Passed = false
		result.Reason = fmt.Sprintf("position limit exceeded: current %.4f + delta %.4f = %.4f, max ±%d",
			currentQty, delta, newTotal, c.maxLots)
		return result
	}

	result.Passed = true
	result.Reason = fmt.Sprintf("within limit: current %.4f + delta %.4f = %.4f, max ±%d",
		currentQty, delta, newTotal, c.maxLots)
	return result
}
