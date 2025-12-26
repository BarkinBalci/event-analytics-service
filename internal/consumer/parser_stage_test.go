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

	"github.com/BarkinBalci/event-analytics-service/internal/domain"
)

const (
	testTimestamp int64 = 1766702552
)

// MockMessageParser is a mock implementation of MessageParser
type MockMessageParser struct {
	mock.Mock
}

func (m *MockMessageParser) Parse(body []byte) (*domain.Event, error) {
	args := m.Called(body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Event), args.Error(1)
}

func TestParserStage_Start_Success(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	mockParser := new(MockMessageParser)
	log := zap.NewNop()

	parserStage := NewParserStage(mockConsumer, mockParser, log)

	mockConsumer.On("QueueURL").Return("https://sqs.eu-central-1.amazonaws.com/123/test-queue")
	mockConsumer.On("DeleteMessage", mock.Anything, mock.AnythingOfType("*sqs.DeleteMessageInput")).
		Return(&sqs.DeleteMessageOutput{}, nil).Maybe()

	message := types.Message{
		MessageId:     aws.String("msg-1"),
		Body:          aws.String(`{"event_id": "1"}`),
		ReceiptHandle: aws.String("receipt-1"),
	}

	event := &domain.Event{
		EventID:   "1",
		EventName: "test_event",
		UserID:    "user123",
		Timestamp: testTimestamp,
	}

	mockParser.On("Parse", []byte(`{"event_id": "1"}`)).Return(event, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := make(chan types.Message, 1)
	out := make(chan *Envelope, 1)

	go parserStage.Start(ctx, in, out)

	// Send message
	in <- message
	close(in)

	// Receive envelope
	envelope := <-out

	assert.NotNil(t, envelope)
	assert.Equal(t, "1", envelope.Event.EventID)
	assert.Equal(t, "test_event", envelope.Event.EventName)

	mockParser.AssertExpectations(t)
}

func TestParserStage_Start_MalformedMessage(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	mockParser := new(MockMessageParser)
	log := zap.NewNop()

	parserStage := NewParserStage(mockConsumer, mockParser, log)

	mockConsumer.On("QueueURL").Return("https://sqs.eu-central-1.amazonaws.com/123/test-queue")
	mockConsumer.On("DeleteMessage", mock.Anything, mock.AnythingOfType("*sqs.DeleteMessageInput")).
		Return(&sqs.DeleteMessageOutput{}, nil)

	message := types.Message{
		MessageId:     aws.String("msg-1"),
		Body:          aws.String(`{invalid json}`),
		ReceiptHandle: aws.String("receipt-1"),
	}

	parseErr := errors.New("invalid JSON format")
	mockParser.On("Parse", []byte(`{invalid json}`)).Return(nil, parseErr)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	in := make(chan types.Message, 1)
	out := make(chan *Envelope, 1)

	go parserStage.Start(ctx, in, out)

	// Send malformed message
	in <- message

	// Wait for processing before closing input
	time.Sleep(20 * time.Millisecond)
	close(in)

	// Wait for output channel to close (which happens when input closes)
	timeout := time.After(100 * time.Millisecond)
	envelopeReceived := false

	for {
		select {
		case envelope, ok := <-out:
			if !ok {
				// Channel closed, exit loop
				goto done
			}
			if envelope != nil {
				envelopeReceived = true
				t.Fatalf("Expected no envelope for malformed message, but got: %v", envelope)
			}
		case <-timeout:
			goto done
		}
	}

done:
	assert.False(t, envelopeReceived, "Should not receive any envelope for malformed message")
	mockParser.AssertExpectations(t)
	mockConsumer.AssertCalled(t, "DeleteMessage", mock.Anything, mock.AnythingOfType("*sqs.DeleteMessageInput"))
}

func TestParserStage_Start_DeleteMessageFailure(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	mockParser := new(MockMessageParser)
	log := zap.NewNop()

	parserStage := NewParserStage(mockConsumer, mockParser, log)

	mockConsumer.On("QueueURL").Return("https://sqs.eu-central-1.amazonaws.com/123/test-queue")

	deleteErr := errors.New("failed to delete message from SQS")
	mockConsumer.On("DeleteMessage", mock.Anything, mock.AnythingOfType("*sqs.DeleteMessageInput")).
		Return(nil, deleteErr)

	message := types.Message{
		MessageId:     aws.String("msg-1"),
		Body:          aws.String(`{invalid}`),
		ReceiptHandle: aws.String("receipt-1"),
	}

	parseErr := errors.New("invalid JSON")
	mockParser.On("Parse", []byte(`{invalid}`)).Return(nil, parseErr)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	in := make(chan types.Message, 1)
	out := make(chan *Envelope, 1)

	go parserStage.Start(ctx, in, out)

	// Send message
	in <- message

	// Wait for processing before closing input
	time.Sleep(20 * time.Millisecond)
	close(in)

	// Wait for output channel to close (which happens when input closes)
	timeout := time.After(100 * time.Millisecond)
	envelopeReceived := false

	for {
		select {
		case envelope, ok := <-out:
			if !ok {
				// Channel closed, exit loop
				goto done
			}
			if envelope != nil {
				envelopeReceived = true
				t.Fatalf("Expected no envelope, but got: %v", envelope)
			}
		case <-timeout:
			goto done
		}
	}

done:
	assert.False(t, envelopeReceived, "Should not receive any envelope for malformed message")
	mockParser.AssertExpectations(t)
	mockConsumer.AssertCalled(t, "DeleteMessage", mock.Anything, mock.AnythingOfType("*sqs.DeleteMessageInput"))
}

func TestParserStage_Start_ContextCancellation(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	mockParser := new(MockMessageParser)
	log := zap.NewNop()

	parserStage := NewParserStage(mockConsumer, mockParser, log)

	ctx, cancel := context.WithCancel(context.Background())

	in := make(chan types.Message)
	out := make(chan *Envelope, 1)

	// Cancel immediately
	cancel()

	parserStage.Start(ctx, in, out)

	// Output channel should be closed
	_, ok := <-out
	assert.False(t, ok, "Output channel should be closed after context cancellation")
}

func TestParserStage_Start_InputChannelClosed(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	mockParser := new(MockMessageParser)
	log := zap.NewNop()

	parserStage := NewParserStage(mockConsumer, mockParser, log)

	ctx := context.Background()

	in := make(chan types.Message)
	out := make(chan *Envelope, 1)

	// Close input channel immediately
	close(in)

	parserStage.Start(ctx, in, out)

	// Output channel should be closed
	_, ok := <-out
	assert.False(t, ok, "Output channel should be closed when input channel is closed")
}

func TestParserStage_Start_MultipleMessages(t *testing.T) {
	mockConsumer := new(MockQueueConsumer)
	mockParser := new(MockMessageParser)
	log := zap.NewNop()

	parserStage := NewParserStage(mockConsumer, mockParser, log)

	mockConsumer.On("QueueURL").Return("https://sqs.eu-central-1.amazonaws.com/123/test-queue")
	mockConsumer.On("DeleteMessage", mock.Anything, mock.AnythingOfType("*sqs.DeleteMessageInput")).
		Return(&sqs.DeleteMessageOutput{}, nil).Maybe()

	messages := []types.Message{
		{
			MessageId:     aws.String("msg-1"),
			Body:          aws.String(`{"event_id": "1"}`),
			ReceiptHandle: aws.String("receipt-1"),
		},
		{
			MessageId:     aws.String("msg-2"),
			Body:          aws.String(`{invalid}`),
			ReceiptHandle: aws.String("receipt-2"),
		},
		{
			MessageId:     aws.String("msg-3"),
			Body:          aws.String(`{"event_id": "3"}`),
			ReceiptHandle: aws.String("receipt-3"),
		},
	}

	event1 := &domain.Event{EventID: "1", EventName: "event1"}
	event3 := &domain.Event{EventID: "3", EventName: "event3"}

	mockParser.On("Parse", []byte(`{"event_id": "1"}`)).Return(event1, nil)
	mockParser.On("Parse", []byte(`{invalid}`)).Return(nil, errors.New("parse error"))
	mockParser.On("Parse", []byte(`{"event_id": "3"}`)).Return(event3, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := make(chan types.Message, 3)
	out := make(chan *Envelope, 3)

	go parserStage.Start(ctx, in, out)

	// Send messages
	for _, msg := range messages {
		in <- msg
	}
	close(in)

	// Collect envelopes
	var envelopes []*Envelope
	timeout := time.After(100 * time.Millisecond)
	done := false

	for !done {
		select {
		case envelope, ok := <-out:
			if !ok {
				done = true
				break
			}
			envelopes = append(envelopes, envelope)
		case <-timeout:
			done = true
		}
	}

	// Should receive 2 valid envelopes (msg-1 and msg-3), msg-2 should be deleted
	assert.Len(t, envelopes, 2)
	assert.Equal(t, "1", envelopes[0].Event.EventID)
	assert.Equal(t, "3", envelopes[1].Event.EventID)

	mockParser.AssertExpectations(t)
	mockConsumer.AssertNumberOfCalls(t, "DeleteMessage", 1) // Only for malformed message
}
