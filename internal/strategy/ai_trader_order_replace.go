package strategy

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/24alert/trading-bot/pkg/metrics"
)

func (r *Runner) cancelReplaceRateOK(s *AITraderSession) bool {
	if s == nil {
		return false
	}
	limit := s.Limits.MaxCancelReplacePerMinute
	if limit <= 0 {
		limit = 10
	}
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	var kept []time.Time
	for _, t := range s.cancelReplaceTimestamps {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	s.cancelReplaceTimestamps = kept
	return len(kept) < limit
}

func (r *Runner) recordCancelReplace(s *AITraderSession) {
	if s == nil {
		return
	}
	s.cancelReplaceTimestamps = append(s.cancelReplaceTimestamps, time.Now())
}

func signalWantsReplace(sig *AITraderTradeSignal) bool {
	if sig == nil {
		return false
	}
	return strings.EqualFold(sig.OrderAction, "replace_limit")
}

func (r *Runner) replaceLiveOrderIfNeeded(ctx context.Context, s *AITraderSession, sig *AITraderTradeSignal) bool {
	if s == nil || s.LiveState == nil || sig == nil || !sig.actionableWith(effectivePolicy(s).EntryMinConfidence) {
		return false
	}
	if s.reconnectPaused {
		return false
	}
	side := normalizeTradeSide(sig.Side)
	if side == "" || side == "none" {
		return false
	}
	var target *LiveOrder
	for i := range s.LiveState.WorkingOrders {
		o := &s.LiveState.WorkingOrders[i]
		if o.Status != "working" || o.Side != side {
			continue
		}
		if math.Abs(o.Price-sig.LevelPrice) < tickEpsilon(sig.LevelPrice) {
			return false
		}
		target = o
		break
	}
	if target == nil && !signalWantsReplace(sig) {
		return false
	}
	if target == nil {
		return false
	}
	if !r.cancelReplaceRateOK(s) {
		return false
	}
	if r.orderSvc != nil && target.BrokerOrderID != "" {
		_, _ = r.orderSvc.CancelOrder(ctx, s.AccountID, target.BrokerOrderID)
		metrics.AITraderOrdersTotal.WithLabelValues("live", target.Side, "cancelled").Inc()
	}
	mode := "live"
	if s.isArmedLive() {
		mode = "live"
	}
	metrics.AITraderCancelReplaceTotal.WithLabelValues(mode, "replace").Inc()
	r.recordCancelReplace(s)
	remaining := make([]LiveOrder, 0, len(s.LiveState.WorkingOrders))
	for _, o := range s.LiveState.WorkingOrders {
		if o.ID != target.ID {
			remaining = append(remaining, o)
		}
	}
	s.LiveState.WorkingOrders = remaining
	r.placeLiveLimit(ctx, s, side, sig.LevelPrice, 1, "llm_signal", sig.Reason)
	return true
}

func (r *Runner) replacePaperOrderIfNeeded(s *AITraderSession, sig *AITraderTradeSignal) bool {
	if s == nil || s.PaperState == nil || sig == nil || !sig.actionableWith(effectivePolicy(s).EntryMinConfidence) {
		return false
	}
	if s.reconnectPaused {
		return false
	}
	side := normalizeTradeSide(sig.Side)
	if side == "" || side == "none" {
		return false
	}
	var target *PaperOrder
	for i := range s.PaperState.WorkingOrders {
		o := &s.PaperState.WorkingOrders[i]
		if o.Status != "working" || o.Side != side {
			continue
		}
		if math.Abs(o.Price-sig.LevelPrice) < tickEpsilon(sig.LevelPrice) {
			return false
		}
		target = o
		break
	}
	if target == nil && !signalWantsReplace(sig) {
		return false
	}
	if target == nil {
		return false
	}
	if !r.cancelReplaceRateOK(s) {
		return false
	}
	metrics.AITraderCancelReplaceTotal.WithLabelValues("paper", "replace").Inc()
	r.recordCancelReplace(s)
	remaining := make([]PaperOrder, 0, len(s.PaperState.WorkingOrders))
	for _, o := range s.PaperState.WorkingOrders {
		if o.ID != target.ID {
			remaining = append(remaining, o)
		}
	}
	s.PaperState.WorkingOrders = remaining
	r.placePaperOrder(s, side, sig.LevelPrice, 1, "llm_signal", sig.Reason)
	return true
}
