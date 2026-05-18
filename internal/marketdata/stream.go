package marketdata

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/pkg/config"
	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/tinvest"
	"github.com/russianinvestments/invest-api-go-sdk/investgo"
)

// SubscriptionType identifies what kind of market data subscription this is.
type SubscriptionType int

const (
	SubCandles SubscriptionType = iota
	SubOrderbook
	SubTrades
	SubLastPrice
)

// candleStreamWaitClose must match T-Invest SubscribeCandle(waiting_close).
// false streams every in-bar tick (high volume); our fan-out used non-blocking sends and could
// drop candles, leaving the strategy chart hours behind the exchange.
// true emits one candle per interval when the bar has closed (correct for strategy OnCandle).
const candleStreamWaitClose = true

func (s SubscriptionType) String() string {
	switch s {
	case SubCandles:
		return "candles"
	case SubOrderbook:
		return "orderbook"
	case SubTrades:
		return "trades"
	case SubLastPrice:
		return "last_price"
	default:
		return "unknown"
	}
}

// subscriptionKey uniquely identifies a subscription.
type subscriptionKey struct {
	InstrumentUID    string
	SubscriptionType SubscriptionType
	// CandleInterval is set for SubCandles (must not be UNSPECIFIED).
	CandleInterval pb.SubscriptionInterval
	// OrderbookDepth is set for SubOrderbook (must match SubscribeOrderbook depth).
	OrderbookDepth int32
}

// StreamManager manages MarketDataStream connections to T-Invest.
// It multiplexes subscriptions across a shared stream and handles reconnection.
type StreamManager struct {
	tinvestClient *tinvest.Client
	prices        *PriceCache
	logger        *logging.Logger
	streamCfg     config.MarketDataStreamConfig

	mu            sync.Mutex
	subscriptions map[subscriptionKey]*streamSubscription
	stream        *investgo.MarketDataStream
}

type streamSubscription struct {
	key    subscriptionKey
	refs   int
	ctx    context.Context
	cancel context.CancelFunc

	candles    *streamFanout[*pb.Candle]
	orderbooks *streamFanout[*pb.OrderBook]
	trades     *streamFanout[*pb.Trade]
	lastPrices *streamFanout[*pb.LastPrice]
}

type streamFanout[T any] struct {
	mu     sync.Mutex
	subs   map[chan T]struct{}
	closed bool
	drops  uint64
}

func newStreamFanout[T any]() *streamFanout[T] {
	return &streamFanout[T]{subs: make(map[chan T]struct{})}
}

func (f *streamFanout[T]) subscribe(ctx context.Context, buf int) (<-chan T, error) {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil, fmt.Errorf("stream fanout closed")
	}
	ch := make(chan T, buf)
	f.subs[ch] = struct{}{}
	f.mu.Unlock()

	go func() {
		<-ctx.Done()
		f.mu.Lock()
		delete(f.subs, ch)
		f.mu.Unlock()
	}()

	return ch, nil
}

func (f *streamFanout[T]) close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	f.subs = make(map[chan T]struct{})
}

func (f *streamFanout[T]) publish(ctx context.Context, item T, block bool) bool {
	f.mu.Lock()
	subs := make([]chan T, 0, len(f.subs))
	for ch := range f.subs {
		subs = append(subs, ch)
	}
	f.mu.Unlock()

	for _, ch := range subs {
		if block {
			select {
			case <-ctx.Done():
				return false
			case ch <- item:
			}
			continue
		}

		select {
		case <-ctx.Done():
			return false
		case ch <- item:
		default:
			f.mu.Lock()
			f.drops++
			f.mu.Unlock()
		}
	}

	return true
}

func NewStreamManager(
	client *tinvest.Client,
	prices *PriceCache,
	streamCfg config.MarketDataStreamConfig,
	logger *logging.Logger,
) *StreamManager {
	return &StreamManager{
		tinvestClient: client,
		prices:        prices,
		logger:        logger,
		streamCfg:     streamCfg,
		subscriptions: make(map[subscriptionKey]*streamSubscription),
	}
}

// SubscribeCandles subscribes to streaming candles for the given instrument.
// Returns a receive channel the caller reads from.
func (sm *StreamManager) SubscribeCandles(ctx context.Context, instrumentUID string, interval pb.SubscriptionInterval) (<-chan *pb.Candle, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.ensureStream(); err != nil {
		return nil, fmt.Errorf("SubscribeCandles: %w", err)
	}

	if interval == pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_UNSPECIFIED {
		return nil, fmt.Errorf("SubscribeCandles: subscription interval must be specified")
	}
	key := subscriptionKey{
		InstrumentUID:    instrumentUID,
		SubscriptionType: SubCandles,
		CandleInterval:   interval,
	}
	if sub, exists := sm.subscriptions[key]; exists {
		sub.refs++
		ch, err := sub.candles.subscribe(ctx, 256)
		if err != nil {
			sub.refs--
			return nil, fmt.Errorf("SubscribeCandles: %w", err)
		}
		sm.logger.Info("SubscribeCandles shared", "instrument_uid", instrumentUID, "interval", interval.String(), "refs", sub.refs, "total_subs", len(sm.subscriptions))
		return ch, nil
	}
	if len(sm.subscriptions) >= sm.maxSubscriptions() {
		return nil, fmt.Errorf("SubscribeCandles: max subscriptions (%d) reached", sm.maxSubscriptions())
	}

	ch, err := sm.stream.SubscribeCandle([]string{instrumentUID}, interval, candleStreamWaitClose, nil)
	if err != nil {
		return nil, fmt.Errorf("SubscribeCandles: %w", err)
	}

	subCtx, cancel := context.WithCancel(context.Background())
	hub := newStreamFanout[*pb.Candle]()
	out, err := hub.subscribe(ctx, 256)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("SubscribeCandles: %w", err)
	}
	sm.subscriptions[key] = &streamSubscription{
		key:     key,
		refs:    1,
		ctx:     subCtx,
		cancel:  cancel,
		candles: hub,
	}

	go sm.forwardCandles(subCtx, ch, hub)

	sm.logger.Info("SubscribeCandles", "instrument_uid", instrumentUID, "interval", interval.String(), "total_subs", len(sm.subscriptions))
	return out, nil
}

// SubscribeOrderbook subscribes to streaming order book for the given instrument.
func (sm *StreamManager) SubscribeOrderbook(ctx context.Context, instrumentUID string, depth int32) (<-chan *pb.OrderBook, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.ensureStream(); err != nil {
		return nil, fmt.Errorf("SubscribeOrderbook: %w", err)
	}

	if depth <= 0 {
		depth = 10
	}
	key := subscriptionKey{
		InstrumentUID:    instrumentUID,
		SubscriptionType: SubOrderbook,
		OrderbookDepth:   depth,
	}
	if sub, exists := sm.subscriptions[key]; exists {
		sub.refs++
		ch, err := sub.orderbooks.subscribe(ctx, 64)
		if err != nil {
			sub.refs--
			return nil, fmt.Errorf("SubscribeOrderbook: %w", err)
		}
		sm.logger.Info("SubscribeOrderbook shared", "instrument_uid", instrumentUID, "depth", depth, "refs", sub.refs, "total_subs", len(sm.subscriptions))
		return ch, nil
	}
	if len(sm.subscriptions) >= sm.maxSubscriptions() {
		return nil, fmt.Errorf("SubscribeOrderbook: max subscriptions (%d) reached", sm.maxSubscriptions())
	}

	ch, err := sm.stream.SubscribeOrderBook([]string{instrumentUID}, depth)
	if err != nil {
		return nil, fmt.Errorf("SubscribeOrderbook: %w", err)
	}

	subCtx, cancel := context.WithCancel(context.Background())
	hub := newStreamFanout[*pb.OrderBook]()
	out, err := hub.subscribe(ctx, 64)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("SubscribeOrderbook: %w", err)
	}
	sm.subscriptions[key] = &streamSubscription{
		key:        key,
		refs:       1,
		ctx:        subCtx,
		cancel:     cancel,
		orderbooks: hub,
	}

	go sm.forwardOrderbooks(subCtx, ch, hub)

	sm.logger.Info("SubscribeOrderbook", "instrument_uid", instrumentUID, "depth", depth, "total_subs", len(sm.subscriptions))
	return out, nil
}

// SubscribeTrades subscribes to streaming trades for the given instrument.
func (sm *StreamManager) SubscribeTrades(ctx context.Context, instrumentUID string) (<-chan *pb.Trade, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.ensureStream(); err != nil {
		return nil, fmt.Errorf("SubscribeTrades: %w", err)
	}

	key := subscriptionKey{InstrumentUID: instrumentUID, SubscriptionType: SubTrades}
	if sub, exists := sm.subscriptions[key]; exists {
		sub.refs++
		ch, err := sub.trades.subscribe(ctx, 64)
		if err != nil {
			sub.refs--
			return nil, fmt.Errorf("SubscribeTrades: %w", err)
		}
		sm.logger.Info("SubscribeTrades shared", "instrument_uid", instrumentUID, "refs", sub.refs, "total_subs", len(sm.subscriptions))
		return ch, nil
	}
	if len(sm.subscriptions) >= sm.maxSubscriptions() {
		return nil, fmt.Errorf("SubscribeTrades: max subscriptions (%d) reached", sm.maxSubscriptions())
	}

	ch, err := sm.stream.SubscribeTrade([]string{instrumentUID}, pb.TradeSourceType_TRADE_SOURCE_UNSPECIFIED, false)
	if err != nil {
		return nil, fmt.Errorf("SubscribeTrades: %w", err)
	}

	subCtx, cancel := context.WithCancel(context.Background())
	hub := newStreamFanout[*pb.Trade]()
	out, err := hub.subscribe(ctx, 64)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("SubscribeTrades: %w", err)
	}
	sm.subscriptions[key] = &streamSubscription{
		key:    key,
		refs:   1,
		ctx:    subCtx,
		cancel: cancel,
		trades: hub,
	}

	go sm.forwardTrades(subCtx, ch, hub)

	sm.logger.Info("SubscribeTrades", "instrument_uid", instrumentUID, "total_subs", len(sm.subscriptions))
	return out, nil
}

// SubscribeLastPrice subscribes to streaming last prices for the given instrument.
func (sm *StreamManager) SubscribeLastPrice(ctx context.Context, instrumentUID string) (<-chan *pb.LastPrice, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.ensureStream(); err != nil {
		return nil, fmt.Errorf("SubscribeLastPrice: %w", err)
	}

	key := subscriptionKey{InstrumentUID: instrumentUID, SubscriptionType: SubLastPrice}
	if sub, exists := sm.subscriptions[key]; exists {
		sub.refs++
		ch, err := sub.lastPrices.subscribe(ctx, 64)
		if err != nil {
			sub.refs--
			return nil, fmt.Errorf("SubscribeLastPrice: %w", err)
		}
		sm.logger.Info("SubscribeLastPrice shared", "instrument_uid", instrumentUID, "refs", sub.refs, "total_subs", len(sm.subscriptions))
		return ch, nil
	}
	if len(sm.subscriptions) >= sm.maxSubscriptions() {
		return nil, fmt.Errorf("SubscribeLastPrice: max subscriptions (%d) reached", sm.maxSubscriptions())
	}

	ch, err := sm.stream.SubscribeLastPrice([]string{instrumentUID})
	if err != nil {
		return nil, fmt.Errorf("SubscribeLastPrice: %w", err)
	}

	subCtx, cancel := context.WithCancel(context.Background())
	hub := newStreamFanout[*pb.LastPrice]()
	out, err := hub.subscribe(ctx, 64)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("SubscribeLastPrice: %w", err)
	}
	sm.subscriptions[key] = &streamSubscription{
		key:        key,
		refs:       1,
		ctx:        subCtx,
		cancel:     cancel,
		lastPrices: hub,
	}

	go sm.forwardLastPrices(subCtx, ch, hub)

	sm.logger.Info("SubscribeLastPrice", "instrument_uid", instrumentUID, "total_subs", len(sm.subscriptions))
	return out, nil
}

// UnsubscribeCandles removes a candle subscription for the given instrument and interval.
func (sm *StreamManager) UnsubscribeCandles(instrumentUID string, interval pb.SubscriptionInterval) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := subscriptionKey{
		InstrumentUID:    instrumentUID,
		SubscriptionType: SubCandles,
		CandleInterval:   interval,
	}
	sub, exists := sm.subscriptions[key]
	if !exists {
		return fmt.Errorf("UnsubscribeCandles: no subscription for %s interval %s", instrumentUID, interval.String())
	}
	sub.refs--
	if sub.refs > 0 {
		sm.logger.Info("UnsubscribeCandles shared", "instrument_uid", instrumentUID, "interval", interval.String(), "refs", sub.refs, "total_subs", len(sm.subscriptions))
		return nil
	}

	if sm.stream != nil {
		if err := sm.stream.UnSubscribeCandle([]string{instrumentUID}, interval, candleStreamWaitClose, nil); err != nil {
			return fmt.Errorf("UnsubscribeCandles: %w", err)
		}
	}

	sub.cancel()
	if sub.candles != nil {
		sub.candles.close()
	}
	delete(sm.subscriptions, key)
	sm.logger.Info("UnsubscribeCandles", "instrument_uid", instrumentUID, "interval", interval.String(), "total_subs", len(sm.subscriptions))
	return nil
}

// UnsubscribeOrderbook removes an order book subscription for the given instrument and depth.
func (sm *StreamManager) UnsubscribeOrderbook(instrumentUID string, depth int32) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if depth <= 0 {
		depth = 10
	}
	key := subscriptionKey{
		InstrumentUID:    instrumentUID,
		SubscriptionType: SubOrderbook,
		OrderbookDepth:   depth,
	}
	sub, exists := sm.subscriptions[key]
	if !exists {
		return fmt.Errorf("UnsubscribeOrderbook: no subscription for %s depth %d", instrumentUID, depth)
	}
	sub.refs--
	if sub.refs > 0 {
		sm.logger.Info("UnsubscribeOrderbook shared", "instrument_uid", instrumentUID, "depth", depth, "refs", sub.refs, "total_subs", len(sm.subscriptions))
		return nil
	}

	if sm.stream != nil {
		if err := sm.stream.UnSubscribeOrderBook([]string{instrumentUID}, depth); err != nil {
			return fmt.Errorf("UnsubscribeOrderbook: %w", err)
		}
	}

	sub.cancel()
	if sub.orderbooks != nil {
		sub.orderbooks.close()
	}
	delete(sm.subscriptions, key)
	sm.logger.Info("UnsubscribeOrderbook", "instrument_uid", instrumentUID, "depth", depth, "total_subs", len(sm.subscriptions))
	return nil
}

// Unsubscribe removes a subscription for trades or last price (instrument-level only).
func (sm *StreamManager) Unsubscribe(instrumentUID string, subType SubscriptionType) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if subType == SubCandles || subType == SubOrderbook {
		return fmt.Errorf("Unsubscribe: use UnsubscribeCandles or UnsubscribeOrderbook for type %s", subType.String())
	}

	key := subscriptionKey{InstrumentUID: instrumentUID, SubscriptionType: subType}
	sub, exists := sm.subscriptions[key]
	if !exists {
		return fmt.Errorf("Unsubscribe: no subscription for %s/%s", instrumentUID, subType)
	}
	sub.refs--
	if sub.refs > 0 {
		sm.logger.Info("Unsubscribe shared", "instrument_uid", instrumentUID, "type", subType.String(), "refs", sub.refs, "total_subs", len(sm.subscriptions))
		return nil
	}

	if sm.stream != nil {
		var err error
		switch subType {
		case SubTrades:
			err = sm.stream.UnSubscribeTrade([]string{instrumentUID}, pb.TradeSourceType_TRADE_SOURCE_UNSPECIFIED, false)
		case SubLastPrice:
			err = sm.stream.UnSubscribeLastPrice([]string{instrumentUID})
		default:
			return fmt.Errorf("Unsubscribe: unsupported type %s", subType.String())
		}
		if err != nil {
			return fmt.Errorf("Unsubscribe: %w", err)
		}
	}

	sub.cancel()
	switch subType {
	case SubTrades:
		if sub.trades != nil {
			sub.trades.close()
		}
	case SubLastPrice:
		if sub.lastPrices != nil {
			sub.lastPrices.close()
		}
	}
	delete(sm.subscriptions, key)
	sm.logger.Info("Unsubscribe", "instrument_uid", instrumentUID, "type", subType.String(), "total_subs", len(sm.subscriptions))
	return nil
}

// Listen starts the SDK stream listener. It blocks until ctx is cancelled.
// On stream error it reconnects with exponential back-off.
func (sm *StreamManager) Listen(ctx context.Context) error {
	for {
		sm.mu.Lock()
		s := sm.stream
		sm.mu.Unlock()

		if s == nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(500 * time.Millisecond):
				continue
			}
		}

		err := s.Listen()
		if ctx.Err() != nil {
			return ctx.Err()
		}

		sm.logger.Warn("stream disconnected, attempting reconnect", "error", err)

		if reconnectErr := sm.reconnect(ctx); reconnectErr != nil {
			return fmt.Errorf("Listen: reconnect failed: %w", reconnectErr)
		}
	}
}

// Stop shuts down the underlying stream.
func (sm *StreamManager) Stop() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.stream != nil {
		sm.stream.Stop()
		sm.stream = nil
	}
}

// ensureStream creates the SDK stream if it doesn't exist. Must be called under sm.mu.
func (sm *StreamManager) ensureStream() error {
	if sm.stream != nil {
		return nil
	}
	streamClient := sm.tinvestClient.MarketDataStreamClient()
	s, err := streamClient.MarketDataStream()
	if err != nil {
		return fmt.Errorf("ensureStream: %w", err)
	}
	sm.stream = s
	return nil
}

// reconnect attempts to re-establish the stream with configurable retries.
func (sm *StreamManager) reconnect(ctx context.Context) error {
	sm.mu.Lock()
	if sm.stream != nil {
		sm.stream.Stop()
		sm.stream = nil
	}
	sm.mu.Unlock()

	delay := time.Duration(sm.streamCfg.ReconnectDelayMs) * time.Millisecond
	if delay == 0 {
		delay = 1 * time.Second
	}
	maxAttempts := sm.streamCfg.MaxReconnectAttempts
	if maxAttempts == 0 {
		maxAttempts = 10
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		sm.logger.Info("reconnecting stream", "attempt", attempt, "max_attempts", maxAttempts, "delay", delay)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		sm.mu.Lock()
		err := sm.ensureStream()
		if err != nil {
			sm.mu.Unlock()
			sm.logger.Warn("reconnect attempt failed", "attempt", attempt, "error", err)
			continue
		}

		// Re-subscribe all active subscriptions on the new stream.
		if resubErr := sm.resubscribeAll(); resubErr != nil {
			sm.stream.Stop()
			sm.stream = nil
			sm.mu.Unlock()
			sm.logger.Warn("resubscribe failed", "attempt", attempt, "error", resubErr)
			continue
		}
		sm.mu.Unlock()

		sm.logger.Info("stream reconnected", "attempt", attempt)
		return nil
	}

	return fmt.Errorf("reconnect: exhausted %d attempts", maxAttempts)
}

// resubscribeAll re-creates all SDK subscriptions on the current stream. Must be called under sm.mu.
func (sm *StreamManager) resubscribeAll() error {
	for key, sub := range sm.subscriptions {
		switch key.SubscriptionType {
		case SubCandles:
			ch, err := sm.stream.SubscribeCandle(
				[]string{key.InstrumentUID},
				key.CandleInterval,
				candleStreamWaitClose,
				nil,
			)
			if err != nil {
				return err
			}
			go sm.forwardCandles(sub.ctx, ch, sub.candles)
		case SubOrderbook:
			depth := key.OrderbookDepth
			if depth <= 0 {
				depth = 10
			}
			ch, err := sm.stream.SubscribeOrderBook([]string{key.InstrumentUID}, depth)
			if err != nil {
				return err
			}
			go sm.forwardOrderbooks(sub.ctx, ch, sub.orderbooks)
		case SubTrades:
			ch, err := sm.stream.SubscribeTrade([]string{key.InstrumentUID}, pb.TradeSourceType_TRADE_SOURCE_UNSPECIFIED, false)
			if err != nil {
				return err
			}
			go sm.forwardTrades(sub.ctx, ch, sub.trades)
		case SubLastPrice:
			ch, err := sm.stream.SubscribeLastPrice([]string{key.InstrumentUID})
			if err != nil {
				return err
			}
			go sm.forwardLastPrices(sub.ctx, ch, sub.lastPrices)
		}
	}
	return nil
}

func (sm *StreamManager) maxSubscriptions() int {
	if sm.streamCfg.MaxSubscriptions > 0 {
		return sm.streamCfg.MaxSubscriptions
	}
	return 300
}

// forwardCandles reads from an SDK channel and dispatches to downstream subscribers + updates cache.
func (sm *StreamManager) forwardCandles(ctx context.Context, src <-chan *pb.Candle, dst *streamFanout[*pb.Candle]) {
	for {
		select {
		case <-ctx.Done():
			return
		case c, ok := <-src:
			if !ok {
				return
			}
			if c != nil && c.GetClose() != nil {
				sm.prices.SetLastPrice(c.GetInstrumentUid(), quotationToFloat(c.GetClose()))
			}
			// Never drop: losing candles desyncs the strategy from the exchange session (evening bars missing on chart).
			if ok := dst.publish(ctx, c, true); !ok {
				return
			}
		}
	}
}

func (sm *StreamManager) forwardOrderbooks(ctx context.Context, src <-chan *pb.OrderBook, dst *streamFanout[*pb.OrderBook]) {
	for {
		select {
		case <-ctx.Done():
			return
		case ob, ok := <-src:
			if !ok {
				return
			}
			if ok := dst.publish(ctx, ob, false); !ok {
				return
			}
		}
	}
}

func (sm *StreamManager) forwardTrades(ctx context.Context, src <-chan *pb.Trade, dst *streamFanout[*pb.Trade]) {
	for {
		select {
		case <-ctx.Done():
			return
		case t, ok := <-src:
			if !ok {
				return
			}
			if t != nil && t.GetPrice() != nil {
				sm.prices.SetLastPrice(t.GetInstrumentUid(), quotationToFloat(t.GetPrice()))
			}
			if ok := dst.publish(ctx, t, false); !ok {
				return
			}
		}
	}
}

func (sm *StreamManager) forwardLastPrices(ctx context.Context, src <-chan *pb.LastPrice, dst *streamFanout[*pb.LastPrice]) {
	for {
		select {
		case <-ctx.Done():
			return
		case lp, ok := <-src:
			if !ok {
				return
			}
			if lp != nil && lp.GetPrice() != nil {
				sm.prices.SetLastPrice(lp.GetInstrumentUid(), quotationToFloat(lp.GetPrice()))
			}
			if ok := dst.publish(ctx, lp, false); !ok {
				return
			}
		}
	}
}
