package strategy

import (
	"strings"

	"github.com/russianinvestments/invest-api-go-sdk/investgo"
	pb "github.com/russianinvestments/invest-api-go-sdk/proto"
)

func buildPostOrderRequest(accountID string, sig Signal) *investgo.PostOrderRequest {
	dir := parseOrderDirection(sig.Direction)
	ot := parseOrderType(sig.OrderType)
	if ot == pb.OrderType_ORDER_TYPE_UNSPECIFIED {
		ot = pb.OrderType_ORDER_TYPE_MARKET
	}
	price := sig.Price
	if ot == pb.OrderType_ORDER_TYPE_MARKET {
		price = 0
	}
	return &investgo.PostOrderRequest{
		InstrumentId: sig.InstrumentUID,
		Quantity:     sig.Quantity,
		Direction:    dir,
		AccountId:    accountID,
		OrderType:    ot,
		Price:        floatToQuotation(price),
	}
}

func parseOrderDirection(s string) pb.OrderDirection {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "buy":
		return pb.OrderDirection_ORDER_DIRECTION_BUY
	case "sell":
		return pb.OrderDirection_ORDER_DIRECTION_SELL
	default:
		return pb.OrderDirection_ORDER_DIRECTION_UNSPECIFIED
	}
}

func parseOrderType(s string) pb.OrderType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "limit":
		return pb.OrderType_ORDER_TYPE_LIMIT
	case "market", "":
		return pb.OrderType_ORDER_TYPE_MARKET
	case "bestprice":
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
