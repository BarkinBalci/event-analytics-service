package repository

import (
	"context"

	"github.com/BarkinBalci/event-analytics-service/internal/domain"
)

// MetricsQuery represents a metrics query parameters
type MetricsQuery struct {
	EventName string
	From      int64
	To        int64
	GroupBy   string
}

// MetricsGroupResult represents aggregated metrics for a specific group
type MetricsGroupResult struct {
	GroupValue string
	TotalCount uint64
}

// MetricsResult represents the result of a metrics query
type MetricsResult struct {
	TotalCount  uint64
	UniqueCount uint64
	Groups      []MetricsGroupResult
}

// EventRepository defines the interface for event storage operations
type EventRepository interface {
	// InsertBatch inserts a batch of events into the storage
	InsertBatch(ctx context.Context, events []*domain.Event) (int, error)

	// InitSchema initializes the database schema (creates tables if they don't exist)
	InitSchema(ctx context.Context) error

	// Ping checks if the database connection is alive
	Ping(ctx context.Context) error

	// Close closes the repository and releases resources
	Close() error

	// GetMetrics retrieves aggregated metrics based on the query
	GetMetrics(ctx context.Context, query MetricsQuery) (*MetricsResult, error)
}
