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
	svc          *service.AdminService
	cookieSecure bool
}

func NewAdminHandler(svc *service.AdminService, cookieSecure bool) *AdminHandler {
	return &AdminHandler{svc: svc, cookieSecure: cookieSecure}
}

type adminLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type adminLoginResponse struct {
	Admin  *models.Admin   `json:"admin"`
	Tokens accessTokenBody `json:"tokens"`
}

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req adminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	admin, sess, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}

	httpx.SetRefreshCookie(w, sess.RefreshToken, h.cookieSecure, sess.RefreshExpires)
	httpx.WriteJSON(w, http.StatusOK, adminLoginResponse{
		Admin:  admin,
		Tokens: accessTokenBody{AccessToken: sess.AccessToken},
	})
}

// Refresh reads the staff refresh token from the httpOnly cookie, rotates it,
// and sets the new cookie. On failure the cookie is cleared.
func (h *AdminHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken, ok := httpx.RefreshCookie(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "missing refresh token")
		return
	}

	sess, err := h.svc.Refresh(r.Context(), refreshToken)
	if err != nil {
		if errors.Is(err, service.ErrRefreshAlreadyRotated) {
			httpx.WriteError(w, http.StatusConflict, "refresh_already_rotated", "refresh token was already rotated; retry")
			return
		}
		httpx.ClearRefreshCookie(w, h.cookieSecure)
		writeAdminServiceError(w, err)
		return
	}

	httpx.SetRefreshCookie(w, sess.RefreshToken, h.cookieSecure, sess.RefreshExpires)
	httpx.WriteJSON(w, http.StatusOK, refreshResponse{Tokens: accessTokenBody{AccessToken: sess.AccessToken}})
}

// Logout revokes the current staff refresh token and clears the cookie.
func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if refreshToken, ok := httpx.RefreshCookie(r); ok {
		_ = h.svc.Logout(r.Context(), refreshToken)
	}
	httpx.ClearRefreshCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
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

var allowedVehicleVerificationStatuses = map[models.VehicleVerificationStatus]bool{
	models.VehicleVerificationPending:  true,
	models.VehicleVerificationVerified: true,
	models.VehicleVerificationRejected: true,
}

func (h *AdminHandler) VehicleVerificationQueue(w http.ResponseWriter, r *http.Request) {
	status := models.VehicleVerificationStatus(r.URL.Query().Get("status"))
	if status == "" {
		status = models.VehicleVerificationPending
	}
	if !allowedVehicleVerificationStatuses[status] {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_status", "unknown vehicle verification status")
		return
	}
	items, err := h.svc.VehicleVerificationQueue(r.Context(), status)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *AdminHandler) VehicleVerificationDetail(w http.ResponseWriter, r *http.Request) {
	vehicleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid vehicle id")
		return
	}
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	detail, err := h.svc.VehicleVerificationDetail(r.Context(), adminID, vehicleID)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, detail)
}

func (h *AdminHandler) ApproveVehicleVerification(w http.ResponseWriter, r *http.Request) {
	vehicleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid vehicle id")
		return
	}
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.ApproveVehicleVerification(r.Context(), adminID, vehicleID); err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

func (h *AdminHandler) RejectVehicleVerification(w http.ResponseWriter, r *http.Request) {
	vehicleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid vehicle id")
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
	if err := h.svc.RejectVehicleVerification(r.Context(), adminID, vehicleID, req.Reason); err != nil {
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
	case errors.Is(err, service.ErrVerificationDocumentsRequired):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "verification_documents_required", err.Error())
	case errors.Is(err, repository.ErrInvalidToolID):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_tool_id", "one or more tool ids do not exist")
	case errors.Is(err, repository.ErrKeyTaken):
		httpx.WriteError(w, http.StatusConflict, "key_taken", "tool key already exists")
	case errors.Is(err, repository.ErrRouteExists):
		httpx.WriteError(w, http.StatusConflict, "route_exists", "this route is already added")
	case errors.Is(err, repository.ErrEmailTaken):
		httpx.WriteError(w, http.StatusConflict, "email_taken", "email is already registered")
	case errors.Is(err, service.ErrForbiddenRole):
		httpx.WriteError(w, http.StatusForbidden, "admin_role_required", "this action requires the admin role")
	case errors.Is(err, repository.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "not found")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
