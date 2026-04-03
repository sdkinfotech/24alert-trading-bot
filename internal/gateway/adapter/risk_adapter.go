package adapter

import (
	"context"

	"github.com/24alert/trading-bot/internal/gateway/handlers"
	"github.com/24alert/trading-bot/internal/risk"
)

// RiskAdapter implements handlers.RiskService by wrapping *risk.Service.
type RiskAdapter struct {
	svc *risk.Service
}

func NewRiskAdapter(svc *risk.Service) *RiskAdapter {
	return &RiskAdapter{svc: svc}
}

func (a *RiskAdapter) GetRiskStatus(ctx context.Context) (*handlers.RiskStatus, error) {
	st := a.svc.GetRiskStatus(ctx)

	var checks []handlers.RiskCheckResult
	return &handlers.RiskStatus{
		CircuitBreakerTripped: st.CircuitBreakerTripped,
		FailureCount:          st.FailureCount,
		LastFailure:           st.LastFailure,
		Threshold:             st.Threshold,
		Cooldown:              st.Cooldown.String(),
		Checks:                checks,
	}, nil
}

func (a *RiskAdapter) ResetCircuitBreaker(ctx context.Context) error {
	return a.svc.ResetCircuitBreaker(ctx)
}

var _ handlers.RiskService = (*RiskAdapter)(nil)
