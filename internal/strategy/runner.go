package strategy

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/24alert/trading-bot/internal/journal"
	"github.com/24alert/trading-bot/internal/marketdata"
	"github.com/24alert/trading-bot/internal/order"
	"github.com/24alert/trading-bot/internal/portfolio"
	"github.com/24alert/trading-bot/internal/risk"
	"github.com/24alert/trading-bot/internal/strategy/ledger"
	"github.com/24alert/trading-bot/pkg/config"
	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/metrics"
	"github.com/24alert/trading-bot/pkg/notify/telegram"
	"github.com/24alert/trading-bot/pkg/tinvest"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"
)

// GRPCStrategyBuilder builds a gRPC-backed Strategy for a given endpoint (optional).
type GRPCStrategyBuilder func(endpoint string, timeout time.Duration) (Strategy, error)

// RunnerDeps wires optional factories into the runner.
type RunnerDeps struct {
	GRPC GRPCStrategyBuilder
}

// Runner orchestrates strategy instances: market data → signals → risk → orders.
type Runner struct {
	cfg           *config.Config
	configPath    string
	strategiesCfg config.StrategiesRunnerConfig
	reg           *Registry
	deps          RunnerDeps
	schedule      *TradingSchedule

	tinvestClient *tinvest.Client
	rlm           *tinvest.RateLimiterManager
	mdSvc         *marketdata.Service
	streamMgr     *marketdata.StreamManager
	candleHub     *CandleHub
	instrCache    *marketdata.InstrumentCache
	priceCache    *marketdata.PriceCache

	orderSvc    *order.Service
	orderRepo   *order.Repository
	riskSvc     *risk.Service
	orderStream *order.StreamManager

	logger *logging.Logger

	portfolioSvc *portfolio.Service
	journal      journal.Journal
	ledger       *ledger.Registry
	tg           *telegram.Client

	mu          sync.Mutex
	orderOwners map[string]string // orderID → instance id
	signalRefPx map[string]float64
	lastFilled  map[string]int64 // orderID → last reported cumulative filled qty (lots)
	stopOrders  map[string]string
	aiTrader    *AITraderManager

	equityPeak        map[string]float64 // instance → peak RUB (realized+unrealized)
	dailyReportDayUTC string             // YYYY-MM-DD
	fillWins          map[string]int64
	fillLosses        map[string]int64

	instances map[string]*instanceRuntime
	byID      map[string]config.StrategyInstanceConfig
}

type instanceRuntime struct {
	id      string
	account string
	strat   Strategy //nolint:misspell // short for "strategy"
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewRunner constructs a strategy runner.
func NewRunner(
	cfg *config.Config,
	configPath string,
	strategiesCfg config.StrategiesRunnerConfig,
	reg *Registry,
	deps RunnerDeps,
	client *tinvest.Client,
	rlm *tinvest.RateLimiterManager,
	mdSvc *marketdata.Service,
	streamMgr *marketdata.StreamManager,
	instrCache *marketdata.InstrumentCache,
	priceCache *marketdata.PriceCache,
	orderSvc *order.Service,
	orderRepo *order.Repository,
	riskSvc *risk.Service,
	orderStream *order.StreamManager,
	portfolioSvc *portfolio.Service,
	journ journal.Journal,
	tg *telegram.Client,
	logger *logging.Logger,
) *Runner {
	if journ == nil {
		journ = journal.Noop{}
	}
	sched, err := NewTradingScheduleFromConfig(strategiesCfg.TradingSchedule)
	if err != nil {
		logger.Warn("invalid trading_schedule config, using MOEX defaults", "error", err)
		sched, _ = NewTradingSchedule("", "", "")
	}
	return &Runner{
		cfg:           cfg,
		configPath:    configPath,
		strategiesCfg: strategiesCfg,
		reg:           reg,
		deps:          deps,
		schedule:      sched,
		tinvestClient: client,
		rlm:           rlm,
		mdSvc:         mdSvc,
		streamMgr:     streamMgr,
		candleHub:     NewCandleHub(streamMgr, logger),
		instrCache:    instrCache,
		priceCache:    priceCache,
		orderSvc:      orderSvc,
		orderRepo:     orderRepo,
		riskSvc:       riskSvc,
		orderStream:   orderStream,
		portfolioSvc:  portfolioSvc,
		journal:       journ,
		ledger:        ledger.NewRegistry(),
		tg:            tg,
		logger:        logger,
		orderOwners:   make(map[string]string),
		signalRefPx:   make(map[string]float64),
		lastFilled:    make(map[string]int64),
		stopOrders:    make(map[string]string),
		aiTrader:      NewAITraderManager(),
		equityPeak:    make(map[string]float64),
		fillWins:      make(map[string]int64),
		fillLosses:    make(map[string]int64),
		instances:     make(map[string]*instanceRuntime),
		byID:          make(map[string]config.StrategyInstanceConfig),
	}
}

// Start loads config instances, connects streams, and runs enabled strategies until ctx is cancelled.
func (r *Runner) Start(ctx context.Context) error {
	for _, inst := range r.strategiesCfg.Instances {
		r.byID[inst.ID] = inst
	}
	r.updateSessionMetrics(time.Now())

	if err := r.prefetchInstruments(ctx); err != nil {
		r.logger.Warn("prefetch instruments: some failures", "error", err)
	}
	r.updateInstanceMetrics()

	go func() {
		if err := r.streamMgr.Listen(ctx); err != nil && ctx.Err() == nil {
			r.logger.Error("market stream manager stopped", "error", err)
		}
	}()

	accounts := r.uniqueAccounts(true)
	if len(accounts) > 0 {
		states := r.orderStream.SubscribeOrderStates(512)
		go r.consumeOrderStates(ctx, states)
		go func() {
			if err := r.orderStream.StreamOrderStates(ctx, accounts); err != nil && ctx.Err() == nil {
				r.logger.Error("order state stream stopped", "error", err)
			}
		}()
		go func() {
			if err := r.orderStream.StreamTrades(ctx, accounts); err != nil && ctx.Err() == nil {
				r.logger.Error("trades stream stopped", "error", err)
			}
		}()
	}

	for _, inst := range r.strategiesCfg.Instances {
		if !inst.Enabled {
			continue
		}
		if err := r.startInstance(ctx, inst); err != nil {
			return fmt.Errorf("start instance %q: %w", inst.ID, err)
		}
	}

	go r.runBrokerReconciler(ctx)
	go r.runWatchdog(ctx)

	<-ctx.Done()
	r.StopAll()
	return ctx.Err()
}

func (r *Runner) prefetchInstruments(ctx context.Context) error {
	seen := make(map[string]struct{})
	var firstErr error
	for _, inst := range r.strategiesCfg.Instances {
		for _, uid := range inst.Instruments {
			if _, ok := seen[uid]; ok {
				continue
			}
			seen[uid] = struct{}{}
			resp, err := r.tinvestClient.InstrumentsServiceClient().InstrumentByUid(uid)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				r.logger.Warn("InstrumentByUid failed", "uid", uid, "error", err)
				continue
			}
			in := resp.GetInstrument()
			r.instrCache.SetInstrument(marketdata.InstrumentInfo{
				UID:            in.GetUid(),
				FIGI:           in.GetFigi(),
				Ticker:         in.GetTicker(),
				ClassCode:      in.GetClassCode(),
				Name:           in.GetName(),
				LotSize:        in.GetLot(),
				InstrumentType: in.GetInstrumentType(),
				MinPriceIncr:   quotationToFloat(in.GetMinPriceIncrement()),
				Currency:       in.GetCurrency(),
			})
		}
	}
	return firstErr
}

// InstanceTickers returns comma-separated MOEX tickers for the instance's instruments
// (from InstrumentByUid prefetch). Empty if metadata is not yet in cache.
func (r *Runner) InstanceTickers(inst config.StrategyInstanceConfig) string {
	var parts []string
	for _, uid := range inst.Instruments {
		uid = strings.TrimSpace(uid)
		if uid == "" {
			continue
		}
		if inf, ok := r.instrCache.GetInstrument(uid); ok && inf.Ticker != "" {
			parts = append(parts, inf.Ticker)
		}
	}
	return strings.Join(parts, ", ")
}

func (r *Runner) uniqueAccounts(enabledOnly bool) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, inst := range r.strategiesCfg.Instances {
		if enabledOnly && !inst.Enabled {
			continue
		}
		if inst.AccountID == "" {
			continue
		}
		if _, ok := seen[inst.AccountID]; ok {
			continue
		}
		seen[inst.AccountID] = struct{}{}
		out = append(out, inst.AccountID)
	}
	return out
}

func (r *Runner) evalTimeout() time.Duration {
	ms := r.strategiesCfg.EvaluationTimeoutMS
	if ms <= 0 {
		ms = r.cfg.Strategy.EvaluationTimeoutMs
	}
	if ms <= 0 {
		return 5 * time.Second
	}
	return time.Duration(ms) * time.Millisecond
}

func (r *Runner) buildStrategy(inst config.StrategyInstanceConfig) (Strategy, error) {
	switch inst.Type {
	case "grpc":
		if r.deps.GRPC == nil {
			return nil, fmt.Errorf("grpc strategy %q: no GRPC builder configured", inst.ID)
		}
		if inst.Endpoint == "" {
			return nil, fmt.Errorf("grpc strategy %q: empty endpoint", inst.ID)
		}
		return r.deps.GRPC(inst.Endpoint, r.evalTimeout())
	default:
		return r.reg.Create(inst.Type)
	}
}

func (r *Runner) startInstance(ctx context.Context, inst config.StrategyInstanceConfig) error {
	if err := validateInstanceSafety(inst); err != nil {
		return err
	}
	st, err := r.buildStrategy(inst)
	if err != nil {
		return err
	}
	if err := st.Configure(inst.Params); err != nil {
		st.Stop()
		return fmt.Errorf("Configure: %w", err)
	}
	if ss, ok := st.(StatefulStrategy); ok {
		if blob, err := r.journal.LoadStrategyState(ctx, inst.ID); err == nil && len(blob) > 0 {
			if err := ss.Restore(blob); err != nil {
				r.logger.Warn("strategy state restore failed", "instance", inst.ID, "error", err)
			}
		}
	}

	intervalStr := inst.Params["interval"]
	interval, err := ParseSubscriptionInterval(intervalStr)
	if err != nil {
		st.Stop()
		return err
	}
	if len(inst.Instruments) == 0 {
		st.Stop()
		return fmt.Errorf("instance %q: no instruments", inst.ID)
	}

	// Daily warmup: some strategies (e.g. Level Bounce) need daily bars
	// for S/R levels before the intraday data starts flowing.
	if dwh, ok := st.(DailyWarmupHint); ok {
		r.warmupDaily(ctx, inst, dwh)
	}

	// Warmup: prefetch historical candles so the strategy is ready immediately.
	// Use the larger of WarmupCandles (for trading) and ChartCandles (for visualization).
	var lastWarmupTimes map[string]time.Time
	if wh, ok := st.(WarmupHint); ok {
		needed := wh.WarmupCandles()
		if ch, ok2 := st.(ChartHint); ok2 {
			if cn := ch.ChartCandles(); cn > needed {
				needed = cn
			}
		}
		lastWarmupTimes = r.warmupStrategy(ctx, inst, st, needed, interval)
	}
	if err := r.syncBrokerStateBeforeTrading(ctx, inst, st); err != nil {
		st.Stop()
		return err
	}
	if ss, ok := st.(StatefulStrategy); ok {
		if blob, err := ss.Snapshot(); err == nil {
			_ = r.journal.SaveStrategyState(ctx, inst.ID, blob)
		}
	}

	ictx, cancel := context.WithCancel(ctx)
	rt := &instanceRuntime{id: inst.ID, account: inst.AccountID, strat: st, cancel: cancel} //nolint:misspell // short for "strategy"

	r.mu.Lock()
	r.instances[inst.ID] = rt
	r.mu.Unlock()

	r.primeLiveCandleHandlers(ictx, inst, rt, interval)

	for _, uid := range inst.Instruments {
		uid := uid
		ch, cleanup, err := r.candleHub.Subscribe(ictx, uid, interval)
		if err != nil {
			cancel()
			r.mu.Lock()
			delete(r.instances, inst.ID)
			r.mu.Unlock()
			st.Stop()
			return fmt.Errorf("subscribe candles %s: %w", uid, err)
		}
		warmupCutoff := lastWarmupTimes[uid]
		rt.wg.Add(1)
		go func() {
			defer cleanup()
			defer rt.wg.Done()
			pending := make(map[string]Candle)
			for {
				select {
				case <-ictx.Done():
					return
				case c, ok := <-ch:
					if !ok {
						return
					}
					// Deduplicate completed bars already fed during warmup.
					// In-progress stream candles can share the last warmup bar timestamp
					// after a restart, but protective live handlers still need them.
					if c.IsComplete && !warmupCutoff.IsZero() && !c.Time.After(warmupCutoff) {
						continue
					}
					if lh, ok := st.(LiveCandleHandler); ok && !c.IsComplete {
						start := time.Now()
						sigs := lh.OnLiveCandle(c)
						metrics.StrategyEvaluationDuration.WithLabelValues(inst.ID).Observe(time.Since(start).Seconds())
						ref := c.Close
						if entry, ok := r.priceCache.GetLastPrice(c.InstrumentUID); ok {
							ref = entry.Price
						}
						r.handleSignals(ictx, rt, ref, sigs)
					}
					prev, has := pending[c.InstrumentUID]
					if has && prev.Time.Equal(c.Time) {
						pending[c.InstrumentUID] = c
						continue
					}
					if has {
						pc := prev
						pc.IsComplete = true
						start := time.Now()
						sigs := st.OnCandle(pc)
						metrics.StrategyEvaluationDuration.WithLabelValues(inst.ID).Observe(time.Since(start).Seconds())
						ref := pc.Close
						if entry, ok := r.priceCache.GetLastPrice(pc.InstrumentUID); ok {
							ref = entry.Price
						}
						r.handleSignals(ictx, rt, ref, sigs)
					}
					pending[c.InstrumentUID] = c
				}
			}
		}()
	}

	r.logger.Info("strategy instance started", "id", inst.ID, "type", inst.Type, "account", inst.AccountID)
	r.updateInstanceMetrics()
	return nil
}

func (r *Runner) syncBrokerStateBeforeTrading(ctx context.Context, inst config.StrategyInstanceConfig, st Strategy) error {
	return r.syncBrokerState(ctx, inst, st, "startup broker sync", true)
}

func (r *Runner) syncBrokerState(ctx context.Context, inst config.StrategyInstanceConfig, st Strategy, phase string, recordAll bool) error {
	if r.portfolioSvc == nil {
		r.logger.Warn("broker position sync skipped: portfolio service is not configured", "instance", inst.ID, "phase", phase)
		return nil
	}
	if inst.AccountID == "" {
		return fmt.Errorf("%s %q: empty account_id", phase, inst.ID)
	}
	positions, err := r.portfolioSvc.GetPositions(ctx, inst.AccountID)
	if err != nil {
		return fmt.Errorf("%s %q: %w", phase, inst.ID, err)
	}
	byUID := make(map[string]portfolio.Position, len(positions))
	for _, p := range positions {
		if p.InstrumentUID == "" {
			continue
		}
		byUID[p.InstrumentUID] = p
	}
	led := r.ledger.Ledger(inst.ID)
	syncer, canSyncStrategy := st.(BrokerPositionSyncer)
	for _, uid := range inst.Instruments {
		uid = strings.TrimSpace(uid)
		if uid == "" {
			continue
		}
		p := byUID[uid]
		const tol = 1e-3
		changed := led.ReconcileFromBroker(uid, p.Quantity, p.AveragePrice, tol)
		if changed {
			metrics.StrategyReconcileMismatch.WithLabelValues(inst.ID).Inc()
			r.logger.Warn("ledger reconciled from broker", "instance", inst.ID, "instrument", uid, "phase", phase, "broker_qty", p.Quantity, "broker_avg", p.AveragePrice)
		}
		if canSyncStrategy {
			syncer.SyncBrokerPosition(uid, p.Quantity, p.AveragePrice, p.CurrentPrice)
		} else if p.Quantity != 0 {
			return fmt.Errorf("%s %q: strategy type %q cannot restore non-flat broker position for %s", phase, inst.ID, inst.Type, uid)
		}
		if p.Quantity != 0 {
			r.ensureProtectiveStop(ctx, inst.ID, inst.AccountID, uid, p.Quantity, p.AveragePrice, p.CurrentPrice)
		} else {
			r.cancelTrackedProtectiveStop(ctx, inst.ID, inst.AccountID, uid, "broker flat")
		}
		if recordAll || changed {
			_ = r.journal.RecordEvent(ctx, journal.EventRecord{
				Type:          "broker_position_sync",
				InstanceID:    inst.ID,
				InstrumentUID: uid,
				Status:        "synced",
				Message:       fmt.Sprintf("%s: qty=%.4f avg=%.4f strategy_synced=%t", phase, p.Quantity, p.AveragePrice, canSyncStrategy),
				CreatedAt:     time.Now().UTC(),
			})
		}
	}
	r.updateBizMetrics(inst.ID)
	return nil
}

func (r *Runner) runBrokerReconciler(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mu.Lock()
			runtimes := make([]*instanceRuntime, 0, len(r.instances))
			for _, rt := range r.instances {
				runtimes = append(runtimes, rt)
			}
			r.mu.Unlock()
			for _, rt := range runtimes {
				inst, ok := r.byID[rt.id]
				if !ok {
					continue
				}
				if err := r.syncBrokerState(ctx, inst, rt.strat, "periodic broker sync", false); err != nil {
					r.logger.Warn("periodic broker position sync failed", "instance", rt.id, "error", err)
				}
				if ss, ok := rt.strat.(StatefulStrategy); ok {
					if blob, err := ss.Snapshot(); err == nil && len(blob) > 0 {
						_ = r.journal.SaveStrategyState(ctx, rt.id, blob)
					}
				}
			}
		}
	}
}

func (r *Runner) primeLiveCandleHandlers(ctx context.Context, inst config.StrategyInstanceConfig, rt *instanceRuntime, subInterval pb.SubscriptionInterval) {
	lh, ok := rt.strat.(LiveCandleHandler)
	if !ok {
		return
	}
	candleInterval, err := SubscriptionToCandleInterval(subInterval)
	if err != nil {
		r.logger.Warn("live candle prime: cannot map interval", "instance", inst.ID, "error", err)
		return
	}
	dur := IntervalDuration(subInterval)
	if dur <= 0 {
		dur = time.Hour
	}
	now := time.Now()
	from := now.Add(-2 * dur)
	for _, uid := range inst.Instruments {
		uid = strings.TrimSpace(uid)
		if uid == "" {
			continue
		}
		candles, err := r.mdSvc.GetCandles(ctx, uid, from, now, candleInterval)
		if err != nil {
			r.logger.Warn("live candle prime: GetCandles failed", "instance", inst.ID, "uid", uid, "error", err)
			continue
		}
		if len(candles) == 0 {
			continue
		}
		mc := candles[len(candles)-1]
		sc := Candle{
			InstrumentUID: uid,
			Open:          mc.Open,
			High:          mc.High,
			Low:           mc.Low,
			Close:         mc.Close,
			Volume:        mc.Volume,
			Time:          mc.Time,
			IsComplete:    false,
		}
		start := time.Now()
		sigs := lh.OnLiveCandle(sc)
		metrics.StrategyEvaluationDuration.WithLabelValues(inst.ID).Observe(time.Since(start).Seconds())
		ref := sc.Close
		if entry, ok := r.priceCache.GetLastPrice(sc.InstrumentUID); ok {
			ref = entry.Price
		}
		r.logger.Info("live candle prime complete", "instance", inst.ID, "uid", uid, "time", sc.Time.Format(time.RFC3339), "low", sc.Low, "high", sc.High, "close", sc.Close, "signals", len(sigs))
		r.handleSignals(ctx, rt, ref, sigs)
	}
}

// warmupStrategy fetches historical candles and feeds them to the strategy,
// discarding any generated signals. Returns the timestamp of the last warmup
// candle per instrument for deduplication with the live stream.
func (r *Runner) warmupStrategy(
	ctx context.Context,
	inst config.StrategyInstanceConfig,
	st Strategy,
	needed int,
	subInterval pb.SubscriptionInterval,
) map[string]time.Time {
	if needed <= 0 {
		return nil
	}

	candleInterval, err := SubscriptionToCandleInterval(subInterval)
	if err != nil {
		r.logger.Warn("warmup: cannot map interval", "instance", inst.ID, "error", err)
		return nil
	}

	dur := IntervalDuration(subInterval)
	// Request extra calendar time to account for weekends, nights and clearing
	// gaps. FORTS intraday candles occupy much less than 24h/day, so 20% was
	// not enough to build a useful dashboard history after restart.
	multiplier := 1.5
	if dur <= time.Hour {
		multiplier = 2.0
	}
	lookback := time.Duration(float64(needed)*multiplier+2) * dur
	if dur <= time.Hour {
		lookback += 5 * 24 * time.Hour
	}
	now := time.Now()
	from := now.Add(-lookback)

	result := make(map[string]time.Time)
	for _, uid := range inst.Instruments {
		candles, err := r.mdSvc.GetCandles(ctx, uid, from, now, candleInterval)
		if err != nil {
			r.logger.Warn("warmup: GetCandles failed", "instance", inst.ID, "uid", uid, "error", err)
			continue
		}
		if len(candles) == 0 {
			r.logger.Info("warmup: no historical candles", "instance", inst.ID, "uid", uid)
			continue
		}

		// Take only the last `needed` candles.
		if len(candles) > needed {
			candles = candles[len(candles)-needed:]
		}

		fed := 0
		for _, mc := range candles {
			sc := Candle{
				InstrumentUID: uid,
				Open:          mc.Open,
				High:          mc.High,
				Low:           mc.Low,
				Close:         mc.Close,
				Volume:        mc.Volume,
				Time:          mc.Time,
				IsComplete:    true,
			}
			discardWarmupSignals(st, st.OnCandle(sc)) // fills buffers; signals discarded
			fed++
		}

		last := candles[len(candles)-1].Time
		result[uid] = last
		r.logger.Info("warmup complete",
			"instance", inst.ID, "uid", uid,
			"fed", fed, "needed", needed,
			"last_candle", last.Format(time.RFC3339))
	}
	if pc, ok := st.(PostWarmupCleanup); ok {
		pc.ResetTradingStateAfterWarmup()
	}
	return result
}

func discardWarmupSignals(st Strategy, sigs []Signal) {
	if len(sigs) == 0 {
		return
	}
	if fh, ok := st.(SignalDispatchFailureHandler); ok {
		for _, sig := range sigs {
			fh.OnSignalDispatchFailed(sig, "warmup_discarded")
		}
	}
}

// warmupDaily fetches daily candles and feeds them to the strategy via OnDailyCandle.
func (r *Runner) warmupDaily(ctx context.Context, inst config.StrategyInstanceConfig, dwh DailyWarmupHint) {
	needed := dwh.DailyWarmupCandles()
	if needed <= 0 {
		return
	}
	lookback := time.Duration(float64(needed)*1.5+5) * 24 * time.Hour
	now := time.Now()
	from := now.Add(-lookback)
	for _, uid := range inst.Instruments {
		candles, err := r.mdSvc.GetCandles(ctx, uid, from, now, pb.CandleInterval_CANDLE_INTERVAL_DAY)
		if err != nil {
			r.logger.Warn("daily warmup: GetCandles failed", "instance", inst.ID, "uid", uid, "error", err)
			continue
		}
		if len(candles) > needed {
			candles = candles[len(candles)-needed:]
		}
		for _, mc := range candles {
			dwh.OnDailyCandle(Candle{
				InstrumentUID: uid,
				Open:          mc.Open,
				High:          mc.High,
				Low:           mc.Low,
				Close:         mc.Close,
				Volume:        mc.Volume,
				Time:          mc.Time,
				IsComplete:    true,
			})
		}
		r.logger.Info("daily warmup complete", "instance", inst.ID, "uid", uid, "fed", len(candles))
	}
}

func (r *Runner) estimateCost(sig Signal, markPrice float64) float64 {
	if strings.EqualFold(sig.Direction, "sell") {
		return 0
	}
	inf, ok := r.instrCache.GetInstrument(sig.InstrumentUID)
	if ok && inf.IsFuture() {
		// Futures are margined instruments — real cost equals GO (guarantee obligation).
		// Full GetFuturesMargin integration is a follow-up; for now skip balance check.
		return 0
	}
	lot := int32(1)
	if ok && inf.LotSize > 0 {
		lot = inf.LotSize
	}
	px := markPrice
	if strings.EqualFold(strings.TrimSpace(sig.OrderType), "limit") && sig.Price > 0 {
		px = sig.Price
	}
	return float64(sig.Quantity) * px * float64(lot)
}

func (r *Runner) recordSignalCancellation(ctx context.Context, rt *instanceRuntime, sig Signal, markPrice float64, status, message string) {
	ts := sig.CandleTime
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	if err := r.journal.RecordEvent(ctx, journal.EventRecord{
		Type:          "signal_cancelled",
		InstanceID:    rt.id,
		InstrumentUID: sig.InstrumentUID,
		Direction:     strings.ToLower(strings.TrimSpace(sig.Direction)),
		Quantity:      sig.Quantity,
		OrderType:     strings.TrimSpace(sig.OrderType),
		RefPrice:      markPrice,
		Reason:        sig.Reason,
		Status:        status,
		Message:       message,
		CreatedAt:     ts.UTC(),
	}); err != nil {
		r.logger.Warn("record signal cancellation event failed", "instance", rt.id, "stage", status, "error", err)
	}
	metrics.StrategyEventsTotal.WithLabelValues(rt.id, "signal_cancelled", status, normalizeMetricLabel(sig.Reason)).Inc()
}

func (r *Runner) sessionBlockMessage(now time.Time) string {
	if r.schedule == nil {
		return "signal blocked before order dispatch: trading schedule is not configured"
	}
	local := now.In(r.schedule.tz)
	window := fmt.Sprintf("%s %s", r.schedule.WindowString(), r.schedule.TimezoneName())
	if local.Weekday() == time.Saturday || local.Weekday() == time.Sunday {
		return fmt.Sprintf("weekday-only trading schedule blocked signal: %s is %s; weekend futures trading is intentionally disabled due to low liquidity/thin market. Allowed FORTS sessions: Mon-Fri %s",
			local.Format("2006-01-02 15:04:05"), local.Weekday(), window)
	}
	return fmt.Sprintf("FORTS session schedule blocked signal: %s is outside allowed sessions Mon-Fri %s",
		local.Format("2006-01-02 15:04:05"), window)
}

func (r *Runner) handleSignals(ctx context.Context, rt *instanceRuntime, markPrice float64, sigs []Signal) {
	r.updateSessionMetrics(time.Now())
	for _, sig := range sigs {
		dir := strings.ToLower(strings.TrimSpace(sig.Direction))
		metrics.StrategySignalsTotal.WithLabelValues(rt.id, dir).Inc()

		if r.schedule != nil && !r.schedule.IsMainSession(time.Now()) {
			now := time.Now()
			msg := r.sessionBlockMessage(now)
			r.logger.Info("signal cancelled before order dispatch",
				"instance", rt.id,
				"stage", "session_guard",
				"status", "cancelled",
				"direction", dir,
				"reason", sig.Reason,
				"message", msg,
				"candle_time", sig.CandleTime,
				"checked_at", now.UTC())
			r.recordSignalCancellation(ctx, rt, sig, markPrice, "session_blocked", msg)
			metrics.StrategyOrdersTotal.WithLabelValues(rt.id, "session_blocked").Inc()
			if fh, ok := rt.strat.(SignalDispatchFailureHandler); ok {
				fh.OnSignalDispatchFailed(sig, "session_blocked")
			}
			continue
		}

		rec := journal.SignalRecord{
			InstanceID:    rt.id,
			InstrumentUID: sig.InstrumentUID,
			Direction:     dir,
			Quantity:      sig.Quantity,
			OrderType:     strings.TrimSpace(sig.OrderType),
			RefPrice:      markPrice,
			Reason:        sig.Reason,
		}
		if !sig.CandleTime.IsZero() {
			rec.CreatedAt = sig.CandleTime.UTC()
		}
		_ = r.journal.RecordSignal(ctx, rec)

		intent := risk.OrderIntent{
			AccountID:     rt.account,
			InstrumentUID: sig.InstrumentUID,
			Direction:     dir,
			Quantity:      sig.Quantity,
			EstimatedCost: r.estimateCost(sig, markPrice),
		}
		resp, err := r.riskSvc.ValidateOrderIntent(ctx, intent)
		if err != nil {
			msg := fmt.Sprintf("risk validation failed before order dispatch: %v", err)
			r.logger.Warn("signal cancelled before order dispatch",
				"instance", rt.id, "stage", "risk_error", "status", "cancelled",
				"direction", dir, "reason", sig.Reason, "message", msg, "error", err)
			r.recordSignalCancellation(ctx, rt, sig, markPrice, "risk_error", msg)
			metrics.StrategyOrdersTotal.WithLabelValues(rt.id, "risk_error").Inc()
			if fh, ok := rt.strat.(SignalDispatchFailureHandler); ok { //nolint:misspell // strat is short for strategy
				fh.OnSignalDispatchFailed(sig, "risk_error")
			}
			continue
		}
		if resp == nil || !resp.Allowed {
			msg := fmt.Sprintf("risk rejected signal before order dispatch: %s", formatRiskDenial(resp))
			r.logger.Info("signal cancelled before order dispatch",
				"instance", rt.id, "stage", "risk_rejected", "status", "cancelled",
				"instrument", sig.InstrumentUID, "direction", dir, "reason", sig.Reason, "message", msg)
			r.recordSignalCancellation(ctx, rt, sig, markPrice, "risk_rejected", msg)
			metrics.StrategyOrdersTotal.WithLabelValues(rt.id, "risk_rejected").Inc()
			if fh, ok := rt.strat.(SignalDispatchFailureHandler); ok { //nolint:misspell // strat is short for strategy
				fh.OnSignalDispatchFailed(sig, "risk_rejected")
			}
			continue
		}

		req := buildPostOrderRequest(rt.account, sig)
		postResp, err := r.orderSvc.PostOrder(ctx, req)
		if err != nil {
			msg := fmt.Sprintf("broker/order service rejected order submission after risk approval: %v", err)
			r.logger.Warn("signal cancelled before broker order",
				"instance", rt.id, "stage", "post_error", "status", "cancelled",
				"direction", dir, "reason", sig.Reason, "message", msg, "error", err)
			r.recordSignalCancellation(ctx, rt, sig, markPrice, "post_error", msg)
			metrics.StrategyOrdersTotal.WithLabelValues(rt.id, "post_error").Inc()
			if fh, ok := rt.strat.(SignalDispatchFailureHandler); ok { //nolint:misspell // strat is short for strategy
				fh.OnSignalDispatchFailed(sig, "post_error")
			}
			continue
		}
		oid := postResp.GetOrderId()
		refPx := markPrice
		if strings.EqualFold(strings.TrimSpace(sig.OrderType), "limit") && sig.Price > 0 {
			refPx = sig.Price
		}
		r.mu.Lock()
		r.orderOwners[oid] = rt.id
		r.signalRefPx[oid] = refPx
		r.mu.Unlock()
		_ = r.journal.RecordOrder(ctx, journal.OrderRecord{
			InstanceID:    rt.id,
			OrderID:       oid,
			InstrumentUID: sig.InstrumentUID,
			Direction:     dir,
			Quantity:      sig.Quantity,
			OrderType:     strings.TrimSpace(sig.OrderType),
			RefPrice:      refPx,
		})
		metrics.StrategyOrdersTotal.WithLabelValues(rt.id, "submitted").Inc()
		r.logger.Info("order submitted from strategy", "instance", rt.id, "order_id", oid)
		status := order.MapExecutionStatus(postResp.GetExecutionReportStatus())
		if postResp.GetLotsExecuted() > 0 || isTerminalOrderStatus(status) {
			r.dispatchExecution(order.OrderStateEvent{
				OrderID:   oid,
				AccountID: rt.account,
				Status:    status,
				FilledQty: postResp.GetLotsExecuted(),
				UpdatedAt: time.Now().UTC(),
			})
		}
		go r.pollOrderStateAfterSubmit(ctx, rt, oid)
	}
	if len(sigs) > 0 {
		if ss, ok := rt.strat.(StatefulStrategy); ok { //nolint:misspell // strat is short for strategy
			if blob, err := ss.Snapshot(); err == nil && len(blob) > 0 {
				_ = r.journal.SaveStrategyState(ctx, rt.id, blob)
			}
		}
	}
}

func (r *Runner) consumeOrderStates(ctx context.Context, ch <-chan order.OrderStateEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			r.dispatchExecution(evt)
		}
	}
}

// StopInstance stops a running instance by id.
func (r *Runner) StopInstance(id string) {
	r.mu.Lock()
	rt := r.instances[id]
	r.mu.Unlock()
	if rt == nil {
		return
	}
	rt.cancel()
	rt.wg.Wait()
	if ss, ok := rt.strat.(StatefulStrategy); ok { //nolint:misspell // strat is short for strategy
		if blob, err := ss.Snapshot(); err == nil && len(blob) > 0 {
			_ = r.journal.SaveStrategyState(context.Background(), id, blob)
		}
	}
	rt.strat.Stop() //nolint:misspell // short for "strategy"
	r.ledger.Remove(id)
	r.mu.Lock()
	delete(r.instances, id)
	r.mu.Unlock()
	r.logger.Info("strategy instance stopped", "id", id)
	r.updateInstanceMetrics()
}

// StopAll stops every running instance.
func (r *Runner) StopAll() {
	r.mu.Lock()
	ids := make([]string, 0, len(r.instances))
	for id := range r.instances {
		ids = append(ids, id)
	}
	r.mu.Unlock()
	for _, id := range ids {
		r.StopInstance(id)
	}
}

// ReloadConfig re-reads config.yaml from disk, diffs strategy instances,
// stops removed/changed ones, and starts new/changed ones.
func (r *Runner) ReloadConfig(ctx context.Context) (added, removed, changed int, err error) {
	newCfg, err := config.LoadStrategiesOnly(r.configPath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("reload config: %w", err)
	}

	oldMap := make(map[string]config.StrategyInstanceConfig, len(r.strategiesCfg.Instances))
	for _, inst := range r.strategiesCfg.Instances {
		oldMap[inst.ID] = inst
	}
	newMap := make(map[string]config.StrategyInstanceConfig, len(newCfg.Instances))
	for _, inst := range newCfg.Instances {
		newMap[inst.ID] = inst
	}

	// Stop instances removed from config or whose params changed.
	for id, oldInst := range oldMap {
		newInst, exists := newMap[id]
		if !exists {
			if r.InstanceRunning(id) {
				r.StopInstance(id)
			}
			removed++
			continue
		}
		if instanceChanged(oldInst, newInst) {
			if r.InstanceRunning(id) {
				r.StopInstance(id)
			}
			changed++
		}
	}

	// Prefetch instruments for any new UIDs.
	r.strategiesCfg = *newCfg
	r.mu.Lock()
	r.byID = make(map[string]config.StrategyInstanceConfig, len(newCfg.Instances))
	for _, inst := range newCfg.Instances {
		r.byID[inst.ID] = inst
	}
	r.mu.Unlock()

	if err := r.prefetchInstruments(ctx); err != nil {
		r.logger.Warn("reload: prefetch instruments: some failures", "error", err)
	}
	r.updateInstanceMetrics()

	// Start new or changed instances that are enabled.
	for _, inst := range newCfg.Instances {
		if !inst.Enabled {
			continue
		}
		if r.InstanceRunning(inst.ID) {
			continue
		}
		oldInst, existed := oldMap[inst.ID]
		if !existed {
			added++
		} else if instanceChanged(oldInst, inst) {
			// already counted in changed above
		} else {
			continue // unchanged and already running or disabled
		}
		if err := r.startInstance(ctx, inst); err != nil {
			r.logger.Error("reload: start instance failed", "id", inst.ID, "error", err)
		}
	}

	r.logger.Info("config reloaded", "added", added, "removed", removed, "changed", changed)
	r.updateInstanceMetrics()
	return added, removed, changed, nil
}

func normalizeMetricLabel(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "unknown"
	}
	if len(out) > 64 {
		return out[:64]
	}
	return out
}

func (r *Runner) updateInstanceMetrics() {
	r.mu.Lock()
	byID := make(map[string]config.StrategyInstanceConfig, len(r.byID))
	for id, inst := range r.byID {
		byID[id] = inst
	}
	running := make(map[string]struct{}, len(r.instances))
	for id := range r.instances {
		running[id] = struct{}{}
	}
	r.mu.Unlock()

	for _, inst := range byID {
		ticker := r.InstanceTickers(inst)
		if ticker == "" {
			ticker = strings.Join(inst.Instruments, ",")
		}
		enabled := 0.0
		if inst.Enabled {
			enabled = 1
		}
		isRunning := 0.0
		if _, ok := running[inst.ID]; ok {
			isRunning = 1
		}
		metrics.StrategyInstanceEnabled.WithLabelValues(inst.ID, inst.Type, ticker).Set(enabled)
		metrics.StrategyInstanceRunning.WithLabelValues(inst.ID, inst.Type, ticker).Set(isRunning)
	}
}

func (r *Runner) updateSessionMetrics(now time.Time) {
	if r.schedule == nil {
		metrics.StrategySessionAllowed.Set(0)
		metrics.StrategyNextSessionOpenTimestamp.Set(0)
		return
	}
	if r.schedule.IsMainSession(now) {
		metrics.StrategySessionAllowed.Set(1)
	} else {
		metrics.StrategySessionAllowed.Set(0)
	}
	if next := r.schedule.NextSessionOpen(now); !next.IsZero() {
		metrics.StrategyNextSessionOpenTimestamp.Set(float64(next.Unix()))
	}
}

func instanceChanged(a, b config.StrategyInstanceConfig) bool {
	if a.Type != b.Type || a.AccountID != b.AccountID || a.Enabled != b.Enabled || a.Endpoint != b.Endpoint {
		return true
	}
	if len(a.Instruments) != len(b.Instruments) {
		return true
	}
	for i := range a.Instruments {
		if a.Instruments[i] != b.Instruments[i] {
			return true
		}
	}
	if len(a.Params) != len(b.Params) {
		return true
	}
	for k, v := range a.Params {
		if b.Params[k] != v {
			return true
		}
	}
	return false
}

func validateInstanceSafety(inst config.StrategyInstanceConfig) error {
	switch strings.ToLower(strings.TrimSpace(inst.Type)) {
	case "sma_crossover":
		raw := strings.TrimSpace(inst.Params["trailing_stop_pct"])
		if raw == "" {
			return fmt.Errorf("instance %q refused: sma_crossover live trading requires trailing_stop_pct > 0", inst.ID)
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v <= 0 {
			return fmt.Errorf("instance %q refused: invalid trailing_stop_pct %q, must be > 0", inst.ID, raw)
		}
	case "orb_breakout":
		return fmt.Errorf("instance %q refused: orb_breakout has no protective stop and is blocked for live runner", inst.ID)
	}
	return nil
}

// InstanceIDs returns configured instance ids (from file order).
func (r *Runner) InstanceIDs() []string {
	out := make([]string, 0, len(r.strategiesCfg.Instances))
	for _, inst := range r.strategiesCfg.Instances {
		out = append(out, inst.ID)
	}
	return out
}

// InstanceRunning reports whether an instance is currently running in-memory.
func (r *Runner) InstanceRunning(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.instances[id]
	return ok
}

// StartInstanceByID starts an instance from config if it exists and is not already running.
func (r *Runner) StartInstanceByID(ctx context.Context, id string) error {
	inst, ok := r.byID[id]
	if !ok {
		return fmt.Errorf("unknown instance %q", id)
	}
	if !inst.Enabled {
		return fmt.Errorf("instance %q is disabled in config and cannot be started manually", id)
	}
	r.mu.Lock()
	if _, exists := r.instances[id]; exists {
		r.mu.Unlock()
		return fmt.Errorf("instance %q already running", id)
	}
	r.mu.Unlock()
	return r.startInstance(ctx, inst)
}

// formatRiskDenial builds a short human-readable summary from failed risk checks (for logs).
func formatRiskDenial(resp *risk.RiskResponse) string {
	if resp == nil {
		return "nil_response"
	}
	var b strings.Builder
	for _, c := range resp.Checks {
		if c.Passed {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("; ")
		}
		if c.Name != "" {
			b.WriteString(c.Name)
			b.WriteString(": ")
		}
		b.WriteString(c.Reason)
	}
	if b.Len() == 0 {
		return "denied_no_check_details"
	}
	return b.String()
}
