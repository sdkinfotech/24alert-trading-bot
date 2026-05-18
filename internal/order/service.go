package order

import (
	"context"
	"fmt"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/pkg/idempotency"
	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/metrics"
	"github.com/24alert/trading-bot/pkg/tinvest"
	"github.com/russianinvestments/invest-api-go-sdk/investgo"
)

type Service struct {
	tinvestClient *tinvest.Client
	rateLimiter   *tinvest.RateLimiterManager
	repo          *Repository
	idGen         *idempotency.OrderIDGenerator
	logger        *logging.Logger
}

func NewService(
	client *tinvest.Client,
	rl *tinvest.RateLimiterManager,
	repo *Repository,
	logger *logging.Logger,
) *Service {
	return &Service{
		tinvestClient: client,
		rateLimiter:   rl,
		repo:          repo,
		idGen:         idempotency.NewOrderIDGenerator(),
		logger:        logger,
	}
}

// PostOrder places a new exchange order via T-Invest API.
func (s *Service) PostOrder(ctx context.Context, req *investgo.PostOrderRequest) (*investgo.PostOrderResponse, error) {
	start := time.Now()
	l := s.logger.WithContext(ctx)

	orderType := req.OrderType.String()
	direction := req.Direction.String()
	defer func() {
		metrics.OrderLatency.WithLabelValues(direction, orderType).Observe(time.Since(start).Seconds())
	}()

	if req.OrderId == "" {
		req.OrderId = s.idGen.NewOrderID()
	}

	l.Info("PostOrder",
		"instrument_id", req.InstrumentId,
		"direction", req.Direction.String(),
		"quantity", req.Quantity,
		"order_id", req.OrderId,
	)

	if err := s.rateLimiter.Wait(ctx, "post_order"); err != nil {
		metrics.OrdersTotal.WithLabelValues(direction, orderType, "failure").Inc()
		return nil, fmt.Errorf("PostOrder: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.OrdersServiceClient().PostOrder(req)
	if err != nil {
		metrics.OrdersTotal.WithLabelValues(direction, orderType, "failure").Inc()
		return nil, fmt.Errorf("PostOrder: %w", err)
	}

	metrics.OrdersTotal.WithLabelValues(direction, orderType, "success").Inc()

	orderID := resp.GetOrderId()
	if orderID == "" {
		orderID = req.OrderId
	}
	s.repo.SaveOrder(&OrderRecord{
		OrderID:       orderID,
		AccountID:     req.AccountId,
		InstrumentUID: req.InstrumentId,
		Direction:     req.Direction.String(),
		OrderType:     req.OrderType.String(),
		RequestedQty:  req.Quantity,
		Status:        OrderStatusNew,
	})

	l.Info("PostOrder completed",
		"order_id", req.OrderId,
		"execution_status", resp.GetExecutionReportStatus().String(),
	)
	return resp, nil
}

// CancelOrder cancels an active exchange order.
func (s *Service) CancelOrder(ctx context.Context, accountID, orderID string) (*investgo.CancelOrderResponse, error) {
	l := s.logger.WithContext(ctx)
	l.Info("CancelOrder", "account_id", accountID, "order_id", orderID)

	if err := s.rateLimiter.Wait(ctx, "cancel_order"); err != nil {
		return nil, fmt.Errorf("CancelOrder: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.OrdersServiceClient().CancelOrder(accountID, orderID, nil)
	if err != nil {
		return nil, fmt.Errorf("CancelOrder: %w", err)
	}

	_ = s.repo.UpdateOrderState(orderID, OrderStatusCancelled, 0)

	l.Info("CancelOrder completed", "order_id", orderID)
	return resp, nil
}

// ReplaceOrder modifies an existing exchange order.
func (s *Service) ReplaceOrder(ctx context.Context, req *investgo.ReplaceOrderRequest) (*investgo.PostOrderResponse, error) {
	l := s.logger.WithContext(ctx)

	if req.NewOrderId == "" {
		req.NewOrderId = s.idGen.NewOrderID()
	}

	l.Info("ReplaceOrder",
		"account_id", req.AccountId,
		"order_id", req.OrderId,
		"new_order_id", req.NewOrderId,
		"quantity", req.Quantity,
	)

	if err := s.rateLimiter.Wait(ctx, "replace_order"); err != nil {
		return nil, fmt.Errorf("ReplaceOrder: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.OrdersServiceClient().ReplaceOrder(req)
	if err != nil {
		return nil, fmt.Errorf("ReplaceOrder: %w", err)
	}

	_ = s.repo.UpdateOrderState(req.OrderId, OrderStatusReplaced, 0)

	s.repo.SaveOrder(&OrderRecord{
		OrderID:      req.NewOrderId,
		AccountID:    req.AccountId,
		RequestedQty: req.Quantity,
		Status:       OrderStatusNew,
	})

	l.Info("ReplaceOrder completed", "new_order_id", req.NewOrderId)
	return resp, nil
}

// GetOrders returns active orders for the given account.
func (s *Service) GetOrders(ctx context.Context, accountID string) (*investgo.GetOrdersResponse, error) {
	l := s.logger.WithContext(ctx)
	l.Info("GetOrders", "account_id", accountID)

	if err := s.rateLimiter.Wait(ctx, "get_orders"); err != nil {
		return nil, fmt.Errorf("GetOrders: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.OrdersServiceClient().GetOrders(accountID, nil)
	if err != nil {
		return nil, fmt.Errorf("GetOrders: %w", err)
	}

	l.Info("GetOrders completed", "account_id", accountID, "count", len(resp.GetOrders()))
	return resp, nil
}

// GetOrderState returns the current state of a specific order.
func (s *Service) GetOrderState(ctx context.Context, accountID, orderID string, priceType pb.PriceType) (*investgo.GetOrderStateResponse, error) {
	l := s.logger.WithContext(ctx)
	l.Info("GetOrderState", "account_id", accountID, "order_id", orderID)

	if err := s.rateLimiter.Wait(ctx, "get_order_state"); err != nil {
		return nil, fmt.Errorf("GetOrderState: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.OrdersServiceClient().GetOrderState(accountID, orderID, priceType, nil)
	if err != nil {
		return nil, fmt.Errorf("GetOrderState: %w", err)
	}

	if resp.OrderState != nil {
		_ = s.repo.UpdateOrderState(orderID, MapExecutionStatus(resp.GetExecutionReportStatus()), resp.GetLotsExecuted())
	}

	l.Info("GetOrderState completed", "order_id", orderID, "status", resp.GetExecutionReportStatus().String())
	return resp, nil
}

// PostStopOrder places a new exchange order via T-Invest API.
func (s *Service) PostStopOrder(ctx context.Context, req *investgo.PostStopOrderRequest) (*investgo.PostStopOrderResponse, error) {
	start := time.Now()
	l := s.logger.WithContext(ctx)

	orderType := "stop_" + req.StopOrderType.String()
	direction := req.Direction.String()
	defer func() {
		metrics.OrderLatency.WithLabelValues(direction, orderType).Observe(time.Since(start).Seconds())
	}()

	if req.OrderID == "" {
		req.OrderID = s.idGen.NewOrderID()
	}

	l.Info("PostStopOrder",
		"instrument_id", req.InstrumentId,
		"direction", req.Direction.String(),
		"stop_order_type", req.StopOrderType.String(),
	)

	if err := s.rateLimiter.Wait(ctx, "post_stop_order"); err != nil {
		metrics.OrdersTotal.WithLabelValues(direction, orderType, "failure").Inc()
		return nil, fmt.Errorf("PostStopOrder: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.StopOrdersServiceClient().PostStopOrder(req)
	if err != nil {
		metrics.OrdersTotal.WithLabelValues(direction, orderType, "failure").Inc()
		return nil, fmt.Errorf("PostStopOrder: %w", err)
	}

	metrics.OrdersTotal.WithLabelValues(direction, orderType, "success").Inc()

	l.Info("PostStopOrder completed", "stop_order_id", resp.GetStopOrderId())
	return resp, nil
}

// CancelStopOrder cancels an active stop order.
func (s *Service) CancelStopOrder(ctx context.Context, accountID, stopOrderID string) (*investgo.CancelStopOrderResponse, error) {
	l := s.logger.WithContext(ctx)
	l.Info("CancelStopOrder", "account_id", accountID, "stop_order_id", stopOrderID)

	if err := s.rateLimiter.Wait(ctx, "cancel_stop_order"); err != nil {
		return nil, fmt.Errorf("CancelStopOrder: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.StopOrdersServiceClient().CancelStopOrder(accountID, stopOrderID)
	if err != nil {
		return nil, fmt.Errorf("CancelStopOrder: %w", err)
	}

	l.Info("CancelStopOrder completed", "stop_order_id", stopOrderID)
	return resp, nil
}

// GetStopOrders returns active stop orders for the given account.
func (s *Service) GetStopOrders(ctx context.Context, accountID string) (*investgo.GetStopOrdersResponse, error) {
	l := s.logger.WithContext(ctx)
	l.Info("GetStopOrders", "account_id", accountID)

	if err := s.rateLimiter.Wait(ctx, "get_stop_orders"); err != nil {
		return nil, fmt.Errorf("GetStopOrders: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.StopOrdersServiceClient().GetStopOrders(accountID)
	if err != nil {
		return nil, fmt.Errorf("GetStopOrders: %w", err)
	}

	l.Info("GetStopOrders completed", "account_id", accountID, "count", len(resp.GetStopOrders()))
	return resp, nil
}

func MapExecutionStatus(s pb.OrderExecutionReportStatus) OrderStatus {
	switch s {
	case pb.OrderExecutionReportStatus_EXECUTION_REPORT_STATUS_NEW:
		return OrderStatusNew
	case pb.OrderExecutionReportStatus_EXECUTION_REPORT_STATUS_PARTIALLYFILL:
		return OrderStatusPartiallyFilled
	case pb.OrderExecutionReportStatus_EXECUTION_REPORT_STATUS_FILL:
		return OrderStatusFilled
	case pb.OrderExecutionReportStatus_EXECUTION_REPORT_STATUS_CANCELLED:
		return OrderStatusCancelled
	case pb.OrderExecutionReportStatus_EXECUTION_REPORT_STATUS_REJECTED:
		return OrderStatusRejected
	default:
		return OrderStatusNew
	}
}
