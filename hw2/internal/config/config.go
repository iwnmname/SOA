package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL      string
	Port             string
	MigrationsPath   string
	RateLimitMinutes int
	JWTSecret        string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
}

func Load() *Config {
	return &Config{
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/marketplace?sslmode=disable"),
		Port:             getEnv("PORT", "8080"),
		MigrationsPath:   getEnv("MIGRATIONS_PATH", "file://migrations"),
		RateLimitMinutes: getEnvInt("ORDER_RATE_LIMIT_MINUTES", 1),
		JWTSecret:        getEnv("JWT_SECRET", "super-secret-key-change-me"),
		AccessTokenTTL:   time.Duration(getEnvInt("ACCESS_TOKEN_TTL_MINUTES", 15)) * time.Minute,
		RefreshTokenTTL:  time.Duration(getEnvInt("REFRESH_TOKEN_TTL_DAYS", 7)) * 24 * time.Hour,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
