package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// StopOrderResult is the gateway-level representation of a stop-order placement.
type StopOrderResult struct {
	StopOrderID string `json:"stop_order_id"`
}

// StopOrderSummary is a short representation for list responses.
type StopOrderSummary struct {
	StopOrderID   string    `json:"stop_order_id"`
	InstrumentUID string    `json:"instrument_uid"`
	Direction     string    `json:"direction"`
	StopOrderType string    `json:"stop_order_type"`
	Lots          int64     `json:"lots"`
	StopPrice     float64   `json:"stop_price"`
	Price         float64   `json:"price"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	ExpirationAt  time.Time `json:"expiration_at,omitempty"`
}

// CancelStopOrderResult is the gateway-level representation of a cancel response.
type CancelStopOrderResult struct {
	CancelledAt time.Time `json:"cancelled_at"`
}

// StopOrderService abstracts the stop-order backend.
type StopOrderService interface {
	PostStopOrder(ctx context.Context, accountID, instrumentUID string, qty int64, direction, stopOrderType string, stopPrice, price float64) (*StopOrderResult, error)
	GetStopOrders(ctx context.Context, accountID string) ([]StopOrderSummary, error)
	CancelStopOrder(ctx context.Context, accountID, stopOrderID string) (*CancelStopOrderResult, error)
}

type StopOrderHandlers struct {
	svc StopOrderService
}

func NewStopOrderHandlers(svc StopOrderService) *StopOrderHandlers {
	return &StopOrderHandlers{svc: svc}
}

func (h *StopOrderHandlers) Routes(r chi.Router) {
	r.Post("/api/v1/stop-orders", h.PostStopOrder)
	r.Get("/api/v1/stop-orders", h.ListStopOrders)
	r.Delete("/api/v1/stop-orders/{id}", h.CancelStopOrder)
}

type postStopOrderRequest struct {
	InstrumentUID string  `json:"instrument_uid"`
	Quantity      int64   `json:"quantity"`
	Direction     string  `json:"direction"`
	StopOrderType string  `json:"stop_order_type"`
	StopPrice     float64 `json:"stop_price"`
	Price         float64 `json:"price"`
	AccountID     string  `json:"account_id"`
}

func (h *StopOrderHandlers) PostStopOrder(w http.ResponseWriter, r *http.Request) {
	var req postStopOrderRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.AccountID == "" || req.InstrumentUID == "" {
		respondError(w, http.StatusBadRequest, "account_id and instrument_uid are required")
		return
	}

	result, err := h.svc.PostStopOrder(r.Context(), req.AccountID, req.InstrumentUID, req.Quantity, req.Direction, req.StopOrderType, req.StopPrice, req.Price)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, result)
}

func (h *StopOrderHandlers) ListStopOrders(w http.ResponseWriter, r *http.Request) {
	accountID := queryString(r, "account_id", "")
	if accountID == "" {
		respondError(w, http.StatusBadRequest, "account_id query param is required")
		return
	}
	orders, err := h.svc.GetStopOrders(r.Context(), accountID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, orders)
}

func (h *StopOrderHandlers) CancelStopOrder(w http.ResponseWriter, r *http.Request) {
	stopOrderID := chi.URLParam(r, "id")
	accountID := queryString(r, "account_id", "")
	if accountID == "" {
		respondError(w, http.StatusBadRequest, "account_id query param is required")
		return
	}
	result, err := h.svc.CancelStopOrder(r.Context(), accountID, stopOrderID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}
