package consumer

import (
	"github.com/BarkinBalci/event-analytics-service/internal/domain"
)

// MessageParser defines the interface for parsing raw message bytes into events
type MessageParser interface {
	Parse(body []byte) (*domain.Event, error)
}
