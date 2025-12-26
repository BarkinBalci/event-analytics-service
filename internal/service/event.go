package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/BarkinBalci/event-analytics-service/internal/dto"
	"github.com/BarkinBalci/event-analytics-service/internal/queue"
	"github.com/BarkinBalci/event-analytics-service/internal/repository"
)

// EventService represents event service
type EventService struct {
	publisher  queue.QueuePublisher
	repository repository.EventRepository
	log        *zap.Logger
}

// NewEventService creates a new event service
func NewEventService(publisher queue.QueuePublisher, repo repository.EventRepository, log *zap.Logger) *EventService {
	return &EventService{
		publisher:  publisher,
		repository: repo,
		log:        log,
	}
}

// computeEventID generates a deterministic event ID based on event content
// Uses SHA-256 hash of: user_id|event_name|timestamp|campaign_id|channel
func computeEventID(event *dto.PublishEventRequest) string {
	// Concatenate fields that uniquely identify an event
	data := fmt.Sprintf("%s|%s|%d|%s|%s",
		event.UserID,
		event.EventName,
		event.Timestamp,
		event.CampaignID,
		event.Channel,
	)

	// SHA-256 hash for deterministic ID
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// ProcessEvent processes a single event
func (s *EventService) ProcessEvent(event *dto.PublishEventRequest) (string, error) {
	ctx := context.Background()

	currentTime := time.Now().Unix()
	if event.Timestamp > currentTime+1 {
		s.log.Warn("Timestamp validation failed: future timestamp",
			zap.Int64("event_timestamp", event.Timestamp),
			zap.Int64("current_time", currentTime),
			zap.String("event_name", event.EventName))
		return "", fmt.Errorf("timestamp cannot be in the future: %d > %d", event.Timestamp, currentTime)
	}

	eventID := computeEventID(event)

	err := s.publisher.PublishEvent(ctx, event, eventID)
	if err != nil {
		return "", fmt.Errorf("failed to publish event to queue: %w", err)
	}

	return eventID, nil
}

// ProcessBulkEvents validates and processes multiple events
func (s *EventService) ProcessBulkEvents(events []dto.PublishEventRequest) ([]string, []string, error) {
	var eventIDs []string
	var errors []string

	for i, event := range events {
		eventID, err := s.ProcessEvent(&event)
		if err != nil {
			errors = append(errors, err.Error())
			s.log.Warn("Failed to process event in bulk",
				zap.Int("index", i),
				zap.Error(err),
				zap.String("event_name", event.EventName))
			continue
		}
		eventIDs = append(eventIDs, eventID)
	}

	return eventIDs, errors, nil
}

// GetMetrics retrieves aggregated metrics from the repository
func (s *EventService) GetMetrics(req *dto.GetMetricsRequest) (*dto.GetMetricsResponse, error) {
	ctx := context.Background()

	// Validate time range
	if req.From > req.To {
		s.log.Warn("Invalid time range for metrics",
			zap.Int64("from", req.From),
			zap.Int64("to", req.To),
			zap.String("event_name", req.EventName))
		return nil, fmt.Errorf("from timestamp must be less than or equal to to timestamp")
	}

	// Validate group_by parameter
	if req.GroupBy != "" {
		validGroupBy := map[string]bool{"channel": true, "hour": true, "day": true}
		if !validGroupBy[req.GroupBy] {
			s.log.Warn("Invalid group_by value",
				zap.String("group_by", req.GroupBy))
			return nil, fmt.Errorf("invalid group_by value: %s (supported: channel, hour, day)", req.GroupBy)
		}

		// Warn if time range is too large for hourly grouping
		rangeSeconds := req.To - req.From
		if req.GroupBy == "hour" && rangeSeconds > 90*24*3600 {
			s.log.Warn("Large time range for hourly grouping",
				zap.Int64("range_days", rangeSeconds/(24*3600)))
			return nil, fmt.Errorf("time range too large for hourly grouping (max 90 days, got %d days)", rangeSeconds/(24*3600))
		}
	}

	// Build repository query
	query := repository.MetricsQuery{
		EventName: req.EventName,
		From:      req.From,
		To:        req.To,
		GroupBy:   req.GroupBy,
	}

	s.log.Info("Querying metrics",
		zap.String("event_name", req.EventName),
		zap.Int64("from", req.From),
		zap.Int64("to", req.To),
		zap.String("group_by", req.GroupBy))

	// Query repository
	result, err := s.repository.GetMetrics(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics from repository: %w", err)
	}

	// Build response
	response := &dto.GetMetricsResponse{
		EventName:   req.EventName,
		From:        req.From,
		To:          req.To,
		TotalCount:  result.TotalCount,
		UniqueCount: result.UniqueCount,
		GroupBy:     req.GroupBy,
		Groups:      make([]dto.MetricsGroupData, 0, len(result.Groups)),
	}

	// Convert repository groups to DTO groups
	for _, group := range result.Groups {
		response.Groups = append(response.Groups, dto.MetricsGroupData{
			GroupValue: group.GroupValue,
			TotalCount: group.TotalCount,
		})
	}

	return response, nil
}
