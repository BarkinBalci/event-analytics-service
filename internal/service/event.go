package service

import (
	"log"

	"github.com/BarkinBalci/event-analytics-service/internal/models"
)

// EventService represents event service
type EventService struct {
	// TODO: add SQS and Redis Clients
}

// NewEventService creates a new event service
func NewEventService() *EventService {
	return &EventService{}
}

// ProcessEvent processes a single event
func (s *EventService) ProcessEvent(event *models.PublishEventRequest) (string, error) {
	// TODO: Validate timestamp must not be in the future when using seconds precision

	// TODO: Check idempotency in Redis

	// TODO: Publish to SQS

	log.Printf("Event received - Name: %s, User: %s, Channel: %s",
		event.EventName, event.UserID, event.Channel)

	return event.EventName, nil // FIXME: Return event ID
}

// ProcessBulkEvents validates and processes multiple events
func (s *EventService) ProcessBulkEvents(events []models.PublishEventRequest) ([]string, []string, error) {
	var eventIDs []string
	var errors []string

	for i, event := range events {
		eventID, err := s.ProcessEvent(&event)
		if err != nil {
			errors = append(errors, err.Error())
			log.Printf("Failed to process event at index %d: %v", i, err)
			continue
		}
		eventIDs = append(eventIDs, eventID)
	}

	return eventIDs, errors, nil
}
