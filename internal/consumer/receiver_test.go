package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockQueueConsumer is a mock implementation of queue.QueueConsumer
type MockQueueConsumer struct {
	mock.Mock
}

func (m *MockQueueConsumer) ReceiveMessages(ctx context.Context, input *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sqs.ReceiveMessageOutput), args.Error(1)
}

func (m *MockQueueConsumer) DeleteMessage(ctx context.Context, input *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sqs.DeleteMessageOutput), args.Error(1)
}

func (m *MockQueueConsumer) QueueURL() string {
	args := m.Called()
	return args.String(0)
}

func TestReceiver_Start_Success(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	log := zap.NewNop()

	config := ReceiverConfig{
		MaxMessages:     10,
		WaitTimeSeconds: 20,
		BufferSize:      100,
	}

	receiver := NewReceiver(mockConsumer, config, log)

	mockConsumer.On("QueueURL").Return("https://sqs.eu-central-1.amazonaws.com/123/test-queue")

	messages := []types.Message{
		{
			MessageId: aws.String("msg-1"),
			Body:      aws.String(`{"event_id": "1"}`),
		},
		{
			MessageId: aws.String("msg-2"),
			Body:      aws.String(`{"event_id": "2"}`),
		},
	}

	// First call returns messages, second call returns empty (to trigger context cancellation)
	mockConsumer.On("ReceiveMessages", mock.Anything, mock.AnythingOfType("*sqs.ReceiveMessageInput")).
		Return(&sqs.ReceiveMessageOutput{Messages: messages}, nil).Once()
	mockConsumer.On("ReceiveMessages", mock.Anything, mock.AnythingOfType("*sqs.ReceiveMessageInput")).
		Return(&sqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil).Maybe()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	out := make(chan types.Message, 10)

	go receiver.Start(ctx, out)

	// Collect messages from output channel
	var receivedMessages []types.Message
	timeout := time.After(200 * time.Millisecond)
	done := false

	for !done {
		select {
		case msg, ok := <-out:
			if !ok {
				done = true
				break
			}
			receivedMessages = append(receivedMessages, msg)
		case <-timeout:
			done = true
		}
	}

	assert.Len(t, receivedMessages, 2)
	assert.Equal(t, "msg-1", *receivedMessages[0].MessageId)
	assert.Equal(t, "msg-2", *receivedMessages[1].MessageId)
}

func TestReceiver_Start_SQSReceiveError(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	log := zap.NewNop()

	config := ReceiverConfig{
		MaxMessages:     10,
		WaitTimeSeconds: 20,
		BufferSize:      100,
	}

	receiver := NewReceiver(mockConsumer, config, log)

	mockConsumer.On("QueueURL").Return("https://sqs.eu-central-1.amazonaws.com/123/test-queue")

	receiveErr := errors.New("SQS connection error")
	mockConsumer.On("ReceiveMessages", mock.Anything, mock.AnythingOfType("*sqs.ReceiveMessageInput")).
		Return(nil, receiveErr).Once()
	mockConsumer.On("ReceiveMessages", mock.Anything, mock.AnythingOfType("*sqs.ReceiveMessageInput")).
		Return(&sqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil).Maybe()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	out := make(chan types.Message, 10)

	go receiver.Start(ctx, out)

	// Wait for context to cancel
	<-ctx.Done()

	// Verify no messages were sent
	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("Expected no messages but got one")
		}
	default:
		// Channel is empty, which is expected
	}

	mockConsumer.AssertCalled(t, "ReceiveMessages", mock.Anything, mock.AnythingOfType("*sqs.ReceiveMessageInput"))
}

func TestReceiver_Start_ContextCancellation(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	log := zap.NewNop()

	config := ReceiverConfig{
		MaxMessages:     10,
		WaitTimeSeconds: 20,
		BufferSize:      100,
	}

	receiver := NewReceiver(mockConsumer, config, log)

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan types.Message, 10)

	// Cancel immediately
	cancel()

	receiver.Start(ctx, out)

	// Channel should be closed
	_, ok := <-out
	assert.False(t, ok, "Channel should be closed after context cancellation")
}

func TestReceiver_Start_EmptyMessages(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	log := zap.NewNop()

	config := ReceiverConfig{
		MaxMessages:     10,
		WaitTimeSeconds: 20,
		BufferSize:      100,
	}

	receiver := NewReceiver(mockConsumer, config, log)

	mockConsumer.On("QueueURL").Return("https://sqs.eu-central-1.amazonaws.com/123/test-queue")

	// Return empty messages
	mockConsumer.On("ReceiveMessages", mock.Anything, mock.AnythingOfType("*sqs.ReceiveMessageInput")).
		Return(&sqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil).Maybe()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	out := make(chan types.Message, 10)

	go receiver.Start(ctx, out)

	// Wait for context to cancel
	<-ctx.Done()

	// Verify no messages were sent
	select {
	case msg, ok := <-out:
		if ok {
			t.Fatalf("Expected no messages but got: %v", msg)
		}
	default:
		// Channel might not be closed yet, that's fine
	}

	mockConsumer.AssertCalled(t, "ReceiveMessages", mock.Anything, mock.AnythingOfType("*sqs.ReceiveMessageInput"))
}

func TestReceiver_Start_BufferBackpressure(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	log := zap.NewNop()

	config := ReceiverConfig{
		MaxMessages:     10,
		WaitTimeSeconds: 20,
		BufferSize:      2, // Small buffer
	}

	receiver := NewReceiver(mockConsumer, config, log)

	mockConsumer.On("QueueURL").Return("https://sqs.eu-central-1.amazonaws.com/123/test-queue")

	// Create more messages than buffer can hold
	messages := make([]types.Message, 5)
	for i := 0; i < 5; i++ {
		messages[i] = types.Message{
			MessageId: aws.String("msg-" + string(rune(i+'0'))),
			Body:      aws.String(`{"event_id": "` + string(rune(i+'0')) + `"}`),
		}
	}

	mockConsumer.On("ReceiveMessages", mock.Anything, mock.AnythingOfType("*sqs.ReceiveMessageInput")).
		Return(&sqs.ReceiveMessageOutput{Messages: messages}, nil).Once()
	mockConsumer.On("ReceiveMessages", mock.Anything, mock.AnythingOfType("*sqs.ReceiveMessageInput")).
		Return(&sqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil).Maybe()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	out := make(chan types.Message, 2) // Small buffer

	go receiver.Start(ctx, out)

	// Slowly consume messages
	var receivedMessages []types.Message
	for i := 0; i < 5; i++ {
		select {
		case msg := <-out:
			receivedMessages = append(receivedMessages, msg)
			time.Sleep(10 * time.Millisecond) // Simulate slow consumer
		case <-ctx.Done():
		}
	}

	assert.GreaterOrEqual(t, len(receivedMessages), 2, "Should receive at least some messages even with backpressure")
}
