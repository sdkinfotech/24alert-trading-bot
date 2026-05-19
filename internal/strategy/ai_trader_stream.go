package strategy

import (
	"context"
	"os"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/internal/marketdata"
)

func aiTraderStreamBookEnabled() bool {
	v := os.Getenv("AI_TRADER_STREAM_BOOK")
	return v == "1" || v == "true" || v == "yes"
}

func (r *Runner) runAITraderOrderbookStream(ctx context.Context, s *AITraderSession, depth int32) {
	if r.streamMgr == nil || !aiTraderStreamBookEnabled() {
		return
	}
	ch, err := r.streamMgr.SubscribeOrderbook(ctx, s.InstrumentID, depth)
	if err != nil {
		r.logger.Warn("ai trader orderbook stream", "error", err)
		return
	}
	defer func() { _ = r.streamMgr.Unsubscribe(s.InstrumentID, marketdata.SubOrderbook) }()
	for {
		select {
		case <-ctx.Done():
			return
		case ob, ok := <-ch:
			if !ok || ob == nil {
				return
			}
			book := pbOrderBookToMarket(ob, s.InstrumentID, depth)
			if book == nil {
				continue
			}
			r.aiTrader.mu.Lock()
			if cur := r.aiTrader.findLocked(s.ID); cur != nil {
				cur.streamBook = book
				cur.streamBookAt = time.Now()
			}
			r.aiTrader.mu.Unlock()
		}
	}
}

func pbOrderBookToMarket(ob *pb.OrderBook, uid string, depth int32) *marketdata.Orderbook {
	if ob == nil {
		return nil
	}
	out := &marketdata.Orderbook{
		InstrumentUID: uid,
		Depth:         depth,
		Time:          time.Now().UTC(),
	}
	for _, b := range ob.GetBids() {
		out.Bids = append(out.Bids, marketdata.OrderbookRow{
			Price:    quotationToFloat(b.GetPrice()),
			Quantity: b.GetQuantity(),
		})
	}
	for _, a := range ob.GetAsks() {
		out.Asks = append(out.Asks, marketdata.OrderbookRow{
			Price:    quotationToFloat(a.GetPrice()),
			Quantity: a.GetQuantity(),
		})
	}
	return out
}

func (r *Runner) preferStreamBook(s *AITraderSession) *marketdata.Orderbook {
	if s == nil {
		return nil
	}
	r.aiTrader.mu.Lock()
	defer r.aiTrader.mu.Unlock()
	cur := r.aiTrader.findLocked(s.ID)
	if cur == nil || cur.streamBook == nil {
		return nil
	}
	if time.Since(cur.streamBookAt) > 3*time.Second {
		return nil
	}
	return cur.streamBook
}
