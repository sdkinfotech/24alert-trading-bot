package strategy

import (
	"context"
	"sync"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/internal/marketdata"
	"github.com/24alert/trading-bot/pkg/logging"
)

type candleKey struct {
	instrumentUID string
	interval      pb.SubscriptionInterval
}

type candleGroup struct {
	hub    *CandleHub
	key    candleKey
	cancel context.CancelFunc

	mu   sync.Mutex
	subs []chan Candle
}

// CandleHub shares one marketdata candle subscription per (instrument, interval).
type CandleHub struct {
	sm     *marketdata.StreamManager
	logger *logging.Logger

	mu     sync.Mutex
	groups map[candleKey]*candleGroup
}

func NewCandleHub(sm *marketdata.StreamManager, logger *logging.Logger) *CandleHub {
	return &CandleHub{
		sm:     sm,
		logger: logger,
		groups: make(map[candleKey]*candleGroup),
	}
}

// Subscribe returns a receive-only candle channel and a cleanup function.
// parentCtx should be the runner lifetime context; cancelling the last subscriber cancels the hub group.
func (h *CandleHub) Subscribe(parentCtx context.Context, instrumentUID string, interval pb.SubscriptionInterval) (<-chan Candle, func(), error) {
	key := candleKey{instrumentUID: instrumentUID, interval: interval}

	h.mu.Lock()
	g, ok := h.groups[key]
	if !ok {
		subCtx, cancel := context.WithCancel(parentCtx)
		src, err := h.sm.SubscribeCandles(subCtx, instrumentUID, interval)
		if err != nil {
			cancel()
			h.mu.Unlock()
			return nil, nil, err
		}
		g = &candleGroup{hub: h, key: key, cancel: cancel}
		h.groups[key] = g
		go g.run(subCtx, src)
	}
	out := make(chan Candle, 128)
	g.mu.Lock()
	g.subs = append(g.subs, out)
	g.mu.Unlock()

	cleanup := func() {
		g.mu.Lock()
		for i, ch := range g.subs {
			if ch == out {
				g.subs = append(g.subs[:i], g.subs[i+1:]...)
				break
			}
		}
		last := len(g.subs) == 0
		g.mu.Unlock()
		if last {
			g.cancel()
		}
	}

	h.mu.Unlock()
	return out, cleanup, nil
}

func (g *candleGroup) run(ctx context.Context, src <-chan *pb.Candle) {
	defer func() {
		g.hub.mu.Lock()
		delete(g.hub.groups, g.key)
		g.hub.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case c, ok := <-src:
			if !ok {
				return
			}
			if c == nil {
				continue
			}
			// The T-Invest SDK shares a single WebSocket stream, so candles
			// for other instruments may arrive on this channel. Filter them.
			if c.GetInstrumentUid() != g.key.instrumentUID {
				continue
			}
			sc := pbCandleToStrategy(c)
			g.mu.Lock()
			subs := append([]chan Candle(nil), g.subs...)
			g.mu.Unlock()
			for _, ch := range subs {
				select {
				case ch <- sc:
				default:
					if g.hub.logger != nil {
						g.hub.logger.Warn("CandleHub: subscriber slow, dropping candle",
							"instrument_uid", g.key.instrumentUID)
					}
				}
			}
		}
	}
}

func pbCandleToStrategy(c *pb.Candle) Candle {
	return Candle{
		InstrumentUID: c.GetInstrumentUid(),
		Open:          quotationToFloat(c.GetOpen()),
		High:          quotationToFloat(c.GetHigh()),
		Low:           quotationToFloat(c.GetLow()),
		Close:         quotationToFloat(c.GetClose()),
		Volume:        c.GetVolume(),
		Time:          c.GetTime().AsTime(),
		IsComplete:    false, // streaming candle; runner merges interval boundaries
	}
}

func quotationToFloat(q *pb.Quotation) float64 {
	if q == nil {
		return 0
	}
	return float64(q.GetUnits()) + float64(q.GetNano())/1e9
}
