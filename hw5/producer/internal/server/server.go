package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/online-cinema/producer/internal/api"
	"github.com/online-cinema/producer/internal/api/middleware"
	"github.com/online-cinema/producer/internal/config"
	"github.com/online-cinema/producer/internal/kafka"
)

type Server struct {
	httpServer *http.Server
	logger     *zap.Logger
}

func New(cfg config.HTTPConfig, producer *kafka.Producer, logger *zap.Logger) *Server {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(middleware.Logging(logger))
	router.Use(middleware.Recovery(logger))

	handler := api.NewHandler(producer, logger)
	handler.RegisterRoutes(router)

	return &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%s", cfg.Port),
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
		},
		logger: logger,
	}
}

func (s *Server) Start() error {
	s.logger.Info("http server starting", zap.String("addr", s.httpServer.Addr))
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}
