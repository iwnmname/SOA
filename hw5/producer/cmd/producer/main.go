package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/online-cinema/producer/internal/config"
	"github.com/online-cinema/producer/internal/generator"
	kafkaproducer "github.com/online-cinema/producer/internal/kafka"
	"github.com/online-cinema/producer/internal/schema"
	"github.com/online-cinema/producer/internal/server"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := config.Load()

	logger.Info("starting producer service",
		zap.Strings("brokers", cfg.Kafka.Brokers),
		zap.String("topic", cfg.Kafka.Topic),
		zap.String("mode", cfg.Mode),
	)

	if cfg.Schema.RegistryURL != "" {
		registerSchema(cfg.Schema, logger)
	}

	producer, err := kafkaproducer.NewProducer(kafkaproducer.Config{
		Brokers:        cfg.Kafka.Brokers,
		Topic:          cfg.Kafka.Topic,
		MaxRetries:     cfg.Kafka.MaxRetries,
		InitialBackoff: cfg.Kafka.InitialBackoff,
		MaxBackoff:     cfg.Kafka.MaxBackoff,
	}, logger)
	if err != nil {
		logger.Fatal("failed to create kafka producer", zap.Error(err))
	}
	defer producer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.Mode == "generator" || cfg.Mode == "both" {
		gen := generator.New(
			producer,
			logger,
			cfg.Generator.NumUsers,
			cfg.Generator.NumMovies,
			cfg.Generator.RateMs,
		)
		go gen.Run(ctx)
	}

	if cfg.Mode == "api" || cfg.Mode == "both" {
		srv := server.New(cfg.HTTP, producer, logger)
		go func() {
			if err := srv.Start(); err != nil {
				logger.Error("server error", zap.Error(err))
			}
		}()
		defer srv.Shutdown(5 * time.Second)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	logger.Info("shutting down", zap.String("signal", sig.String()))
	cancel()
	time.Sleep(1 * time.Second)
	logger.Info("producer service stopped")
}

func registerSchema(cfg config.SchemaConfig, logger *zap.Logger) {
	reg := schema.NewRegistry(cfg.RegistryURL, logger)

	if err := reg.WaitForReady(60 * time.Second); err != nil {
		logger.Warn("schema registry unavailable", zap.Error(err))
		return
	}

	if _, err := reg.RegisterProto(cfg.Subject, cfg.ProtoFilePath); err != nil {
		logger.Warn("schema registration failed", zap.Error(err))
	}
}
