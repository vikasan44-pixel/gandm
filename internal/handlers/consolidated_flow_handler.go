package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
)

// consolidatedIDAndUser is the shared prologue of every /consolidated/{id}
// flow handler.
func consolidatedIDAndUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	consolidatedID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid consolidated request id")
		return uuid.Nil, uuid.Nil, false
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return uuid.Nil, uuid.Nil, false
	}
	return consolidatedID, userID, true
}

func (h *CargoHandler) InviteConsolidated(w http.ResponseWriter, r *http.Request) {
	consolidatedID, userID, ok := consolidatedIDAndUser(w, r)
	if !ok {
		return
	}
	if err := h.svc.InviteConsolidated(r.Context(), userID, consolidatedID); err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "invited"})
}

func (h *CargoHandler) PayConsolidated(w http.ResponseWriter, r *http.Request) {
	consolidatedID, userID, ok := consolidatedIDAndUser(w, r)
	if !ok {
		return
	}
	paymentRow, err := h.svc.PayConsolidated(r.Context(), userID, consolidatedID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, paymentRow)
}

func (h *CargoHandler) AcceptConsolidated(w http.ResponseWriter, r *http.Request) {
	consolidatedID, userID, ok := consolidatedIDAndUser(w, r)
	if !ok {
		return
	}
	chat, err := h.svc.AcceptConsolidated(r.Context(), userID, consolidatedID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "accepted", "chat_id": chat.ID})
}

func (h *CargoHandler) GetConsolidatedStatus(w http.ResponseWriter, r *http.Request) {
	consolidatedID, userID, ok := consolidatedIDAndUser(w, r)
	if !ok {
		return
	}
	view, err := h.svc.GetConsolidatedStatus(r.Context(), userID, consolidatedID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (h *CargoHandler) SelectConsolidatedOffer(w http.ResponseWriter, r *http.Request) {
	consolidatedID, userID, ok := consolidatedIDAndUser(w, r)
	if !ok {
		return
	}

	var req selectOfferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OfferID == uuid.Nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "offer_id is required")
		return
	}

	result, err := h.svc.SelectConsolidatedOffer(r.Context(), userID, consolidatedID, req.OfferID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

type markPaymentRequest struct {
	ClientID uuid.UUID `json:"client_id"`
}

func (h *AdminHandler) MarkConsolidatedPayment(w http.ResponseWriter, r *http.Request) {
	consolidatedID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid consolidated request id")
		return
	}
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req markPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ClientID == uuid.Nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "client_id is required")
		return
	}

	if err := h.svc.MarkConsolidatedPayment(r.Context(), adminID, consolidatedID, req.ClientID); err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
