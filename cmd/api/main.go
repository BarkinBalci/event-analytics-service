package main

import (
	"context"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/BarkinBalci/event-analytics-service/docs"
	"github.com/BarkinBalci/event-analytics-service/internal/config"
	"github.com/BarkinBalci/event-analytics-service/internal/handler"
	"github.com/BarkinBalci/event-analytics-service/internal/logger"
	"github.com/BarkinBalci/event-analytics-service/internal/queue/sqs"
	"github.com/BarkinBalci/event-analytics-service/internal/repository/clickhouse"
	"github.com/BarkinBalci/event-analytics-service/internal/service"
)

// @title Event Analytics Service API
// @version 1.0
// @description API for publishing and managing analytics events
// @host localhost:8080
// @BasePath /
// @schemes http https
func main() {
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

	log.Info("Starting API service",
		zap.String("environment", cfg.Service.Environment),
		zap.String("port", cfg.Service.APIPort))

	// Configure Swagger host dynamically
	docs.SwaggerInfo.Host = cfg.Service.Host

	ctx := context.Background()

	// Initialize SQS client
	sqsClient, err := sqs.NewClient(ctx, cfg.SQS, log)
	if err != nil {
		log.Fatal("Failed to create SQS client", zap.Error(err))
	}

	// Initialize ClickHouse client
	clickhouseClient, err := clickhouse.NewClient(ctx, &cfg.ClickHouse, log)
	if err != nil {
		log.Fatal("Failed to create ClickHouse client", zap.Error(err))
	}
	defer func(clickhouseClient *clickhouse.Client) {
		if err := clickhouseClient.Close(); err != nil {
			log.Error("Failed to close ClickHouse client", zap.Error(err))
		}
	}(clickhouseClient)

	// Initialize repository
	repo := clickhouse.NewRepository(clickhouseClient, log)

	// Initialize event service
	eventService := service.NewEventService(sqsClient, repo, log)

	// Initialize handler
	h := handler.NewHandler(eventService, log)

	addr := fmt.Sprintf(":%s", cfg.Service.APIPort)
	log.Info("API server starting", zap.String("address", addr))

	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal("Failed to start API server", zap.Error(err))
	}
}
