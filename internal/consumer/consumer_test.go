package consumer

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/BarkinBalci/event-analytics-service/internal/config"
	"github.com/BarkinBalci/event-analytics-service/internal/domain"
)

func TestConsumer_Start_PipelineCoordination(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	mockRepo := new(MockEventRepository)
	mockParser := new(MockMessageParser)
	log := zap.NewNop()

	cfg := &config.Config{
		Consumer: config.Consumer{
			BatchSizeMax:    10,
			BatchTimeoutSec: 1,
		},
	}

	mockConsumer.On("QueueURL").Return("https://sqs.eu-central-1.amazonaws.com/123/test-queue")

	messages := []types.Message{
		{
			MessageId:     aws.String("msg-1"),
			Body:          aws.String(`{"event_id": "1"}`),
			ReceiptHandle: aws.String("receipt-1"),
		},
	}

	event := &domain.Event{
		EventID:   "1",
		EventName: "test_event",
		UserID:    "user123",
		Timestamp: testTimestamp,
	}

	mockConsumer.On("ReceiveMessages", mock.Anything, mock.AnythingOfType("*sqs.ReceiveMessageInput")).
		Return(&sqs.ReceiveMessageOutput{Messages: messages}, nil).Once()
	mockConsumer.On("ReceiveMessages", mock.Anything, mock.AnythingOfType("*sqs.ReceiveMessageInput")).
		Return(&sqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil).Maybe()

	mockParser.On("Parse", []byte(`{"event_id": "1"}`)).Return(event, nil)
	mockConsumer.On("DeleteMessage", mock.Anything, mock.AnythingOfType("*sqs.DeleteMessageInput")).
		Return(&sqs.DeleteMessageOutput{}, nil)

	mockRepo.On("InsertBatch", mock.Anything, mock.MatchedBy(func(events []*domain.Event) bool {
		return len(events) == 1 && events[0].EventID == "1"
	})).Return(1, nil)

	// Create consumer with mocked components
	receiver := NewReceiver(mockConsumer, ReceiverConfig{
		MaxMessages:     10,
		WaitTimeSeconds: 20,
		BufferSize:      100,
	}, log)

	parser := NewParserStage(mockConsumer, mockParser, log)

	batchWriter := NewBatchWriter(mockRepo, BatchWriterConfig{
		MaxBatchSize: cfg.Consumer.BatchSizeMax,
		FlushTimeout: time.Duration(cfg.Consumer.BatchTimeoutSec) * time.Second,
	}, log)

	consumer := &Consumer{
		receiver:    receiver,
		parser:      parser,
		batchWriter: batchWriter,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Start consumer
	err := consumer.Start(ctx)

	assert.NoError(t, err)

	// Wait a bit to ensure processing
	time.Sleep(150 * time.Millisecond)

	mockRepo.AssertCalled(t, "InsertBatch", mock.Anything, mock.Anything)
}

func TestConsumer_Start_GracefulShutdown(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	cfg := &config.Config{
		Consumer: config.Consumer{
			BatchSizeMax:    10,
			BatchTimeoutSec: 1,
		},
	}

	mockConsumer.On("QueueURL").Return("https://sqs.eu-central-1.amazonaws.com/123/test-queue").Maybe()
	mockConsumer.On("ReceiveMessages", mock.Anything, mock.AnythingOfType("*sqs.ReceiveMessageInput")).
		Return(&sqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil).Maybe()

	consumer := NewConsumer(cfg, mockConsumer, mockRepo, log)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan bool)
	go func() {
		err := consumer.Start(ctx)
		assert.NoError(t, err)
		done <- true
	}()

	// Let it run for a bit
	time.Sleep(50 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Wait for shutdown
	select {
	case <-done:
		// Shutdown completed successfully
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Graceful shutdown took too long")
	}
}

func TestConsumer_NewConsumer_ComponentInitialization(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	cfg := &config.Config{
		Consumer: config.Consumer{
			BatchSizeMax:    100,
			BatchTimeoutSec: 5,
		},
	}

	consumer := NewConsumer(cfg, mockConsumer, mockRepo, log)

	assert.NotNil(t, consumer)
	assert.NotNil(t, consumer.receiver)
	assert.NotNil(t, consumer.parser)
	assert.NotNil(t, consumer.batchWriter)
}

func TestConsumer_Start_EmptyQueueScenario(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	cfg := &config.Config{
		Consumer: config.Consumer{
			BatchSizeMax:    10,
			BatchTimeoutSec: 1,
		},
	}

	mockConsumer.On("QueueURL").Return("https://sqs.eu-central-1.amazonaws.com/123/test-queue")
	mockConsumer.On("ReceiveMessages", mock.Anything, mock.AnythingOfType("*sqs.ReceiveMessageInput")).
		Return(&sqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil).Maybe()

	consumer := NewConsumer(cfg, mockConsumer, mockRepo, log)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := consumer.Start(ctx)

	assert.NoError(t, err)

	// InsertBatch should not be called since no messages were processed
	mockRepo.AssertNotCalled(t, "InsertBatch")
}
