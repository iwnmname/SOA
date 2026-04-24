package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Kafka     KafkaConfig
	HTTP      HTTPConfig
	Generator GeneratorConfig
	Schema    SchemaConfig
	Mode      string
}

type KafkaConfig struct {
	Brokers        []string
	Topic          string
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

type HTTPConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type GeneratorConfig struct {
	Enabled   bool
	RateMs    int
	NumUsers  int
	NumMovies int
}

type SchemaConfig struct {
	RegistryURL   string
	ProtoFilePath string
	Subject       string
}

func Load() *Config {
	return &Config{
		Kafka: KafkaConfig{
			Brokers:        strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
			Topic:          getEnv("KAFKA_TOPIC", "movie-events"),
			MaxRetries:     getIntEnv("KAFKA_MAX_RETRIES", 5),
			InitialBackoff: getDurationEnv("KAFKA_INITIAL_BACKOFF", 100*time.Millisecond),
			MaxBackoff:     getDurationEnv("KAFKA_MAX_BACKOFF", 2*time.Second),
		},
		HTTP: HTTPConfig{
			Port:         getEnv("HTTP_PORT", "8080"),
			ReadTimeout:  getDurationEnv("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: getDurationEnv("HTTP_WRITE_TIMEOUT", 10*time.Second),
		},
		Generator: GeneratorConfig{
			Enabled:   getEnv("GENERATOR_ENABLED", "false") == "true",
			RateMs:    getIntEnv("GENERATOR_RATE_MS", 500),
			NumUsers:  getIntEnv("GENERATOR_NUM_USERS", 100),
			NumMovies: getIntEnv("GENERATOR_NUM_MOVIES", 50),
		},
		Schema: SchemaConfig{
			RegistryURL:   getEnv("SCHEMA_REGISTRY_URL", ""),
			ProtoFilePath: getEnv("PROTO_FILE_PATH", "/proto/movie_event.proto"),
			Subject:       getEnv("SCHEMA_SUBJECT", "movie-events-value"),
		},
		Mode: getEnv("MODE", "both"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
