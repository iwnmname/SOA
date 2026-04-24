package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort          string
	AggregateInterval time.Duration
	ExportInterval    time.Duration
	ClickHouseDSN     string
	PostgresDSN       string
	S3Endpoint        string
	S3AccessKey       string
	S3SecretKey       string
	S3Bucket          string
	S3Region          string
	S3UseSSL          bool
	S3Prefix          string
	ExportMaxRetries  int
}

func Load() Config {
	return Config{
		HTTPPort:          getEnv("AGG_HTTP_PORT", "8090"),
		AggregateInterval: getDurationEnv("AGG_INTERVAL", 30*time.Second),
		ExportInterval:    getDurationEnv("EXPORT_INTERVAL", 24*time.Hour),
		ClickHouseDSN:     getEnv("CLICKHOUSE_DSN", "clickhouse://app:app@clickhouse:9000/default"),
		PostgresDSN:       getEnv("POSTGRES_DSN", "postgres://app:app@postgres:5432/analytics?sslmode=disable"),
		S3Endpoint:        getEnv("S3_ENDPOINT", "minio:9000"),
		S3AccessKey:       getEnv("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:       getEnv("S3_SECRET_KEY", "minioadmin"),
		S3Bucket:          getEnv("S3_BUCKET", "movie-analytics"),
		S3Region:          getEnv("S3_REGION", "us-east-1"),
		S3UseSSL:          getEnv("S3_USE_SSL", "false") == "true",
		S3Prefix:          getEnv("S3_PREFIX", "daily"),
		ExportMaxRetries:  getIntEnv("EXPORT_MAX_RETRIES", 3),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	seconds, err := strconv.Atoi(value)
	if err == nil {
		return time.Duration(seconds) * time.Second
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return d
}

func getIntEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	if parsed < 1 {
		return fallback
	}
	return parsed
}

