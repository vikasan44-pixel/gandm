package handlers

import (
	"encoding/json"
	"net/http"

	"gandm/internal/auth"
	"gandm/internal/httpx"
	"gandm/internal/service"
)

func (h *AdminHandler) GetPlatformSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.svc.GetPlatformSettings(r.Context())
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, settings)
}

func (h *AdminHandler) UpdatePlatformSettings(w http.ResponseWriter, r *http.Request) {
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req service.PlatformSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	settings, err := h.svc.UpdatePlatformSettings(r.Context(), adminID, req)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, settings)
}
