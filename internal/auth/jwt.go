package auth

import (
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
	Typ string `json:"typ"`
	jwt.RegisteredClaims
}

type Manager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
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
	access, err := m.sign(subjectID, subjectType, m.accessSecret, m.accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := m.sign(subjectID, subjectType, m.refreshSecret, m.refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

func (m *Manager) sign(subjectID uuid.UUID, subjectType string, secret []byte, ttl time.Duration) (string, error) {
	now := time.Now()
	c := claims{
		Typ: subjectType,
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
	return m.parse(tokenStr, m.accessSecret, subjectType)
}

func (m *Manager) ParseRefreshToken(tokenStr string, subjectType string) (uuid.UUID, error) {
	return m.parse(tokenStr, m.refreshSecret, subjectType)
}

func (m *Manager) parse(tokenStr string, secret []byte, subjectType string) (uuid.UUID, error) {
	c := &claims{}
	token, err := jwt.ParseWithClaims(tokenStr, c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return secret, nil
	})
	if err != nil || !token.Valid || c.Typ != subjectType {
		return uuid.Nil, ErrInvalidToken
	}

	subjectID, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	return subjectID, nil
}
