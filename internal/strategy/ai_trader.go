package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/24alert/trading-bot/internal/marketdata"
)

const (
	AITraderModeObserve = "observe"
	AITraderModePaper   = "paper"
	AITraderModeLive    = "armed_live"
)

const aiTraderMaxEvents = 200

// AITraderSessionRequest starts a safe observe/paper session. armed_live is
// intentionally rejected until OrderControl and live risk gates are implemented.
type AITraderSessionRequest struct {
	InstanceID  string         `json:"instance_id"`
	Mode        string         `json:"mode"`
	Instruction string         `json:"instruction"`
	Depth       int32          `json:"depth"`
	Limits      AITraderLimits `json:"limits"`
}

type AITraderLimits struct {
	MaxPositionLots           int     `json:"max_position_lots"`
	MaxOrderSize              int     `json:"max_order_size"`
	MaxActiveOrders           int     `json:"max_active_orders"`
	MaxTradesPerMinute        int     `json:"max_trades_per_minute"`
	MaxCancelReplacePerMinute int     `json:"max_cancel_replace_per_minute"`
	MaxSessionLossRUB         float64 `json:"max_session_loss_rub"`
	MaxDailyLossRUB           float64 `json:"max_daily_loss_rub"`
	MaxSpreadBPS              float64 `json:"max_spread_bps"`
	StaleDataMS               int64   `json:"stale_data_ms"`
	SessionTimeoutMinutes     int     `json:"session_timeout_minutes"`
	ObservationIntervalMillis int     `json:"observation_interval_ms"`
}

type AITraderSession struct {
	ID           string                  `json:"id"`
	InstanceID   string                  `json:"instance_id"`
	AccountID    string                  `json:"account_id"`
	InstrumentID string                  `json:"instrument_uid"`
	Ticker       string                  `json:"ticker,omitempty"`
	Mode         string                  `json:"mode"`
	Instruction  string                  `json:"instruction"`
	Limits       AITraderLimits          `json:"limits"`
	Status       string                  `json:"status"`
	StartedAt    string                  `json:"started_at"`
	UpdatedAt    string                  `json:"updated_at"`
	StoppedAt    string                  `json:"stopped_at,omitempty"`
	LastError    string                  `json:"last_error,omitempty"`
	Features     *AITraderFeatures       `json:"features,omitempty"`
	LastDecision *AITraderDecisionEvent  `json:"last_decision,omitempty"`
	Events       []AITraderDecisionEvent `json:"events,omitempty"`

	cancel context.CancelFunc `json:"-"`
}

type AITraderFeatures struct {
	UID               string          `json:"uid"`
	Ticker            string          `json:"ticker,omitempty"`
	ObservedAt        string          `json:"observed_at"`
	ExchangeTime      string          `json:"exchange_time,omitempty"`
	Depth             int32           `json:"depth"`
	BestBid           float64         `json:"best_bid"`
	BestAsk           float64         `json:"best_ask"`
	Mid               float64         `json:"mid"`
	SpreadAbs         float64         `json:"spread_abs"`
	SpreadBPS         float64         `json:"spread_bps"`
	TopBidVolume      int64           `json:"top_bid_volume"`
	TopAskVolume      int64           `json:"top_ask_volume"`
	Imbalance         float64         `json:"imbalance"`
	DepthSkew         float64         `json:"depth_skew"`
	LargestBidWall    AITraderWall    `json:"largest_bid_wall"`
	LargestAskWall    AITraderWall    `json:"largest_ask_wall"`
	DataFreshnessMS   int64           `json:"data_freshness_ms"`
	Stale             bool            `json:"stale"`
	OrderBookSnapshot AITraderBookTop `json:"orderbook_top"`
}

type AITraderBookTop struct {
	Bids []AITraderBookLevel `json:"bids"`
	Asks []AITraderBookLevel `json:"asks"`
}

type AITraderBookLevel struct {
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
}

type AITraderWall struct {
	Side     string  `json:"side"`
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
	Rank     int     `json:"rank"`
}

type AITraderDecisionEvent struct {
	Time       string            `json:"time"`
	SessionID  string            `json:"session_id"`
	Mode       string            `json:"mode"`
	Action     string            `json:"action"`
	Intent     string            `json:"intent"`
	Reason     string            `json:"reason"`
	Confidence float64           `json:"confidence"`
	RiskResult string            `json:"risk_result"`
	Features   *AITraderFeatures `json:"features,omitempty"`
}

type AITraderManager struct {
	mu       sync.Mutex
	sessions map[string]*AITraderSession
}

func NewAITraderManager() *AITraderManager {
	return &AITraderManager{sessions: make(map[string]*AITraderSession)}
}

func defaultAITraderLimits() AITraderLimits {
	return AITraderLimits{
		MaxPositionLots:           1,
		MaxOrderSize:              1,
		MaxActiveOrders:           2,
		MaxTradesPerMinute:        3,
		MaxCancelReplacePerMinute: 10,
		MaxSessionLossRUB:         300,
		MaxDailyLossRUB:           500,
		MaxSpreadBPS:              15,
		StaleDataMS:               1500,
		SessionTimeoutMinutes:     30,
		ObservationIntervalMillis: 2000,
	}
}

func mergeAITraderLimits(in AITraderLimits) AITraderLimits {
	out := defaultAITraderLimits()
	if in.MaxPositionLots > 0 {
		out.MaxPositionLots = in.MaxPositionLots
	}
	if in.MaxOrderSize > 0 {
		out.MaxOrderSize = in.MaxOrderSize
	}
	if in.MaxActiveOrders > 0 {
		out.MaxActiveOrders = in.MaxActiveOrders
	}
	if in.MaxTradesPerMinute > 0 {
		out.MaxTradesPerMinute = in.MaxTradesPerMinute
	}
	if in.MaxCancelReplacePerMinute > 0 {
		out.MaxCancelReplacePerMinute = in.MaxCancelReplacePerMinute
	}
	if in.MaxSessionLossRUB > 0 {
		out.MaxSessionLossRUB = in.MaxSessionLossRUB
	}
	if in.MaxDailyLossRUB > 0 {
		out.MaxDailyLossRUB = in.MaxDailyLossRUB
	}
	if in.MaxSpreadBPS > 0 {
		out.MaxSpreadBPS = in.MaxSpreadBPS
	}
	if in.StaleDataMS > 0 {
		out.StaleDataMS = in.StaleDataMS
	}
	if in.SessionTimeoutMinutes > 0 {
		out.SessionTimeoutMinutes = in.SessionTimeoutMinutes
	}
	if in.ObservationIntervalMillis >= 500 {
		out.ObservationIntervalMillis = in.ObservationIntervalMillis
	}
	return out
}

func (r *Runner) StartAITraderSession(parent context.Context, req AITraderSessionRequest) (*AITraderSession, error) {
	mode := strings.TrimSpace(strings.ToLower(req.Mode))
	if mode == "" {
		mode = AITraderModeObserve
	}
	if mode == AITraderModeLive {
		return nil, fmt.Errorf("armed_live is disabled: implement OrderControl and live RiskGate first")
	}
	if mode != AITraderModeObserve && mode != AITraderModePaper {
		return nil, fmt.Errorf("unsupported ai trader mode %q", req.Mode)
	}
	inst, ok := r.byID[strings.TrimSpace(req.InstanceID)]
	if !ok {
		return nil, fmt.Errorf("unknown instance %q", req.InstanceID)
	}
	if len(inst.Instruments) != 1 {
		return nil, fmt.Errorf("ai trader requires exactly one selected futures instrument")
	}
	uid := strings.TrimSpace(inst.Instruments[0])
	if uid == "" {
		return nil, fmt.Errorf("instance %q has empty instrument uid", inst.ID)
	}
	limits := mergeAITraderLimits(req.Limits)
	depth := req.Depth
	switch {
	case depth <= 0:
		depth = 20
	case depth <= 10:
		depth = 10
	case depth <= 20:
		depth = 20
	case depth <= 30:
		depth = 30
	case depth <= 40:
		depth = 40
	default:
		depth = 50
	}
	ctx, cancel := context.WithCancel(parent)
	if limits.SessionTimeoutMinutes > 0 {
		ctx, cancel = context.WithTimeout(parent, time.Duration(limits.SessionTimeoutMinutes)*time.Minute)
	}
	now := time.Now().UTC()
	id := fmt.Sprintf("ai-trader-%s-%s", inst.ID, now.Format("20060102-150405"))
	s := &AITraderSession{
		ID:           id,
		InstanceID:   inst.ID,
		AccountID:    inst.AccountID,
		InstrumentID: uid,
		Ticker:       r.InstanceTickers(inst),
		Mode:         mode,
		Instruction:  strings.TrimSpace(req.Instruction),
		Limits:       limits,
		Status:       "running",
		StartedAt:    now.Format(time.RFC3339),
		UpdatedAt:    now.Format(time.RFC3339),
		cancel:       cancel,
	}
	r.aiTrader.mu.Lock()
	if old := r.aiTrader.sessions[inst.ID]; old != nil && old.cancel != nil && old.Status == "running" {
		old.cancel()
		old.Status = "stopped"
		old.StoppedAt = now.Format(time.RFC3339)
		old.UpdatedAt = now.Format(time.RFC3339)
	}
	r.aiTrader.sessions[inst.ID] = s
	r.aiTrader.mu.Unlock()

	r.appendAITraderEvent(AITraderDecisionEvent{
		Time:       now.Format(time.RFC3339),
		SessionID:  id,
		Mode:       mode,
		Action:     "start",
		Intent:     "observe_market",
		Reason:     "operator started safe AI Trader session without live order permissions",
		Confidence: 1,
		RiskResult: "live_orders_disabled",
	})
	go r.runAITraderSession(ctx, s, depth)
	return cloneAITraderSession(s), nil
}

func (r *Runner) StopAITraderSession(instanceID string) (*AITraderSession, bool) {
	r.aiTrader.mu.Lock()
	defer r.aiTrader.mu.Unlock()
	s := r.aiTrader.sessions[instanceID]
	if s == nil {
		return nil, false
	}
	if s.cancel != nil && s.Status == "running" {
		s.cancel()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	s.Status = "stopped"
	s.StoppedAt = now
	s.UpdatedAt = now
	return cloneAITraderSession(s), true
}

func (r *Runner) AITraderSessions() []*AITraderSession {
	r.aiTrader.mu.Lock()
	defer r.aiTrader.mu.Unlock()
	out := make([]*AITraderSession, 0, len(r.aiTrader.sessions))
	for _, s := range r.aiTrader.sessions {
		out = append(out, cloneAITraderSession(s))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt > out[j].StartedAt })
	return out
}

func (r *Runner) AITraderSession(instanceID string) (*AITraderSession, bool) {
	r.aiTrader.mu.Lock()
	defer r.aiTrader.mu.Unlock()
	s := r.aiTrader.sessions[instanceID]
	if s == nil {
		return nil, false
	}
	return cloneAITraderSession(s), true
}

func (r *Runner) runAITraderSession(ctx context.Context, s *AITraderSession, depth int32) {
	interval := time.Duration(s.Limits.ObservationIntervalMillis) * time.Millisecond
	if interval < 500*time.Millisecond {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	r.observeAITraderOnce(ctx, s, depth)
	for {
		select {
		case <-ctx.Done():
			r.finishAITraderSession(s, ctx.Err())
			return
		case <-ticker.C:
			r.observeAITraderOnce(ctx, s, depth)
		}
	}
}

func (r *Runner) observeAITraderOnce(ctx context.Context, s *AITraderSession, depth int32) {
	if r.mdSvc == nil {
		r.updateAITraderError(s, "market data service is not configured")
		return
	}
	reqCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	book, err := r.mdSvc.GetOrderbook(reqCtx, s.InstrumentID, depth)
	if err != nil {
		r.updateAITraderError(s, err.Error())
		return
	}
	features := computeAITraderFeatures(book, s.Ticker, s.Limits.StaleDataMS)
	decision := decideAITrader(s, features)
	r.updateAITraderState(s, features, decision)
	r.appendAITraderEvent(decision)
}

func computeAITraderFeatures(book *marketdata.Orderbook, ticker string, staleMS int64) *AITraderFeatures {
	now := time.Now().UTC()
	f := &AITraderFeatures{
		UID:          book.InstrumentUID,
		Ticker:       ticker,
		ObservedAt:   now.Format(time.RFC3339),
		Depth:        book.Depth,
		ExchangeTime: book.Time.UTC().Format(time.RFC3339),
	}
	if !book.Time.IsZero() {
		f.DataFreshnessMS = now.Sub(book.Time).Milliseconds()
		f.Stale = staleMS > 0 && f.DataFreshnessMS > staleMS
	}
	if len(book.Bids) > 0 {
		f.BestBid = book.Bids[0].Price
	}
	if len(book.Asks) > 0 {
		f.BestAsk = book.Asks[0].Price
	}
	if f.BestBid > 0 && f.BestAsk > 0 {
		f.Mid = (f.BestBid + f.BestAsk) / 2
		f.SpreadAbs = f.BestAsk - f.BestBid
		if f.Mid > 0 {
			f.SpreadBPS = f.SpreadAbs / f.Mid * 10000
		}
	}
	bids := firstBookRows(book.Bids, 5)
	asks := firstBookRows(book.Asks, 5)
	f.OrderBookSnapshot = AITraderBookTop{Bids: toAITraderBookLevels(bids), Asks: toAITraderBookLevels(asks)}
	f.TopBidVolume = sumBookQty(bids)
	f.TopAskVolume = sumBookQty(asks)
	total := f.TopBidVolume + f.TopAskVolume
	if total > 0 {
		f.Imbalance = float64(f.TopBidVolume-f.TopAskVolume) / float64(total)
	}
	if f.TopAskVolume > 0 {
		f.DepthSkew = float64(f.TopBidVolume) / float64(f.TopAskVolume)
	}
	f.LargestBidWall = largestWall("bid", bids)
	f.LargestAskWall = largestWall("ask", asks)
	return f
}

func decideAITrader(s *AITraderSession, f *AITraderFeatures) AITraderDecisionEvent {
	action := "hold"
	intent := "observe"
	reason := "no actionable microstructure edge; live orders are disabled"
	confidence := 0.45
	risk := "allowed_observe_only"

	switch {
	case f.Stale:
		action = "block"
		intent = "wait_for_fresh_feed"
		reason = fmt.Sprintf("orderbook is stale: %dms > limit %dms", f.DataFreshnessMS, s.Limits.StaleDataMS)
		confidence = 0.95
		risk = "blocked_stale_feed"
	case s.Limits.MaxSpreadBPS > 0 && f.SpreadBPS > s.Limits.MaxSpreadBPS:
		action = "hold"
		intent = "avoid_wide_spread"
		reason = fmt.Sprintf("spread %.2fbps exceeds limit %.2fbps", f.SpreadBPS, s.Limits.MaxSpreadBPS)
		confidence = 0.85
		risk = "blocked_spread"
	case f.Imbalance > 0.35:
		action = "paper_plan"
		intent = "bid_pressure_passive_long_watch"
		reason = fmt.Sprintf("top depth imbalance %.2f favors bids; observe pull/add before any entry", f.Imbalance)
		confidence = math.Min(0.9, 0.5+math.Abs(f.Imbalance)/2)
	case f.Imbalance < -0.35:
		action = "paper_plan"
		intent = "ask_pressure_passive_short_watch"
		reason = fmt.Sprintf("top depth imbalance %.2f favors asks; observe prints confirmation before any entry", f.Imbalance)
		confidence = math.Min(0.9, 0.5+math.Abs(f.Imbalance)/2)
	case f.LargestBidWall.Quantity > 0 || f.LargestAskWall.Quantity > 0:
		action = "observe_wall"
		intent = "track_liquidity_wall"
		reason = "large visible wall detected; track persistence and pull/add behavior"
		confidence = 0.55
	}
	if s.Mode == AITraderModeObserve && action == "paper_plan" {
		action = "observe_plan"
	}
	return AITraderDecisionEvent{
		Time:       time.Now().UTC().Format(time.RFC3339),
		SessionID:  s.ID,
		Mode:       s.Mode,
		Action:     action,
		Intent:     intent,
		Reason:     reason,
		Confidence: confidence,
		RiskResult: risk,
		Features:   f,
	}
}

func firstBookRows(rows []marketdata.OrderbookRow, n int) []marketdata.OrderbookRow {
	if len(rows) < n {
		n = len(rows)
	}
	out := make([]marketdata.OrderbookRow, n)
	copy(out, rows[:n])
	return out
}

func toAITraderBookLevels(rows []marketdata.OrderbookRow) []AITraderBookLevel {
	out := make([]AITraderBookLevel, len(rows))
	for i, row := range rows {
		out[i] = AITraderBookLevel{Price: row.Price, Quantity: row.Quantity}
	}
	return out
}

func sumBookQty(rows []marketdata.OrderbookRow) int64 {
	var total int64
	for _, row := range rows {
		total += row.Quantity
	}
	return total
}

func largestWall(side string, rows []marketdata.OrderbookRow) AITraderWall {
	var out AITraderWall
	out.Side = side
	for i, row := range rows {
		if row.Quantity > out.Quantity {
			out.Price = row.Price
			out.Quantity = row.Quantity
			out.Rank = i + 1
		}
	}
	return out
}

func (r *Runner) updateAITraderState(s *AITraderSession, f *AITraderFeatures, ev AITraderDecisionEvent) {
	r.aiTrader.mu.Lock()
	defer r.aiTrader.mu.Unlock()
	cur := r.aiTrader.sessions[s.InstanceID]
	if cur == nil || cur.ID != s.ID {
		return
	}
	cur.Features = f
	cur.LastDecision = &ev
	cur.Events = append([]AITraderDecisionEvent{ev}, cur.Events...)
	if len(cur.Events) > aiTraderMaxEvents {
		cur.Events = cur.Events[:aiTraderMaxEvents]
	}
	cur.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	cur.LastError = ""
}

func (r *Runner) updateAITraderError(s *AITraderSession, msg string) {
	r.aiTrader.mu.Lock()
	defer r.aiTrader.mu.Unlock()
	cur := r.aiTrader.sessions[s.InstanceID]
	if cur == nil || cur.ID != s.ID {
		return
	}
	cur.LastError = msg
	cur.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (r *Runner) finishAITraderSession(s *AITraderSession, err error) {
	r.aiTrader.mu.Lock()
	defer r.aiTrader.mu.Unlock()
	cur := r.aiTrader.sessions[s.InstanceID]
	if cur == nil || cur.ID != s.ID || cur.Status != "running" {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	cur.Status = "stopped"
	cur.StoppedAt = now
	cur.UpdatedAt = now
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		cur.LastError = err.Error()
	}
}

func cloneAITraderSession(s *AITraderSession) *AITraderSession {
	if s == nil {
		return nil
	}
	cp := *s
	cp.cancel = nil
	if s.Events != nil {
		cp.Events = append([]AITraderDecisionEvent(nil), s.Events...)
	}
	if s.Features != nil {
		f := *s.Features
		f.OrderBookSnapshot.Bids = append([]AITraderBookLevel(nil), s.Features.OrderBookSnapshot.Bids...)
		f.OrderBookSnapshot.Asks = append([]AITraderBookLevel(nil), s.Features.OrderBookSnapshot.Asks...)
		cp.Features = &f
	}
	if s.LastDecision != nil {
		ev := *s.LastDecision
		cp.LastDecision = &ev
	}
	return &cp
}

func (r *Runner) appendAITraderEvent(ev AITraderDecisionEvent) {
	path := strings.TrimSpace(os.Getenv("AI_TRADER_JOURNAL_PATH"))
	if path == "" {
		path = "data/ai_trader_journal.jsonl"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		r.logger.Warn("ai trader journal dir", "error", err)
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		r.logger.Warn("ai trader journal open", "error", err)
		return
	}
	defer f.Close()
	line, err := json.Marshal(ev)
	if err != nil {
		r.logger.Warn("ai trader journal marshal", "error", err)
		return
	}
	_, _ = f.Write(append(line, '\n'))
}
