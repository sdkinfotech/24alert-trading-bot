package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Position is the gateway-level position type.
type Position struct {
	InstrumentUID  string  `json:"instrument_uid"`
	InstrumentType string  `json:"instrument_type"`
	FIGI           string  `json:"figi"`
	Quantity       float64 `json:"quantity"`
	AveragePrice   float64 `json:"average_price"`
	ExpectedYield  float64 `json:"expected_yield"`
	CurrentPrice   float64 `json:"current_price"`
	Currency       string  `json:"currency"`
	Blocked        bool    `json:"blocked"`
}

// PortfolioInfo is the gateway-level portfolio info type.
type PortfolioInfo struct {
	AccountID             string     `json:"account_id"`
	TotalAmountShares     float64    `json:"total_amount_shares"`
	TotalAmountBonds      float64    `json:"total_amount_bonds"`
	TotalAmountETF        float64    `json:"total_amount_etf"`
	TotalAmountCurrencies float64    `json:"total_amount_currencies"`
	TotalAmountFutures    float64    `json:"total_amount_futures"`
	ExpectedYield         float64    `json:"expected_yield"`
	Positions             []Position `json:"positions"`
}

// WithdrawLimit is the gateway-level withdraw limit type.
type WithdrawLimit struct {
	Currency       string  `json:"currency"`
	BlockedAmount  float64 `json:"blocked_amount"`
	WithdrawAmount float64 `json:"withdraw_amount"`
}

// Operation is the gateway-level operation type.
type Operation struct {
	ID            string           `json:"id"`
	AccountID     string           `json:"account_id"`
	InstrumentUID string           `json:"instrument_uid"`
	Type          string           `json:"type"`
	State         string           `json:"state"`
	Payment       float64          `json:"payment"`
	Currency      string           `json:"currency"`
	Quantity      int64            `json:"quantity"`
	Date          time.Time        `json:"date"`
	Trades        []OperationTrade `json:"trades,omitempty"`
}

// OperationTrade is the gateway-level operation trade type.
type OperationTrade struct {
	TradeID  string    `json:"trade_id"`
	Price    float64   `json:"price"`
	Quantity int64     `json:"quantity"`
	Date     time.Time `json:"date"`
}

// OperationsPage is a paginated operations response.
type OperationsPage struct {
	Operations []Operation `json:"operations"`
	NextCursor string      `json:"next_cursor,omitempty"`
	HasNext    bool        `json:"has_next"`
}

// PortfolioService abstracts the portfolio backend.
type PortfolioService interface {
	GetPositions(ctx context.Context, accountID string) ([]Position, error)
	GetPortfolio(ctx context.Context, accountID string) (*PortfolioInfo, error)
	GetWithdrawLimits(ctx context.Context, accountID string) ([]WithdrawLimit, error)
	GetOperations(ctx context.Context, accountID, instrumentUID string, from, to time.Time) (*OperationsPage, error)
}

type PortfolioHandlers struct {
	svc PortfolioService
}

func NewPortfolioHandlers(svc PortfolioService) *PortfolioHandlers {
	return &PortfolioHandlers{svc: svc}
}

func (h *PortfolioHandlers) Routes(r chi.Router) {
	r.Get("/api/v1/positions", h.GetPositions)
	r.Get("/api/v1/portfolio", h.GetPortfolio)
	r.Get("/api/v1/limits", h.GetLimits)
	r.Get("/api/v1/operations", h.GetOperations)
}

func (h *PortfolioHandlers) GetPositions(w http.ResponseWriter, r *http.Request) {
	accountID := queryString(r, "account_id", "")
	if accountID == "" {
		respondError(w, http.StatusBadRequest, "account_id query param is required")
		return
	}
	positions, err := h.svc.GetPositions(r.Context(), accountID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, positions)
}

func (h *PortfolioHandlers) GetPortfolio(w http.ResponseWriter, r *http.Request) {
	accountID := queryString(r, "account_id", "")
	if accountID == "" {
		respondError(w, http.StatusBadRequest, "account_id query param is required")
		return
	}
	info, err := h.svc.GetPortfolio(r.Context(), accountID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, info)
}

func (h *PortfolioHandlers) GetLimits(w http.ResponseWriter, r *http.Request) {
	accountID := queryString(r, "account_id", "")
	if accountID == "" {
		respondError(w, http.StatusBadRequest, "account_id query param is required")
		return
	}
	limits, err := h.svc.GetWithdrawLimits(r.Context(), accountID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, limits)
}

func (h *PortfolioHandlers) GetOperations(w http.ResponseWriter, r *http.Request) {
	accountID := queryString(r, "account_id", "")
	if accountID == "" {
		respondError(w, http.StatusBadRequest, "account_id query param is required")
		return
	}
	instrumentUID := queryString(r, "instrument_uid", "")

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

	page, err := h.svc.GetOperations(r.Context(), accountID, instrumentUID, from, to)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, page)
}
