package adapter

import (
	"context"

	"github.com/24alert/trading-bot/internal/risk/checker"
)

// StubPortfolioQuerier satisfies checker.PortfolioQuerier with passthrough
// stubs so the risk checkers always pass. Replace with real adapters when
// the gateway needs live risk validation.
type StubPortfolioQuerier struct{}

func (StubPortfolioQuerier) GetPositions(_ context.Context, _ string) ([]checker.PositionInfo, error) {
	return nil, nil
}

func (StubPortfolioQuerier) GetWithdrawLimits(_ context.Context, _ string) ([]checker.WithdrawLimitInfo, error) {
	return []checker.WithdrawLimitInfo{
		{Currency: "rub", WithdrawAmount: 1e12},
	}, nil
}

// StubMarketDataQuerier satisfies checker.MarketDataQuerier with passthrough
// stubs so the risk checkers always pass.
type StubMarketDataQuerier struct{}

func (StubMarketDataQuerier) GetTradingStatus(_ context.Context, instrumentUID string) (*checker.TradingStatusInfo, error) {
	return &checker.TradingStatusInfo{
		InstrumentUID:        instrumentUID,
		TradingStatus:        "NORMAL_TRADING",
		APITradeAvailable:    true,
		MarketOrderAvailable: true,
		LimitOrderAvailable:  true,
	}, nil
}
