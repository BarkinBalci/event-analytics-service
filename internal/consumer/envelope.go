package consumer

import (
	"context"

	"github.com/BarkinBalci/event-analytics-service/internal/domain"
)

// Envelope wraps a domain event with acknowledgment callbacks
type Envelope struct {
	Event *domain.Event
	ack   func(context.Context) error
	nack  func(context.Context) error
}

// NewEnvelope creates a new message envelope
func NewEnvelope(event *domain.Event, ack, nack func(context.Context) error) *Envelope {
	return &Envelope{
		Event: event,
		ack:   ack,
		nack:  nack,
	}
}

// Ack acknowledges successful processing
func (e *Envelope) Ack(ctx context.Context) error {
	if e.ack != nil {
		return e.ack(ctx)
	}
	return nil
}

// Nack negatively acknowledges processing
func (e *Envelope) Nack(ctx context.Context) error {
	if e.nack != nil {
		return e.nack(ctx)
	}
	return nil
}
