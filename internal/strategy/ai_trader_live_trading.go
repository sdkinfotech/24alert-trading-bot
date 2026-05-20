package strategy

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/internal/order"
	"github.com/24alert/trading-bot/internal/risk"
	"github.com/24alert/trading-bot/pkg/metrics"
)

const (
	ExecutionModePaper     = "paper"
	ExecutionModeArmedLive = "armed_live"
)

// LiveTradingState tracks real broker orders for armed_live sessions.
type LiveTradingState struct {
	PositionLots  int64        `json:"position_lots"`
	AvgPrice      float64      `json:"avg_price"`
	RealizedRUB   float64      `json:"realized_rub"`
	StopLoss      float64      `json:"stop_loss,omitempty"`
	TakeProfit    float64      `json:"take_profit,omitempty"`
	WorkingOrders []LiveOrder  `json:"working_orders,omitempty"`
	Fills         []LiveFill   `json:"fills,omitempty"`
	Halted        bool         `json:"halted,omitempty"`
	HaltReason    string       `json:"halt_reason,omitempty"`
	UpdatedAt     string       `json:"updated_at"`
}

type LiveOrder struct {
	ID            string  `json:"id"`
	BrokerOrderID string  `json:"broker_order_id,omitempty"`
	Side          string  `json:"side"`
	Price         float64 `json:"price"`
	Quantity      int64   `json:"quantity"`
	LevelRef      string  `json:"level_ref,omitempty"`
	Status        string  `json:"status"`
	PlacedAt      string  `json:"placed_at"`
}

type LiveFill struct {
	Time          string  `json:"time"`
	Side          string  `json:"side"`
	Price         float64 `json:"price"`
	Quantity      int64   `json:"quantity"`
	BrokerOrderID string  `json:"broker_order_id,omitempty"`
	Note          string  `json:"note,omitempty"`
}

func aiTraderArmedLiveEnabled() bool {
	v := strings.TrimSpace(os.Getenv("AI_TRADER_ARMED_LIVE"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

func newLiveTradingState() *LiveTradingState {
	return &LiveTradingState{UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
}

func (s *AITraderSession) isArmedLive() bool {
	return s != nil && s.ExecutionMode == ExecutionModeArmedLive
}

func (r *Runner) startLiveTradingFromPlaybook(ctx context.Context, s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext, sig *AITraderTradeSignal) {
	if s == nil || f == nil || s.LevelPlaybook == nil || s.LiveState == nil {
		return
	}
	if s.LiveState.Halted || r.aiTraderKillSwitchActive() {
		return
	}
	regime := s.SessionRegime
	if regime == "" {
		regime = detectSessionRegime(mctx)
	}
	if !allowNewEntry(s, regime) {
		return
	}
	mid := f.Mid
	if mid <= 0 {
		return
	}
	pol := effectivePolicy(s)
	if len(s.LiveState.WorkingOrders) > 0 {
		r.cancelAllLiveOrders(ctx, s)
	}
	if sig != nil && sig.actionableWith(pol.EntryMinConfidence) {
		r.placeLiveLimit(ctx, s, sig.Side, sig.LevelPrice, 1, "llm_signal", sig.Reason)
		return
	}
	lvls := levelsForConfluence(s, pol)
	scored := scoreLevels(lvls, f, mctx, nil)
	minScore := pol.ConfluenceMinScore
	allowBuy := regime != RegimeTrend || trendDirection(mctx) != "down"
	allowSell := regime != RegimeTrend || trendDirection(mctx) != "up"
	if pol.MarketBias == "bearish" {
		allowBuy = false
	} else if pol.MarketBias == "bullish" {
		allowSell = false
	}
	if sup, ok := bestSupportLevel(scored, mid, minScore); ok && allowBuy {
		r.placeLiveLimit(ctx, s, "buy", sup.Price, 1, sup.Source, "confluence support")
	}
	if res, ok := bestResistanceLevel(scored, mid, minScore); ok && allowSell {
		r.placeLiveLimit(ctx, s, "sell", res.Price, 1, res.Source, "confluence resistance")
	}
	s.LiveState.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (r *Runner) tickLiveTrading(ctx context.Context, s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext, sig *AITraderTradeSignal, regime string) {
	r.syncAITraderLiveFromBroker(ctx, s)
	if s == nil || s.LiveState == nil || f == nil || f.Mid <= 0 {
		return
	}
	st := s.LiveState
	if st.Halted || r.aiTraderKillSwitchActive() {
		st.Halted = true
		st.HaltReason = "kill_switch"
		r.cancelAllLiveOrders(ctx, s)
		return
	}
	if f.Stale || f.SpreadBPS > s.Limits.MaxSpreadBPS {
		return
	}
	if err := applyLiveRiskGate(s, f); err != nil {
		return
	}
	if sig != nil && (strings.EqualFold(sig.RiskOverride, "cancel_all") || strings.EqualFold(sig.OrderAction, "cancel_all")) {
		r.cancelAllLiveOrders(ctx, s)
	}
	if sig != nil && (strings.EqualFold(sig.RiskOverride, "flatten") || strings.EqualFold(sig.OrderAction, "flatten")) && st.PositionLots != 0 {
		r.closeLivePosition(ctx, s, f, "llm_flatten")
	}
	r.syncLiveOrderStates(ctx, s)
	if st.PositionLots != 0 {
		r.checkLiveSLTP(ctx, s, f)
		applyLLMPositionManagement(s, sig, f.Mid, true)
	} else if sig != nil && sig.actionableWith(effectivePolicy(s).EntryMinConfidence) && !s.reconnectPaused {
		r.syncLiveOrdersFromSignal(ctx, s, sig)
	}
	if !s.reconnectPaused && st.PositionLots == 0 && len(st.WorkingOrders) == 0 && allowNewEntry(s, regime) {
		r.startLiveTradingFromPlaybook(ctx, s, f, mctx, sig)
	}
	st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (r *Runner) placeLiveLimit(ctx context.Context, s *AITraderSession, side string, price float64, qty int64, levelRef, reason string) {
	if s == nil || s.LiveState == nil || r.orderSvc == nil || r.riskSvc == nil {
		return
	}
	st := s.LiveState
	if len(st.WorkingOrders) >= s.Limits.MaxActiveOrders {
		return
	}
	if qty > int64(s.Limits.MaxOrderSize) {
		qty = int64(s.Limits.MaxOrderSize)
	}
	if st.PositionLots != 0 && abs64(st.PositionLots)+qty > int64(s.Limits.MaxPositionLots) {
		return
	}
	intent := AITraderOrderIntent{Side: side, Price: price, Quantity: qty, Kind: "limit"}
	if err := validateOrderControl(intent, s); err != nil {
		return
	}
	sig := Signal{
		InstrumentUID: s.InstrumentID,
		Direction:     side,
		Quantity:      qty,
		Price:         price,
		OrderType:     "limit",
		Reason:        "ai_trader:" + levelRef + " " + reason,
	}
	ri := risk.OrderIntent{
		AccountID:     s.AccountID,
		InstrumentUID: s.InstrumentID,
		Direction:     side,
		Quantity:      qty,
		EstimatedCost: r.estimateCost(sig, fMid(s)),
	}
	resp, err := r.riskSvc.ValidateOrderIntent(ctx, ri)
	if err != nil || resp == nil || !resp.Allowed {
		return
	}
	req := buildPostOrderRequest(s.AccountID, sig)
	postResp, err := r.orderSvc.PostOrder(ctx, req)
	if err != nil {
		s.LastError = err.Error()
		return
	}
	oid := postResp.GetOrderId()
	s.LiveState.WorkingOrders = append(s.LiveState.WorkingOrders, LiveOrder{
		ID: fmt.Sprintf("lo-%d", time.Now().UnixNano()), BrokerOrderID: oid,
		Side: side, Price: price, Quantity: qty, LevelRef: levelRef,
		Status: "working", PlacedAt: time.Now().UTC().Format(time.RFC3339),
	})
	metrics.AITraderOrdersTotal.WithLabelValues("live", side, "placed").Inc()
	go r.pollLiveOrderAfterSubmit(ctx, s, oid)
}

func fMid(s *AITraderSession) float64 {
	if s != nil && s.Features != nil {
		return s.Features.Mid
	}
	return 0
}

func (r *Runner) pollLiveOrderAfterSubmit(ctx context.Context, s *AITraderSession, orderID string) {
	if r.orderSvc == nil || s == nil {
		return
	}
	time.Sleep(500 * time.Millisecond)
	r.syncLiveOrderStates(ctx, s)
}

func (r *Runner) syncLiveOrderStates(ctx context.Context, s *AITraderSession) {
	if s == nil || s.LiveState == nil || r.orderSvc == nil {
		return
	}
	st := s.LiveState
	remaining := make([]LiveOrder, 0, len(st.WorkingOrders))
	for _, o := range st.WorkingOrders {
		if o.Status != "working" || o.BrokerOrderID == "" {
			continue
		}
		resp, err := r.orderSvc.GetOrderState(ctx, s.AccountID, o.BrokerOrderID, pb.PriceType_PRICE_TYPE_UNSPECIFIED)
		if err != nil {
			remaining = append(remaining, o)
			continue
		}
		status := order.MapExecutionStatus(resp.GetExecutionReportStatus())
		switch status {
		case order.OrderStatusFilled:
			fillPx := o.Price
			if mv := resp.GetExecutedOrderPrice(); mv != nil {
				fillPx = float64(mv.GetUnits()) + float64(mv.GetNano())/1e9
			}
			r.recordLiveFill(s, LiveFill{
				Time: time.Now().UTC().Format(time.RFC3339), Side: o.Side,
				Price: fillPx, Quantity: resp.GetLotsExecuted(), BrokerOrderID: o.BrokerOrderID,
				Note: "broker fill " + o.LevelRef,
			})
			if st.PositionLots == 0 {
				setStopsFromPolicy(s, o.Side, fillPx, true)
			}
			metrics.AITraderOrdersTotal.WithLabelValues("live", o.Side, "filled").Inc()
		case order.OrderStatusCancelled, order.OrderStatusRejected:
			metrics.AITraderOrdersTotal.WithLabelValues("live", o.Side, string(status)).Inc()
		default:
			remaining = append(remaining, o)
		}
	}
	st.WorkingOrders = remaining
}

func (r *Runner) recordLiveFill(s *AITraderSession, fill LiveFill) {
	st := s.LiveState
	if st == nil {
		return
	}
	st.Fills = append(st.Fills, fill)
	qty := fill.Quantity
	if fill.Side == "sell" {
		qty = -qty
	}
	lotVal := paperLotValueRUB(s.Ticker)
	if st.PositionLots == 0 {
		st.PositionLots = qty
		st.AvgPrice = fill.Price
		return
	}
	if (st.PositionLots > 0 && qty < 0) || (st.PositionLots < 0 && qty > 0) {
		closed := min64(abs64(st.PositionLots), abs64(qty))
		pnl := float64(closed) * (fill.Price - st.AvgPrice) * lotVal
		if st.PositionLots < 0 {
			pnl = -pnl
		}
		st.RealizedRUB += pnl
		if net := st.RealizedRUB; s.Limits.MaxSessionLossRUB > 0 && net <= -s.Limits.MaxSessionLossRUB {
			st.Halted = true
			st.HaltReason = "max_session_loss"
		}
	}
	st.PositionLots += qty
	if st.PositionLots == 0 {
		st.AvgPrice = 0
		st.StopLoss, st.TakeProfit = 0, 0
	} else {
		st.AvgPrice = fill.Price
	}
}

func (r *Runner) checkLiveSLTP(ctx context.Context, s *AITraderSession, f *AITraderFeatures) {
	st := s.LiveState
	if st == nil || st.PositionLots == 0 {
		return
	}
	mid := f.Mid
	hitSL, hitTP := false, false
	if st.PositionLots > 0 {
		if st.StopLoss > 0 && mid <= st.StopLoss {
			hitSL = true
		}
		if st.TakeProfit > 0 && mid >= st.TakeProfit {
			hitTP = true
		}
	} else {
		if st.StopLoss > 0 && mid >= st.StopLoss {
			hitSL = true
		}
		if st.TakeProfit > 0 && mid <= st.TakeProfit {
			hitTP = true
		}
	}
	if hitSL || hitTP {
		note := "stop_loss"
		if hitTP {
			note = "take_profit"
		}
		r.closeLivePosition(ctx, s, f, note)
	}
}

func (r *Runner) closeLivePosition(ctx context.Context, s *AITraderSession, f *AITraderFeatures, note string) {
	st := s.LiveState
	if st == nil || st.PositionLots == 0 {
		return
	}
	r.cancelAllLiveOrders(ctx, s)
	side := "sell"
	if st.PositionLots < 0 {
		side = "buy"
	}
	sig := Signal{
		InstrumentUID: s.InstrumentID,
		Direction:     side,
		Quantity:      abs64(st.PositionLots),
		OrderType:     "market",
		Reason:        "ai_trader close: " + note,
	}
	if err := applyLiveRiskGate(s, f); err != nil {
		return
	}
	ri := risk.OrderIntent{
		AccountID: s.AccountID, InstrumentUID: s.InstrumentID,
		Direction: side, Quantity: abs64(st.PositionLots),
	}
	resp, err := r.riskSvc.ValidateOrderIntent(ctx, ri)
	if err != nil || resp == nil || !resp.Allowed {
		return
	}
	req := buildPostOrderRequest(s.AccountID, sig)
	if _, err := r.orderSvc.PostOrder(ctx, req); err != nil {
		s.LastError = err.Error()
		return
	}
	st.PositionLots = 0
	st.AvgPrice = 0
	st.StopLoss, st.TakeProfit = 0, 0
	metrics.AITraderOrdersTotal.WithLabelValues("live", side, "close").Inc()
}

func (r *Runner) cancelAllLiveOrders(ctx context.Context, s *AITraderSession) {
	if s == nil || s.LiveState == nil {
		return
	}
	if r.orderSvc != nil {
		for _, o := range s.LiveState.WorkingOrders {
			if o.BrokerOrderID == "" || o.Status != "working" {
				continue
			}
			_, _ = r.orderSvc.CancelOrder(ctx, s.AccountID, o.BrokerOrderID)
			metrics.AITraderOrdersTotal.WithLabelValues("live", o.Side, "cancelled").Inc()
		}
	}
	s.LiveState.WorkingOrders = nil
}

func (r *Runner) syncLiveOrdersFromSignal(ctx context.Context, s *AITraderSession, sig *AITraderTradeSignal) {
	if s.LiveState == nil || !sig.actionableWith(effectivePolicy(s).EntryMinConfidence) {
		return
	}
	if r.replaceLiveOrderIfNeeded(ctx, s, sig) {
		return
	}
	for _, o := range s.LiveState.WorkingOrders {
		if o.Side == sig.Side && math.Abs(o.Price-sig.LevelPrice) < tickEpsilon(sig.LevelPrice) {
			return
		}
	}
	r.placeLiveLimit(ctx, s, sig.Side, sig.LevelPrice, 1, "llm_signal", sig.Reason)
}

// quotationToFloat for pb.Quotation is in candlehub.go (same package).
