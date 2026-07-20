package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestRequireAuthRejectsReplacedSession(t *testing.T) {
	manager := newTestManager()
	userID := uuid.New()
	sessionID := uuid.New()
	token, err := manager.IssueAccessToken(userID, SubjectUser, sessionID)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	manager.SetSessionValidator(func(_ context.Context, subjectType string, gotUserID, gotSessionID uuid.UUID) (bool, error) {
		if subjectType != SubjectUser || gotUserID != userID || gotSessionID != sessionID {
			t.Fatalf("validator identity = %s/%s/%s, want %s/%s/%s", subjectType, gotUserID, gotSessionID, SubjectUser, userID, sessionID)
		}
		return false, nil
	})

	handler := manager.RequireAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("replaced session reached protected handler")
	}))
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(res.Body.String(), `"code":"session_replaced"`) {
		t.Fatalf("body = %s, want session_replaced code", res.Body.String())
	}
}

func TestRequireAuthAllowsCurrentSession(t *testing.T) {
	manager := newTestManager()
	userID := uuid.New()
	sessionID := uuid.New()
	token, err := manager.IssueAccessToken(userID, SubjectUser, sessionID)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	manager.SetSessionValidator(func(_ context.Context, _ string, _, _ uuid.UUID) (bool, error) {
		return true, nil
	})

	handler := manager.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, ok := UserIDFromContext(r.Context())
		if !ok || gotUserID != userID {
			t.Fatalf("context user = %s/%v, want %s/true", gotUserID, ok, userID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
}
