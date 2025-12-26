package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/BarkinBalci/event-analytics-service/internal/config"
	"github.com/BarkinBalci/event-analytics-service/internal/consumer"
	"github.com/BarkinBalci/event-analytics-service/internal/logger"
	"github.com/BarkinBalci/event-analytics-service/internal/queue/sqs"
	"github.com/BarkinBalci/event-analytics-service/internal/repository/clickhouse"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	// Initialize logger
	log, err := logger.New(cfg.Service.Environment)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	defer func(log *zap.Logger) {
		err := log.Sync()
		if err != nil {
			log.Error("Failed to sync logger", zap.Error(err))
		}
	}(log)

	log.Info("Starting consumer service",
		zap.String("environment", cfg.Service.Environment))

	ctx := context.Background()

	// Initialize ClickHouse client
	chClient, err := clickhouse.NewClient(ctx, &cfg.ClickHouse, log)
	if err != nil {
		log.Fatal("Failed to create ClickHouse client", zap.Error(err))
	}
	defer func() {
		if err := chClient.Close(); err != nil {
			log.Error("Failed to close ClickHouse client", zap.Error(err))
		}
	}()

	// Initialize repository
	repo := clickhouse.NewRepository(chClient, log)

	// Initialize schema (create tables if not exist)
	if err := repo.InitSchema(ctx); err != nil {
		log.Fatal("Failed to initialize schema", zap.Error(err))
	}
	log.Info("Database schema initialized")

	// Initialize SQS client
	sqsClient, err := sqs.NewClient(ctx, cfg.SQS, log)
	if err != nil {
		log.Fatal("Failed to create SQS client", zap.Error(err))
	}

	// Initialize consumer
	c := consumer.NewConsumer(cfg, sqsClient, repo, log)

	// Start health check endpoint
	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			if err := repo.Ping(r.Context()); err != nil {
				log.Warn("Health check failed", zap.Error(err))
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		addr := ":" + cfg.Consumer.HealthCheckPort
		log.Info("Health check server starting", zap.String("address", addr))
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Error("Health check server error", zap.Error(err))
		}
	}()

	// Start consumer
	consumerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	log.Info("Consumer starting")

	go func() {
		if err := c.Start(consumerCtx); err != nil {
			log.Fatal("Consumer error", zap.Error(err))
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down consumer gracefully")
	cancel()
}
