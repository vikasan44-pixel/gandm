package handlers

import (
	"net/http"
	"strconv"

	"gandm/internal/httpx"
)

const (
	defaultAuditLogLimit = 20
	maxAuditLogLimit     = 100
)

func (h *AdminHandler) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	limit := defaultAuditLogLimit
	offset := 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= maxAuditLogLimit {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	entries, err := h.svc.ListAuditLog(r.Context(), limit, offset)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, entries)
}
