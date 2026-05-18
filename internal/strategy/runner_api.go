package strategy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/24alert/trading-bot/internal/journal"
	"github.com/24alert/trading-bot/internal/strategy/pnl"
	pb "github.com/russianinvestments/invest-api-go-sdk/proto"
)

// PortfolioPositionSnapshot is the broker-side truth for one account position.
type PortfolioPositionSnapshot struct {
	InstrumentUID  string  `json:"instrument_uid"`
	Ticker         string  `json:"ticker,omitempty"`
	InstrumentType string  `json:"instrument_type,omitempty"`
	FIGI           string  `json:"figi,omitempty"`
	Quantity       float64 `json:"quantity"`
	AveragePrice   float64 `json:"average_price"`
	CurrentPrice   float64 `json:"current_price"`
	ExpectedYield  float64 `json:"expected_yield"`
	Currency       string  `json:"currency,omitempty"`
	Blocked        bool    `json:"blocked"`
	InInstance     bool    `json:"in_instance"`
}

// PortfolioSnapshotData is returned to the dashboard so the UI can show broker
// positions even after a runner/container restart, before in-memory ledger warms up.
type PortfolioSnapshotData struct {
	InstanceID            string                      `json:"instance_id"`
	AccountID             string                      `json:"account_id"`
	TotalAmountShares     float64                     `json:"total_amount_shares"`
	TotalAmountBonds      float64                     `json:"total_amount_bonds"`
	TotalAmountETF        float64                     `json:"total_amount_etf"`
	TotalAmountCurrencies float64                     `json:"total_amount_currencies"`
	TotalAmountFutures    float64                     `json:"total_amount_futures"`
	ExpectedYield         float64                     `json:"expected_yield"`
	Positions             []PortfolioPositionSnapshot `json:"positions"`
	InstancePositionCount int                         `json:"instance_position_count"`
	LastBrokerSync        string                      `json:"last_broker_sync"`
	PortfolioError        string                      `json:"portfolio_error,omitempty"`
}

// StopOrderSnapshot is a dashboard-safe view of a broker stop order.
type StopOrderSnapshot struct {
	StopOrderID   string  `json:"stop_order_id"`
	InstrumentUID string  `json:"instrument_uid"`
	Direction     string  `json:"direction"`
	StopOrderType string  `json:"stop_order_type"`
	Lots          int64   `json:"lots"`
	StopPrice     float64 `json:"stop_price"`
	Price         float64 `json:"price"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at,omitempty"`
	ExpirationAt  string  `json:"expiration_at,omitempty"`
}

// InstancePNL returns realized, unrealized, total P&L in RUB for a running instance.
func (r *Runner) InstancePNL(id string) (realized, unrealized, total float64, ok bool) {
	if !r.InstanceRunning(id) {
		return 0, 0, 0, false
	}
	led := r.ledger.Ledger(id)
	qty, _, realized := led.Snapshot()
	marks := make(map[string]float64)
	for uid := range qty {
		if px, okp := r.priceCache.GetLastPrice(uid); okp {
			marks[uid] = px.Price
		}
	}
	unreal := pnl.UnrealizedRUB(led, marks)
	return realized, unreal, realized + unreal, true
}

// InstancePNLBrokerAware returns P&L for UI/operator answers.
// For futures, broker ExpectedYield is the only reliable RUB mark-to-market here:
// runner ledger stores price differences and does not know futures point value.
func (r *Runner) InstancePNLBrokerAware(ctx context.Context, id string) (realized, unrealized, total float64, source string, ok bool) {
	realized, ledgerUnrealized, ledgerTotal, ok := r.InstancePNL(id)
	if !ok {
		return 0, 0, 0, "", false
	}
	inst, exists := r.byID[id]
	if !exists || r.portfolioSvc == nil || inst.AccountID == "" {
		return realized, ledgerUnrealized, ledgerTotal, "runner_ledger_estimate", true
	}
	info, err := r.portfolioSvc.GetPortfolio(ctx, inst.AccountID)
	if err != nil {
		r.logger.Warn("broker-aware pnl: portfolio unavailable", "instance", id, "error", err)
		return realized, ledgerUnrealized, ledgerTotal, "runner_ledger_estimate", true
	}
	allowed := make(map[string]struct{}, len(inst.Instruments))
	for _, uid := range inst.Instruments {
		allowed[strings.TrimSpace(uid)] = struct{}{}
	}
	var brokerUnrealized float64
	var matched bool
	for _, p := range info.Positions {
		if _, ok := allowed[p.InstrumentUID]; !ok || p.Quantity == 0 {
			continue
		}
		brokerUnrealized += p.ExpectedYield
		matched = true
	}
	if !matched {
		return realized, 0, realized, "broker_expected_yield", true
	}
	return realized, brokerUnrealized, realized + brokerUnrealized, "broker_expected_yield", true
}

// InstanceLedgerPositions returns qty (shares) and avg price maps plus cumulative realized RUB.
func (r *Runner) InstanceLedgerPositions(id string) (qty map[string]float64, avg map[string]float64, realized float64, ok bool) {
	if !r.InstanceRunning(id) {
		return nil, nil, 0, false
	}
	q, a, rl := r.ledger.Ledger(id).Snapshot()
	return q, a, rl, true
}

// InstancePortfolio returns broker-side portfolio positions for the instance account.
// Unlike the runner ledger, this is available after restart as soon as broker API
// responds, and is the source the dashboard should treat as the position truth.
func (r *Runner) InstancePortfolio(ctx context.Context, id string) (PortfolioSnapshotData, bool, error) {
	inst, ok := r.byID[id]
	if !ok {
		return PortfolioSnapshotData{}, false, nil
	}
	out := PortfolioSnapshotData{
		InstanceID:     id,
		AccountID:      inst.AccountID,
		LastBrokerSync: time.Now().UTC().Format(time.RFC3339),
	}
	if r.portfolioSvc == nil {
		out.PortfolioError = "portfolio service is not configured"
		return out, true, nil
	}
	info, err := r.portfolioSvc.GetPortfolio(ctx, inst.AccountID)
	if err != nil {
		out.PortfolioError = err.Error()
		return out, true, nil
	}
	out.TotalAmountShares = info.TotalAmountShares
	out.TotalAmountBonds = info.TotalAmountBonds
	out.TotalAmountETF = info.TotalAmountETF
	out.TotalAmountCurrencies = info.TotalAmountCurrencies
	out.TotalAmountFutures = info.TotalAmountFutures
	out.ExpectedYield = info.ExpectedYield

	allowed := make(map[string]struct{}, len(inst.Instruments))
	for _, uid := range inst.Instruments {
		allowed[strings.TrimSpace(uid)] = struct{}{}
	}
	for _, p := range info.Positions {
		if p.Quantity == 0 {
			continue
		}
		_, inInstance := allowed[p.InstrumentUID]
		ticker := ""
		if inf, ok := r.instrCache.GetInstrument(p.InstrumentUID); ok {
			ticker = inf.Ticker
		}
		if inInstance {
			out.InstancePositionCount++
		}
		out.Positions = append(out.Positions, PortfolioPositionSnapshot{
			InstrumentUID:  p.InstrumentUID,
			Ticker:         ticker,
			InstrumentType: p.InstrumentType,
			FIGI:           p.FIGI,
			Quantity:       p.Quantity,
			AveragePrice:   p.AveragePrice,
			CurrentPrice:   p.CurrentPrice,
			ExpectedYield:  p.ExpectedYield,
			Currency:       p.Currency,
			Blocked:        p.Blocked,
			InInstance:     inInstance,
		})
	}
	return out, true, nil
}

// InstanceRecentExecutions returns persisted executions from the journal.
func (r *Runner) InstanceRecentExecutions(ctx context.Context, id string, limit int) ([]journal.ExecutionRecord, error) {
	return r.journal.ListRecentExecutions(ctx, id, limit)
}

// InstanceRecentSignals returns persisted signals from the journal.
func (r *Runner) InstanceRecentSignals(ctx context.Context, id string, limit int) ([]journal.SignalRecord, error) {
	return r.journal.ListRecentSignals(ctx, id, limit)
}

// InstanceRecentOrders returns persisted orders from the journal.
func (r *Runner) InstanceRecentOrders(ctx context.Context, id string, limit int) ([]journal.OrderRecord, error) {
	return r.journal.ListRecentOrders(ctx, id, limit)
}

// InstanceEvents returns a unified timeline of signals, orders, and executions.
func (r *Runner) InstanceEvents(ctx context.Context, id string, limit int) ([]journal.TimelineEvent, error) {
	return r.journal.ListEvents(ctx, id, limit)
}

// InstanceStopOrders returns active broker stop orders for the instance account/instruments.
func (r *Runner) InstanceStopOrders(ctx context.Context, id string) ([]StopOrderSnapshot, bool, error) {
	inst, ok := r.byID[id]
	if !ok {
		return nil, false, nil
	}
	if r.orderSvc == nil || inst.AccountID == "" {
		return nil, true, nil
	}
	resp, err := r.orderSvc.GetStopOrders(ctx, inst.AccountID)
	if err != nil {
		return nil, true, err
	}
	allowed := make(map[string]struct{}, len(inst.Instruments))
	for _, uid := range inst.Instruments {
		allowed[strings.TrimSpace(uid)] = struct{}{}
	}
	out := make([]StopOrderSnapshot, 0, len(resp.GetStopOrders()))
	for _, so := range resp.GetStopOrders() {
		if _, ok := allowed[so.GetInstrumentUid()]; !ok {
			continue
		}
		item := StopOrderSnapshot{
			StopOrderID:   so.GetStopOrderId(),
			InstrumentUID: so.GetInstrumentUid(),
			Direction:     so.GetDirection().String(),
			StopOrderType: so.GetOrderType().String(),
			Lots:          so.GetLotsRequested(),
			StopPrice:     moneyValueToFloat(so.GetStopPrice()),
			Price:         moneyValueToFloat(so.GetPrice()),
			Status:        so.GetStatus().String(),
		}
		if so.GetCreateDate() != nil {
			item.CreatedAt = so.GetCreateDate().AsTime().UTC().Format(time.RFC3339)
		}
		if so.GetExpirationTime() != nil {
			item.ExpirationAt = so.GetExpirationTime().AsTime().UTC().Format(time.RFC3339)
		}
		out = append(out, item)
	}
	return out, true, nil
}

// InstanceIndicatorData returns indicator visualization data if the strategy supports it.
func (r *Runner) InstanceIndicatorData(id string) (interface{}, bool) {
	r.mu.Lock()
	rt := r.instances[id]
	r.mu.Unlock()
	if rt == nil {
		return nil, false
	}
	if ip, ok := rt.strat.(IndicatorProvider); ok { //nolint:misspell // strat is short for strategy
		return ip.IndicatorData(), true
	}
	return nil, false
}

// DailyJournalSummary wraps journal daily aggregation.
func (r *Runner) DailyJournalSummary(ctx context.Context, day time.Time) (journal.DailySummary, error) {
	return r.journal.DailySummary(ctx, day)
}

func moneyValueToFloat(m *pb.MoneyValue) float64 {
	if m == nil {
		return 0
	}
	return float64(m.GetUnits()) + float64(m.GetNano())/1e9
}

// PortfolioSnapshot returns broker portfolio and positions for the given account.
func (r *Runner) PortfolioSnapshot(ctx context.Context, accountID string) (portfolioText string) {
	if r.portfolioSvc == nil || accountID == "" {
		return ""
	}
	info, err := r.portfolioSvc.GetPortfolio(ctx, accountID)
	if err != nil {
		return fmt.Sprintf("(portfolio unavailable: %v)", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Account: %s\n", accountID)
	fmt.Fprintf(&b, "Акции: %.2f₽ | Облигации: %.2f₽ | ETF: %.2f₽ | Валюта: %.2f₽ | Фьючерсы: %.2f₽\n",
		info.TotalAmountShares, info.TotalAmountBonds, info.TotalAmountETF,
		info.TotalAmountCurrencies, info.TotalAmountFutures)
	fmt.Fprintf(&b, "Ожидаемая доходность: %.2f₽\n", info.ExpectedYield)
	if len(info.Positions) > 0 {
		b.WriteString("Позиции брокера:\n")
		for _, p := range info.Positions {
			ticker := p.InstrumentUID[:8]
			if inf, ok := r.instrCache.GetInstrument(p.InstrumentUID); ok && inf.Ticker != "" {
				ticker = inf.Ticker
			}
			if p.Quantity == 0 {
				continue
			}
			fmt.Fprintf(&b, "  %s: %.0f шт, avg=%.2f₽, текущая=%.2f₽, доход=%.2f₽\n",
				ticker, p.Quantity, p.AveragePrice, p.CurrentPrice, p.ExpectedYield)
		}
	} else {
		b.WriteString("Позиции: нет\n")
	}
	return b.String()
}

func (r *Runner) runWatchdog(ctx context.Context) {
	wd := r.strategiesCfg.Watchdog
	if !wd.Enabled {
		return
	}
	interval := time.Duration(wd.CheckIntervalSec) * time.Second
	if interval < 10*time.Second {
		interval = 60 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.tickWatchdog(ctx)
		}
	}
}

func (r *Runner) tickWatchdog(ctx context.Context) {
	r.updateSessionMetrics(time.Now())
	r.updateInstanceMetrics()

	r.mu.Lock()
	ids := make([]string, 0, len(r.instances))
	accounts := make(map[string]struct{})
	for id, rt := range r.instances {
		ids = append(ids, id)
		if rt.account != "" {
			accounts[rt.account] = struct{}{}
		}
	}
	r.mu.Unlock()

	for _, id := range ids {
		if err := r.ReconcileInstance(ctx, id); err != nil {
			r.logger.Warn("watchdog reconcile", "instance", id, "error", err)
		}
		r.maybePauseOnLoss(ctx, id)
	}
	for acc := range accounts {
		r.CheckStuckOrders(ctx, acc)
	}

	// Daily report (UTC date) once per day around 15:50 UTC (~evening MSK)
	now := time.Now().UTC()
	if now.Hour() == 15 && now.Minute() >= 50 {
		day := now.Format("2006-01-02")
		r.mu.Lock()
		sent := r.dailyReportDayUTC
		r.mu.Unlock()
		if sent != day {
			sum, err := r.journal.DailySummary(ctx, now)
			if err == nil && r.tg != nil {
				msg := fmt.Sprintf("strategy-runner daily (UTC %s): signals=%d orders=%d executions=%d",
					sum.DayUTC.Format("2006-01-02"), sum.SignalsCount, sum.OrdersCount, sum.ExecutionsCount)
				_ = r.tg.SendMessage(ctx, msg)
			}
			r.mu.Lock()
			r.dailyReportDayUTC = day
			r.mu.Unlock()
		}
	}
}
