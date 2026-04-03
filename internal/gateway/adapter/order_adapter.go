package adapter

import (
	"context"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/internal/gateway/handlers"
	"github.com/24alert/trading-bot/internal/order"
	"github.com/russianinvestments/invest-api-go-sdk/investgo"
)

// OrderAdapter implements handlers.OrderService by wrapping *order.Service.
type OrderAdapter struct {
	svc *order.Service
}

func NewOrderAdapter(svc *order.Service) *OrderAdapter {
	return &OrderAdapter{svc: svc}
}

func (a *OrderAdapter) PostOrder(ctx context.Context, accountID, instrumentUID string, qty int64, direction, orderType string, price float64) (*handlers.OrderResult, error) {
	req := &investgo.PostOrderRequest{
		InstrumentId: instrumentUID,
		Quantity:     qty,
		Direction:    parseOrderDirection(direction),
		AccountId:    accountID,
		OrderType:    parseOrderType(orderType),
		Price:        floatToQuotation(price),
	}

	resp, err := a.svc.PostOrder(ctx, req)
	if err != nil {
		return nil, err
	}

	return &handlers.OrderResult{
		OrderID:         resp.GetOrderId(),
		ExecutionStatus: resp.GetExecutionReportStatus().String(),
		LotsRequested:   resp.GetLotsRequested(),
		LotsExecuted:    resp.GetLotsExecuted(),
		TotalPrice:      moneyToFloat(resp.GetTotalOrderAmount()),
		Direction:       resp.GetDirection().String(),
		OrderType:       resp.GetOrderType().String(),
		Message:         resp.GetMessage(),
	}, nil
}

func (a *OrderAdapter) GetOrders(ctx context.Context, accountID string) ([]handlers.OrderSummary, error) {
	resp, err := a.svc.GetOrders(ctx, accountID)
	if err != nil {
		return nil, err
	}

	out := make([]handlers.OrderSummary, 0, len(resp.GetOrders()))
	for _, o := range resp.GetOrders() {
		out = append(out, handlers.OrderSummary{
			OrderID:       o.GetOrderId(),
			InstrumentUID: o.GetInstrumentUid(),
			Direction:     o.GetDirection().String(),
			OrderType:     o.GetOrderType().String(),
			Lots:          o.GetLotsRequested(),
			Price:         moneyToFloat(o.GetInitialOrderPrice()),
			Status:        o.GetExecutionReportStatus().String(),
			CreatedAt:     o.GetOrderDate().AsTime(),
		})
	}
	return out, nil
}

func (a *OrderAdapter) GetOrderState(ctx context.Context, accountID, orderID string) (*handlers.OrderState, error) {
	resp, err := a.svc.GetOrderState(ctx, accountID, orderID, pb.PriceType_PRICE_TYPE_UNSPECIFIED)
	if err != nil {
		return nil, err
	}

	return &handlers.OrderState{
		OrderID:         resp.GetOrderId(),
		ExecutionStatus: resp.GetExecutionReportStatus().String(),
		LotsRequested:   resp.GetLotsRequested(),
		LotsExecuted:    resp.GetLotsExecuted(),
		TotalPrice:      moneyToFloat(resp.GetTotalOrderAmount()),
		Direction:       resp.GetDirection().String(),
		OrderType:       resp.GetOrderType().String(),
		InstrumentUID:   resp.GetInstrumentUid(),
		AccountID:       accountID,
	}, nil
}

func (a *OrderAdapter) CancelOrder(ctx context.Context, accountID, orderID string) (*handlers.CancelOrderResult, error) {
	resp, err := a.svc.CancelOrder(ctx, accountID, orderID)
	if err != nil {
		return nil, err
	}

	return &handlers.CancelOrderResult{
		CancelledAt: resp.GetTime().AsTime(),
	}, nil
}

func (a *OrderAdapter) ReplaceOrder(ctx context.Context, accountID, orderID string, qty int64, price float64) (*handlers.OrderResult, error) {
	req := &investgo.ReplaceOrderRequest{
		AccountId: accountID,
		OrderId:   orderID,
		Quantity:  qty,
		Price:     floatToQuotation(price),
	}

	resp, err := a.svc.ReplaceOrder(ctx, req)
	if err != nil {
		return nil, err
	}

	return &handlers.OrderResult{
		OrderID:         resp.GetOrderId(),
		ExecutionStatus: resp.GetExecutionReportStatus().String(),
		LotsRequested:   resp.GetLotsRequested(),
		LotsExecuted:    resp.GetLotsExecuted(),
		TotalPrice:      moneyToFloat(resp.GetTotalOrderAmount()),
		Direction:       resp.GetDirection().String(),
		OrderType:       resp.GetOrderType().String(),
		Message:         resp.GetMessage(),
	}, nil
}

func parseOrderDirection(s string) pb.OrderDirection {
	switch s {
	case "buy", "BUY":
		return pb.OrderDirection_ORDER_DIRECTION_BUY
	case "sell", "SELL":
		return pb.OrderDirection_ORDER_DIRECTION_SELL
	default:
		return pb.OrderDirection_ORDER_DIRECTION_UNSPECIFIED
	}
}

func parseOrderType(s string) pb.OrderType {
	switch s {
	case "limit", "LIMIT":
		return pb.OrderType_ORDER_TYPE_LIMIT
	case "market", "MARKET":
		return pb.OrderType_ORDER_TYPE_MARKET
	case "bestprice", "BESTPRICE":
		return pb.OrderType_ORDER_TYPE_BESTPRICE
	default:
		return pb.OrderType_ORDER_TYPE_UNSPECIFIED
	}
}

func floatToQuotation(v float64) *pb.Quotation {
	units := int64(v)
	nano := int32((v - float64(units)) * 1e9)
	return &pb.Quotation{Units: units, Nano: nano}
}

func moneyToFloat(m *pb.MoneyValue) float64 {
	if m == nil {
		return 0
	}
	return float64(m.GetUnits()) + float64(m.GetNano())/1e9
}

var _ handlers.OrderService = (*OrderAdapter)(nil)
