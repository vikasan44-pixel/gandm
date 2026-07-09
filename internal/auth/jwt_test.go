package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestManager() *Manager {
	return NewManager("access-secret", "refresh-secret", 15*time.Minute, 720*time.Hour)
}

func TestTokenRoundTrip(t *testing.T) {
	m := newTestManager()
	userID := uuid.New()

	pair, err := m.IssueTokenPair(userID, SubjectUser)
	if err != nil {
		t.Fatalf("IssueTokenPair: %v", err)
	}

	got, err := m.ParseAccessToken(pair.AccessToken, SubjectUser)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if got != userID {
		t.Errorf("subject = %s, want %s", got, userID)
	}

	got, err = m.ParseRefreshToken(pair.RefreshToken, SubjectUser)
	if err != nil {
		t.Fatalf("ParseRefreshToken: %v", err)
	}
	if got != userID {
		t.Errorf("refresh subject = %s, want %s", got, userID)
	}
}

func TestSubjectTypeSeparation(t *testing.T) {
	m := newTestManager()
	pair, err := m.IssueTokenPair(uuid.New(), SubjectUser)
	if err != nil {
		t.Fatalf("IssueTokenPair: %v", err)
	}

	// A participant token must not pass admin-route validation.
	if _, err := m.ParseAccessToken(pair.AccessToken, SubjectAdmin); err == nil {
		t.Error("user access token accepted as admin token")
	}
	// A refresh token must not work where an access token is expected —
	// they are signed with different secrets.
	if _, err := m.ParseAccessToken(pair.RefreshToken, SubjectUser); err == nil {
		t.Error("refresh token accepted as access token")
	}
	if _, err := m.ParseRefreshToken(pair.AccessToken, SubjectUser); err == nil {
		t.Error("access token accepted as refresh token")
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	m := NewManager("access-secret", "refresh-secret", -time.Minute, -time.Minute)
	pair, err := m.IssueTokenPair(uuid.New(), SubjectUser)
	if err != nil {
		t.Fatalf("IssueTokenPair: %v", err)
	}
	if _, err := m.ParseAccessToken(pair.AccessToken, SubjectUser); err == nil {
		t.Error("expired token accepted")
	}
}

func TestForeignSecretRejected(t *testing.T) {
	m := newTestManager()
	other := NewManager("other-access", "other-refresh", 15*time.Minute, time.Hour)
	pair, err := other.IssueTokenPair(uuid.New(), SubjectUser)
	if err != nil {
		t.Fatalf("IssueTokenPair: %v", err)
	}
	if _, err := m.ParseAccessToken(pair.AccessToken, SubjectUser); err == nil {
		t.Error("token signed with a foreign secret accepted")
	}
}
