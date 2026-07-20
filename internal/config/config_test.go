package config

import (
	"strings"
	"testing"
)

func setRequiredEnv(t *testing.T) {
	t.Setenv("DATABASE_DSN", "postgres://u:p@localhost:5432/db")
	// Distinct, ≥32-char, non-placeholder secrets — the strength rules Load
	// enforces.
	t.Setenv("JWT_ACCESS_SECRET", "access-secret-0123456789abcdef0123456789")
	t.Setenv("JWT_REFRESH_SECRET", "refresh-secret-0123456789abcdef0123456789")
	t.Setenv("S3_ENDPOINT", "localhost:9000")
	t.Setenv("S3_ACCESS_KEY", "key")
	t.Setenv("S3_SECRET_KEY", "secret")
}

func TestLoadDefaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MatchRadiusCNKm != 100 {
		t.Errorf("MatchRadiusCNKm = %v, want 100", cfg.MatchRadiusCNKm)
	}
	if cfg.MatchRadiusKZKm != 50 {
		t.Errorf("MatchRadiusKZKm = %v, want 50", cfg.MatchRadiusKZKm)
	}
	if cfg.ServerPort != "8080" {
		t.Errorf("ServerPort = %q, want 8080", cfg.ServerPort)
	}
	if cfg.ContactLimitFree != 1 || cfg.ContactLimitSubscribed != 5 {
		t.Errorf("contact limits = %d/%d, want 1/5", cfg.ContactLimitFree, cfg.ContactLimitSubscribed)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("DATABASE_DSN", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_DSN")
	}
	if !strings.Contains(err.Error(), "DATABASE_DSN") {
		t.Errorf("error %q does not name the missing variable", err)
	}
}

func TestLoadRejectsNonPositiveRadius(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("MATCH_RADIUS_CN_KM", "-5")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for negative radius")
	}
}

func TestLoadRejectsPlaceholderJWTSecret(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("JWT_ACCESS_SECRET", "change-me-access")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for placeholder JWT_ACCESS_SECRET")
	}
	if !strings.Contains(err.Error(), "JWT_ACCESS_SECRET") {
		t.Errorf("error %q does not name the offending secret", err)
	}
}

func TestLoadRejectsShortJWTSecret(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("JWT_REFRESH_SECRET", "too-short")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for a JWT secret shorter than 32 chars")
	}
}

func TestLoadRejectsIdenticalJWTSecrets(t *testing.T) {
	setRequiredEnv(t)
	same := "identical-secret-0123456789abcdef0123456789"
	t.Setenv("JWT_ACCESS_SECRET", same)
	t.Setenv("JWT_REFRESH_SECRET", same)

	if _, err := Load(); err == nil {
		t.Fatal("expected error when access and refresh secrets are identical")
	}
}
