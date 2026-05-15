package grpcadapter

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	commonv1 "github.com/24alert/trading-bot/gen/go/common/v1"
	strategyv1 "github.com/24alert/trading-bot/gen/go/strategy/v1"
	"github.com/24alert/trading-bot/internal/strategy"
	"github.com/24alert/trading-bot/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Adapter wraps a remote StrategyService gRPC server as strategy.Strategy.
type Adapter struct {
	endpoint string
	timeout  time.Duration
	logger   *logging.Logger

	mu      sync.Mutex
	conn    *grpc.ClientConn
	client  strategyv1.StrategyServiceClient
	info    *commonv1.StrategyInfo
	stopped bool
}

// New creates a gRPC strategy adapter. The actual connection is established lazily
// on the first call to OnCandle/Configure/Info, allowing reconnect with backoff.
func New(endpoint string, evalTimeout time.Duration, logger *logging.Logger) (strategy.Strategy, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("grpc strategy: empty endpoint")
	}
	if evalTimeout <= 0 {
		evalTimeout = 5 * time.Second
	}
	return &Adapter{endpoint: endpoint, timeout: evalTimeout, logger: logger}, nil
}

func (a *Adapter) ensureConn(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn != nil {
		return nil
	}
	dctx, dcancel := context.WithTimeout(ctx, 10*time.Second)
	defer dcancel()
	conn, err := grpc.DialContext(dctx, a.endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("grpc dial %q: %w", a.endpoint, err)
	}
	a.conn = conn
	a.client = strategyv1.NewStrategyServiceClient(conn)
	return nil
}

func (a *Adapter) closeConn() {
	a.mu.Lock()
	if a.conn != nil {
		_ = a.conn.Close()
		a.conn = nil
		a.client = nil
	}
	a.mu.Unlock()
}

func (a *Adapter) Info() strategy.StrategyInfo {
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()
	if err := a.ensureConn(ctx); err != nil {
		return strategy.StrategyInfo{Name: "grpc", Description: err.Error()}
	}
	a.mu.Lock()
	if a.info != nil {
		cp := a.info
		a.mu.Unlock()
		return protoInfoToLocal(cp)
	}
	cl := a.client
	a.mu.Unlock()
	if cl == nil {
		return strategy.StrategyInfo{Name: "grpc"}
	}
	resp, err := cl.GetInfo(ctx, &strategyv1.GetStrategyInfoRequest{})
	if err != nil || resp == nil {
		return strategy.StrategyInfo{Name: "grpc", Version: "unknown"}
	}
	a.mu.Lock()
	a.info = resp
	a.mu.Unlock()
	return protoInfoToLocal(resp)
}

func protoInfoToLocal(in *commonv1.StrategyInfo) strategy.StrategyInfo {
	if in == nil {
		return strategy.StrategyInfo{}
	}
	return strategy.StrategyInfo{
		Name:        in.GetName(),
		Version:     in.GetVersion(),
		Description: in.GetDescription(),
	}
}

func (a *Adapter) Configure(params map[string]string) error {
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()
	if err := a.ensureConn(ctx); err != nil {
		return err
	}
	a.mu.Lock()
	cl := a.client
	a.mu.Unlock()
	name := params["name"]
	ver := params["version"]
	_, err := cl.Configure(ctx, &commonv1.StrategyConfig{
		Name:       name,
		Version:    ver,
		Parameters: params,
	})
	return err
}

func (a *Adapter) OnCandle(c strategy.Candle) []strategy.Signal {
	if a.stopped {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()
	if err := a.ensureConn(ctx); err != nil {
		if a.logger != nil {
			a.logger.Warn("grpc strategy: ensureConn", "error", err)
		}
		return nil
	}
	a.mu.Lock()
	cl := a.client
	a.mu.Unlock()
	if cl == nil {
		return nil
	}

	ms := &commonv1.MarketState{
		LastPrices:    []*commonv1.Quotation{floatToQuot(c.Close)},
		Volumes:       []int64{c.Volume},
		TimestampUs:   c.Time.UnixMicro(),
		InstrumentUid: c.InstrumentUID,
		Candles: []*commonv1.StrategyCandlePoint{{
			TimeUs:     c.Time.UnixMicro(),
			Open:       floatToQuot(c.Open),
			High:       floatToQuot(c.High),
			Low:        floatToQuot(c.Low),
			Close:      floatToQuot(c.Close),
			Volume:     c.Volume,
			IsComplete: c.IsComplete,
		}},
	}

	ps, err := cl.Evaluate(ctx, ms)
	if err != nil {
		if a.logger != nil {
			a.logger.Warn("grpc Evaluate failed", "error", err)
		}
		a.closeConn()
		return nil
	}
	if ps == nil || ps.GetDirection() == commonv1.OrderDirection_ORDER_DIRECTION_UNSPECIFIED || ps.GetQuantity() <= 0 {
		return nil
	}
	return []strategy.Signal{protoSignalToLocal(ps)}
}

func (a *Adapter) OnOrderbook(ob strategy.Orderbook) []strategy.Signal {
	if a.stopped {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()
	if err := a.ensureConn(ctx); err != nil {
		return nil
	}
	a.mu.Lock()
	cl := a.client
	a.mu.Unlock()
	if cl == nil {
		return nil
	}
	ms := &commonv1.MarketState{
		TimestampUs:   time.Now().UnixMicro(),
		InstrumentUid: ob.InstrumentUID,
	}
	for i, b := range ob.Bids {
		ms.Bids = append(ms.Bids, &commonv1.OrderbookLevel{
			DepthRank: int32(i + 1),
			Price:     floatToQuot(b.Price),
			Quantity:  b.Quantity,
		})
	}
	for i, ask := range ob.Asks {
		ms.Asks = append(ms.Asks, &commonv1.OrderbookLevel{
			DepthRank: int32(i + 1),
			Price:     floatToQuot(ask.Price),
			Quantity:  ask.Quantity,
		})
	}
	ps, err := cl.Evaluate(ctx, ms)
	if err != nil {
		a.closeConn()
		return nil
	}
	if ps == nil || ps.GetDirection() == commonv1.OrderDirection_ORDER_DIRECTION_UNSPECIFIED || ps.GetQuantity() <= 0 {
		return nil
	}
	return []strategy.Signal{protoSignalToLocal(ps)}
}

func (a *Adapter) OnExecution(ev strategy.ExecutionEvent) {
	if a.logger != nil {
		a.logger.Info("grpc strategy execution",
			"order_id", ev.OrderID,
			"instrument", ev.InstrumentUID,
			"status", ev.Status,
			"filled_qty", ev.FilledQty,
			"avg_price", ev.AvgPrice,
			"message", ev.Message,
		)
	}
}

func (a *Adapter) Stop() {
	a.stopped = true
	a.closeConn()
}

func floatToQuot(x float64) *commonv1.Quotation {
	u := int64(x)
	n := int32((x - float64(u)) * 1e9)
	return &commonv1.Quotation{Units: u, Nano: n}
}

func protoSignalToLocal(ps *commonv1.Signal) strategy.Signal {
	ot := "market"
	switch ps.GetOrderType() {
	case commonv1.OrderType_ORDER_TYPE_LIMIT:
		ot = "limit"
	case commonv1.OrderType_ORDER_TYPE_BESTPRICE:
		ot = "bestprice"
	}
	dir := ""
	switch ps.GetDirection() {
	case commonv1.OrderDirection_ORDER_DIRECTION_SELL:
		dir = "sell"
	case commonv1.OrderDirection_ORDER_DIRECTION_BUY:
		dir = "buy"
	default:
		dir = strings.ToLower(strings.TrimPrefix(ps.GetDirection().String(), "ORDER_DIRECTION_"))
	}
	px := quotationToFloat(ps.GetPrice())
	return strategy.Signal{
		InstrumentUID: ps.GetInstrumentUid(),
		Direction:     dir,
		Quantity:      ps.GetQuantity(),
		Price:         px,
		OrderType:     ot,
		Reason:        ps.GetReason(),
	}
}

func quotationToFloat(q *commonv1.Quotation) float64 {
	if q == nil {
		return 0
	}
	return float64(q.GetUnits()) + float64(q.GetNano())/1e9
}
