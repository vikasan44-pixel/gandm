package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
	"gandm/internal/service"
)

type WarehouseHandler struct{ svc *service.WarehouseService }

func NewWarehouseHandler(svc *service.WarehouseService) *WarehouseHandler {
	return &WarehouseHandler{svc: svc}
}

func (h *WarehouseHandler) CreateFillReport(w http.ResponseWriter, r *http.Request) {
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
	expected, err := strconv.ParseFloat(r.FormValue("expected_fill_percent"), 64)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", "expected_fill_percent must be a number")
		return
	}
	actual, err := strconv.ParseFloat(r.FormValue("actual_fill_percent"), 64)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", "actual_fill_percent must be a number")
		return
	}
	reportDate, err := time.Parse("2006-01-02", r.FormValue("report_date"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", "report_date must be YYYY-MM-DD")
		return
	}
	input := service.CreateFillReportInput{ExpectedFillPercent: expected, ActualFillPercent: actual, ReportDate: reportDate}
	if file, header, err := r.FormFile("photo"); err == nil {
		file.Close()
		input.Photo = header
	}
	report, err := h.svc.CreateFillReport(r.Context(), userID, input)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, report)
}

func (h *WarehouseHandler) ListMyFillReports(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	reports, err := h.svc.ListMyFillReports(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, reports)
}

func (h *WarehouseHandler) GetLatestFillReport(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}
	report, err := h.svc.GetLatestFillReport(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, report)
}

func (h *AdminHandler) ListUserFillReports(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}
	reports, err := h.svc.ListUserFillReports(r.Context(), userID)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, reports)
}
