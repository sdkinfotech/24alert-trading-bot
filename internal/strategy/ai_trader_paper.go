package strategy

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/24alert/trading-bot/pkg/metrics"
)

// PaperTradingState is virtual execution state for level_intraday sessions.
type PaperTradingState struct {
	PositionLots  int64        `json:"position_lots"`
	AvgPrice      float64      `json:"avg_price"`
	RealizedRUB   float64      `json:"realized_rub"`
	UnrealizedRUB float64      `json:"unrealized_rub"`
	TotalFeesRUB  float64      `json:"total_fees_rub"`
	PeakPnLRUB    float64      `json:"peak_pnl_rub"`
	DrawdownRUB   float64      `json:"drawdown_rub"`
	Wins          int          `json:"wins"`
	Losses        int          `json:"losses"`
	StopLoss      float64      `json:"stop_loss,omitempty"`
	TakeProfit    float64      `json:"take_profit,omitempty"`
	WorkingOrders []PaperOrder `json:"working_orders,omitempty"`
	Fills         []PaperFill  `json:"fills,omitempty"`
	Halted        bool         `json:"halted,omitempty"`
	HaltReason    string       `json:"halt_reason,omitempty"`
	UpdatedAt     string       `json:"updated_at"`

	tradeTimestamps []time.Time `json:"-"`
}

type PaperOrder struct {
	ID       string  `json:"id"`
	Side     string  `json:"side"`
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
	LevelRef string  `json:"level_ref,omitempty"`
	Status   string  `json:"status"`
	PlacedAt string  `json:"placed_at"`
}

type PaperFill struct {
	Time     string  `json:"time"`
	Side     string  `json:"side"`
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
	LimitPx  float64 `json:"limit_px,omitempty"`
	FeesRUB  float64 `json:"fees_rub,omitempty"`
	Note     string  `json:"note,omitempty"`
}

func newPaperTradingState() *PaperTradingState {
	return &PaperTradingState{UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
}

func paperCommissionBPS() float64 {
	if v := os.Getenv("AI_TRADER_PAPER_COMMISSION_BPS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return f
		}
	}
	return 4.0
}

func paperSlippageBPS() float64 {
	if v := os.Getenv("AI_TRADER_PAPER_SLIPPAGE_BPS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return f
		}
	}
	return 1.0
}

func paperLotValueRUB(ticker string) float64 {
	if v := os.Getenv("AI_TRADER_LOT_VALUE_RUB"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			return f
		}
	}
	t := strings.ToUpper(ticker)
	switch {
	case strings.HasPrefix(t, "BR"), strings.HasPrefix(t, "BMM"):
		return 8.5 // Brent mini approx
	case strings.HasPrefix(t, "Si"), strings.HasPrefix(t, "USDRUB"):
		return 10.0
	default:
		return 1.0
	}
}

func (r *Runner) startPaperTradingFromPlaybook(s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext, sig *AITraderTradeSignal) {
	if s == nil || f == nil || s.LevelPlaybook == nil || s.PaperState == nil {
		return
	}
	if s.PaperState.Halted || r.aiTraderKillSwitchActive() {
		return
	}
	regime := s.SessionRegime
	if regime == "" {
		regime = detectSessionRegime(mctx)
	}
	if !allowNewEntry(s, regime) {
		return
	}
	if f.Mid <= 0 {
		return
	}
	if len(s.PaperState.WorkingOrders) > 0 {
		s.PaperState.WorkingOrders = nil
	}
	if r.tryStructuralPlaybookEntry(nil, s, f, mctx, false) {
		s.PaperState.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		return
	}
	if r.tryValidatedLLMEntry(nil, s, f, mctx, sig, false) {
		s.PaperState.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		return
	}
	s.PaperState.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (r *Runner) placePaperOrder(s *AITraderSession, side string, price float64, qty int64, levelRef, note string) {
	if s == nil || s.PaperState == nil {
		return
	}
	if len(s.PaperState.WorkingOrders) >= s.Limits.MaxActiveOrders {
		return
	}
	if qty > int64(s.Limits.MaxOrderSize) {
		qty = int64(s.Limits.MaxOrderSize)
	}
	s.PaperState.WorkingOrders = append(s.PaperState.WorkingOrders, PaperOrder{
		ID:       fmt.Sprintf("po-%d", time.Now().UnixNano()),
		Side:     side,
		Price:    price,
		Quantity: qty,
		LevelRef: levelRef,
		Status:   "working",
		PlacedAt: time.Now().UTC().Format(time.RFC3339),
	})
	metrics.AITraderOrdersTotal.WithLabelValues("paper", side, "placed").Inc()
	_ = note
}

func (r *Runner) tickPaperTrading(s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext, sig *AITraderTradeSignal, regime string) {
	if s == nil || s.PaperState == nil || f == nil || f.Mid <= 0 {
		return
	}
	st := s.PaperState
	if st.Halted || r.aiTraderKillSwitchActive() {
		st.Halted = true
		st.HaltReason = "kill_switch"
		return
	}
	if f.Stale || f.SpreadBPS > s.Limits.MaxSpreadBPS {
		return
	}

	if sig != nil && (strings.EqualFold(sig.RiskOverride, "cancel_all") || strings.EqualFold(sig.OrderAction, "cancel_all")) {
		st.WorkingOrders = nil
	}
	if sig != nil && (strings.EqualFold(sig.RiskOverride, "flatten") || strings.EqualFold(sig.OrderAction, "flatten")) && st.PositionLots != 0 {
		r.closePaperPosition(s, f, "llm_flatten")
	}

	r.updatePaperUnrealized(st, f.Mid, s.Ticker)
	r.enforcePaperRiskLimits(s, f)

	if st.PositionLots != 0 {
		r.checkPaperSLTP(s, f, mctx)
		applyLLMPositionManagement(s, sig, f.Mid, false)
	} else if sig != nil && !s.reconnectPaused && s.sessionStrategyAllowsEntry() {
		if entrySignalAllowed(sig, tradeableLevelsForSession(s, effectivePolicy(s), f.Mid), f, mctx) {
			r.syncPaperOrdersFromSignal(s, sig)
		}
	}

	r.tryFillWorkingOrders(s, f, mctx)
	r.updatePaperMetrics(s)
	st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (r *Runner) syncPaperOrdersFromSignal(s *AITraderSession, sig *AITraderTradeSignal) {
	if s.PaperState == nil || !sig.actionableWith(effectivePolicy(s).EntryMinConfidence) {
		return
	}
	if !s.sessionStrategyAllowsEntry() || !s.sessionStrategyAllowsSide(sig.Side) {
		return
	}
	if reason := microstructureBlocksEntry(s, sig.Side); reason != "" {
		return
	}
	f := s.Features
	if f == nil || f.Mid <= 0 {
		return
	}
	tradeable := tradeableLevelsForSession(s, effectivePolicy(s), f.Mid)
	if !entrySignalAllowed(sig, tradeable, f, s.MarketContext) {
		return
	}
	px, src, ok := snapSignalToTradeableLevel(sig, tradeable, f)
	if !ok {
		return
	}
	sigSnap := *sig
	sigSnap.LevelPrice = px
	if r.replacePaperOrderIfNeeded(s, &sigSnap) {
		return
	}
	for _, o := range s.PaperState.WorkingOrders {
		if o.Side == sigSnap.Side && math.Abs(o.Price-px) < tickEpsilon(px) {
			return
		}
	}
	r.placePaperOrder(s, sigSnap.Side, px, 1, src, "llm@"+src+": "+sig.Reason)
}

func (r *Runner) tryFillWorkingOrders(s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext) {
	st := s.PaperState
	if st == nil || st.Halted {
		return
	}
	remaining := make([]PaperOrder, 0, len(st.WorkingOrders))
	for _, o := range st.WorkingOrders {
		if o.Status != "working" {
			continue
		}
		if !levelTouched(o.Side, o.Price, f) {
			remaining = append(remaining, o)
			continue
		}
		if !tapeConfirmsRejection(o.Side, o.Price, mctx) {
			remaining = append(remaining, o)
			continue
		}
		if !r.paperRateLimitOK(s) {
			remaining = append(remaining, o)
			continue
		}
		fillPx := applySlippage(o.Price, o.Side, f.Mid)
		fill := PaperFill{
			Time: time.Now().UTC().Format(time.RFC3339), Side: o.Side,
			Price: fillPx, Quantity: o.Quantity, LimitPx: o.Price,
			Note: "level touch + tape rejection " + o.LevelRef,
		}
		r.recordPaperFill(s, fill)
		metrics.AITraderOrdersTotal.WithLabelValues("paper", o.Side, "filled").Inc()
		if st.PositionLots == 0 {
			setStopsFromPolicy(s, o.Side, fillPx, false)
		}
	}
	st.WorkingOrders = remaining
}

func levelTouched(side string, price float64, f *AITraderFeatures) bool {
	eps := tickEpsilon(price)
	switch side {
	case "buy":
		return f.BestAsk > 0 && f.BestAsk <= price+eps
	case "sell":
		return f.BestBid > 0 && f.BestBid >= price-eps
	}
	return false
}

func tapeConfirmsRejection(side string, price float64, mctx *AITraderMarketContext) bool {
	if mctx == nil {
		return true // permissive if no tape yet
	}
	if mctx.TapeStats.TradeCount < 3 {
		return true
	}
	switch side {
	case "buy":
		return mctx.TapeStats.DeltaPct > -0.05 || printsNearPrice(mctx, price, 2)
	case "sell":
		return mctx.TapeStats.DeltaPct < 0.05 || printsNearPrice(mctx, price, 2)
	}
	return false
}

func applySlippage(limitPx float64, side string, mid float64) float64 {
	bps := paperSlippageBPS() / 10000
	switch side {
	case "buy":
		return limitPx * (1 + bps)
	case "sell":
		return limitPx * (1 - bps)
	}
	return mid
}

func (r *Runner) recordPaperFill(s *AITraderSession, fill PaperFill) {
	st := s.PaperState
	if st == nil {
		return
	}
	prevPos := st.PositionLots
	lotVal := paperLotValueRUB(s.Ticker)
	fee := math.Abs(fill.Price*float64(fill.Quantity)) * lotVal * paperCommissionBPS() / 10000
	fill.FeesRUB = fee
	st.TotalFeesRUB += fee
	st.Fills = append(st.Fills, fill)
	closed, pnl := applyPaperFill(st, fill, lotVal)
	r.logPaperFillExecution(s, fill, prevPos, st.PositionLots)
	st.tradeTimestamps = append(st.tradeTimestamps, time.Now())
	if fill.LimitPx > 0 {
		bps := (fill.Price - fill.LimitPx) / fill.LimitPx * 10000
		if fill.Side == "sell" {
			bps = -bps
		}
		metrics.AITraderFillQualityBPS.WithLabelValues(fill.Side).Observe(bps)
	}
	if closed {
		net := pnl - fee
		if net >= 0 {
			st.Wins++
			metrics.AITraderTradesTotal.WithLabelValues(s.ID, fill.Side, "win").Inc()
		} else {
			st.Losses++
			metrics.AITraderTradesTotal.WithLabelValues(s.ID, fill.Side, "loss").Inc()
		}
		st.StopLoss, st.TakeProfit = 0, 0
		// Re-entry: place new limits after flat
		mctx := s.MarketContext
		r.startPaperTradingFromPlaybook(s, s.Features, mctx, s.LastTradeSignal)
	}
}

func (r *Runner) checkPaperSLTP(s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext) {
	st := s.PaperState
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
		r.closePaperPosition(s, f, note)
	}
}

func (r *Runner) closePaperPosition(s *AITraderSession, f *AITraderFeatures, note string) {
	st := s.PaperState
	if st == nil || st.PositionLots == 0 {
		return
	}
	side := "sell"
	if st.PositionLots < 0 {
		side = "buy"
	}
	fill := PaperFill{
		Time: time.Now().UTC().Format(time.RFC3339), Side: side,
		Price: f.Mid, Quantity: abs64(st.PositionLots), Note: note,
	}
	r.recordPaperFill(s, fill)
	st.WorkingOrders = nil
}

func applyPaperFill(st *PaperTradingState, f PaperFill, lotVal float64) (closed bool, roundPnL float64) {
	qty := f.Quantity
	if f.Side == "sell" {
		qty = -qty
	}
	if st.PositionLots == 0 {
		st.PositionLots = qty
		st.AvgPrice = f.Price
		return false, 0
	}
	if (st.PositionLots > 0 && qty < 0) || (st.PositionLots < 0 && qty > 0) {
		closedQty := min64(abs64(st.PositionLots), abs64(qty))
		pnl := float64(closedQty) * (f.Price - st.AvgPrice) * lotVal
		if st.PositionLots < 0 {
			pnl = -pnl
		}
		st.RealizedRUB += pnl
		st.PositionLots += qty
		if st.PositionLots == 0 {
			st.AvgPrice = 0
			return true, pnl
		}
		st.AvgPrice = f.Price
		return true, pnl
	}
	st.PositionLots += qty
	st.AvgPrice = (st.AvgPrice + f.Price) / 2
	return false, 0
}

func (r *Runner) updatePaperUnrealized(st *PaperTradingState, mid float64, ticker string) {
	if st == nil {
		return
	}
	if st.PositionLots == 0 {
		st.UnrealizedRUB = 0
		return
	}
	lotVal := paperLotValueRUB(ticker)
	dir := float64(st.PositionLots)
	st.UnrealizedRUB = dir * (mid - st.AvgPrice) * lotVal
}

func (r *Runner) enforcePaperRiskLimits(s *AITraderSession, f *AITraderFeatures) {
	st := s.PaperState
	if st == nil || st.Halted {
		return
	}
	net := st.RealizedRUB + st.UnrealizedRUB - st.TotalFeesRUB
	if s.Limits.MaxSessionLossRUB > 0 && net <= -s.Limits.MaxSessionLossRUB {
		st.Halted = true
		st.HaltReason = "max_session_loss"
		st.WorkingOrders = nil
		if st.PositionLots != 0 {
			r.closePaperPosition(s, f, "session_loss_limit")
		}
	}
	if abs64(st.PositionLots) > int64(s.Limits.MaxPositionLots) {
		st.Halted = true
		st.HaltReason = "max_position"
	}
}

func (r *Runner) paperRateLimitOK(s *AITraderSession) bool {
	st := s.PaperState
	if st == nil || s.Limits.MaxTradesPerMinute <= 0 {
		return true
	}
	cut := time.Now().Add(-time.Minute)
	n := 0
	var kept []time.Time
	for _, t := range st.tradeTimestamps {
		if t.After(cut) {
			n++
			kept = append(kept, t)
		}
	}
	st.tradeTimestamps = kept
	return n < s.Limits.MaxTradesPerMinute
}

func (r *Runner) updatePaperMetrics(s *AITraderSession) {
	st := s.PaperState
	if st == nil {
		return
	}
	net := st.RealizedRUB + st.UnrealizedRUB - st.TotalFeesRUB
	metrics.AITraderPnLRUB.WithLabelValues(s.ID, s.Ticker).Set(net)
	total := st.Wins + st.Losses
	if total > 0 {
		metrics.AITraderWinRate.WithLabelValues(s.ID).Set(float64(st.Wins) / float64(total))
	}
	if net > st.PeakPnLRUB {
		st.PeakPnLRUB = net
	}
	dd := st.PeakPnLRUB - net
	if dd > st.DrawdownRUB {
		st.DrawdownRUB = dd
	}
	metrics.AITraderDrawdownRUB.WithLabelValues(s.ID).Set(st.DrawdownRUB)
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
