package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/24alert/trading-bot/pkg/metrics"
)

// RiskCheckResult is the gateway-level risk check result.
type RiskCheckResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

// RiskStatus is the gateway-level risk status.
type RiskStatus struct {
	CircuitBreakerTripped bool              `json:"circuit_breaker_tripped"`
	FailureCount          int               `json:"failure_count"`
	LastFailure           time.Time         `json:"last_failure"`
	Threshold             int               `json:"threshold"`
	Cooldown              string            `json:"cooldown"`
	Checks                []RiskCheckResult `json:"checks,omitempty"`
}

// RiskService abstracts the risk backend.
type RiskService interface {
	GetRiskStatus(ctx context.Context) (*RiskStatus, error)
	ResetCircuitBreaker(ctx context.Context) error
}

type RiskHandlers struct {
	svc RiskService
}

func NewRiskHandlers(svc RiskService) *RiskHandlers {
	return &RiskHandlers{svc: svc}
}

func (h *RiskHandlers) Routes(r chi.Router) {
	r.Get("/api/v1/risk/status", h.GetStatus)
	r.Post("/api/v1/risk/reset", h.Reset)
}

func (h *RiskHandlers) GetStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.svc.GetRiskStatus(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if status.CircuitBreakerTripped {
		metrics.CircuitBreakerState.Set(1)
	} else {
		metrics.CircuitBreakerState.Set(0)
	}
	respondJSON(w, http.StatusOK, status)
}

func (h *RiskHandlers) Reset(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.ResetCircuitBreaker(r.Context()); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}
