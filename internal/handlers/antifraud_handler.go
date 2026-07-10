package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
	"gandm/internal/repository"
	"gandm/internal/service"
)

type AntifraudHandler struct {
	svc *service.AntifraudService
}

func NewAntifraudHandler(svc *service.AntifraudService) *AntifraudHandler {
	return &AntifraudHandler{svc: svc}
}

func writeAntifraudError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, service.ErrAccountNotEligible):
		httpx.WriteError(w, http.StatusForbidden, "account_not_eligible", "account status does not allow this action")
	case errors.Is(err, service.ErrNoDealWithPartner):
		httpx.WriteError(w, http.StatusForbidden, "no_deal", "favorites are limited to counterparties of completed deals")
	case errors.Is(err, service.ErrUnsupportedFile):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "unsupported_file", "unsupported file type")
	case errors.Is(err, service.ErrFileTooLarge):
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large", "file exceeds maximum size")
	case errors.Is(err, repository.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "not found")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func (h *AntifraudHandler) ListFavorites(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	items, err := h.svc.ListFavorites(r.Context(), userID)
	if err != nil {
		writeAntifraudError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

type favoriteRequest struct {
	ParticipantID uuid.UUID `json:"participant_id"`
}

func (h *AntifraudHandler) AddFavorite(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	var req favoriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ParticipantID == uuid.Nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "participant_id is required")
		return
	}
	if err := h.svc.AddFavorite(r.Context(), userID, req.ParticipantID); err != nil {
		writeAntifraudError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *AntifraudHandler) RemoveFavorite(w http.ResponseWriter, r *http.Request) {
	participantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid participant id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.RemoveFavorite(r.Context(), userID, participantID); err != nil {
		writeAntifraudError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AntifraudHandler) UploadDealDocument(w http.ResponseWriter, r *http.Request) {
	dealID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid deal id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_upload", "file too large or malformed multipart body")
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_upload", "file is required")
		return
	}
	file.Close()

	doc, err := h.svc.UploadDealDocument(r.Context(), userID, dealID, header)
	if err != nil {
		writeAntifraudError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, doc)
}

func (h *AntifraudHandler) ListDealDocuments(w http.ResponseWriter, r *http.Request) {
	dealID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid deal id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	items, err := h.svc.ListDealDocuments(r.Context(), userID, dealID)
	if err != nil {
		writeAntifraudError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

// SuspiciousPairs — админский список аномалий (ТЗ §6.1).
func (h *AntifraudHandler) SuspiciousPairs(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListSuspiciousPairs(r.Context())
	if err != nil {
		writeAntifraudError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}
