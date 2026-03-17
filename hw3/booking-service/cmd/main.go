package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"hw3/booking-service/internal/grpcauth"
	"hw3/booking-service/internal/handler"
	"hw3/booking-service/internal/repository"
	flightpb "hw3/gen/flight"
)

func main() {
	dbURL := getEnv("DATABASE_URL", "postgres://booking:booking@localhost:5432/booking_db?sslmode=disable")
	flightAddr := getEnv("FLIGHT_SERVICE_ADDR", "localhost:50051")
	httpPort := getEnv("HTTP_PORT", "8080")
	migrationsPath := getEnv("MIGRATIONS_PATH", "file://booking-service/migrations")
	grpcAPIKey := mustGetEnv("FLIGHT_SERVICE_API_KEY")

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

	conn, err := grpc.NewClient(
		flightAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpcauth.APIKeyUnaryClientInterceptor(grpcAPIKey)),
	)
	if err != nil {
		log.Fatalf("failed to create grpc client: %v", err)
	}
	defer conn.Close()

	flightClient := flightpb.NewFlightServiceClient(conn)

	repo := repository.NewBookingRepository(db)
	h := handler.NewHandler(repo, flightClient)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Mount("/", h.Routes())

	log.Printf("booking service starting on :%s", httpPort)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", httpPort), r); err != nil {
		log.Fatalf("server error: %v", err)
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

