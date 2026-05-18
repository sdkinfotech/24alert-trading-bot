package order

import (
	"context"
	"fmt"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/tinvest"
)

// OrderStateEvent is emitted when an order state changes.
type OrderStateEvent struct {
	OrderID   string
	AccountID string
	Status    OrderStatus
	FilledQty int64
	UpdatedAt time.Time
}

// TradeEvent is emitted when a trade executes.
type TradeEvent struct {
	OrderID   string
	AccountID string
	Trades    []*pb.OrderTrade
}

// StreamManager manages T-Invest order/trade streams and fans out to local subscribers.
type StreamManager struct {
	tinvestClient *tinvest.Client
	repo          *Repository
	logger        *logging.Logger

	orderStateSubs []chan<- OrderStateEvent
	tradeSubs      []chan<- TradeEvent
}

func NewStreamManager(client *tinvest.Client, repo *Repository, logger *logging.Logger) *StreamManager {
	return &StreamManager{
		tinvestClient: client,
		repo:          repo,
		logger:        logger,
	}
}

// SubscribeOrderStates returns a channel that receives order state updates.
// Caller is responsible for draining the channel.
func (sm *StreamManager) SubscribeOrderStates(bufSize int) <-chan OrderStateEvent {
	ch := make(chan OrderStateEvent, bufSize)
	sm.orderStateSubs = append(sm.orderStateSubs, ch)
	return ch
}

// SubscribeTrades returns a channel that receives trade events.
func (sm *StreamManager) SubscribeTrades(bufSize int) <-chan TradeEvent {
	ch := make(chan TradeEvent, bufSize)
	sm.tradeSubs = append(sm.tradeSubs, ch)
	return ch
}

// StreamOrderStates subscribes to the T-Invest OrderStateStream and fans out to local subscribers.
// Blocks until ctx is cancelled or an unrecoverable error occurs.
func (sm *StreamManager) StreamOrderStates(ctx context.Context, accounts []string) error {
	l := sm.logger.WithContext(ctx)
	l.Info("StreamOrderStates starting", "accounts", accounts)

	streamClient := sm.tinvestClient.OrdersStreamClient()
	stream, err := streamClient.OrderStateStream(accounts, 5000)
	if err != nil {
		return fmt.Errorf("StreamOrderStates: open stream: %w", err)
	}
	defer stream.Stop()

	states := stream.OrderState()

	// TODO: add WaitGroup for stream goroutines
	go func() {
		<-ctx.Done()
		stream.Stop()
	}()

	for state := range states {
		if state == nil {
			continue
		}

		status := MapExecutionStatus(state.GetExecutionReportStatus())
		orderID := state.GetOrderId()
		filledQty := state.GetLotsExecuted()

		_ = sm.repo.UpdateOrderState(orderID, status, filledQty)

		evt := OrderStateEvent{
			OrderID:   orderID,
			AccountID: state.GetAccountId(),
			Status:    status,
			FilledQty: filledQty,
			UpdatedAt: time.Now(),
		}

		for _, sub := range sm.orderStateSubs {
			select {
			case sub <- evt:
			default:
				l.Warn("StreamOrderStates: subscriber slow, dropping event", "order_id", orderID)
			}
		}
	}

	l.Info("StreamOrderStates stopped")
	return nil
}

// StreamTrades subscribes to the T-Invest TradesStream and fans out to local subscribers.
// Blocks until ctx is cancelled or an unrecoverable error occurs.
func (sm *StreamManager) StreamTrades(ctx context.Context, accounts []string) error {
	l := sm.logger.WithContext(ctx)
	l.Info("StreamTrades starting", "accounts", accounts)

	streamClient := sm.tinvestClient.OrdersStreamClient()
	stream, err := streamClient.TradesStream(accounts, nil)
	if err != nil {
		return fmt.Errorf("StreamTrades: open stream: %w", err)
	}
	defer stream.Stop()

	trades := stream.Trades()

	// TODO: add WaitGroup for stream goroutines
	go func() {
		<-ctx.Done()
		stream.Stop()
	}()

	for trade := range trades {
		if trade == nil {
			continue
		}

		orderID := trade.GetOrderId()
		for _, t := range trade.GetTrades() {
			sm.repo.AddExecution(ExecutionRecord{
				OrderID:    orderID,
				TradeID:    t.GetTradeId(),
				Qty:        t.GetQuantity(),
				Price:      quotationToFloat(t.GetPrice()),
				ExecutedAt: t.GetDateTime().AsTime(),
			})
		}

		evt := TradeEvent{
			OrderID:   orderID,
			AccountID: trade.GetAccountId(),
			Trades:    trade.GetTrades(),
		}

		for _, sub := range sm.tradeSubs {
			select {
			case sub <- evt:
			default:
				l.Warn("StreamTrades: subscriber slow, dropping event", "order_id", orderID)
			}
		}
	}

	l.Info("StreamTrades stopped")
	return nil
}

func quotationToFloat(q *pb.Quotation) float64 {
	if q == nil {
		return 0
	}
	return float64(q.GetUnits()) + float64(q.GetNano())/1e9
}
