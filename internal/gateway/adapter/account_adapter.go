package adapter

import (
	"context"

	"github.com/24alert/trading-bot/internal/gateway/handlers"
	"github.com/24alert/trading-bot/internal/portfolio"
)

// AccountAdapter implements handlers.AccountService by wrapping *portfolio.Service.
type AccountAdapter struct {
	svc *portfolio.Service
}

func NewAccountAdapter(svc *portfolio.Service) *AccountAdapter {
	return &AccountAdapter{svc: svc}
}

func (a *AccountAdapter) GetAccounts(ctx context.Context) ([]handlers.Account, error) {
	accounts, err := a.svc.GetAccounts(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]handlers.Account, 0, len(accounts))
	for _, acc := range accounts {
		out = append(out, handlers.Account{
			ID:          acc.ID,
			Type:        acc.Type,
			Name:        acc.Name,
			Status:      acc.Status,
			OpenedDate:  acc.OpenedDate,
			ClosedDate:  acc.ClosedDate,
			AccessLevel: acc.AccessLevel,
		})
	}
	return out, nil
}

func (a *AccountAdapter) GetMarginAttributes(ctx context.Context, accountID string) (*handlers.MarginInfo, error) {
	info, err := a.svc.GetMarginAttributes(ctx, accountID)
	if err != nil {
		return nil, err
	}

	return &handlers.MarginInfo{
		LiquidPortfolio:       info.LiquidPortfolio,
		StartingMargin:        info.StartingMargin,
		MinimalMargin:         info.MinimalMargin,
		FundsSufficiencyLevel: info.FundsSufficiencyLevel,
		AmountOfMissing:       info.AmountOfMissing,
		CorrectedMargin:       info.CorrectedMargin,
	}, nil
}

var _ handlers.AccountService = (*AccountAdapter)(nil)
