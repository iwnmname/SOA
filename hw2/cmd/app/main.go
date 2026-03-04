package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	"hw2/internal/config"
	"hw2/internal/handler"
	"hw2/internal/middleware"
	"hw2/internal/repository"
	"hw2/internal/service"
	"hw2/pkg/generated"
)

type API struct {
	*handler.ProductHandler
	*handler.OrderHandler
	*handler.PromoHandler
	*handler.AuthHandler
}

func main() {
	ctx := context.Background()
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	runMigrations(cfg.MigrationsPath, cfg.DatabaseURL)

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	productRepo := repository.NewProductRepo()
	orderRepo := repository.NewOrderRepo()
	promoRepo := repository.NewPromoRepo()
	userOpRepo := repository.NewUserOpRepo()
	userRepo := repository.NewUserRepo()

	productSvc := service.NewProductService(pool, productRepo)
	orderSvc := service.NewOrderService(pool, orderRepo, productRepo, promoRepo, userOpRepo, cfg.RateLimitMinutes)
	authSvc := service.NewAuthService(pool, userRepo, cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)

	api := &API{
		ProductHandler: handler.NewProductHandler(productSvc),
		OrderHandler:   handler.NewOrderHandler(orderSvc),
		PromoHandler:   handler.NewPromoHandler(pool, promoRepo),
		AuthHandler:    handler.NewAuthHandler(authSvc),
	}

	r := chi.NewRouter()
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.AuthMiddleware(authSvc))
	r.Use(middleware.LoggingMiddleware(logger))

	generated.HandlerFromMux(api, r)

	log.Printf("server starting on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, r))
}

func runMigrations(source, dbURL string) {
	m, err := migrate.New(source, dbURL)
	if err != nil {
		log.Fatalf("failed to create migrate instance: %v", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations applied")
}
