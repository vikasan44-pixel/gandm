package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
)

type selectOfferRequest struct {
	OfferID uuid.UUID `json:"offer_id"`
}

func (h *CargoHandler) SelectOffer(w http.ResponseWriter, r *http.Request) {
	cargoID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid cargo request id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req selectOfferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OfferID == uuid.Nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "offer_id is required")
		return
	}

	result, err := h.svc.SelectOffer(r.Context(), userID, cargoID, req.OfferID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *CargoHandler) ListMyChats(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	chats, err := h.svc.ListMyChats(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, chats)
}

func (h *CargoHandler) ListChatMessages(w http.ResponseWriter, r *http.Request) {
	chatID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid chat id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	messages, err := h.svc.ListChatMessages(r.Context(), userID, chatID, r.URL.Query().Get("after"))
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, messages)
}

type sendMessageRequest struct {
	Body          string `json:"body"`
	AttachmentURL string `json:"attachment_url"`
}

func (h *CargoHandler) SendChatMessage(w http.ResponseWriter, r *http.Request) {
	chatID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid chat id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	msg, err := h.svc.SendChatMessage(r.Context(), userID, chatID, req.Body, req.AttachmentURL)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, msg)
}
