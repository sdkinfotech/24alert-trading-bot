package marketdata

import (
	"context"
	"fmt"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/pkg/logging"
	"github.com/24alert/trading-bot/pkg/metrics"
	"github.com/24alert/trading-bot/pkg/tinvest"
)

// Candle is a local representation of a historic candle.
type Candle struct {
	Open       float64
	High       float64
	Low        float64
	Close      float64
	Volume     int64
	Time       time.Time
	IsComplete bool
}

// OrderbookRow represents one bid/ask level.
type OrderbookRow struct {
	Price    float64
	Quantity int64
}

// Orderbook holds an order book snapshot.
type Orderbook struct {
	InstrumentUID string
	Depth         int32
	Bids          []OrderbookRow
	Asks          []OrderbookRow
	LastPrice     float64
	ClosePrice    float64
	Time          time.Time
}

// LastPrice holds a last-known price for an instrument.
type LastPrice struct {
	InstrumentUID string
	Price         float64
	Time          time.Time
}

// ClosePrice holds the previous-day close price for an instrument.
type ClosePrice struct {
	InstrumentUID string
	Price         float64
	Time          time.Time
}

// TradingStatus holds trading status information for an instrument.
type TradingStatus struct {
	InstrumentUID        string
	TradingStatus        string
	LimitOrderAvailable  bool
	MarketOrderAvailable bool
	APITradeAvailable    bool
}

// Service wraps T-Invest MarketData API with rate limiting and caching.
type Service struct {
	tinvestClient *tinvest.Client
	rateLimiter   *tinvest.RateLimiterManager
	instruments   *InstrumentCache
	prices        *PriceCache
	logger        *logging.Logger
}

func NewService(
	client *tinvest.Client,
	rl *tinvest.RateLimiterManager,
	instruments *InstrumentCache,
	prices *PriceCache,
	logger *logging.Logger,
) *Service {
	return &Service{
		tinvestClient: client,
		rateLimiter:   rl,
		instruments:   instruments,
		prices:        prices,
		logger:        logger,
	}
}

// GetCandles returns historic candles for the given instrument.
func (s *Service) GetCandles(
	ctx context.Context,
	instrumentUID string,
	from, to time.Time,
	interval pb.CandleInterval,
) ([]Candle, error) {
	l := s.logger.WithContext(ctx)
	l.Info("GetCandles", "instrument_uid", instrumentUID, "from", from, "to", to, "interval", interval.String())

	if err := s.rateLimiter.Wait(ctx, "get_candles"); err != nil {
		return nil, fmt.Errorf("GetCandles: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.MarketDataServiceClient().GetCandles(
		instrumentUID,
		interval,
		from, to,
		pb.GetCandlesRequest_CANDLE_SOURCE_UNSPECIFIED,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("GetCandles: %w", err)
	}

	candles := make([]Candle, 0, len(resp.GetCandles()))
	for _, c := range resp.GetCandles() {
		candles = append(candles, Candle{
			Open:       quotationToFloat(c.GetOpen()),
			High:       quotationToFloat(c.GetHigh()),
			Low:        quotationToFloat(c.GetLow()),
			Close:      quotationToFloat(c.GetClose()),
			Volume:     c.GetVolume(),
			Time:       c.GetTime().AsTime(),
			IsComplete: c.GetIsComplete(),
		})
	}

	l.Info("GetCandles completed", "instrument_uid", instrumentUID, "count", len(candles))
	return candles, nil
}

// GetOrderbook returns the current order book for the given instrument.
func (s *Service) GetOrderbook(ctx context.Context, instrumentUID string, depth int32) (*Orderbook, error) {
	l := s.logger.WithContext(ctx)
	l.Info("GetOrderbook", "instrument_uid", instrumentUID, "depth", depth)

	if err := s.rateLimiter.Wait(ctx, "get_orderbook"); err != nil {
		return nil, fmt.Errorf("GetOrderbook: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.MarketDataServiceClient().GetOrderBook(instrumentUID, depth)
	if err != nil {
		return nil, fmt.Errorf("GetOrderbook: %w", err)
	}

	metrics.MarketDataUpdatesTotal.WithLabelValues("orderbook").Inc()

	ob := &Orderbook{
		InstrumentUID: instrumentUID,
		Depth:         depth,
		LastPrice:     quotationToFloat(resp.GetLastPrice()),
		ClosePrice:    quotationToFloat(resp.GetClosePrice()),
		Time:          time.Now(),
	}
	for _, b := range resp.GetBids() {
		ob.Bids = append(ob.Bids, OrderbookRow{
			Price:    quotationToFloat(b.GetPrice()),
			Quantity: b.GetQuantity(),
		})
	}
	for _, a := range resp.GetAsks() {
		ob.Asks = append(ob.Asks, OrderbookRow{
			Price:    quotationToFloat(a.GetPrice()),
			Quantity: a.GetQuantity(),
		})
	}

	l.Info("GetOrderbook completed", "instrument_uid", instrumentUID, "bids", len(ob.Bids), "asks", len(ob.Asks))
	return ob, nil
}

// GetLastPrices returns last prices for the given instruments.
func (s *Service) GetLastPrices(ctx context.Context, instrumentUIDs []string) ([]LastPrice, error) {
	l := s.logger.WithContext(ctx)
	l.Info("GetLastPrices", "count", len(instrumentUIDs))

	if err := s.rateLimiter.Wait(ctx, "get_last_prices"); err != nil {
		return nil, fmt.Errorf("GetLastPrices: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.MarketDataServiceClient().GetLastPrices(instrumentUIDs)
	if err != nil {
		return nil, fmt.Errorf("GetLastPrices: %w", err)
	}

	metrics.MarketDataUpdatesTotal.WithLabelValues("price").Add(float64(len(resp.GetLastPrices())))

	prices := make([]LastPrice, 0, len(resp.GetLastPrices()))
	for _, lp := range resp.GetLastPrices() {
		price := quotationToFloat(lp.GetPrice())
		uid := lp.GetInstrumentUid()
		ts := lp.GetTime().AsTime()
		prices = append(prices, LastPrice{
			InstrumentUID: uid,
			Price:         price,
			Time:          ts,
		})
		s.prices.SetLastPrice(uid, price)
		metrics.MarketDataStaleness.WithLabelValues(uid).Set(time.Since(ts).Seconds())
	}

	l.Info("GetLastPrices completed", "count", len(prices))
	return prices, nil
}

// GetClosePrices returns previous-day close prices for the given instruments.
func (s *Service) GetClosePrices(ctx context.Context, instrumentUIDs []string) ([]ClosePrice, error) {
	l := s.logger.WithContext(ctx)
	l.Info("GetClosePrices", "count", len(instrumentUIDs))

	if err := s.rateLimiter.Wait(ctx, "get_close_prices"); err != nil {
		return nil, fmt.Errorf("GetClosePrices: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.MarketDataServiceClient().GetClosePrices(instrumentUIDs)
	if err != nil {
		return nil, fmt.Errorf("GetClosePrices: %w", err)
	}

	out := make([]ClosePrice, 0, len(resp.GetClosePrices()))
	for _, cp := range resp.GetClosePrices() {
		out = append(out, ClosePrice{
			InstrumentUID: cp.GetInstrumentUid(),
			Price:         quotationToFloat(cp.GetPrice()),
			Time:          cp.GetTime().AsTime(),
		})
	}

	l.Info("GetClosePrices completed", "count", len(out))
	return out, nil
}

// GetTradingStatus returns the trading status for the given instrument.
func (s *Service) GetTradingStatus(ctx context.Context, instrumentUID string) (*TradingStatus, error) {
	l := s.logger.WithContext(ctx)
	l.Info("GetTradingStatus", "instrument_uid", instrumentUID)

	if err := s.rateLimiter.Wait(ctx, "get_trading_status"); err != nil {
		return nil, fmt.Errorf("GetTradingStatus: rate limit: %w", err)
	}

	resp, err := s.tinvestClient.MarketDataServiceClient().GetTradingStatus(instrumentUID)
	if err != nil {
		return nil, fmt.Errorf("GetTradingStatus: %w", err)
	}

	status := &TradingStatus{
		InstrumentUID:        instrumentUID,
		TradingStatus:        resp.GetTradingStatus().String(),
		LimitOrderAvailable:  resp.GetLimitOrderAvailableFlag(),
		MarketOrderAvailable: resp.GetMarketOrderAvailableFlag(),
		APITradeAvailable:    resp.GetApiTradeAvailableFlag(),
	}

	l.Info("GetTradingStatus completed", "instrument_uid", instrumentUID, "status", status.TradingStatus)
	return status, nil
}

func quotationToFloat(q *pb.Quotation) float64 {
	if q == nil {
		return 0
	}
	return float64(q.GetUnits()) + float64(q.GetNano())/1e9
}
