package adapter

import (
	"context"
	"time"

	"github.com/24alert/trading-bot/internal/gateway/handlers"
	"github.com/24alert/trading-bot/internal/portfolio"
)

// PortfolioAdapter implements handlers.PortfolioService by wrapping *portfolio.Service.
type PortfolioAdapter struct {
	svc *portfolio.Service
}

func NewPortfolioAdapter(svc *portfolio.Service) *PortfolioAdapter {
	return &PortfolioAdapter{svc: svc}
}

func (a *PortfolioAdapter) GetPositions(ctx context.Context, accountID string) ([]handlers.Position, error) {
	positions, err := a.svc.GetPositions(ctx, accountID)
	if err != nil {
		return nil, err
	}

	out := make([]handlers.Position, 0, len(positions))
	for _, p := range positions {
		out = append(out, handlers.Position{
			InstrumentUID:  p.InstrumentUID,
			InstrumentType: p.InstrumentType,
			FIGI:           p.FIGI,
			Quantity:       p.Quantity,
			AveragePrice:   p.AveragePrice,
			ExpectedYield:  p.ExpectedYield,
			CurrentPrice:   p.CurrentPrice,
			Currency:       p.Currency,
			Blocked:        p.Blocked,
		})
	}
	return out, nil
}

func (a *PortfolioAdapter) GetPortfolio(ctx context.Context, accountID string) (*handlers.PortfolioInfo, error) {
	info, err := a.svc.GetPortfolio(ctx, accountID)
	if err != nil {
		return nil, err
	}

	positions := make([]handlers.Position, 0, len(info.Positions))
	for _, p := range info.Positions {
		positions = append(positions, handlers.Position{
			InstrumentUID:  p.InstrumentUID,
			InstrumentType: p.InstrumentType,
			FIGI:           p.FIGI,
			Quantity:       p.Quantity,
			AveragePrice:   p.AveragePrice,
			ExpectedYield:  p.ExpectedYield,
			CurrentPrice:   p.CurrentPrice,
			Currency:       p.Currency,
			Blocked:        p.Blocked,
		})
	}

	return &handlers.PortfolioInfo{
		AccountID:             info.AccountID,
		TotalAmountShares:     info.TotalAmountShares,
		TotalAmountBonds:      info.TotalAmountBonds,
		TotalAmountETF:        info.TotalAmountETF,
		TotalAmountCurrencies: info.TotalAmountCurrencies,
		TotalAmountFutures:    info.TotalAmountFutures,
		ExpectedYield:         info.ExpectedYield,
		Positions:             positions,
	}, nil
}

func (a *PortfolioAdapter) GetWithdrawLimits(ctx context.Context, accountID string) ([]handlers.WithdrawLimit, error) {
	limits, err := a.svc.GetWithdrawLimits(ctx, accountID)
	if err != nil {
		return nil, err
	}

	out := make([]handlers.WithdrawLimit, 0, len(limits))
	for _, l := range limits {
		out = append(out, handlers.WithdrawLimit{
			Currency:       l.Currency,
			BlockedAmount:  l.BlockedAmount,
			WithdrawAmount: l.WithdrawAmount,
		})
	}
	return out, nil
}

func (a *PortfolioAdapter) GetOperations(ctx context.Context, accountID, instrumentUID string, from, to time.Time) (*handlers.OperationsPage, error) {
	page, err := a.svc.GetOperations(ctx, accountID, instrumentUID, from, to, "", 100)
	if err != nil {
		return nil, err
	}

	ops := make([]handlers.Operation, 0, len(page.Operations))
	for _, op := range page.Operations {
		trades := make([]handlers.OperationTrade, 0, len(op.Trades))
		for _, t := range op.Trades {
			trades = append(trades, handlers.OperationTrade{
				TradeID:  t.TradeID,
				Price:    t.Price,
				Quantity: t.Quantity,
				Date:     t.Date,
			})
		}
		ops = append(ops, handlers.Operation{
			ID:            op.ID,
			AccountID:     op.AccountID,
			InstrumentUID: op.InstrumentUID,
			Type:          op.Type,
			State:         op.State,
			Payment:       op.Payment,
			Currency:      op.Currency,
			Quantity:      op.Quantity,
			Date:          op.Date,
			Trades:        trades,
		})
	}

	return &handlers.OperationsPage{
		Operations: ops,
		NextCursor: page.NextCursor,
		HasNext:    page.HasNext,
	}, nil
}

var _ handlers.PortfolioService = (*PortfolioAdapter)(nil)
