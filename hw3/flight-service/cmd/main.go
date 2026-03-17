package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	"hw3/flight-service/internal/repository"
	"hw3/flight-service/internal/server"
	flightpb "hw3/gen/flight"
)

func main() {
	dbURL := getEnv("DATABASE_URL", "postgres://flight:flight@localhost:5433/flight_db?sslmode=disable")
	grpcPort := getEnv("GRPC_PORT", "50051")
	migrationsPath := getEnv("MIGRATIONS_PATH", "file://flight-service/migrations")
	grpcAPIKey := mustGetEnv("FLIGHT_SERVICE_API_KEY")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	cacheTTL := mustParseDuration(getEnv("CACHE_TTL", "7m"))

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}
	log.Println("connected to database")

	runMigrations(dbURL, migrationsPath)

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	log.Printf("connected to redis at %s", redisAddr)

	repo := repository.NewFlightRepository(db)
	flightServer := server.NewFlightServer(repo, redisClient, cacheTTL)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(server.APIKeyAuthUnaryInterceptor(grpcAPIKey)))
	flightpb.RegisterFlightServiceServer(grpcSrv, flightServer)

	log.Printf("flight service starting on :%s", grpcPort)
	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("grpc server error: %v", err)
	}
}

func runMigrations(dbURL string, migrationsPath string) {
	m, err := migrate.New(migrationsPath, dbURL)
	if err != nil {
		log.Fatalf("failed to create migrator: %v", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("migration failed: %v", err)
	}

	log.Println("migrations applied")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable is not set: %s", key)
	}
	return v
}

func mustParseDuration(v string) time.Duration {
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Fatalf("invalid duration value %q: %v", v, err)
	}
	return d
}

