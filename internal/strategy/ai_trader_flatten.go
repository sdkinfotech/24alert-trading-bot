package strategy

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (r *Runner) featuresForAITraderFlatten(ctx context.Context, s *AITraderSession) (*AITraderFeatures, error) {
	r.aiTrader.mu.Lock()
	if s.Features != nil && s.Features.Mid > 0 && !s.Features.Stale {
		f := s.Features
		r.aiTrader.mu.Unlock()
		return f, nil
	}
	depth := int32(50)
	if s.Features != nil && s.Features.Depth > 0 {
		depth = s.Features.Depth
	}
	r.aiTrader.mu.Unlock()
	if r.mdSvc == nil {
		return nil, fmt.Errorf("market data service is not configured")
	}
	book, err := r.mdSvc.GetOrderbook(ctx, s.InstrumentID, depth)
	if err != nil {
		return nil, err
	}
	if sb := r.preferStreamBook(s); sb != nil {
		book = sb
	}
	return computeAITraderFeatures(book, s.Ticker, s.Limits.StaleDataMS), nil
}

// FlattenAITraderSession closes live/paper position and cancels working orders.
func (r *Runner) FlattenAITraderSession(instanceID string) (*AITraderSession, error) {
	instanceID = strings.TrimSpace(instanceID)
	r.aiTrader.mu.Lock()
	s := r.aiTrader.findLocked(instanceID)
	if s == nil {
		r.aiTrader.mu.Unlock()
		return nil, fmt.Errorf("ai trader session not found")
	}
	if s.Status != "running" || s.Phase != AITraderPhaseTrading {
		r.aiTrader.mu.Unlock()
		return nil, fmt.Errorf("session must be in trading phase")
	}
	r.aiTrader.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	f, err := r.featuresForAITraderFlatten(ctx, s)
	if err != nil {
		return nil, err
	}
	if s.isArmedLive() {
		r.cancelAllLiveOrders(ctx, s)
		if s.LiveState != nil && s.LiveState.PositionLots != 0 {
			r.closeLivePosition(ctx, s, f, "manual_flatten")
		}
		r.syncAITraderLiveFromBroker(ctx, s)
	} else if s.PaperState != nil && s.PaperState.PositionLots != 0 {
		r.closePaperPosition(s, f, "manual_flatten")
	}
	ev := AITraderDecisionEvent{
		Time:           time.Now().UTC().Format(time.RFC3339),
		SessionID:      s.ID,
		Mode:           s.StrategyKind,
		Action:         "flatten",
		Intent:         "manual",
		Summary:        "Ручное закрытие позиции оператором",
		RiskResult:     "manual_flatten",
		AnalysisSource: "session",
	}
	r.appendAITraderEvent(ev)
	r.aiTrader.mu.Lock()
	if cur := r.aiTrader.findLocked(instanceID); cur != nil {
		cur.Events = append([]AITraderDecisionEvent{ev}, cur.Events...)
		if len(cur.Events) > aiTraderMaxSessionEvents {
			cur.Events = cur.Events[:aiTraderMaxSessionEvents]
		}
		s = cloneAITraderSession(cur)
	}
	r.aiTrader.mu.Unlock()
	return s, nil
}
