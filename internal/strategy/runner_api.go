package strategy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/24alert/trading-bot/internal/journal"
	"github.com/24alert/trading-bot/internal/strategy/pnl"
)

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

// InstanceLedgerPositions returns qty (shares) and avg price maps plus cumulative realized RUB.
func (r *Runner) InstanceLedgerPositions(id string) (qty map[string]float64, avg map[string]float64, realized float64, ok bool) {
	if !r.InstanceRunning(id) {
		return nil, nil, 0, false
	}
	q, a, rl := r.ledger.Ledger(id).Snapshot()
	return q, a, rl, true
}

// InstanceRecentExecutions returns persisted executions from the journal.
func (r *Runner) InstanceRecentExecutions(ctx context.Context, id string, limit int) ([]journal.ExecutionRecord, error) {
	return r.journal.ListRecentExecutions(ctx, id, limit)
}

// InstanceRecentSignals returns persisted signals from the journal.
func (r *Runner) InstanceRecentSignals(ctx context.Context, id string, limit int) ([]journal.SignalRecord, error) {
	return r.journal.ListRecentSignals(ctx, id, limit)
}

// InstanceEvents returns a unified timeline of signals, orders, and executions.
func (r *Runner) InstanceEvents(ctx context.Context, id string, limit int) ([]journal.TimelineEvent, error) {
	return r.journal.ListEvents(ctx, id, limit)
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
