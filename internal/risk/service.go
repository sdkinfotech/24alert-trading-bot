package risk

import (
	"context"

	"github.com/24alert/trading-bot/internal/risk/checker"
	"github.com/24alert/trading-bot/pkg/config"
	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/metrics"
)

// Service orchestrates risk checks for inbound order intents.
type Service struct {
	sessionChecker  *checker.SessionChecker
	balanceChecker  *checker.BalanceChecker
	positionChecker *checker.PositionLimitChecker
	cb              *CircuitBreaker
	cfg             config.RiskConfig
	logger          *logging.Logger
}

func NewService(
	session *checker.SessionChecker,
	balance *checker.BalanceChecker,
	position *checker.PositionLimitChecker,
	cb *CircuitBreaker,
	cfg config.RiskConfig,
	logger *logging.Logger,
) *Service {
	return &Service{
		sessionChecker:  session,
		balanceChecker:  balance,
		positionChecker: position,
		cb:              cb,
		cfg:             cfg,
		logger:          logger,
	}
}

// ValidateOrderIntent runs all configured risk checks and returns an
// aggregated response.
func (s *Service) ValidateOrderIntent(ctx context.Context, intent OrderIntent) (*RiskResponse, error) {
	l := s.logger.WithContext(ctx)
	l.Info("ValidateOrderIntent",
		"account_id", intent.AccountID,
		"instrument_uid", intent.InstrumentUID,
		"direction", intent.Direction,
		"quantity", intent.Quantity,
	)

	resp := &RiskResponse{Allowed: true}

	if s.cb.IsTripped() {
		metrics.CircuitBreakerState.Set(1)
		resp.Allowed = false
		resp.Checks = append(resp.Checks, RiskCheckResult{
			Name:   "circuit_breaker",
			Passed: false,
			Reason: "circuit breaker is tripped — rejecting all orders",
		})
		metrics.RiskChecksTotal.WithLabelValues("tripped").Inc()
		l.Warn("order rejected: circuit breaker tripped")
		return resp, nil
	}
	metrics.CircuitBreakerState.Set(0)

	if s.cfg.CheckTradingSession {
		r := s.sessionChecker.Check(ctx, intent.InstrumentUID)
		resp.Checks = append(resp.Checks, toRiskCheckResult(r))
		if !r.Passed {
			resp.Allowed = false
		}
	}

	r := s.balanceChecker.Check(ctx, intent.AccountID, intent.EstimatedCost)
	resp.Checks = append(resp.Checks, toRiskCheckResult(r))
	if !r.Passed {
		resp.Allowed = false
	}

	r = s.positionChecker.Check(ctx, intent.AccountID, intent.InstrumentUID, intent.Direction, intent.Quantity)
	resp.Checks = append(resp.Checks, toRiskCheckResult(r))
	if !r.Passed {
		resp.Allowed = false
	}

	if resp.Allowed {
		s.cb.RecordSuccess()
		metrics.RiskChecksTotal.WithLabelValues("approved").Inc()
		l.Info("order intent approved")
	} else {
		s.cb.RecordFailure()
		metrics.RiskChecksTotal.WithLabelValues("rejected").Inc()
		l.Warn("order intent rejected", "checks", len(resp.Checks))
	}

	return resp, nil
}

// GetRiskStatus returns the current health/state of the risk subsystem.
func (s *Service) GetRiskStatus(_ context.Context) *RiskStatus {
	st := s.cb.State()
	return &RiskStatus{
		CircuitBreakerTripped: st.Tripped,
		FailureCount:          st.FailureCount,
		LastFailure:           st.LastFailure,
		Threshold:             st.Threshold,
		Cooldown:              st.Cooldown,
	}
}

// ResetCircuitBreaker allows manual intervention to re-open the breaker.
func (s *Service) ResetCircuitBreaker(_ context.Context) error {
	s.cb.Reset()
	metrics.CircuitBreakerState.Set(0)
	s.logger.Info("circuit breaker manually reset")
	return nil
}

func toRiskCheckResult(r *checker.RiskCheckResult) RiskCheckResult {
	return RiskCheckResult{
		Name:   r.Name,
		Passed: r.Passed,
		Reason: r.Reason,
	}
}
