package strategy

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/24alert/trading-bot/internal/journal"
	"github.com/24alert/trading-bot/internal/order"
	"github.com/24alert/trading-bot/internal/strategy/pnl"
	"github.com/24alert/trading-bot/pkg/metrics"
	"github.com/russianinvestments/invest-api-go-sdk/investgo"
	pb "github.com/russianinvestments/invest-api-go-sdk/proto"
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
		qty, avg, _ := instL.Snapshot()
		if q := qty[instrument]; q != 0 {
			r.ensureProtectiveStop(context.Background(), iid, rt.account, instrument, q, avg[instrument], avgPrice)
		} else {
			r.cancelTrackedProtectiveStop(context.Background(), iid, rt.account, instrument, "position closed")
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

	rt.strat.OnExecution(execEvt) //nolint:misspell // strat is short for strategy
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

func (r *Runner) pollOrderStateAfterSubmit(ctx context.Context, rt *instanceRuntime, orderID string) {
	if r.orderSvc == nil || rt == nil || orderID == "" {
		return
	}
	delays := []time.Duration{500 * time.Millisecond, 2 * time.Second, 5 * time.Second}
	for _, delay := range delays {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		resp, err := r.orderSvc.GetOrderState(ctx, rt.account, orderID, pb.PriceType_PRICE_TYPE_UNSPECIFIED)
		if err != nil {
			r.logger.Warn("poll order state after submit failed", "instance", rt.id, "order_id", orderID, "error", err)
			continue
		}
		status := order.MapExecutionStatus(resp.GetExecutionReportStatus())
		r.dispatchExecution(order.OrderStateEvent{
			OrderID:   orderID,
			AccountID: rt.account,
			Status:    status,
			FilledQty: resp.GetLotsExecuted(),
			UpdatedAt: time.Now().UTC(),
		})
		if isTerminalOrderStatus(status) {
			return
		}
	}
}

func isTerminalOrderStatus(status order.OrderStatus) bool {
	return status == order.OrderStatusFilled ||
		status == order.OrderStatusCancelled ||
		status == order.OrderStatusRejected ||
		status == order.OrderStatusReplaced
}

func stopKey(instanceID, instrumentUID string) string {
	return instanceID + "|" + instrumentUID
}

func (r *Runner) protectiveStopPct(instanceID string) float64 {
	inst, ok := r.byID[instanceID]
	if !ok {
		return 0
	}
	for _, key := range []string{"hard_stop_pct", "broker_stop_pct", "trailing_stop_pct"} {
		raw := strings.TrimSpace(inst.Params[key])
		if raw == "" {
			continue
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err == nil && v > 0 && v < 0.5 {
			return v
		}
	}
	return 0
}

func (r *Runner) ensureProtectiveStop(ctx context.Context, instanceID, accountID, instrumentUID string, quantity, avgPrice, fallbackPrice float64) {
	if r.orderSvc == nil || accountID == "" || instrumentUID == "" || quantity == 0 {
		return
	}
	pct := r.protectiveStopPct(instanceID)
	if pct <= 0 {
		r.logger.Warn("protective stop skipped: no stop pct configured", "instance", instanceID, "instrument", instrumentUID)
		return
	}
	base := avgPrice
	if base <= 0 {
		base = fallbackPrice
	}
	if base <= 0 {
		if px, ok := r.priceCache.GetLastPrice(instrumentUID); ok {
			base = px.Price
		}
	}
	if base <= 0 {
		r.logger.Warn("protective stop skipped: no base price", "instance", instanceID, "instrument", instrumentUID)
		return
	}

	dir := pb.StopOrderDirection_STOP_ORDER_DIRECTION_SELL
	stopPrice := base * (1 - pct)
	if quantity < 0 {
		dir = pb.StopOrderDirection_STOP_ORDER_DIRECTION_BUY
		stopPrice = base * (1 + pct)
	}
	lots := int64(math.Ceil(math.Abs(quantity)))
	if lots <= 0 {
		return
	}

	if r.hasBrokerProtectiveStop(ctx, accountID, instrumentUID, dir) {
		return
	}

	req := &investgo.PostStopOrderRequest{
		InstrumentId:      instrumentUID,
		Quantity:          lots,
		Direction:         dir,
		AccountId:         accountID,
		StopOrderType:     pb.StopOrderType_STOP_ORDER_TYPE_STOP_LOSS,
		ExpirationType:    pb.StopOrderExpirationType_STOP_ORDER_EXPIRATION_TYPE_GOOD_TILL_CANCEL,
		ExchangeOrderType: pb.ExchangeOrderType_EXCHANGE_ORDER_TYPE_MARKET,
		StopPrice:         floatToQuotation(stopPrice),
	}
	resp, err := r.orderSvc.PostStopOrder(ctx, req)
	if err != nil {
		r.logger.Error("protective broker stop failed", "instance", instanceID, "instrument", instrumentUID, "error", err)
		_ = r.journal.RecordEvent(ctx, journal.EventRecord{
			Type:          "protective_stop",
			InstanceID:    instanceID,
			InstrumentUID: instrumentUID,
			Direction:     stopDirectionLabel(dir),
			Quantity:      lots,
			OrderType:     "stop_loss",
			RefPrice:      stopPrice,
			Status:        "post_error",
			Message:       err.Error(),
			CreatedAt:     time.Now().UTC(),
		})
		return
	}
	stopID := resp.GetStopOrderId()
	r.mu.Lock()
	r.stopOrders[stopKey(instanceID, instrumentUID)] = stopID
	r.mu.Unlock()
	r.logger.Warn("protective broker stop submitted", "instance", instanceID, "instrument", instrumentUID, "stop_order_id", stopID, "stop_price", stopPrice)
	_ = r.journal.RecordEvent(ctx, journal.EventRecord{
		Type:          "protective_stop",
		InstanceID:    instanceID,
		InstrumentUID: instrumentUID,
		Direction:     stopDirectionLabel(dir),
		Quantity:      lots,
		OrderType:     "stop_loss",
		RefPrice:      stopPrice,
		Status:        "submitted",
		Message:       "broker-side protective stop submitted",
		CreatedAt:     time.Now().UTC(),
	})
}

func (r *Runner) hasBrokerProtectiveStop(ctx context.Context, accountID, instrumentUID string, dir pb.StopOrderDirection) bool {
	resp, err := r.orderSvc.GetStopOrders(ctx, accountID)
	if err != nil {
		r.logger.Warn("protective stop coverage check failed", "account", accountID, "instrument", instrumentUID, "error", err)
		return false
	}
	for _, so := range resp.GetStopOrders() {
		if so.GetInstrumentUid() == instrumentUID &&
			so.GetDirection() == dir &&
			so.GetOrderType() == pb.StopOrderType_STOP_ORDER_TYPE_STOP_LOSS {
			return true
		}
	}
	return false
}

func (r *Runner) cancelTrackedProtectiveStop(ctx context.Context, instanceID, accountID, instrumentUID, reason string) {
	if r.orderSvc == nil || accountID == "" || instrumentUID == "" {
		return
	}
	key := stopKey(instanceID, instrumentUID)
	r.mu.Lock()
	stopID := r.stopOrders[key]
	delete(r.stopOrders, key)
	r.mu.Unlock()
	if stopID == "" {
		return
	}
	if _, err := r.orderSvc.CancelStopOrder(ctx, accountID, stopID); err != nil {
		r.logger.Warn("cancel protective stop failed", "instance", instanceID, "instrument", instrumentUID, "stop_order_id", stopID, "error", err)
		return
	}
	_ = r.journal.RecordEvent(ctx, journal.EventRecord{
		Type:          "protective_stop",
		InstanceID:    instanceID,
		InstrumentUID: instrumentUID,
		OrderType:     "stop_loss",
		Status:        "cancelled",
		Message:       reason,
		CreatedAt:     time.Now().UTC(),
	})
}

func stopDirectionLabel(dir pb.StopOrderDirection) string {
	if dir == pb.StopOrderDirection_STOP_ORDER_DIRECTION_BUY {
		return "buy"
	}
	if dir == pb.StopOrderDirection_STOP_ORDER_DIRECTION_SELL {
		return "sell"
	}
	return "unknown"
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
	if _, _, brokerTotal, _, ok := r.InstancePNLBrokerAware(ctx, instanceID); ok {
		total = brokerTotal
	}
	if wd.MaxDailyLossRub > 0 && total < -wd.MaxDailyLossRub {
		r.logger.Warn("watchdog: max loss exceeded, flattening instance", "instance", instanceID, "total_pnl", total)
		if r.tg != nil {
			_ = r.tg.SendMessage(ctx, fmt.Sprintf("strategy-runner: instance %s flattening (total PnL %.2f < -%.2f RUB)",
				instanceID, total, wd.MaxDailyLossRub))
		}
		r.flattenInstance(ctx, instanceID, "watchdog max daily loss")
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
				r.logger.Warn("watchdog: drawdown exceeded, flattening instance", "instance", instanceID, "dd_pct", dd)
				if r.tg != nil {
					_ = r.tg.SendMessage(ctx, fmt.Sprintf("strategy-runner: instance %s flattening (drawdown %.1f%% > %.1f%%)",
						instanceID, dd, wd.MaxDrawdownPercent))
				}
				r.flattenInstance(ctx, instanceID, "watchdog drawdown")
				r.StopInstance(instanceID)
			}
		}
	}
}

func (r *Runner) flattenInstance(ctx context.Context, instanceID, reason string) {
	if r.portfolioSvc == nil || r.orderSvc == nil {
		r.logger.Error("watchdog flatten unavailable: missing services", "instance", instanceID)
		return
	}
	inst, ok := r.byID[instanceID]
	if !ok {
		r.logger.Error("watchdog flatten unknown instance", "instance", instanceID)
		return
	}
	r.mu.Lock()
	rt := r.instances[instanceID]
	r.mu.Unlock()
	if rt == nil {
		return
	}
	positions, err := r.portfolioSvc.GetPositions(ctx, inst.AccountID)
	if err != nil {
		r.logger.Error("watchdog flatten: get broker positions failed", "instance", instanceID, "error", err)
		return
	}
	allowed := make(map[string]struct{}, len(inst.Instruments))
	for _, uid := range inst.Instruments {
		allowed[strings.TrimSpace(uid)] = struct{}{}
	}
	for _, o := range r.orderRepo.GetActiveOrders(inst.AccountID) {
		if _, ok := allowed[o.InstrumentUID]; !ok {
			continue
		}
		if _, err := r.orderSvc.CancelOrder(ctx, inst.AccountID, o.OrderID); err != nil {
			r.logger.Warn("watchdog flatten: cancel active order failed", "instance", instanceID, "order_id", o.OrderID, "error", err)
		}
	}
	for _, p := range positions {
		if _, ok := allowed[p.InstrumentUID]; !ok || p.Quantity == 0 {
			continue
		}
		r.cancelTrackedProtectiveStop(ctx, instanceID, inst.AccountID, p.InstrumentUID, "watchdog flatten")
		dir := "sell"
		if p.Quantity < 0 {
			dir = "buy"
		}
		qty := int64(math.Ceil(math.Abs(p.Quantity)))
		if qty <= 0 {
			continue
		}
		ref := p.CurrentPrice
		if ref <= 0 {
			ref = p.AveragePrice
		}
		sig := Signal{
			InstrumentUID: p.InstrumentUID,
			Direction:     dir,
			Quantity:      qty,
			OrderType:     "market",
			Reason:        reason,
			CandleTime:    time.Now().UTC(),
		}
		resp, err := r.orderSvc.PostOrder(ctx, buildPostOrderRequest(inst.AccountID, sig))
		if err != nil {
			r.logger.Error("watchdog flatten: close order failed", "instance", instanceID, "instrument", p.InstrumentUID, "error", err)
			_ = r.journal.RecordEvent(ctx, journal.EventRecord{
				Type:          "watchdog_flatten",
				InstanceID:    instanceID,
				InstrumentUID: p.InstrumentUID,
				Direction:     dir,
				Quantity:      qty,
				OrderType:     "market",
				RefPrice:      ref,
				Status:        "post_error",
				Message:       err.Error(),
				CreatedAt:     time.Now().UTC(),
			})
			continue
		}
		oid := resp.GetOrderId()
		r.mu.Lock()
		r.orderOwners[oid] = instanceID
		r.signalRefPx[oid] = ref
		r.mu.Unlock()
		_ = r.journal.RecordOrder(ctx, journal.OrderRecord{
			InstanceID:    instanceID,
			OrderID:       oid,
			InstrumentUID: p.InstrumentUID,
			Direction:     dir,
			Quantity:      qty,
			OrderType:     "market",
			RefPrice:      ref,
		})
		_ = r.journal.RecordEvent(ctx, journal.EventRecord{
			Type:          "watchdog_flatten",
			InstanceID:    instanceID,
			InstrumentUID: p.InstrumentUID,
			Direction:     dir,
			Quantity:      qty,
			OrderType:     "market",
			RefPrice:      ref,
			Status:        "submitted",
			Message:       reason,
			CreatedAt:     time.Now().UTC(),
		})
		status := order.MapExecutionStatus(resp.GetExecutionReportStatus())
		if resp.GetLotsExecuted() > 0 || isTerminalOrderStatus(status) {
			r.dispatchExecution(order.OrderStateEvent{
				OrderID:   oid,
				AccountID: inst.AccountID,
				Status:    status,
				FilledQty: resp.GetLotsExecuted(),
				UpdatedAt: time.Now().UTC(),
			})
		}
		r.pollOrderStateAfterSubmit(ctx, rt, oid)
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
