package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
	"gandm/internal/service"
)

type createRatingRequest struct {
	RatedUserID uuid.UUID  `json:"rated_user_id"`
	Score       int        `json:"score"`
	Comment     string     `json:"comment"`
	DealID      *uuid.UUID `json:"deal_id"`
}

func (h *CargoHandler) CreateRating(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req createRatingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RatedUserID == uuid.Nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "rated_user_id and score are required")
		return
	}

	rating, err := h.svc.CreateRating(r.Context(), userID, service.CreateRatingInput{
		RatedUserID: req.RatedUserID,
		Score:       req.Score,
		Comment:     req.Comment,
		DealID:      req.DealID,
	})
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, rating)
}

func (h *CargoHandler) GetUserRating(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}

	summary, err := h.svc.GetUserRating(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, summary)
}

func (h *CargoHandler) ListMyReceivedRatings(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	ratings, err := h.svc.ListMyReceivedRatings(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ratings)
}
