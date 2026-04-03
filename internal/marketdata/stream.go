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
}

// StreamManager manages MarketDataStream connections to T-Invest.
// It multiplexes subscriptions across a shared stream and handles reconnection.
type StreamManager struct {
	tinvestClient *tinvest.Client
	prices        *PriceCache
	logger        *logging.Logger
	streamCfg     config.MarketDataStreamConfig

	mu            sync.Mutex
	subscriptions map[subscriptionKey]struct{}
	stream        *investgo.MarketDataStream

	candleChs    []<-chan *pb.Candle
	orderbookChs []<-chan *pb.OrderBook
	tradeChs     []<-chan *pb.Trade
	lastPriceChs []<-chan *pb.LastPrice

	// Subscriber fan-out channels.
	candleSubs    []chan<- *pb.Candle
	orderbookSubs []chan<- *pb.OrderBook
	tradeSubs     []chan<- *pb.Trade
	lastPriceSubs []chan<- *pb.LastPrice
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
		subscriptions: make(map[subscriptionKey]struct{}),
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

	key := subscriptionKey{InstrumentUID: instrumentUID, SubscriptionType: SubCandles}
	if _, exists := sm.subscriptions[key]; exists {
		return nil, fmt.Errorf("SubscribeCandles: already subscribed to candles for %s", instrumentUID)
	}
	if len(sm.subscriptions) >= sm.maxSubscriptions() {
		return nil, fmt.Errorf("SubscribeCandles: max subscriptions (%d) reached", sm.maxSubscriptions())
	}

	ch, err := sm.stream.SubscribeCandle([]string{instrumentUID}, interval, false, nil)
	if err != nil {
		return nil, fmt.Errorf("SubscribeCandles: %w", err)
	}

	sm.subscriptions[key] = struct{}{}
	sm.candleChs = append(sm.candleChs, ch)

	fanOut := make(chan *pb.Candle, 64)
	sm.candleSubs = append(sm.candleSubs, fanOut)

	// TODO: add WaitGroup for stream goroutines
	go sm.forwardCandles(ctx, ch, fanOut)

	sm.logger.Info("SubscribeCandles", "instrument_uid", instrumentUID, "total_subs", len(sm.subscriptions))
	return fanOut, nil
}

// SubscribeOrderbook subscribes to streaming order book for the given instrument.
func (sm *StreamManager) SubscribeOrderbook(ctx context.Context, instrumentUID string, depth int32) (<-chan *pb.OrderBook, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.ensureStream(); err != nil {
		return nil, fmt.Errorf("SubscribeOrderbook: %w", err)
	}

	key := subscriptionKey{InstrumentUID: instrumentUID, SubscriptionType: SubOrderbook}
	if _, exists := sm.subscriptions[key]; exists {
		return nil, fmt.Errorf("SubscribeOrderbook: already subscribed to orderbook for %s", instrumentUID)
	}
	if len(sm.subscriptions) >= sm.maxSubscriptions() {
		return nil, fmt.Errorf("SubscribeOrderbook: max subscriptions (%d) reached", sm.maxSubscriptions())
	}

	ch, err := sm.stream.SubscribeOrderBook([]string{instrumentUID}, depth)
	if err != nil {
		return nil, fmt.Errorf("SubscribeOrderbook: %w", err)
	}

	sm.subscriptions[key] = struct{}{}
	sm.orderbookChs = append(sm.orderbookChs, ch)

	fanOut := make(chan *pb.OrderBook, 64)
	sm.orderbookSubs = append(sm.orderbookSubs, fanOut)

	// TODO: add WaitGroup for stream goroutines
	go sm.forwardOrderbooks(ctx, ch, fanOut)

	sm.logger.Info("SubscribeOrderbook", "instrument_uid", instrumentUID, "depth", depth, "total_subs", len(sm.subscriptions))
	return fanOut, nil
}

// SubscribeTrades subscribes to streaming trades for the given instrument.
func (sm *StreamManager) SubscribeTrades(ctx context.Context, instrumentUID string) (<-chan *pb.Trade, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.ensureStream(); err != nil {
		return nil, fmt.Errorf("SubscribeTrades: %w", err)
	}

	key := subscriptionKey{InstrumentUID: instrumentUID, SubscriptionType: SubTrades}
	if _, exists := sm.subscriptions[key]; exists {
		return nil, fmt.Errorf("SubscribeTrades: already subscribed to trades for %s", instrumentUID)
	}
	if len(sm.subscriptions) >= sm.maxSubscriptions() {
		return nil, fmt.Errorf("SubscribeTrades: max subscriptions (%d) reached", sm.maxSubscriptions())
	}

	ch, err := sm.stream.SubscribeTrade([]string{instrumentUID}, pb.TradeSourceType_TRADE_SOURCE_UNSPECIFIED, false)
	if err != nil {
		return nil, fmt.Errorf("SubscribeTrades: %w", err)
	}

	sm.subscriptions[key] = struct{}{}
	sm.tradeChs = append(sm.tradeChs, ch)

	fanOut := make(chan *pb.Trade, 64)
	sm.tradeSubs = append(sm.tradeSubs, fanOut)

	// TODO: add WaitGroup for stream goroutines
	go sm.forwardTrades(ctx, ch, fanOut)

	sm.logger.Info("SubscribeTrades", "instrument_uid", instrumentUID, "total_subs", len(sm.subscriptions))
	return fanOut, nil
}

// SubscribeLastPrice subscribes to streaming last prices for the given instrument.
func (sm *StreamManager) SubscribeLastPrice(ctx context.Context, instrumentUID string) (<-chan *pb.LastPrice, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.ensureStream(); err != nil {
		return nil, fmt.Errorf("SubscribeLastPrice: %w", err)
	}

	key := subscriptionKey{InstrumentUID: instrumentUID, SubscriptionType: SubLastPrice}
	if _, exists := sm.subscriptions[key]; exists {
		return nil, fmt.Errorf("SubscribeLastPrice: already subscribed to last_price for %s", instrumentUID)
	}
	if len(sm.subscriptions) >= sm.maxSubscriptions() {
		return nil, fmt.Errorf("SubscribeLastPrice: max subscriptions (%d) reached", sm.maxSubscriptions())
	}

	ch, err := sm.stream.SubscribeLastPrice([]string{instrumentUID})
	if err != nil {
		return nil, fmt.Errorf("SubscribeLastPrice: %w", err)
	}

	sm.subscriptions[key] = struct{}{}
	sm.lastPriceChs = append(sm.lastPriceChs, ch)

	fanOut := make(chan *pb.LastPrice, 64)
	sm.lastPriceSubs = append(sm.lastPriceSubs, fanOut)

	// TODO: add WaitGroup for stream goroutines
	go sm.forwardLastPrices(ctx, ch, fanOut)

	sm.logger.Info("SubscribeLastPrice", "instrument_uid", instrumentUID, "total_subs", len(sm.subscriptions))
	return fanOut, nil
}

// Unsubscribe removes a subscription for a specific instrument and type.
func (sm *StreamManager) Unsubscribe(instrumentUID string, subType SubscriptionType) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := subscriptionKey{InstrumentUID: instrumentUID, SubscriptionType: subType}
	if _, exists := sm.subscriptions[key]; !exists {
		return fmt.Errorf("Unsubscribe: no subscription for %s/%s", instrumentUID, subType)
	}

	if sm.stream != nil {
		var err error
		switch subType {
		case SubCandles:
			err = sm.stream.UnSubscribeCandle([]string{instrumentUID}, pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_UNSPECIFIED, false, nil)
		case SubOrderbook:
			err = sm.stream.UnSubscribeOrderBook([]string{instrumentUID}, 0)
		case SubTrades:
			err = sm.stream.UnSubscribeTrade([]string{instrumentUID}, pb.TradeSourceType_TRADE_SOURCE_UNSPECIFIED, false)
		case SubLastPrice:
			err = sm.stream.UnSubscribeLastPrice([]string{instrumentUID})
		}
		if err != nil {
			return fmt.Errorf("Unsubscribe: %w", err)
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
	for key := range sm.subscriptions {
		switch key.SubscriptionType {
		case SubCandles:
			if _, err := sm.stream.SubscribeCandle(
				[]string{key.InstrumentUID},
				pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_UNSPECIFIED,
				false,
				nil,
			); err != nil {
				return err
			}
		case SubOrderbook:
			if _, err := sm.stream.SubscribeOrderBook([]string{key.InstrumentUID}, 10); err != nil {
				return err
			}
		case SubTrades:
			if _, err := sm.stream.SubscribeTrade([]string{key.InstrumentUID}, pb.TradeSourceType_TRADE_SOURCE_UNSPECIFIED, false); err != nil {
				return err
			}
		case SubLastPrice:
			if _, err := sm.stream.SubscribeLastPrice([]string{key.InstrumentUID}); err != nil {
				return err
			}
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

// forwardCandles reads from an SDK channel and dispatches to the subscriber + updates cache.
func (sm *StreamManager) forwardCandles(ctx context.Context, src <-chan *pb.Candle, dst chan<- *pb.Candle) {
	defer close(dst)
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
			select {
			case dst <- c:
			default:
			}
		}
	}
}

func (sm *StreamManager) forwardOrderbooks(ctx context.Context, src <-chan *pb.OrderBook, dst chan<- *pb.OrderBook) {
	defer close(dst)
	for {
		select {
		case <-ctx.Done():
			return
		case ob, ok := <-src:
			if !ok {
				return
			}
			select {
			case dst <- ob:
			default:
			}
		}
	}
}

func (sm *StreamManager) forwardTrades(ctx context.Context, src <-chan *pb.Trade, dst chan<- *pb.Trade) {
	defer close(dst)
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
			select {
			case dst <- t:
			default:
			}
		}
	}
}

func (sm *StreamManager) forwardLastPrices(ctx context.Context, src <-chan *pb.LastPrice, dst chan<- *pb.LastPrice) {
	defer close(dst)
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
			select {
			case dst <- lp:
			default:
			}
		}
	}
}
