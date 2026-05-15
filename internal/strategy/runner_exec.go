package strategy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/24alert/trading-bot/internal/journal"
	"github.com/24alert/trading-bot/internal/order"
	"github.com/24alert/trading-bot/internal/strategy/pnl"
	"github.com/24alert/trading-bot/pkg/metrics"
)

func (r *Runner) dispatchExecution(evt order.OrderStateEvent) {
	r.mu.Lock()
	iid := r.orderOwners[evt.OrderID]
	rt := r.instances[iid]
	r.mu.Unlock()
	if rt == nil {
		return
	}

	rec, err := r.orderRepo.GetOrder(evt.OrderID)
	instrument := ""
	dir := ""
	if err == nil {
		instrument = rec.InstrumentUID
		dir = rec.Direction
	}

	avgPrice, _ := r.orderRepo.WeightedAvgExecutionPrice(evt.OrderID)
	if (evt.Status == order.OrderStatusPartiallyFilled || evt.Status == order.OrderStatusFilled) &&
		evt.FilledQty > 0 && avgPrice == 0 {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && avgPrice == 0 {
			time.Sleep(25 * time.Millisecond)
			avgPrice, _ = r.orderRepo.WeightedAvgExecutionPrice(evt.OrderID)
		}
	}

	msg := fmt.Sprintf("order_state=%s cumulative_filled_lots=%d", evt.Status, evt.FilledQty)

	lot := int32(1)
	if inf, ok := r.instrCache.GetInstrument(instrument); ok && inf.LotSize > 0 {
		lot = inf.LotSize
	}

	r.mu.Lock()
	prev := r.lastFilled[evt.OrderID]
	deltaLots := evt.FilledQty - prev
	if deltaLots < 0 {
		deltaLots = 0
	}
	if deltaLots > 0 {
		r.lastFilled[evt.OrderID] = evt.FilledQty
	}
	r.mu.Unlock()

	shares := float64(deltaLots) * float64(lot)

	execEvt := ExecutionEvent{
		OrderID:       evt.OrderID,
		InstrumentUID: instrument,
		Status:        strings.ToLower(string(evt.Status)),
		FilledQty:     evt.FilledQty,
		AvgPrice:      avgPrice,
		Message:       msg,
	}

	if deltaLots > 0 && avgPrice > 0 && instrument != "" {
		instL := r.ledger.Ledger(iid)
		realizedDelta := instL.ApplyFill(instrument, dir, shares, avgPrice)
		if realizedDelta > 0 {
			metrics.StrategyTradesTotal.WithLabelValues(iid, "win").Inc()
			r.mu.Lock()
			r.fillWins[iid]++
			r.mu.Unlock()
		} else if realizedDelta < 0 {
			metrics.StrategyTradesTotal.WithLabelValues(iid, "loss").Inc()
			r.mu.Lock()
			r.fillLosses[iid]++
			r.mu.Unlock()
		}
		if bps := r.slippageBPS(evt.OrderID, avgPrice); bps != 0 {
			metrics.StrategySlippageBps.WithLabelValues(iid).Observe(bps)
		}
	}

	_ = r.journal.RecordExecution(context.Background(), journal.ExecutionRecord{
		InstanceID:    iid,
		OrderID:       evt.OrderID,
		InstrumentUID: instrument,
		Status:        execEvt.Status,
		FilledQty:     evt.FilledQty,
		AvgPrice:      avgPrice,
		Message:       msg,
	})

	rt.strat.OnExecution(execEvt)
	r.updateBizMetrics(iid)

	if evt.Status == order.OrderStatusFilled || evt.Status == order.OrderStatusCancelled ||
		evt.Status == order.OrderStatusRejected || evt.Status == order.OrderStatusReplaced {
		r.mu.Lock()
		delete(r.orderOwners, evt.OrderID)
		delete(r.signalRefPx, evt.OrderID)
		delete(r.lastFilled, evt.OrderID)
		r.mu.Unlock()
	}
}

func (r *Runner) slippageBPS(orderID string, avgPrice float64) float64 {
	r.mu.Lock()
	ref, ok := r.signalRefPx[orderID]
	r.mu.Unlock()
	if !ok || ref <= 0 || avgPrice <= 0 {
		return 0
	}
	return (avgPrice - ref) / ref * 10000
}

func (r *Runner) updateBizMetrics(instanceID string) {
	led := r.ledger.Ledger(instanceID)
	qty, _, realized := led.Snapshot()
	marks := make(map[string]float64)
	for uid := range qty {
		if p, ok := r.priceCache.GetLastPrice(uid); ok {
			marks[uid] = p.Price
		}
	}
	unreal := pnl.UnrealizedRUB(led, marks)
	total := realized + unreal

	metrics.StrategyRealizedPnLRub.WithLabelValues(instanceID).Set(realized)
	metrics.StrategyUnrealizedPnLRub.WithLabelValues(instanceID).Set(unreal)
	metrics.StrategyTotalPnLRub.WithLabelValues(instanceID).Set(total)

	r.mu.Lock()
	peak := r.equityPeak[instanceID]
	if total > peak {
		peak = total
		r.equityPeak[instanceID] = peak
	}
	w := r.fillWins[instanceID]
	l := r.fillLosses[instanceID]
	r.mu.Unlock()

	var winRate float64
	if w+l > 0 {
		winRate = float64(w) / float64(w+l)
	}
	metrics.StrategyWinRate.WithLabelValues(instanceID).Set(winRate)

	var dd float64
	if peak > 0 && total < peak {
		dd = 100 * (peak - total) / peak
	}
	metrics.StrategyDrawdownPercent.WithLabelValues(instanceID).Set(dd)

	// Reset gauges for instruments no longer in book.
	for _, uid := range r.instrumentsInstrumentKeys(instanceID) {
		if _, ok := qty[uid]; !ok {
			metrics.StrategyPositionQty.WithLabelValues(instanceID, uid).Set(0)
		}
	}
	for uid, q := range qty {
		metrics.StrategyPositionQty.WithLabelValues(instanceID, uid).Set(q)
	}
}

func (r *Runner) instrumentsInstrumentKeys(instanceID string) []string {
	inst, ok := r.byID[instanceID]
	if !ok {
		return nil
	}
	return append([]string(nil), inst.Instruments...)
}

// ReconcileInstance compares ledger vs broker positions for one instance.
func (r *Runner) ReconcileInstance(ctx context.Context, instanceID string) error {
	if r.portfolioSvc == nil {
		return nil
	}
	inst, ok := r.byID[instanceID]
	if !ok {
		return fmt.Errorf("unknown instance %q", instanceID)
	}
	positions, err := r.portfolioSvc.GetPositions(ctx, inst.AccountID)
	if err != nil {
		return err
	}
	led := r.ledger.Ledger(instanceID)
	const tol = 1e-3
	allowed := make(map[string]struct{}, len(inst.Instruments))
	for _, u := range inst.Instruments {
		allowed[u] = struct{}{}
	}
	for _, p := range positions {
		if p.InstrumentUID == "" {
			continue
		}
		if _, ok := allowed[p.InstrumentUID]; !ok {
			continue
		}
		if led.ReconcileFromBroker(p.InstrumentUID, p.Quantity, p.AveragePrice, tol) {
			metrics.StrategyReconcileMismatch.WithLabelValues(instanceID).Inc()
			r.logger.Warn("ledger reconciled from broker (drift)", "instance", instanceID, "instrument", p.InstrumentUID)
			if r.tg != nil {
				_ = r.tg.SendMessage(ctx, fmt.Sprintf("ledger drift reconciled: instance=%s instrument=%s broker_qty=%.4f",
					instanceID, p.InstrumentUID, p.Quantity))
			}
		}
	}
	r.updateBizMetrics(instanceID)
	return nil
}

func (r *Runner) equityTotal(instanceID string) float64 {
	led := r.ledger.Ledger(instanceID)
	qty, _, realized := led.Snapshot()
	marks := make(map[string]float64)
	for uid := range qty {
		if px, ok := r.priceCache.GetLastPrice(uid); ok {
			marks[uid] = px.Price
		}
	}
	return realized + pnl.UnrealizedRUB(led, marks)
}

// maybePauseOnLoss stops the instance if drawdown / daily loss thresholds hit.
func (r *Runner) maybePauseOnLoss(ctx context.Context, instanceID string) {
	wd := r.strategiesCfg.Watchdog
	if !wd.Enabled {
		return
	}
	total := r.equityTotal(instanceID)
	if wd.MaxDailyLossRub > 0 && total < -wd.MaxDailyLossRub {
		r.logger.Warn("watchdog: max loss exceeded, stopping instance", "instance", instanceID, "total_pnl", total)
		if r.tg != nil {
			_ = r.tg.SendMessage(ctx, fmt.Sprintf("strategy-runner: instance %s stopped (total PnL %.2f < -%.2f RUB)",
				instanceID, total, wd.MaxDailyLossRub))
		}
		r.StopInstance(instanceID)
		return
	}
	if wd.MaxDrawdownPercent > 0 && wd.PauseOnDrawdown {
		r.mu.Lock()
		peak := r.equityPeak[instanceID]
		r.mu.Unlock()
		if peak > 0 {
			dd := 100 * (peak - total) / peak
			if dd > wd.MaxDrawdownPercent {
				r.logger.Warn("watchdog: drawdown exceeded, stopping instance", "instance", instanceID, "dd_pct", dd)
				if r.tg != nil {
					_ = r.tg.SendMessage(ctx, fmt.Sprintf("strategy-runner: instance %s stopped (drawdown %.1f%% > %.1f%%)",
						instanceID, dd, wd.MaxDrawdownPercent))
				}
				r.StopInstance(instanceID)
			}
		}
	}
}

// StuckOrderMinutesOrDefault returns configured stuck-order window.
func (r *Runner) StuckOrderMinutesOrDefault() int {
	m := r.strategiesCfg.Watchdog.StuckOrderMinutes
	if m <= 0 {
		return 30
	}
	return m
}

// CheckStuckOrders logs and notifies on orders pending longer than configured minutes.
func (r *Runner) CheckStuckOrders(ctx context.Context, accountID string) {
	min := r.StuckOrderMinutesOrDefault()
	limit := time.Now().Add(-time.Duration(min) * time.Minute)
	for _, o := range r.orderRepo.GetActiveOrders(accountID) {
		if o.UpdatedAt.Before(limit) && o.CreatedAt.Before(limit) {
			r.logger.Warn("watchdog: stuck order", "account", accountID, "order_id", o.OrderID, "status", o.Status)
			if r.tg != nil {
				_ = r.tg.SendMessage(ctx, fmt.Sprintf("stuck order: account=%s order=%s status=%s age>%dm",
					accountID, o.OrderID, o.Status, min))
			}
		}
	}
}
