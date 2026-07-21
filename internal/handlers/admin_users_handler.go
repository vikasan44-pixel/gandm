package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
	"gandm/internal/service"
)

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pageRequest, err := pageRequestFromQuery(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	users, err := h.svc.ListUsers(r.Context(), service.UserListFilter{
		ParticipantType: q.Get("type"),
		Status:          q.Get("status"),
		Search:          q.Get("search"),
	}, pageRequest)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, users)
}

func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}

	detail, err := h.svc.GetUser(r.Context(), id)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, detail)
}

type setUserToolsRequest struct {
	ToolIDs []uuid.UUID `json:"tool_ids"`
}

func (h *AdminHandler) SetUserTools(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req setUserToolsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	if err := h.svc.SetUserTools(r.Context(), adminID, id, req.ToolIDs); err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type applyPermissionSetRequest struct {
	SetID uuid.UUID `json:"set_id"`
}

func (h *AdminHandler) ApplyPermissionSet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req applyPermissionSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	if err := h.svc.ApplyPermissionSet(r.Context(), adminID, id, req.SetID); err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type setSubscriptionRequest struct {
	HasSubscription bool `json:"has_subscription"`
}

func (h *AdminHandler) SetUserSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req setSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	if err := h.svc.SetUserSubscription(r.Context(), adminID, id, req.HasSubscription); err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AdminHandler) BlockUser(w http.ResponseWriter, r *http.Request) {
	h.setUserStatus(w, r, h.svc.BlockUser)
}

func (h *AdminHandler) UnblockUser(w http.ResponseWriter, r *http.Request) {
	h.setUserStatus(w, r, h.svc.UnblockUser)
}

func (h *AdminHandler) setUserStatus(w http.ResponseWriter, r *http.Request, action func(ctx context.Context, adminID, userID uuid.UUID) error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if err := action(r.Context(), adminID, id); err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
