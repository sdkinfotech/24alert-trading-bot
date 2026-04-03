package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Account is the gateway-level account type.
type Account struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	OpenedDate  time.Time `json:"opened_date"`
	ClosedDate  time.Time `json:"closed_date,omitempty"`
	AccessLevel string    `json:"access_level"`
}

// MarginInfo is the gateway-level margin info.
type MarginInfo struct {
	LiquidPortfolio       float64 `json:"liquid_portfolio"`
	StartingMargin        float64 `json:"starting_margin"`
	MinimalMargin         float64 `json:"minimal_margin"`
	FundsSufficiencyLevel float64 `json:"funds_sufficiency_level"`
	AmountOfMissing       float64 `json:"amount_of_missing"`
	CorrectedMargin       float64 `json:"corrected_margin"`
}

// AccountService abstracts the account/user backend.
type AccountService interface {
	GetAccounts(ctx context.Context) ([]Account, error)
	GetMarginAttributes(ctx context.Context, accountID string) (*MarginInfo, error)
}

type AccountHandlers struct {
	svc AccountService
}

func NewAccountHandlers(svc AccountService) *AccountHandlers {
	return &AccountHandlers{svc: svc}
}

func (h *AccountHandlers) Routes(r chi.Router) {
	r.Get("/api/v1/accounts", h.ListAccounts)
	r.Get("/api/v1/margin/{account_id}", h.GetMargin)
}

func (h *AccountHandlers) ListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.svc.GetAccounts(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, accounts)
}

func (h *AccountHandlers) GetMargin(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "account_id")
	info, err := h.svc.GetMarginAttributes(r.Context(), accountID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, info)
}
