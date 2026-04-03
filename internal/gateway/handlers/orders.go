package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/24alert/trading-bot/pkg/metrics"
)

// OrderResult is the gateway-level representation of an order placement result.
type OrderResult struct {
	OrderID         string  `json:"order_id"`
	ExecutionStatus string  `json:"execution_status"`
	LotsRequested   int64   `json:"lots_requested"`
	LotsExecuted    int64   `json:"lots_executed"`
	TotalPrice      float64 `json:"total_price"`
	Direction       string  `json:"direction"`
	OrderType       string  `json:"order_type"`
	Message         string  `json:"message,omitempty"`
}

// OrderState is the gateway-level view of a single order's state.
type OrderState struct {
	OrderID         string  `json:"order_id"`
	ExecutionStatus string  `json:"execution_status"`
	LotsRequested   int64   `json:"lots_requested"`
	LotsExecuted    int64   `json:"lots_executed"`
	TotalPrice      float64 `json:"total_price"`
	Direction       string  `json:"direction"`
	OrderType       string  `json:"order_type"`
	InstrumentUID   string  `json:"instrument_uid"`
	AccountID       string  `json:"account_id"`
}

// OrderSummary is a short representation used in list responses.
type OrderSummary struct {
	OrderID       string    `json:"order_id"`
	InstrumentUID string    `json:"instrument_uid"`
	Direction     string    `json:"direction"`
	OrderType     string    `json:"order_type"`
	Lots          int64     `json:"lots"`
	Price         float64   `json:"price"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// CancelOrderResult is the gateway-level representation of a cancel response.
type CancelOrderResult struct {
	CancelledAt time.Time `json:"cancelled_at"`
}

// OrderService abstracts the order backend for the gateway handlers.
type OrderService interface {
	PostOrder(ctx context.Context, accountID, instrumentUID string, qty int64, direction, orderType string, price float64) (*OrderResult, error)
	GetOrders(ctx context.Context, accountID string) ([]OrderSummary, error)
	GetOrderState(ctx context.Context, accountID, orderID string) (*OrderState, error)
	CancelOrder(ctx context.Context, accountID, orderID string) (*CancelOrderResult, error)
	ReplaceOrder(ctx context.Context, accountID, orderID string, qty int64, price float64) (*OrderResult, error)
}

type OrderHandlers struct {
	svc OrderService
}

func NewOrderHandlers(svc OrderService) *OrderHandlers {
	return &OrderHandlers{svc: svc}
}

func (h *OrderHandlers) Routes(r chi.Router) {
	r.Post("/api/v1/orders", h.PostOrder)
	r.Get("/api/v1/orders", h.ListOrders)
	r.Get("/api/v1/orders/{id}", h.GetOrderState)
	r.Delete("/api/v1/orders/{id}", h.CancelOrder)
	r.Put("/api/v1/orders/{id}", h.ReplaceOrder)
}

type postOrderRequest struct {
	InstrumentUID string  `json:"instrument_uid"`
	Quantity      int64   `json:"quantity"`
	Direction     string  `json:"direction"`
	OrderType     string  `json:"order_type"`
	Price         float64 `json:"price"`
	AccountID     string  `json:"account_id"`
}

func (h *OrderHandlers) PostOrder(w http.ResponseWriter, r *http.Request) {
	var req postOrderRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.AccountID == "" || req.InstrumentUID == "" {
		respondError(w, http.StatusBadRequest, "account_id and instrument_uid are required")
		return
	}

	start := time.Now()
	result, err := h.svc.PostOrder(r.Context(), req.AccountID, req.InstrumentUID, req.Quantity, req.Direction, req.OrderType, req.Price)
	elapsed := time.Since(start).Seconds()

	metrics.OrderLatency.WithLabelValues(req.Direction, req.OrderType).Observe(elapsed)
	if err != nil {
		metrics.OrderErrorsTotal.WithLabelValues(req.Direction, req.OrderType).Inc()
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	metrics.OrdersTotal.WithLabelValues(req.Direction, req.OrderType, result.ExecutionStatus).Inc()
	respondJSON(w, http.StatusCreated, result)
}

func (h *OrderHandlers) ListOrders(w http.ResponseWriter, r *http.Request) {
	accountID := queryString(r, "account_id", "")
	if accountID == "" {
		respondError(w, http.StatusBadRequest, "account_id query param is required")
		return
	}
	orders, err := h.svc.GetOrders(r.Context(), accountID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, orders)
}

func (h *OrderHandlers) GetOrderState(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	accountID := queryString(r, "account_id", "")
	if accountID == "" {
		respondError(w, http.StatusBadRequest, "account_id query param is required")
		return
	}
	state, err := h.svc.GetOrderState(r.Context(), accountID, orderID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, state)
}

func (h *OrderHandlers) CancelOrder(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	accountID := queryString(r, "account_id", "")
	if accountID == "" {
		respondError(w, http.StatusBadRequest, "account_id query param is required")
		return
	}
	result, err := h.svc.CancelOrder(r.Context(), accountID, orderID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}

type replaceOrderRequest struct {
	Quantity int64   `json:"quantity"`
	Price    float64 `json:"price"`
}

func (h *OrderHandlers) ReplaceOrder(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	accountID := queryString(r, "account_id", "")
	if accountID == "" {
		respondError(w, http.StatusBadRequest, "account_id query param is required")
		return
	}

	var req replaceOrderRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	result, err := h.svc.ReplaceOrder(r.Context(), accountID, orderID, req.Quantity, req.Price)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}
