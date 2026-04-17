package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/internal/marketdata"
	"github.com/24alert/trading-bot/pkg/logging"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type StreamCandle struct {
	InstrumentUID string  `json:"instrument_uid"`
	Open          float64 `json:"open"`
	High          float64 `json:"high"`
	Low           float64 `json:"low"`
	Close         float64 `json:"close"`
	Volume        int64   `json:"volume"`
	Time          string  `json:"time"`
	IsComplete    bool    `json:"is_complete"`
}

type StreamHandlers struct {
	sm     *marketdata.StreamManager
	logger *logging.Logger
}

func NewStreamHandlers(sm *marketdata.StreamManager, logger *logging.Logger) *StreamHandlers {
	return &StreamHandlers{sm: sm, logger: logger}
}

func (h *StreamHandlers) Routes(r chi.Router) {
	r.Get("/api/v1/stream/candles", h.StreamCandles)
}

func pbQuotationToFloat(q *pb.Quotation) float64 {
	if q == nil {
		return 0
	}
	return float64(q.GetUnits()) + float64(q.GetNano())/1e9
}

func (h *StreamHandlers) StreamCandles(w http.ResponseWriter, r *http.Request) {
	uidsParam := r.URL.Query().Get("uids")
	if uidsParam == "" {
		respondError(w, http.StatusBadRequest, "uids query param is required")
		return
	}

	uids := strings.Split(uidsParam, ",")
	if len(uids) > 50 {
		uids = uids[:50]
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	merged := make(chan StreamCandle, 256)

	for _, uid := range uids {
		uid := uid
		ch, err := h.sm.SubscribeCandles(ctx, uid, pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_MINUTE)
		if err != nil {
			h.logger.Warn("subscribe candles failed", "uid", uid, "error", err)
			continue
		}

		go func() {
			defer func() {
				_ = h.sm.Unsubscribe(uid, marketdata.SubCandles)
			}()
			for {
				select {
				case <-ctx.Done():
					return
				case c, ok := <-ch:
					if !ok {
						return
					}
					sc := StreamCandle{
						InstrumentUID: c.GetInstrumentUid(),
						Open:          pbQuotationToFloat(c.GetOpen()),
						High:          pbQuotationToFloat(c.GetHigh()),
						Low:           pbQuotationToFloat(c.GetLow()),
						Close:         pbQuotationToFloat(c.GetClose()),
						Volume:        c.GetVolume(),
						Time:          c.GetTime().AsTime().Format(time.RFC3339),
						IsComplete:    false,
					}
					select {
					case merged <- sc:
					default:
					}
				}
			}
		}()
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case candle := <-merged:
			data, _ := json.Marshal(candle)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-heartbeat.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
