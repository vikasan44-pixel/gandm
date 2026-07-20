package handlers

import (
	"net/http"

	"gandm/internal/httpx"
	"gandm/internal/service"
)

type RatesHandler struct {
	svc *service.RatesService
}

func NewRatesHandler(svc *service.RatesService) *RatesHandler {
	return &RatesHandler{svc: svc}
}

// CurrencyRates — GET /api/currency-rates. Public reference data: the NBK
// snapshot used for the frontend's approximate conversion hint.
func (h *RatesHandler) CurrencyRates(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.Current(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "could not load currency rates")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}
