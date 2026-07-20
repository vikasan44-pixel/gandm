package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLimitJSONBodyRejectsOversizedRead(t *testing.T) {
	var readErr error
	handler := LimitJSONBody(8)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"long":"payload"}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	var maxErr *http.MaxBytesError
	if !errors.As(readErr, &maxErr) {
		t.Fatalf("read error = %v, want *http.MaxBytesError", readErr)
	}
}

func TestLimitJSONBodyDoesNotLimitMultipart(t *testing.T) {
	var body string
	handler := LimitJSONBody(4)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body = string(data)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if body != "0123456789" {
		t.Fatalf("multipart body = %q, want full body", body)
	}
}
