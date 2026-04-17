package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
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

type StreamLevel struct {
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
}

type StreamOrderBookMsg struct {
	Type  string        `json:"type"`
	UID   string        `json:"uid,omitempty"`
	Depth int32         `json:"depth,omitempty"`
	Bids  []StreamLevel `json:"bids,omitempty"`
	Asks  []StreamLevel `json:"asks,omitempty"`
	TS    int64         `json:"ts,omitempty"`
	Error string        `json:"error,omitempty"`
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
	r.Get("/api/v1/stream/orderbook", h.StreamOrderBook)
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

// StreamOrderBook streams real-time order book snapshots via WebSocket for
// the requested comma-separated list of instrument UIDs.
//
// Query params:
//
//	uids=<csv>    required, up to 50 instruments
//	depth=10..50  optional, default 20
//
// Messages (JSON text frames):
//
//	{"type":"snapshot","uid":"<uid>","depth":20,"bids":[{"price":..,"quantity":..}],"asks":[...],"ts":<ms>}
//	{"type":"ping"}  — server heartbeat every 15s; client replies {"type":"pong"}
//	{"type":"error","error":"<msg>"}  — non-fatal notice
func (h *StreamHandlers) StreamOrderBook(w http.ResponseWriter, r *http.Request) {
	uidsParam := r.URL.Query().Get("uids")
	if uidsParam == "" {
		respondError(w, http.StatusBadRequest, "uids query param is required")
		return
	}

	uids := strings.Split(uidsParam, ",")
	cleaned := uids[:0]
	for _, u := range uids {
		u = strings.TrimSpace(u)
		if u != "" {
			cleaned = append(cleaned, u)
		}
	}
	uids = cleaned
	if len(uids) == 0 {
		respondError(w, http.StatusBadRequest, "uids query param is empty after trim")
		return
	}
	if len(uids) > 50 {
		uids = uids[:50]
	}

	depth := int32(20)
	if raw := r.URL.Query().Get("depth"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			switch {
			case v <= 10:
				depth = 10
			case v <= 20:
				depth = 20
			case v <= 30:
				depth = 30
			case v <= 40:
				depth = 40
			default:
				depth = 50
			}
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Reader goroutine: detects client disconnect and accepts pong frames.
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	merged := make(chan StreamOrderBookMsg, 1024)
	// writeMu guards conn writes across the publisher and the heartbeat ticker.
	var writeMu sync.Mutex
	writeJSON := func(msg StreamOrderBookMsg) error {
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return conn.WriteMessage(websocket.TextMessage, data)
	}

	subscribed := make([]string, 0, len(uids))
	for _, uid := range uids {
		uid := uid
		ch, err := h.sm.SubscribeOrderbook(ctx, uid, depth)
		if err != nil {
			h.logger.Warn("subscribe orderbook failed", "uid", uid, "error", err)
			_ = writeJSON(StreamOrderBookMsg{
				Type:  "error",
				UID:   uid,
				Error: err.Error(),
			})
			continue
		}
		subscribed = append(subscribed, uid)

		go func() {
			defer func() {
				_ = h.sm.Unsubscribe(uid, marketdata.SubOrderbook)
			}()
			for {
				select {
				case <-ctx.Done():
					return
				case ob, ok := <-ch:
					if !ok {
						return
					}
					msg := StreamOrderBookMsg{
						Type:  "snapshot",
						UID:   ob.GetInstrumentUid(),
						Depth: ob.GetDepth(),
						TS:    time.Now().UnixMilli(),
					}
					if msg.UID == "" {
						msg.UID = uid
					}
					if msg.Depth == 0 {
						msg.Depth = depth
					}
					for _, b := range ob.GetBids() {
						msg.Bids = append(msg.Bids, StreamLevel{
							Price:    pbQuotationToFloat(b.GetPrice()),
							Quantity: b.GetQuantity(),
						})
					}
					for _, a := range ob.GetAsks() {
						msg.Asks = append(msg.Asks, StreamLevel{
							Price:    pbQuotationToFloat(a.GetPrice()),
							Quantity: a.GetQuantity(),
						})
					}
					select {
					case merged <- msg:
					case <-ctx.Done():
						return
					default:
						// drop if consumer is slow
					}
				}
			}
		}()
	}

	h.logger.Info("stream orderbook started",
		"subscribed", len(subscribed),
		"requested", len(uids),
		"depth", depth,
	)

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-merged:
			if err := writeJSON(msg); err != nil {
				return
			}
		case <-heartbeat.C:
			if err := writeJSON(StreamOrderBookMsg{Type: "ping"}); err != nil {
				return
			}
		}
	}
}
