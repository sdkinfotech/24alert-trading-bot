package strategy

import (
	"context"
	"fmt"
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
	return &Runner{
		cfg:           cfg,
		configPath:    configPath,
		strategiesCfg: strategiesCfg,
		reg:           reg,
		deps:          deps,
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

	if err := r.prefetchInstruments(ctx); err != nil {
		r.logger.Warn("prefetch instruments: some failures", "error", err)
	}

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
				UID:       in.GetUid(),
				FIGI:      in.GetFigi(),
				Ticker:    in.GetTicker(),
				ClassCode: in.GetClassCode(),
				Name:      in.GetName(),
				LotSize:   in.GetLot(),
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

	ictx, cancel := context.WithCancel(ctx)
	rt := &instanceRuntime{id: inst.ID, account: inst.AccountID, strat: st, cancel: cancel} //nolint:misspell // short for "strategy"

	r.mu.Lock()
	r.instances[inst.ID] = rt
	r.mu.Unlock()

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
					// Deduplicate: skip candles already fed during warmup.
					if !warmupCutoff.IsZero() && !c.Time.After(warmupCutoff) {
						continue
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
	return nil
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
	// Request 20% extra to account for gaps (weekends, non-trading hours).
	lookback := time.Duration(float64(needed)*1.2+2) * dur
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
			_ = st.OnCandle(sc) // fills buffers; signals discarded
			fed++
		}

		last := candles[len(candles)-1].Time
		result[uid] = last
		r.logger.Info("warmup complete",
			"instance", inst.ID, "uid", uid,
			"fed", fed, "needed", needed,
			"last_candle", last.Format(time.RFC3339))
	}
	return result
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
	lot := int32(1)
	if inf, ok := r.instrCache.GetInstrument(sig.InstrumentUID); ok && inf.LotSize > 0 {
		lot = inf.LotSize
	}
	px := markPrice
	if strings.EqualFold(strings.TrimSpace(sig.OrderType), "limit") && sig.Price > 0 {
		px = sig.Price
	}
	return float64(sig.Quantity) * px * float64(lot)
}

func (r *Runner) handleSignals(ctx context.Context, rt *instanceRuntime, markPrice float64, sigs []Signal) {
	for _, sig := range sigs {
		dir := strings.ToLower(strings.TrimSpace(sig.Direction))
		metrics.StrategySignalsTotal.WithLabelValues(rt.id, dir).Inc()

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
			r.logger.Warn("risk validation error", "instance", rt.id, "error", err)
			metrics.StrategyOrdersTotal.WithLabelValues(rt.id, "risk_error").Inc()
			continue
		}
		if resp == nil || !resp.Allowed {
			r.logger.Info("risk rejected signal", "instance", rt.id, "instrument", sig.InstrumentUID)
			metrics.StrategyOrdersTotal.WithLabelValues(rt.id, "risk_rejected").Inc()
			continue
		}

		req := buildPostOrderRequest(rt.account, sig)
		postResp, err := r.orderSvc.PostOrder(ctx, req)
		if err != nil {
			r.logger.Warn("PostOrder failed", "instance", rt.id, "error", err)
			metrics.StrategyOrdersTotal.WithLabelValues(rt.id, "post_error").Inc()
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
	return added, removed, changed, nil
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
	r.mu.Lock()
	if _, exists := r.instances[id]; exists {
		r.mu.Unlock()
		return fmt.Errorf("instance %q already running", id)
	}
	r.mu.Unlock()
	return r.startInstance(ctx, inst)
}
