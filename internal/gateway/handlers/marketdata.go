package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Candle is the gateway-level candle type.
type Candle struct {
	Open       float64   `json:"open"`
	High       float64   `json:"high"`
	Low        float64   `json:"low"`
	Close      float64   `json:"close"`
	Volume     int64     `json:"volume"`
	Time       time.Time `json:"time"`
	IsComplete bool      `json:"is_complete"`
}

// OrderbookRow is a single bid/ask level.
type OrderbookRow struct {
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
}

// Orderbook is the gateway-level order book.
type Orderbook struct {
	InstrumentUID string         `json:"instrument_uid"`
	Depth         int32          `json:"depth"`
	Bids          []OrderbookRow `json:"bids"`
	Asks          []OrderbookRow `json:"asks"`
	LastPrice     float64        `json:"last_price"`
	ClosePrice    float64        `json:"close_price"`
	Time          time.Time      `json:"time"`
}

// LastPrice is the gateway-level last-price type.
type LastPrice struct {
	InstrumentUID string    `json:"instrument_uid"`
	Price         float64   `json:"price"`
	Time          time.Time `json:"time"`
}

// TradingStatus is the gateway-level trading status.
type TradingStatus struct {
	InstrumentUID        string `json:"instrument_uid"`
	TradingStatus        string `json:"trading_status"`
	LimitOrderAvailable  bool   `json:"limit_order_available"`
	MarketOrderAvailable bool   `json:"market_order_available"`
	APITradeAvailable    bool   `json:"api_trade_available"`
}

// MarketDataService abstracts the market-data backend.
type MarketDataService interface {
	GetCandles(ctx context.Context, instrumentUID string, from, to time.Time, interval string) ([]Candle, error)
	GetOrderbook(ctx context.Context, instrumentUID string, depth int32) (*Orderbook, error)
	GetLastPrices(ctx context.Context, instrumentUIDs []string) ([]LastPrice, error)
	GetTradingStatus(ctx context.Context, instrumentUID string) (*TradingStatus, error)
}

type MarketDataHandlers struct {
	svc MarketDataService
}

func NewMarketDataHandlers(svc MarketDataService) *MarketDataHandlers {
	return &MarketDataHandlers{svc: svc}
}

func (h *MarketDataHandlers) Routes(r chi.Router) {
	r.Get("/api/v1/candles", h.GetCandles)
	r.Get("/api/v1/orderbook/{uid}", h.GetOrderbook)
	r.Get("/api/v1/prices", h.GetPrices)
	r.Get("/api/v1/trading-status/{uid}", h.GetTradingStatus)
}

func (h *MarketDataHandlers) GetCandles(w http.ResponseWriter, r *http.Request) {
	instrumentUID := queryString(r, "instrument_uid", "")
	if instrumentUID == "" {
		respondError(w, http.StatusBadRequest, "instrument_uid query param is required")
		return
	}

	from, err := queryTime(r, "from")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid from time: "+err.Error())
		return
	}
	to, err := queryTime(r, "to")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid to time: "+err.Error())
		return
	}
	if to.IsZero() {
		to = time.Now()
	}
	if from.IsZero() {
		from = to.Add(-24 * time.Hour)
	}

	interval := queryString(r, "interval", "1h")

	candles, err := h.svc.GetCandles(r.Context(), instrumentUID, from, to, interval)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, candles)
}

func (h *MarketDataHandlers) GetOrderbook(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	depth := queryInt32(r, "depth", 20)

	book, err := h.svc.GetOrderbook(r.Context(), uid, depth)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, book)
}

func (h *MarketDataHandlers) GetPrices(w http.ResponseWriter, r *http.Request) {
	instrumentUID := queryString(r, "instrument_uid", "")
	if instrumentUID == "" {
		respondError(w, http.StatusBadRequest, "instrument_uid query param is required")
		return
	}
	prices, err := h.svc.GetLastPrices(r.Context(), []string{instrumentUID})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, prices)
}

func (h *MarketDataHandlers) GetTradingStatus(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	status, err := h.svc.GetTradingStatus(r.Context(), uid)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, status)
}
