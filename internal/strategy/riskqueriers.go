package strategy

import (
	"context"

	"github.com/24alert/trading-bot/internal/marketdata"
	"github.com/24alert/trading-bot/internal/portfolio"
	"github.com/24alert/trading-bot/internal/risk/checker"
)

// TinvestPortfolioQuerier implements checker.PortfolioQuerier using portfolio.Service.
type TinvestPortfolioQuerier struct {
	Svc *portfolio.Service
}

func (p *TinvestPortfolioQuerier) GetPositions(ctx context.Context, accountID string) ([]checker.PositionInfo, error) {
	positions, err := p.Svc.GetPositions(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]checker.PositionInfo, 0, len(positions))
	for _, pos := range positions {
		out = append(out, checker.PositionInfo{
			InstrumentUID: pos.InstrumentUID,
			Quantity:      pos.Quantity,
		})
	}
	return out, nil
}

func (p *TinvestPortfolioQuerier) GetWithdrawLimits(ctx context.Context, accountID string) ([]checker.WithdrawLimitInfo, error) {
	limits, err := p.Svc.GetWithdrawLimits(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]checker.WithdrawLimitInfo, 0, len(limits))
	for _, lim := range limits {
		out = append(out, checker.WithdrawLimitInfo{
			Currency:       lim.Currency,
			WithdrawAmount: lim.WithdrawAmount,
		})
	}
	return out, nil
}

// TinvestMarketDataQuerier implements checker.MarketDataQuerier using marketdata.Service.
type TinvestMarketDataQuerier struct {
	Svc *marketdata.Service
}

func (m *TinvestMarketDataQuerier) GetTradingStatus(ctx context.Context, instrumentUID string) (*checker.TradingStatusInfo, error) {
	st, err := m.Svc.GetTradingStatus(ctx, instrumentUID)
	if err != nil {
		return nil, err
	}
	return &checker.TradingStatusInfo{
		InstrumentUID:        st.InstrumentUID,
		TradingStatus:        st.TradingStatus,
		APITradeAvailable:    st.APITradeAvailable,
		MarketOrderAvailable: st.MarketOrderAvailable,
		LimitOrderAvailable:  st.LimitOrderAvailable,
	}, nil
}
