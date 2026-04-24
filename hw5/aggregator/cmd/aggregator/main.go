package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/online-cinema/aggregator/internal/config"
	apphttp "github.com/online-cinema/aggregator/internal/http"
	"github.com/online-cinema/aggregator/internal/repository"
	"github.com/online-cinema/aggregator/internal/service"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := config.Load()

	clickhouseRepo, err := repository.NewClickHouseRepo(cfg.ClickHouseDSN)
	if err != nil {
		logger.Fatal("failed to init clickhouse repository", zap.Error(err))
	}
	defer clickhouseRepo.Close()

	postgresRepo, err := repository.NewPostgresRepo(cfg.PostgresDSN)
	if err != nil {
		logger.Fatal("failed to init postgres repository", zap.Error(err))
	}
	defer postgresRepo.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := postgresRepo.EnsureSchema(ctx); err != nil {
		logger.Fatal("failed to ensure postgres schema", zap.Error(err))
	}

	s3Repo, err := repository.NewS3Repo(
		cfg.S3Endpoint,
		cfg.S3AccessKey,
		cfg.S3SecretKey,
		cfg.S3Bucket,
		cfg.S3Region,
		cfg.S3UseSSL,
	)
	if err != nil {
		logger.Fatal("failed to init s3 repository", zap.Error(err))
	}
	if err := s3Repo.EnsureBucket(ctx); err != nil {
		logger.Fatal("failed to ensure s3 bucket", zap.Error(err))
	}

	svc := service.New(clickhouseRepo, postgresRepo, s3Repo, logger, service.ExportConfig{
		Interval:   cfg.ExportInterval,
		Prefix:     cfg.S3Prefix,
		MaxRetries: cfg.ExportMaxRetries,
	})
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	go svc.RunTicker(runCtx, cfg.AggregateInterval)
	go svc.RunExportTicker(runCtx, cfg.ExportInterval)

	handler := apphttp.NewHandler(svc, clickhouseRepo, postgresRepo, logger)
	mux := http.NewServeMux()
	handler.Register(mux)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("aggregation service started",
			zap.String("addr", srv.Addr),
			zap.Duration("aggregation_interval", cfg.AggregateInterval),
			zap.Duration("export_interval", cfg.ExportInterval),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("aggregation service failed", zap.Error(err))
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	runCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
	}
}
