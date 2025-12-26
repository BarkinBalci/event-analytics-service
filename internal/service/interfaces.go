package service

import (
	"github.com/BarkinBalci/event-analytics-service/internal/dto"
)

// EventServicer defines the interface for event service operations
type EventServicer interface {
	ProcessEvent(event *dto.PublishEventRequest) (string, error)
	ProcessBulkEvents(events []dto.PublishEventRequest) ([]string, []string, error)
	GetMetrics(req *dto.GetMetricsRequest) (*dto.GetMetricsResponse, error)
}
