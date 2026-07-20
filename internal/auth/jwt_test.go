package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

func TestRefreshTokenCarriesJTI(t *testing.T) {
	m := newTestManager()
	subject := uuid.New()
	jti := uuid.New()
	sessionID := uuid.New()

	token, expiresAt, err := m.IssueRefreshToken(subject, SubjectUser, jti, sessionID)
	if err != nil {
		t.Fatalf("IssueRefreshToken: %v", err)
	}
	if !expiresAt.After(time.Now()) {
		t.Errorf("expiresAt = %v, want a future time", expiresAt)
	}

	gotSubject, gotJTI, gotSessionID, err := m.ParseRefreshTokenDetailed(token, SubjectUser)
	if err != nil {
		t.Fatalf("ParseRefreshTokenDetailed: %v", err)
	}
	if gotSubject != subject {
		t.Errorf("subject = %s, want %s", gotSubject, subject)
	}
	if gotJTI != jti {
		t.Errorf("jti = %s, want %s", gotJTI, jti)
	}
	if gotSessionID != sessionID {
		t.Errorf("session id = %s, want %s", gotSessionID, sessionID)
	}

	// Wrong subject type must be rejected.
	if _, _, _, err := m.ParseRefreshTokenDetailed(token, SubjectAdmin); err == nil {
		t.Error("refresh token accepted for the wrong subject type")
	}
	// An access token must not parse as a refresh token.
	access, err := m.IssueAccessToken(subject, SubjectUser, sessionID)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, _, _, err := m.ParseRefreshTokenDetailed(access, SubjectUser); err == nil {
		t.Error("access token accepted as refresh token")
	}
	gotSubject, gotSessionID, err = m.ParseAccessTokenDetailed(access, SubjectUser)
	if err != nil {
		t.Fatalf("ParseAccessTokenDetailed: %v", err)
	}
	if gotSubject != subject || gotSessionID != sessionID {
		t.Errorf("access identity = %s/%s, want %s/%s", gotSubject, gotSessionID, subject, sessionID)
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

func TestNonHS256AlgorithmRejected(t *testing.T) {
	m := newTestManager()
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, claims{
		Typ:       SubjectUser,
		SessionID: uuid.NewString(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
	})
	signed, err := token.SignedString([]byte("access-secret"))
	if err != nil {
		t.Fatalf("sign HS384 token: %v", err)
	}
	if _, err := m.ParseAccessToken(signed, SubjectUser); err == nil {
		t.Error("HS384 token accepted by an HS256-only parser")
	}
}
