package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	AppEnv               string
	AppPort              string
	DatabaseURL          string
	RedisURL             string
	JWTSecret            string
	JWTAccessTTL         time.Duration
	JWTRefreshTTL        time.Duration
	StorageType          string
	StoragePath          string
	LocalSigningSecret   string
	S3Bucket             string
	S3Region             string
	S3Endpoint           string
	S3AccessKey          string
	S3SecretKey          string
	MaxFileSize          int64
	MaxProcessingTime    time.Duration
	JobRetentionTTL      time.Duration
	HistoryRetentionDays int
	WorkerConcurrency    int
	JobMaxRetries        int
	AllowedOrigins       []string
	TrustedProxies       []string
	RateLimitMax         int
	RateLimitWindow      time.Duration
	InstagramGraphBase   string
	InstagramAppID       string
	InstagramAppSecret   string
	InstagramAccessToken string
	ExternalMediaAPIBase string
	ExternalMediaAPIKey  string
}

// Load reads environment variables (optionally from a .env file in non-production)
// and validates required security and runtime fields.
func Load() (*Config, error) {
	// Best-effort .env loading for local development; ignored in production if absent.
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	appEnv := getEnv("APP_ENV", "development")

	maxFileSize, err := parseInt64Env("MAX_FILE_SIZE", 500*1024*1024)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_FILE_SIZE: %w", err)
	}

	maxProcessingTime, err := parseDurationEnv("MAX_PROCESSING_TIME", 300*time.Second)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_PROCESSING_TIME: %w", err)
	}

	accessTTL, err := parseDurationEnv("JWT_ACCESS_TTL", 15*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_TTL: %w", err)
	}

	refreshTTL, err := parseDurationEnv("JWT_REFRESH_TTL", 7*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_TTL: %w", err)
	}

	jobRetentionTTL, err := parseDurationEnv("JOB_RETENTION_TTL", 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("invalid JOB_RETENTION_TTL: %w", err)
	}

	rateLimitWindow, err := parseDurationEnv("RATE_LIMIT_WINDOW", time.Minute)
	if err != nil {
		return nil, fmt.Errorf("invalid RATE_LIMIT_WINDOW: %w", err)
	}

	cfg := &Config{
		AppEnv:               appEnv,
		AppPort:              getEnv("APP_PORT", "8080"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		RedisURL:             getEnv("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:            os.Getenv("JWT_SECRET"),
		JWTAccessTTL:         accessTTL,
		JWTRefreshTTL:        refreshTTL,
		StorageType:          strings.ToLower(getEnv("STORAGE_TYPE", "local")),
		StoragePath:          getEnv("STORAGE_PATH", "./storage_data"),
		LocalSigningSecret:   getEnv("LOCAL_SIGNING_SECRET", ""),
		S3Bucket:             os.Getenv("S3_BUCKET"),
		S3Region:             getEnv("S3_REGION", "us-east-1"),
		S3Endpoint:           os.Getenv("S3_ENDPOINT"),
		S3AccessKey:          os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:          os.Getenv("S3_SECRET_KEY"),
		MaxFileSize:          maxFileSize,
		MaxProcessingTime:    maxProcessingTime,
		JobRetentionTTL:      jobRetentionTTL,
		HistoryRetentionDays: parseIntEnv("HISTORY_RETENTION_DAYS", 90),
		WorkerConcurrency:    parseIntEnv("WORKER_CONCURRENCY", 4),
		JobMaxRetries:        parseIntEnv("JOB_MAX_RETRIES", 3),
		AllowedOrigins:       splitCSV(getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173,http://127.0.0.1:3000,http://127.0.0.1:5173")),
		TrustedProxies:       splitCSV(os.Getenv("TRUSTED_PROXIES")),
		RateLimitMax:         parseIntEnv("RATE_LIMIT_MAX", 100),
		RateLimitWindow:      rateLimitWindow,
		InstagramGraphBase:   strings.TrimRight(getEnv("INSTAGRAM_GRAPH_API_BASE", "https://graph.instagram.com"), "/"),
		InstagramAppID:       os.Getenv("INSTAGRAM_APP_ID"),
		InstagramAppSecret:   os.Getenv("INSTAGRAM_APP_SECRET"),
		InstagramAccessToken: os.Getenv("INSTAGRAM_ACCESS_TOKEN"),
		ExternalMediaAPIBase: strings.TrimRight(strings.TrimSpace(os.Getenv("EXTERNAL_MEDIA_API_BASE")), "/"),
		ExternalMediaAPIKey:  strings.TrimSpace(os.Getenv("EXTERNAL_MEDIA_API_KEY")),
	}

	if cfg.LocalSigningSecret == "" {
		cfg.LocalSigningSecret = cfg.JWTSecret
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate enforces mandatory configuration invariants.
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.RedisURL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if c.IsProduction() {
		if c.JWTSecret == "change-this-to-a-long-random-secret-at-least-32-chars" {
			return fmt.Errorf("JWT_SECRET must not use the example placeholder in production")
		}
		if c.LocalSigningSecret == "change-this-signing-secret-in-production" {
			return fmt.Errorf("LOCAL_SIGNING_SECRET must not use the example placeholder in production")
		}
	}
	if c.StorageType != "local" && c.StorageType != "s3" {
		return fmt.Errorf("STORAGE_TYPE must be 'local' or 's3', got %q", c.StorageType)
	}
	if c.StorageType == "s3" {
		if c.S3Bucket == "" || c.S3Region == "" {
			return fmt.Errorf("S3_BUCKET and S3_REGION are required when STORAGE_TYPE=s3")
		}
	}
	if c.MaxFileSize <= 0 {
		return fmt.Errorf("MAX_FILE_SIZE must be positive")
	}
	if c.MaxProcessingTime <= 0 {
		return fmt.Errorf("MAX_PROCESSING_TIME must be positive")
	}
	if c.WorkerConcurrency <= 0 {
		return fmt.Errorf("WORKER_CONCURRENCY must be positive")
	}
	return nil
}

// IsProduction reports whether the application runs in production mode.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func parseIntEnv(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func parseInt64Env(key string, fallback int64) (int64, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback, nil
	}
	return strconv.ParseInt(v, 10, 64)
}

func parseDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback, nil
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second, nil
	}
	return time.ParseDuration(v)
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

