package adapter

import (
	"context"
	"fmt"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/internal/gateway/handlers"
	"github.com/24alert/trading-bot/internal/marketdata"
)

// MarketDataAdapter implements handlers.MarketDataService by wrapping *marketdata.Service.
type MarketDataAdapter struct {
	svc *marketdata.Service
}

func NewMarketDataAdapter(svc *marketdata.Service) *MarketDataAdapter {
	return &MarketDataAdapter{svc: svc}
}

func (a *MarketDataAdapter) GetCandles(ctx context.Context, instrumentUID string, from, to time.Time, interval string) ([]handlers.Candle, error) {
	pbInterval, err := parseCandleInterval(interval)
	if err != nil {
		return nil, err
	}

	candles, err := a.svc.GetCandles(ctx, instrumentUID, from, to, pbInterval)
	if err != nil {
		return nil, err
	}

	out := make([]handlers.Candle, 0, len(candles))
	for _, c := range candles {
		out = append(out, handlers.Candle{
			Open:       c.Open,
			High:       c.High,
			Low:        c.Low,
			Close:      c.Close,
			Volume:     c.Volume,
			Time:       c.Time,
			IsComplete: c.IsComplete,
		})
	}
	return out, nil
}

func (a *MarketDataAdapter) GetOrderbook(ctx context.Context, instrumentUID string, depth int32) (*handlers.Orderbook, error) {
	ob, err := a.svc.GetOrderbook(ctx, instrumentUID, depth)
	if err != nil {
		return nil, err
	}

	hob := &handlers.Orderbook{
		InstrumentUID: ob.InstrumentUID,
		Depth:         ob.Depth,
		LastPrice:     ob.LastPrice,
		ClosePrice:    ob.ClosePrice,
		Time:          ob.Time,
	}
	for _, b := range ob.Bids {
		hob.Bids = append(hob.Bids, handlers.OrderbookRow{Price: b.Price, Quantity: b.Quantity})
	}
	for _, a := range ob.Asks {
		hob.Asks = append(hob.Asks, handlers.OrderbookRow{Price: a.Price, Quantity: a.Quantity})
	}
	return hob, nil
}

func (a *MarketDataAdapter) GetLastPrices(ctx context.Context, instrumentUIDs []string) ([]handlers.LastPrice, error) {
	prices, err := a.svc.GetLastPrices(ctx, instrumentUIDs)
	if err != nil {
		return nil, err
	}

	out := make([]handlers.LastPrice, 0, len(prices))
	for _, p := range prices {
		out = append(out, handlers.LastPrice{
			InstrumentUID: p.InstrumentUID,
			Price:         p.Price,
			Time:          p.Time,
		})
	}
	return out, nil
}

func (a *MarketDataAdapter) GetTradingStatus(ctx context.Context, instrumentUID string) (*handlers.TradingStatus, error) {
	st, err := a.svc.GetTradingStatus(ctx, instrumentUID)
	if err != nil {
		return nil, err
	}

	return &handlers.TradingStatus{
		InstrumentUID:        st.InstrumentUID,
		TradingStatus:        st.TradingStatus,
		LimitOrderAvailable:  st.LimitOrderAvailable,
		MarketOrderAvailable: st.MarketOrderAvailable,
		APITradeAvailable:    st.APITradeAvailable,
	}, nil
}

func parseCandleInterval(s string) (pb.CandleInterval, error) {
	switch s {
	case "1m", "1min":
		return pb.CandleInterval_CANDLE_INTERVAL_1_MIN, nil
	case "2m", "2min":
		return pb.CandleInterval_CANDLE_INTERVAL_2_MIN, nil
	case "3m", "3min":
		return pb.CandleInterval_CANDLE_INTERVAL_3_MIN, nil
	case "5m", "5min":
		return pb.CandleInterval_CANDLE_INTERVAL_5_MIN, nil
	case "10m", "10min":
		return pb.CandleInterval_CANDLE_INTERVAL_10_MIN, nil
	case "15m", "15min":
		return pb.CandleInterval_CANDLE_INTERVAL_15_MIN, nil
	case "30m", "30min":
		return pb.CandleInterval_CANDLE_INTERVAL_30_MIN, nil
	case "1h", "hour":
		return pb.CandleInterval_CANDLE_INTERVAL_HOUR, nil
	case "2h":
		return pb.CandleInterval_CANDLE_INTERVAL_2_HOUR, nil
	case "4h":
		return pb.CandleInterval_CANDLE_INTERVAL_4_HOUR, nil
	case "1d", "day":
		return pb.CandleInterval_CANDLE_INTERVAL_DAY, nil
	case "1w", "week":
		return pb.CandleInterval_CANDLE_INTERVAL_WEEK, nil
	case "1M", "month":
		return pb.CandleInterval_CANDLE_INTERVAL_MONTH, nil
	default:
		return pb.CandleInterval_CANDLE_INTERVAL_UNSPECIFIED, fmt.Errorf("unknown candle interval: %s", s)
	}
}

var _ handlers.MarketDataService = (*MarketDataAdapter)(nil)
