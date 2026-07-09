package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
	"gandm/internal/models"
	"gandm/internal/repository"
	"gandm/internal/service"
)

type AdminHandler struct {
	svc *service.AdminService
}

func NewAdminHandler(svc *service.AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

type adminLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type adminLoginResponse struct {
	Admin  *models.Admin  `json:"admin"`
	Tokens auth.TokenPair `json:"tokens"`
}

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req adminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	admin, tokens, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, adminLoginResponse{Admin: admin, Tokens: tokens})
}

func (h *AdminHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "refresh_token is required")
		return
	}

	tokens, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, refreshResponse{Tokens: tokens})
}

func (h *AdminHandler) DashboardStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.DashboardStats(r.Context())
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, stats)
}

var allowedVerificationStatuses = map[models.VerificationStatus]bool{
	models.VerificationPending:  true,
	models.VerificationApproved: true,
	models.VerificationRejected: true,
}

func (h *AdminHandler) VerificationQueue(w http.ResponseWriter, r *http.Request) {
	status := models.VerificationStatus(r.URL.Query().Get("status"))
	if status == "" {
		status = models.VerificationPending
	}
	if !allowedVerificationStatuses[status] {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_status", "unknown verification status")
		return
	}

	items, err := h.svc.VerificationQueue(r.Context(), status)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *AdminHandler) VerificationDetail(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid verification id")
		return
	}

	detail, err := h.svc.VerificationDetail(r.Context(), id)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, detail)
}

func (h *AdminHandler) ApproveVerification(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid verification id")
		return
	}
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if err := h.svc.ApproveVerification(r.Context(), adminID, id); err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

type rejectVerificationRequest struct {
	Reason string `json:"reason"`
}

func (h *AdminHandler) RejectVerification(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid verification id")
		return
	}
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req rejectVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	if err := h.svc.RejectVerification(r.Context(), adminID, id, req.Reason); err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func writeAdminServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
	case errors.Is(err, service.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, service.ErrAlreadyReviewed):
		httpx.WriteError(w, http.StatusConflict, "already_reviewed", "verification request already reviewed")
	case errors.Is(err, repository.ErrInvalidToolID):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tool_id", "one or more tool ids do not exist")
	case errors.Is(err, repository.ErrKeyTaken):
		httpx.WriteError(w, http.StatusConflict, "key_taken", "tool key already exists")
	case errors.Is(err, repository.ErrRouteExists):
		httpx.WriteError(w, http.StatusConflict, "route_exists", "this route is already added")
	case errors.Is(err, repository.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "not found")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
