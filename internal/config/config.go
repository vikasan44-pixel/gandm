package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gandm/internal/money"
)

type Config struct {
	ServerPort string

	DatabaseDSN string

	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration

	// CookieSecure sets the Secure attribute on the refresh-token cookie. Must
	// be true in production (HTTPS); false only for local dev over plain HTTP,
	// where browsers refuse to store Secure cookies.
	CookieSecure bool

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3UseSSL    bool

	// Per-country haversine radii within which two points count as "the
	// same place" for cargo/route matching. Chinese cargo often originates
	// from industrial zones 50-100 km outside the city center, hence the
	// wider CN radius. KZ radius doubles as the default for unknown/other
	// countries.
	MatchRadiusCNKm float64
	MatchRadiusKZKm float64

	// Contact-reveal limits per client (lifetime, not per cargo request):
	// without subscription vs with the manually-set subscription flag.
	ContactLimitFree       int
	ContactLimitSubscribed int

	// DefaultCurrency is the ISO-4217 code used when a price is submitted
	// without an explicit currency. Worldwide marketplace: each deal carries
	// its own currency; this is only the fallback. Must be a supported code.
	DefaultCurrency string

	// MatchingServiceURL is the Python consolidation-matching service.
	// Capacity limits live in platform_settings (DB), not here.
	MatchingServiceURL string
	// MatchingSharedSecret guards the matching service when it's exposed
	// beyond localhost. Empty = no auth header sent (local dev).
	MatchingSharedSecret string

	// PaymentProvider selects the payment backend ("sandbox" until a real
	// integration exists).
	PaymentProvider string

	// LoginRateLimitPerMin caps unauthenticated credential endpoints
	// (login/register/refresh) per client IP per minute — password
	// brute-force protection. Local dev / smoke tests need it raised, since
	// everything comes from one IP.
	LoginRateLimitPerMin int
}

func Load() (*Config, error) {
	var missing []string
	req := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := &Config{
		ServerPort:       getEnv("SERVER_PORT", "8080"),
		DatabaseDSN:      req("DATABASE_DSN"),
		JWTAccessSecret:  req("JWT_ACCESS_SECRET"),
		JWTRefreshSecret: req("JWT_REFRESH_SECRET"),
		S3Endpoint:       req("S3_ENDPOINT"),
		S3AccessKey:      req("S3_ACCESS_KEY"),
		S3SecretKey:      req("S3_SECRET_KEY"),
		S3Bucket:         getEnv("S3_BUCKET", "documents"),
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	// JWT secrets must be real, not the placeholders shipped in .env.example:
	// those are public in the repo, so accepting them would let anyone forge a
	// valid (incl. admin) token. Enforce a minimum length and reject the known
	// placeholder, and require the two secrets to differ so a leak of one
	// doesn't compromise the other.
	const minSecretLen = 32
	isPlaceholder := func(v string) bool {
		return strings.Contains(strings.ToLower(v), "change-me")
	}
	for _, s := range []struct{ name, value string }{
		{"JWT_ACCESS_SECRET", cfg.JWTAccessSecret},
		{"JWT_REFRESH_SECRET", cfg.JWTRefreshSecret},
	} {
		if isPlaceholder(s.value) {
			return nil, fmt.Errorf("%s is the placeholder from .env.example — set a real secret (e.g. `openssl rand -hex 32`)", s.name)
		}
		if len(s.value) < minSecretLen {
			return nil, fmt.Errorf("%s must be at least %d characters", s.name, minSecretLen)
		}
	}
	if cfg.JWTAccessSecret == cfg.JWTRefreshSecret {
		return nil, fmt.Errorf("JWT_ACCESS_SECRET and JWT_REFRESH_SECRET must differ")
	}

	useSSL, err := strconv.ParseBool(getEnv("S3_USE_SSL", "false"))
	if err != nil {
		return nil, fmt.Errorf("S3_USE_SSL: %w", err)
	}
	cfg.S3UseSSL = useSSL

	cnRadius, err := strconv.ParseFloat(getEnv("MATCH_RADIUS_CN_KM", "100"), 64)
	if err != nil || cnRadius <= 0 {
		return nil, fmt.Errorf("MATCH_RADIUS_CN_KM must be a positive number, got %q", getEnv("MATCH_RADIUS_CN_KM", "100"))
	}
	cfg.MatchRadiusCNKm = cnRadius

	kzRadius, err := strconv.ParseFloat(getEnv("MATCH_RADIUS_KZ_KM", "50"), 64)
	if err != nil || kzRadius <= 0 {
		return nil, fmt.Errorf("MATCH_RADIUS_KZ_KM must be a positive number, got %q", getEnv("MATCH_RADIUS_KZ_KM", "50"))
	}
	cfg.MatchRadiusKZKm = kzRadius

	limitFree, err := strconv.Atoi(getEnv("CONTACT_LIMIT_FREE", "1"))
	if err != nil || limitFree < 0 {
		return nil, fmt.Errorf("CONTACT_LIMIT_FREE must be a non-negative integer, got %q", getEnv("CONTACT_LIMIT_FREE", "1"))
	}
	cfg.ContactLimitFree = limitFree

	limitSub, err := strconv.Atoi(getEnv("CONTACT_LIMIT_SUBSCRIBED", "5"))
	if err != nil || limitSub < 0 {
		return nil, fmt.Errorf("CONTACT_LIMIT_SUBSCRIBED must be a non-negative integer, got %q", getEnv("CONTACT_LIMIT_SUBSCRIBED", "5"))
	}
	cfg.ContactLimitSubscribed = limitSub

	defaultCurrency := money.Normalize(getEnv("DEFAULT_CURRENCY", money.Fallback))
	if defaultCurrency == "" {
		return nil, fmt.Errorf("DEFAULT_CURRENCY must be a supported ISO-4217 code, got %q", getEnv("DEFAULT_CURRENCY", money.Fallback))
	}
	cfg.DefaultCurrency = defaultCurrency

	cfg.MatchingServiceURL = getEnv("MATCHING_SERVICE_URL", "http://localhost:8000")
	cfg.MatchingSharedSecret = os.Getenv("MATCHING_SHARED_SECRET")

	loginRate, err := strconv.Atoi(getEnv("LOGIN_RATE_LIMIT_PER_MIN", "10"))
	if err != nil || loginRate <= 0 {
		return nil, fmt.Errorf("LOGIN_RATE_LIMIT_PER_MIN must be a positive integer, got %q", getEnv("LOGIN_RATE_LIMIT_PER_MIN", "10"))
	}
	cfg.LoginRateLimitPerMin = loginRate
	cfg.PaymentProvider = getEnv("PAYMENT_PROVIDER", "sandbox")

	accessTTL, err := time.ParseDuration(getEnv("JWT_ACCESS_TTL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("JWT_ACCESS_TTL: %w", err)
	}
	cfg.JWTAccessTTL = accessTTL

	refreshTTL, err := time.ParseDuration(getEnv("JWT_REFRESH_TTL", "720h"))
	if err != nil {
		return nil, fmt.Errorf("JWT_REFRESH_TTL: %w", err)
	}
	cfg.JWTRefreshTTL = refreshTTL

	cookieSecure, err := strconv.ParseBool(getEnv("COOKIE_SECURE", "false"))
	if err != nil {
		return nil, fmt.Errorf("COOKIE_SECURE must be a boolean, got %q", getEnv("COOKIE_SECURE", "false"))
	}
	cfg.CookieSecure = cookieSecure

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
