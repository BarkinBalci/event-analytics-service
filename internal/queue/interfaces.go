package queue

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/BarkinBalci/event-analytics-service/internal/dto"
)

// QueuePublisher defines the interface for publishing events to a queue
type QueuePublisher interface {
	PublishEvent(ctx context.Context, event *dto.PublishEventRequest, eventID string) error
}

// QueueConsumer defines the interface for consuming messages from a queue
type QueueConsumer interface {
	ReceiveMessages(ctx context.Context, input *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, input *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error)
	QueueURL() string
}
