package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServerPort string

	DatabaseDSN string

	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration

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

	// MatchingServiceURL is the Python consolidation-matching service.
	// Capacity limits live in platform_settings (DB), not here.
	MatchingServiceURL string
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

	kzRadius, err := strconv.ParseFloat(getEnv("MATCH_RADIUS_KZ_KM", "40"), 64)
	if err != nil || kzRadius <= 0 {
		return nil, fmt.Errorf("MATCH_RADIUS_KZ_KM must be a positive number, got %q", getEnv("MATCH_RADIUS_KZ_KM", "40"))
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

	cfg.MatchingServiceURL = getEnv("MATCHING_SERVICE_URL", "http://localhost:8000")

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

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
