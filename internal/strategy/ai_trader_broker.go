package strategy

import (
	"context"
	"strings"
	"time"
)

// AITraderBrokerSnapshot is broker truth for the session instrument only.
type AITraderBrokerSnapshot struct {
	SessionID      string  `json:"session_id"`
	AccountID      string  `json:"account_id"`
	InstrumentUID  string  `json:"instrument_uid"`
	Ticker         string  `json:"ticker,omitempty"`
	Quantity       float64 `json:"quantity"`
	AveragePrice   float64 `json:"average_price"`
	CurrentPrice   float64 `json:"current_price"`
	ExpectedYield  float64 `json:"expected_yield"`
	Currency       string  `json:"currency,omitempty"`
	LastBrokerSync string  `json:"last_broker_sync"`
	PortfolioError string  `json:"portfolio_error,omitempty"`
}

func (r *Runner) AITraderBrokerSnapshot(ctx context.Context, sessionID string) (AITraderBrokerSnapshot, bool, error) {
	var out AITraderBrokerSnapshot
	sessionID = strings.TrimSpace(sessionID)
	s, ok := r.AITraderSession(sessionID)
	if !ok {
		return out, false, nil
	}
	out.SessionID = s.ID
	out.AccountID = s.AccountID
	out.InstrumentUID = s.InstrumentID
	out.Ticker = s.Ticker
	out.LastBrokerSync = time.Now().UTC().Format(time.RFC3339)
	if r.portfolioSvc == nil {
		out.PortfolioError = "portfolio service not configured"
		return out, true, nil
	}
	positions, err := r.portfolioSvc.GetPositions(ctx, s.AccountID)
	if err != nil {
		out.PortfolioError = err.Error()
		return out, true, nil
	}
	uid := strings.TrimSpace(s.InstrumentID)
	for _, p := range positions {
		if p.InstrumentUID != uid {
			continue
		}
		out.Quantity = p.Quantity
		out.AveragePrice = p.AveragePrice
		out.CurrentPrice = p.CurrentPrice
		out.ExpectedYield = p.ExpectedYield
		out.Currency = p.Currency
		if out.Ticker == "" {
			if inf, ok := r.instrCache.GetInstrument(uid); ok {
				out.Ticker = inf.Ticker
			}
		}
		return out, true, nil
	}
	return out, true, nil
}

func (r *Runner) syncAITraderLiveFromBroker(ctx context.Context, s *AITraderSession) {
	if s == nil || !s.isArmedLive() || s.LiveState == nil {
		return
	}
	snap, ok, err := r.AITraderBrokerSnapshot(ctx, s.ID)
	if err != nil || !ok || snap.PortfolioError != "" {
		return
	}
	qty := int64(snap.Quantity + 0.5)
	if snap.Quantity < 0 {
		qty = int64(snap.Quantity - 0.5)
	}
	if qty == 0 && s.LiveState.PositionLots != 0 {
		s.LiveState.PositionLots = 0
		s.LiveState.AvgPrice = 0
		s.LiveState.StopLoss = 0
		s.LiveState.TakeProfit = 0
		s.LiveState.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		return
	}
	if qty != 0 && s.LiveState.PositionLots == 0 {
		s.LiveState.PositionLots = qty
		if snap.AveragePrice > 0 {
			s.LiveState.AvgPrice = snap.AveragePrice
		}
		s.LiveState.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
}
