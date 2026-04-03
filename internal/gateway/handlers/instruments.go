package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type InstrumentShort struct {
	UID            string  `json:"uid"`
	FIGI           string  `json:"figi"`
	Ticker         string  `json:"ticker"`
	ClassCode      string  `json:"class_code"`
	Name           string  `json:"name"`
	Currency       string  `json:"currency"`
	Exchange       string  `json:"exchange"`
	Lot            int32   `json:"lot"`
	InstrumentType string  `json:"instrument_type"`
	Sector         string  `json:"sector"`
	MinPriceIncr   float64 `json:"min_price_increment"`
}

type InstrumentsService interface {
	GetShares(ctx context.Context) ([]InstrumentShort, error)
	GetFutures(ctx context.Context) ([]InstrumentShort, error)
	GetClosePrices(ctx context.Context, instrumentUIDs []string) ([]ClosePrice, error)
	GetLastPricesBulk(ctx context.Context, instrumentUIDs []string) ([]LastPrice, error)
}

type ClosePrice struct {
	InstrumentUID string  `json:"instrument_uid"`
	Price         float64 `json:"price"`
}

type InstrumentsHandlers struct {
	svc InstrumentsService
}

func NewInstrumentsHandlers(svc InstrumentsService) *InstrumentsHandlers {
	return &InstrumentsHandlers{svc: svc}
}

func (h *InstrumentsHandlers) Routes(r chi.Router) {
	r.Get("/api/v1/instruments/shares", h.GetShares)
	r.Get("/api/v1/instruments/futures", h.GetFutures)
	r.Get("/api/v1/prices/bulk", h.GetBulkPrices)
	r.Get("/api/v1/prices/close", h.GetClosePrices)
}

func (h *InstrumentsHandlers) GetShares(w http.ResponseWriter, r *http.Request) {
	shares, err := h.svc.GetShares(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, shares)
}

func (h *InstrumentsHandlers) GetFutures(w http.ResponseWriter, r *http.Request) {
	futures, err := h.svc.GetFutures(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, futures)
}

func (h *InstrumentsHandlers) GetBulkPrices(w http.ResponseWriter, r *http.Request) {
	uidsParam := queryString(r, "uids", "")
	if uidsParam == "" {
		respondError(w, http.StatusBadRequest, "uids query param is required (comma-separated)")
		return
	}

	uids := strings.Split(uidsParam, ",")
	for i := range uids {
		uids[i] = strings.TrimSpace(uids[i])
	}

	prices, err := h.svc.GetLastPricesBulk(r.Context(), uids)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, prices)
}

func (h *InstrumentsHandlers) GetClosePrices(w http.ResponseWriter, r *http.Request) {
	uidsParam := queryString(r, "uids", "")
	if uidsParam == "" {
		respondError(w, http.StatusBadRequest, "uids query param is required (comma-separated)")
		return
	}

	uids := strings.Split(uidsParam, ",")
	for i := range uids {
		uids[i] = strings.TrimSpace(uids[i])
	}

	prices, err := h.svc.GetClosePrices(r.Context(), uids)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, prices)
}
