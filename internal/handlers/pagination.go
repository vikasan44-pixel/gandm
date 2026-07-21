package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"gandm/internal/service"
)

func pageRequestFromQuery(r *http.Request) (service.PageRequest, error) {
	request := service.PageRequest{Page: 1, PageSize: service.DefaultPageSize}
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil || page < 1 {
			return request, fmt.Errorf("page must be a positive integer")
		}
		request.Page = page
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("page_size")); raw != "" {
		pageSize, err := strconv.Atoi(raw)
		if err != nil || pageSize < 1 || pageSize > service.MaxPageSize {
			return request, fmt.Errorf("page_size must be between 1 and %d", service.MaxPageSize)
		}
		request.PageSize = pageSize
	}
	return request, nil
}
