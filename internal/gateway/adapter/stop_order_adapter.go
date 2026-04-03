package adapter

import (
	"context"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/internal/gateway/handlers"
	"github.com/24alert/trading-bot/internal/order"
	"github.com/russianinvestments/invest-api-go-sdk/investgo"
)

// StopOrderAdapter implements handlers.StopOrderService by wrapping *order.Service.
type StopOrderAdapter struct {
	svc *order.Service
}

func NewStopOrderAdapter(svc *order.Service) *StopOrderAdapter {
	return &StopOrderAdapter{svc: svc}
}

func (a *StopOrderAdapter) PostStopOrder(ctx context.Context, accountID, instrumentUID string, qty int64, direction, stopOrderType string, stopPrice, price float64) (*handlers.StopOrderResult, error) {
	req := &investgo.PostStopOrderRequest{
		InstrumentId:  instrumentUID,
		Quantity:      qty,
		Direction:     parseStopOrderDirection(direction),
		AccountId:     accountID,
		StopOrderType: parseStopOrderType(stopOrderType),
		StopPrice:     floatToQuotation(stopPrice),
		Price:         floatToQuotation(price),
	}

	resp, err := a.svc.PostStopOrder(ctx, req)
	if err != nil {
		return nil, err
	}

	return &handlers.StopOrderResult{
		StopOrderID: resp.GetStopOrderId(),
	}, nil
}

func (a *StopOrderAdapter) GetStopOrders(ctx context.Context, accountID string) ([]handlers.StopOrderSummary, error) {
	resp, err := a.svc.GetStopOrders(ctx, accountID)
	if err != nil {
		return nil, err
	}

	out := make([]handlers.StopOrderSummary, 0, len(resp.GetStopOrders()))
	for _, so := range resp.GetStopOrders() {
		summary := handlers.StopOrderSummary{
			StopOrderID:   so.GetStopOrderId(),
			InstrumentUID: so.GetInstrumentUid(),
			Direction:     so.GetDirection().String(),
			StopOrderType: so.GetOrderType().String(),
			Lots:          so.GetLotsRequested(),
			StopPrice:     moneyToFloat(so.GetStopPrice()),
			Price:         moneyToFloat(so.GetPrice()),
			Status:        so.GetStatus().String(),
			CreatedAt:     so.GetCreateDate().AsTime(),
		}
		if so.GetExpirationTime() != nil {
			summary.ExpirationAt = so.GetExpirationTime().AsTime()
		}
		out = append(out, summary)
	}
	return out, nil
}

func (a *StopOrderAdapter) CancelStopOrder(ctx context.Context, accountID, stopOrderID string) (*handlers.CancelStopOrderResult, error) {
	resp, err := a.svc.CancelStopOrder(ctx, accountID, stopOrderID)
	if err != nil {
		return nil, err
	}

	return &handlers.CancelStopOrderResult{
		CancelledAt: resp.GetTime().AsTime(),
	}, nil
}

func parseStopOrderDirection(s string) pb.StopOrderDirection {
	switch s {
	case "buy", "BUY":
		return pb.StopOrderDirection_STOP_ORDER_DIRECTION_BUY
	case "sell", "SELL":
		return pb.StopOrderDirection_STOP_ORDER_DIRECTION_SELL
	default:
		return pb.StopOrderDirection_STOP_ORDER_DIRECTION_UNSPECIFIED
	}
}

func parseStopOrderType(s string) pb.StopOrderType {
	switch s {
	case "take_profit", "TAKE_PROFIT":
		return pb.StopOrderType_STOP_ORDER_TYPE_TAKE_PROFIT
	case "stop_loss", "STOP_LOSS":
		return pb.StopOrderType_STOP_ORDER_TYPE_STOP_LOSS
	case "stop_limit", "STOP_LIMIT":
		return pb.StopOrderType_STOP_ORDER_TYPE_STOP_LIMIT
	default:
		return pb.StopOrderType_STOP_ORDER_TYPE_UNSPECIFIED
	}
}

var _ handlers.StopOrderService = (*StopOrderAdapter)(nil)
