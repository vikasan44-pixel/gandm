package config

import (
	"strings"
	"testing"
)

func setRequiredEnv(t *testing.T) {
	t.Setenv("DATABASE_DSN", "postgres://u:p@localhost:5432/db")
	t.Setenv("JWT_ACCESS_SECRET", "a")
	t.Setenv("JWT_REFRESH_SECRET", "r")
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
	if cfg.MatchRadiusKZKm != 40 {
		t.Errorf("MatchRadiusKZKm = %v, want 40", cfg.MatchRadiusKZKm)
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
