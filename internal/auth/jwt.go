package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("invalid or expired token")

// Subject types keep participant tokens and staff (admin/moderator) tokens
// cryptographically distinct, even though they share the same signing
// secrets: a token issued for one subject type is rejected by middleware
// that requires the other.
const (
	SubjectUser  = "user"
	SubjectAdmin = "admin"
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type claims struct {
	Typ       string `json:"typ"`
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

type SessionValidator func(ctx context.Context, subjectType string, subjectID, sessionID uuid.UUID) (bool, error)

type Manager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
	sessions      SessionValidator
}

// SetSessionValidator enables immediate single-session enforcement in the
// auth middleware. It is configured once during application startup.
func (m *Manager) SetSessionValidator(validator SessionValidator) {
	m.sessions = validator
}

func NewManager(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

func (m *Manager) IssueTokenPair(subjectID uuid.UUID, subjectType string) (TokenPair, error) {
	sessionID := uuid.New()
	access, err := m.sign(subjectID, subjectType, sessionID, m.accessSecret, m.accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := m.sign(subjectID, subjectType, sessionID, m.refreshSecret, m.refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

// IssueAccessToken mints just a short-lived access token (the refresh token is
// issued separately now, with a jti recorded in the server-side registry).
func (m *Manager) IssueAccessToken(subjectID uuid.UUID, subjectType string, sessionID uuid.UUID) (string, error) {
	return m.sign(subjectID, subjectType, sessionID, m.accessSecret, m.accessTTL)
}

// IssueRefreshToken signs a refresh token carrying the given jti and returns it
// with its absolute expiry, so the caller can persist the jti in the registry
// and size the cookie's MaxAge to match.
func (m *Manager) IssueRefreshToken(subjectID uuid.UUID, subjectType string, jti, sessionID uuid.UUID) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.refreshTTL)
	c := claims{
		Typ:       subjectType,
		SessionID: sessionID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti.String(),
			Subject:   subjectID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := token.SignedString(m.refreshSecret)
	return signed, expiresAt, err
}

func (m *Manager) sign(subjectID uuid.UUID, subjectType string, sessionID uuid.UUID, secret []byte, ttl time.Duration) (string, error) {
	now := time.Now()
	c := claims{
		Typ:       subjectType,
		SessionID: sessionID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subjectID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString(secret)
}

// ParseAccessToken validates the token and checks it was issued for the
// given subject type (user vs admin), so a participant token can never be
// used against admin-only routes and vice versa.
func (m *Manager) ParseAccessToken(tokenStr string, subjectType string) (uuid.UUID, error) {
	subjectID, _, err := m.ParseAccessTokenDetailed(tokenStr, subjectType)
	return subjectID, err
}

func (m *Manager) ParseRefreshToken(tokenStr string, subjectType string) (uuid.UUID, error) {
	return m.parse(tokenStr, m.refreshSecret, subjectType)
}

// ParseAccessTokenDetailed returns the subject and active-session id carried by
// an access token. Middleware checks that session id against the database so a
// login on a second device invalidates the first device immediately.
func (m *Manager) ParseAccessTokenDetailed(tokenStr, subjectType string) (uuid.UUID, uuid.UUID, error) {
	c, err := m.parseClaims(tokenStr, m.accessSecret, subjectType)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	subjectID, sessionID, err := parseSubjectAndSession(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return subjectID, sessionID, nil
}

// ParseRefreshTokenDetailed validates the refresh token and returns the
// subject, token jti and active-session id for registry and device checks.
func (m *Manager) ParseRefreshTokenDetailed(tokenStr, subjectType string) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	c, err := m.parseClaims(tokenStr, m.refreshSecret, subjectType)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	subjectID, sessionID, err := parseSubjectAndSession(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	jti, err := uuid.Parse(c.ID)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	return subjectID, jti, sessionID, nil
}

func (m *Manager) parseClaims(tokenStr string, secret []byte, subjectType string) (*claims, error) {
	c := &claims{}
	token, err := jwt.ParseWithClaims(tokenStr, c, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid || c.Typ != subjectType {
		return nil, ErrInvalidToken
	}
	return c, nil
}

func parseSubjectAndSession(c *claims) (uuid.UUID, uuid.UUID, error) {
	subjectID, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	sessionID, err := uuid.Parse(c.SessionID)
	if err != nil {
		return uuid.Nil, uuid.Nil, ErrInvalidToken
	}
	return subjectID, sessionID, nil
}

func (m *Manager) parse(tokenStr string, secret []byte, subjectType string) (uuid.UUID, error) {
	c, err := m.parseClaims(tokenStr, secret, subjectType)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	subjectID, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	return subjectID, nil
}
