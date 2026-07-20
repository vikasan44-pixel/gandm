package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
	"gandm/internal/service"
)

func (h *CargoHandler) ListMyDispatchThresholds(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	items, err := h.svc.ListMyDispatchThresholds(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

type dispatchThresholdRequest struct {
	WarehouseID           *uuid.UUID `json:"warehouse_id"`
	ThresholdM3           float64    `json:"threshold_m3"`
	AccruedM3             *float64   `json:"accrued_m3"`
	ManualAccruedM3       *float64   `json:"manual_accrued_m3"`
	EstimatedDispatchDate string     `json:"estimated_dispatch_date"`
	Status                string     `json:"status"`
}

func (h *CargoHandler) SetRouteDispatchThreshold(w http.ResponseWriter, r *http.Request) {
	routeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid route id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req dispatchThresholdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	var estimatedDate *time.Time
	if value := strings.TrimSpace(req.EstimatedDispatchDate); value != "" {
		parsed, err := time.Parse(time.DateOnly, value)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_date", "estimated_dispatch_date must be YYYY-MM-DD")
			return
		}
		estimatedDate = &parsed
	}
	manualAccruedM3 := 0.0
	if req.ManualAccruedM3 != nil {
		manualAccruedM3 = *req.ManualAccruedM3
	} else if req.AccruedM3 != nil {
		// Backward compatibility for clients that still send accrued_m3.
		manualAccruedM3 = *req.AccruedM3
	}
	threshold, err := h.svc.SetRouteDispatchThreshold(r.Context(), userID, routeID, service.DispatchThresholdInput{
		WarehouseID:           req.WarehouseID,
		ThresholdM3:           req.ThresholdM3,
		ManualAccruedM3:       manualAccruedM3,
		EstimatedDispatchDate: estimatedDate,
		Status:                req.Status,
	})
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, threshold)
}

func (h *CargoHandler) DeleteRouteDispatchThreshold(w http.ResponseWriter, r *http.Request) {
	routeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid route id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if err := h.svc.DeleteRouteDispatchThreshold(r.Context(), userID, routeID); err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
