package checker

import "context"

// PortfolioQuerier abstracts the portfolio-svc gRPC calls needed by
// risk checkers.  A real implementation will wrap the generated gRPC
// client; a stub is used until proto is ready.
type PortfolioQuerier interface {
	GetPositions(ctx context.Context, accountID string) ([]PositionInfo, error)
	GetWithdrawLimits(ctx context.Context, accountID string) ([]WithdrawLimitInfo, error)
}

// MarketDataQuerier abstracts the marketdata-svc gRPC calls needed by
// risk checkers.
type MarketDataQuerier interface {
	GetTradingStatus(ctx context.Context, instrumentUID string) (*TradingStatusInfo, error)
}

// PositionInfo is a lightweight projection of a portfolio position.
type PositionInfo struct {
	InstrumentUID string
	Quantity      float64
}

// WithdrawLimitInfo is a lightweight projection of a withdraw limit.
type WithdrawLimitInfo struct {
	Currency       string
	WithdrawAmount float64
}

// TradingStatusInfo is a lightweight projection of a trading status.
type TradingStatusInfo struct {
	InstrumentUID        string
	TradingStatus        string
	APITradeAvailable    bool
	MarketOrderAvailable bool
	LimitOrderAvailable  bool
}

// RiskCheckResult mirrors the parent risk package type so the checker
// sub-package stays independent.
type RiskCheckResult struct {
	Name   string
	Passed bool
	Reason string
}
