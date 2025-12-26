package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/BarkinBalci/event-analytics-service/internal/domain"
	"github.com/BarkinBalci/event-analytics-service/internal/repository"
)

// MockEventRepository is a mock implementation of repository.EventRepository
type MockEventRepository struct {
	mock.Mock
}

func (m *MockEventRepository) InsertBatch(ctx context.Context, events []*domain.Event) (int, error) {
	args := m.Called(ctx, events)
	return args.Int(0), args.Error(1)
}

func (m *MockEventRepository) InitSchema(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockEventRepository) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockEventRepository) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockEventRepository) GetMetrics(ctx context.Context, query repository.MetricsQuery) (*repository.MetricsResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.MetricsResult), args.Error(1)
}

func createTestEnvelope(eventID string) *Envelope {
	event := &domain.Event{
		EventID:   eventID,
		EventName: "test_event",
		UserID:    "user123",
		Timestamp: testTimestamp,
	}

	ack := func(ctx context.Context) error {
		return nil
	}

	nack := func(ctx context.Context) error {
		return nil
	}

	envelope := NewEnvelope(event, ack, nack)
	return envelope
}

func TestBatchWriter_Start_BatchSizeThreshold(t *testing.T) {
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	config := BatchWriterConfig{
		MaxBatchSize: 3,
		FlushTimeout: 10 * time.Second,
	}

	writer := NewBatchWriter(mockRepo, config, log)

	mockRepo.On("InsertBatch", mock.Anything, mock.MatchedBy(func(events []*domain.Event) bool {
		return len(events) == 3
	})).Return(3, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := make(chan *Envelope, 5)
	go writer.Start(ctx, in)

	// Send 3 envelopes to trigger batch size threshold
	in <- createTestEnvelope("1")
	in <- createTestEnvelope("2")
	in <- createTestEnvelope("3")

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertCalled(t, "InsertBatch", mock.Anything, mock.Anything)
}

func TestBatchWriter_Start_TimeoutFlush(t *testing.T) {
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	config := BatchWriterConfig{
		MaxBatchSize: 10,
		FlushTimeout: 50 * time.Millisecond,
	}

	writer := NewBatchWriter(mockRepo, config, log)

	mockRepo.On("InsertBatch", mock.Anything, mock.MatchedBy(func(events []*domain.Event) bool {
		return len(events) == 2
	})).Return(2, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := make(chan *Envelope, 5)
	go writer.Start(ctx, in)

	// Send 2 envelopes (less than max batch size)
	in <- createTestEnvelope("1")
	in <- createTestEnvelope("2")

	// Wait for timeout to trigger flush
	time.Sleep(100 * time.Millisecond)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertCalled(t, "InsertBatch", mock.Anything, mock.Anything)
}

func TestBatchWriter_Start_InsertSuccess(t *testing.T) {
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	config := BatchWriterConfig{
		MaxBatchSize: 2,
		FlushTimeout: 10 * time.Second,
	}

	writer := NewBatchWriter(mockRepo, config, log)

	mockRepo.On("InsertBatch", mock.Anything, mock.MatchedBy(func(events []*domain.Event) bool {
		return len(events) == 2
	})).Return(2, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	in := make(chan *Envelope, 5)
	go writer.Start(ctx, in)

	// Send 2 envelopes
	in <- createTestEnvelope("1")
	in <- createTestEnvelope("2")

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	mockRepo.AssertExpectations(t)
}

func TestBatchWriter_Start_InsertFailure(t *testing.T) {
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	config := BatchWriterConfig{
		MaxBatchSize: 2,
		FlushTimeout: 10 * time.Second,
	}

	writer := NewBatchWriter(mockRepo, config, log)

	insertErr := errors.New("database connection error")
	mockRepo.On("InsertBatch", mock.Anything, mock.Anything).Return(0, insertErr)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	in := make(chan *Envelope, 5)
	go writer.Start(ctx, in)

	// Send 2 envelopes
	in <- createTestEnvelope("1")
	in <- createTestEnvelope("2")

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertCalled(t, "InsertBatch", mock.Anything, mock.Anything)
}

func TestBatchWriter_Start_PartialInsert(t *testing.T) {
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	config := BatchWriterConfig{
		MaxBatchSize: 3,
		FlushTimeout: 10 * time.Second,
	}

	writer := NewBatchWriter(mockRepo, config, log)

	// Repository inserts only 2 out of 3 events
	mockRepo.On("InsertBatch", mock.Anything, mock.MatchedBy(func(events []*domain.Event) bool {
		return len(events) == 3
	})).Return(2, nil) // Partial success

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	in := make(chan *Envelope, 5)
	go writer.Start(ctx, in)

	// Send 3 envelopes
	in <- createTestEnvelope("1")
	in <- createTestEnvelope("2")
	in <- createTestEnvelope("3")

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertCalled(t, "InsertBatch", mock.Anything, mock.Anything)
}

func TestBatchWriter_Start_GracefulShutdown(t *testing.T) {
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	config := BatchWriterConfig{
		MaxBatchSize: 10,
		FlushTimeout: 10 * time.Second,
	}

	writer := NewBatchWriter(mockRepo, config, log)

	mockRepo.On("InsertBatch", mock.Anything, mock.MatchedBy(func(events []*domain.Event) bool {
		return len(events) == 2
	})).Return(2, nil)

	ctx, cancel := context.WithCancel(context.Background())

	in := make(chan *Envelope, 5)
	done := make(chan bool)

	go func() {
		writer.Start(ctx, in)
		done <- true
	}()

	// Send 2 envelopes
	in <- createTestEnvelope("1")
	in <- createTestEnvelope("2")

	// Give time for messages to be received
	time.Sleep(10 * time.Millisecond)

	// Cancel context to trigger graceful shutdown
	cancel()

	// Wait for shutdown
	select {
	case <-done:
		// Shutdown completed
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Graceful shutdown took too long")
	}

	mockRepo.AssertExpectations(t)
	mockRepo.AssertCalled(t, "InsertBatch", mock.Anything, mock.Anything)
}

func TestBatchWriter_Start_InputChannelClosed(t *testing.T) {
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	config := BatchWriterConfig{
		MaxBatchSize: 10,
		FlushTimeout: 10 * time.Second,
	}

	writer := NewBatchWriter(mockRepo, config, log)

	mockRepo.On("InsertBatch", mock.Anything, mock.MatchedBy(func(events []*domain.Event) bool {
		return len(events) == 2
	})).Return(2, nil)

	ctx := context.Background()

	in := make(chan *Envelope, 5)
	done := make(chan bool)

	go func() {
		writer.Start(ctx, in)
		done <- true
	}()

	// Send 2 envelopes
	in <- createTestEnvelope("1")
	in <- createTestEnvelope("2")

	// Close input channel
	close(in)

	// Wait for shutdown
	select {
	case <-done:
		// Shutdown completed
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Shutdown took too long after input channel closed")
	}

	mockRepo.AssertExpectations(t)
	mockRepo.AssertCalled(t, "InsertBatch", mock.Anything, mock.Anything)
}

func TestBatchWriter_Start_EmptyBatchNotFlushed(t *testing.T) {
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	config := BatchWriterConfig{
		MaxBatchSize: 10,
		FlushTimeout: 50 * time.Millisecond,
	}

	writer := NewBatchWriter(mockRepo, config, log)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	in := make(chan *Envelope, 5)
	go writer.Start(ctx, in)

	// Don't send any envelopes

	// Wait for timeout
	<-ctx.Done()

	// InsertBatch should not be called for empty batch
	mockRepo.AssertNotCalled(t, "InsertBatch")
}

func TestBatchWriter_Start_MultipleBatches(t *testing.T) {
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	config := BatchWriterConfig{
		MaxBatchSize: 2,
		FlushTimeout: 10 * time.Second,
	}

	writer := NewBatchWriter(mockRepo, config, log)

	// Expect two batches of 2 events each
	mockRepo.On("InsertBatch", mock.Anything, mock.MatchedBy(func(events []*domain.Event) bool {
		return len(events) == 2
	})).Return(2, nil).Times(2)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	in := make(chan *Envelope, 10)
	go writer.Start(ctx, in)

	// Send 4 envelopes (should create 2 batches)
	in <- createTestEnvelope("1")
	in <- createTestEnvelope("2")
	in <- createTestEnvelope("3")
	in <- createTestEnvelope("4")

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNumberOfCalls(t, "InsertBatch", 2)
}
