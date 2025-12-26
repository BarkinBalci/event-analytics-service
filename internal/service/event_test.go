package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/BarkinBalci/event-analytics-service/internal/domain"
	"github.com/BarkinBalci/event-analytics-service/internal/dto"
	"github.com/BarkinBalci/event-analytics-service/internal/repository"
)

const (
	testCurrentTime int64 = 1766702551
	testFutureTime  int64 = 2556144000
)

// MockQueuePublisher is a mock implementation of queue.QueuePublisher
type MockQueuePublisher struct {
	mock.Mock
}

func (m *MockQueuePublisher) PublishEvent(ctx context.Context, event *dto.PublishEventRequest, eventID string) error {
	args := m.Called(ctx, event, eventID)
	return args.Error(0)
}

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

func TestEventService_ProcessEvent_Success(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	service := NewEventService(mockPublisher, mockRepo, log)

	req := &dto.PublishEventRequest{
		EventName:  "test_event",
		Channel:    "web",
		UserID:     "user123",
		Timestamp:  testCurrentTime,
		CampaignID: "campaign1",
		Tags:       []string{"tag1"},
		Metadata:   map[string]interface{}{"key": "value"},
	}

	mockPublisher.On("PublishEvent", mock.Anything, req, mock.AnythingOfType("string")).Return(nil)

	eventID, err := service.ProcessEvent(req)

	assert.NoError(t, err)
	assert.NotEmpty(t, eventID)
	mockPublisher.AssertExpectations(t)
}

func TestEventService_ProcessEvent_FutureTimestamp(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	service := NewEventService(mockPublisher, mockRepo, log)

	req := &dto.PublishEventRequest{
		EventName: "test_event",
		Channel:   "web",
		UserID:    "user123",
		Timestamp: testFutureTime,
	}

	eventID, err := service.ProcessEvent(req)

	assert.Error(t, err)
	assert.Empty(t, eventID)
	assert.Contains(t, err.Error(), "timestamp cannot be in the future")
	mockPublisher.AssertNotCalled(t, "PublishEvent")
}

func TestEventService_ProcessEvent_SQSPublishError(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	service := NewEventService(mockPublisher, mockRepo, log)

	req := &dto.PublishEventRequest{
		EventName: "test_event",
		Channel:   "web",
		UserID:    "user123",
		Timestamp: testCurrentTime,
	}

	publishErr := errors.New("queue publish error")
	mockPublisher.On("PublishEvent", mock.Anything, req, mock.AnythingOfType("string")).Return(publishErr)

	eventID, err := service.ProcessEvent(req)

	assert.Error(t, err)
	assert.Empty(t, eventID)
	assert.Contains(t, err.Error(), "failed to publish event to queue")
	mockPublisher.AssertExpectations(t)
}

func TestEventService_ProcessEvent_ContentHashIdempotency(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	service := NewEventService(mockPublisher, mockRepo, log)

	req := &dto.PublishEventRequest{
		EventName:  "test_event",
		Channel:    "web",
		UserID:     "user123",
		Timestamp:  testCurrentTime,
		CampaignID: "campaign1",
	}

	mockPublisher.On("PublishEvent", mock.Anything, req, mock.AnythingOfType("string")).Return(nil)

	// Same event should produce same event_id (idempotency)
	eventID1, _ := service.ProcessEvent(req)
	eventID2, _ := service.ProcessEvent(req)
	assert.Equal(t, eventID1, eventID2, "Same event should produce same event_id for idempotency")

	// Different event should produce different event_id
	reqDifferent := &dto.PublishEventRequest{
		EventName:  "different_event",
		Channel:    "mobile",
		UserID:     "user456",
		Timestamp:  testCurrentTime + 100,
		CampaignID: "campaign2",
	}

	mockPublisher.On("PublishEvent", mock.Anything, reqDifferent, mock.AnythingOfType("string")).Return(nil)

	eventID3, _ := service.ProcessEvent(reqDifferent)
	assert.NotEqual(t, eventID1, eventID3, "Different events should produce different event_ids")

	// Same content in different field should produce different event_id
	reqDifferentChannel := &dto.PublishEventRequest{
		EventName:  "test_event",
		Channel:    "mobile", // Different channel
		UserID:     "user123",
		Timestamp:  testCurrentTime,
		CampaignID: "campaign1",
	}

	mockPublisher.On("PublishEvent", mock.Anything, reqDifferentChannel, mock.AnythingOfType("string")).Return(nil)

	eventID4, _ := service.ProcessEvent(reqDifferentChannel)
	assert.NotEqual(t, eventID1, eventID4, "Different channel should produce different event_id")
}

func TestEventService_ProcessBulkEvents_AllSuccess(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	service := NewEventService(mockPublisher, mockRepo, log)

	events := []dto.PublishEventRequest{
		{
			EventName: "event1",
			Channel:   "web",
			UserID:    "user1",
			Timestamp: testCurrentTime,
		},
		{
			EventName: "event2",
			Channel:   "mobile",
			UserID:    "user2",
			Timestamp: testCurrentTime,
		},
	}

	mockPublisher.On("PublishEvent", mock.Anything, mock.Anything, mock.AnythingOfType("string")).Return(nil).Times(2)

	eventIDs, errors, err := service.ProcessBulkEvents(events)

	assert.NoError(t, err)
	assert.Len(t, eventIDs, 2)
	assert.Empty(t, errors)
	mockPublisher.AssertExpectations(t)
}

func TestEventService_ProcessBulkEvents_PartialFailure(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	service := NewEventService(mockPublisher, mockRepo, log)

	events := []dto.PublishEventRequest{
		{
			EventName: "event1",
			Channel:   "web",
			UserID:    "user1",
			Timestamp: testCurrentTime,
		},
		{
			EventName: "event2",
			Channel:   "mobile",
			UserID:    "user2",
			Timestamp: testFutureTime, // This will fail
		},
		{
			EventName: "event3",
			Channel:   "web",
			UserID:    "user3",
			Timestamp: testCurrentTime,
		},
	}

	mockPublisher.On("PublishEvent", mock.Anything, mock.Anything, mock.AnythingOfType("string")).Return(nil).Times(2)

	eventIDs, errs, err := service.ProcessBulkEvents(events)

	assert.NoError(t, err)
	assert.Len(t, eventIDs, 2)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "timestamp cannot be in the future")
}

func TestEventService_ProcessBulkEvents_AllFailure(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	service := NewEventService(mockPublisher, mockRepo, log)

	events := []dto.PublishEventRequest{
		{
			EventName: "event1",
			Channel:   "web",
			UserID:    "user1",
			Timestamp: testFutureTime,
		},
		{
			EventName: "event2",
			Channel:   "mobile",
			UserID:    "user2",
			Timestamp: testFutureTime,
		},
	}

	eventIDs, errs, err := service.ProcessBulkEvents(events)

	assert.NoError(t, err)
	assert.Empty(t, eventIDs)
	assert.Len(t, errs, 2)
	mockPublisher.AssertNotCalled(t, "PublishEvent")
}

func TestEventService_ProcessBulkEvents_EmptyList(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	service := NewEventService(mockPublisher, mockRepo, log)

	events := []dto.PublishEventRequest{}

	eventIDs, errs, err := service.ProcessBulkEvents(events)

	assert.NoError(t, err)
	assert.Empty(t, eventIDs)
	assert.Empty(t, errs)
	mockPublisher.AssertNotCalled(t, "PublishEvent")
}

func TestEventService_GetMetrics_Success(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	service := NewEventService(mockPublisher, mockRepo, log)

	req := &dto.GetMetricsRequest{
		EventName: "test_event",
		From:      1000,
		To:        2000,
		GroupBy:   "",
	}

	expectedResult := &repository.MetricsResult{
		TotalCount:  100,
		UniqueCount: 50,
		Groups:      []repository.MetricsGroupResult{},
	}

	mockRepo.On("GetMetrics", mock.Anything, repository.MetricsQuery{
		EventName: "test_event",
		From:      1000,
		To:        2000,
		GroupBy:   "",
	}).Return(expectedResult, nil)

	response, err := service.GetMetrics(req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, uint64(100), response.TotalCount)
	assert.Equal(t, uint64(50), response.UniqueCount)
	assert.Equal(t, "test_event", response.EventName)
	mockRepo.AssertExpectations(t)
}

func TestEventService_GetMetrics_InvalidTimeRange(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	service := NewEventService(mockPublisher, mockRepo, log)

	req := &dto.GetMetricsRequest{
		EventName: "test_event",
		From:      2000,
		To:        1000, // Invalid: From > To
	}

	response, err := service.GetMetrics(req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "from timestamp must be less than or equal to to timestamp")
	mockRepo.AssertNotCalled(t, "GetMetrics")
}

func TestEventService_GetMetrics_RepositoryError(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	service := NewEventService(mockPublisher, mockRepo, log)

	req := &dto.GetMetricsRequest{
		EventName: "test_event",
		From:      1000,
		To:        2000,
	}

	repoErr := errors.New("database connection error")
	mockRepo.On("GetMetrics", mock.Anything, mock.Anything).Return(nil, repoErr)

	response, err := service.GetMetrics(req)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to get metrics from repository")
	mockRepo.AssertExpectations(t)
}

func TestEventService_GetMetrics_WithGroupBy(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	log := zap.NewNop()

	service := NewEventService(mockPublisher, mockRepo, log)

	req := &dto.GetMetricsRequest{
		EventName: "test_event",
		From:      1000,
		To:        2000,
		GroupBy:   "channel",
	}

	expectedResult := &repository.MetricsResult{
		TotalCount:  100,
		UniqueCount: 50,
		Groups: []repository.MetricsGroupResult{
			{GroupValue: "web", TotalCount: 60},
			{GroupValue: "mobile", TotalCount: 40},
		},
	}

	mockRepo.On("GetMetrics", mock.Anything, repository.MetricsQuery{
		EventName: "test_event",
		From:      1000,
		To:        2000,
		GroupBy:   "channel",
	}).Return(expectedResult, nil)

	response, err := service.GetMetrics(req)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, uint64(100), response.TotalCount)
	assert.Equal(t, uint64(50), response.UniqueCount)
	assert.Len(t, response.Groups, 2)
	assert.Equal(t, "web", response.Groups[0].GroupValue)
	assert.Equal(t, uint64(60), response.Groups[0].TotalCount)
	assert.Equal(t, "mobile", response.Groups[1].GroupValue)
	assert.Equal(t, uint64(40), response.Groups[1].TotalCount)
	mockRepo.AssertExpectations(t)
}

func TestEventService_GetMetrics_GroupByHour(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	logger, _ := zap.NewDevelopment()
	eventService := NewEventService(mockPublisher, mockRepo, logger)

	req := &dto.GetMetricsRequest{
		EventName: "product_view",
		From:      1723475612,
		To:        1723562012, // ~24 hours
		GroupBy:   "hour",
	}

	expectedQuery := repository.MetricsQuery{
		EventName: "product_view",
		From:      1723475612,
		To:        1723562012,
		GroupBy:   "hour",
	}

	expectedResult := &repository.MetricsResult{
		TotalCount:  500,
		UniqueCount: 250,
		Groups: []repository.MetricsGroupResult{
			{GroupValue: "2024-08-12 14:00:00", TotalCount: 150},
			{GroupValue: "2024-08-12 15:00:00", TotalCount: 200},
			{GroupValue: "2024-08-12 16:00:00", TotalCount: 150},
		},
	}

	mockRepo.On("GetMetrics", mock.Anything, expectedQuery).Return(expectedResult, nil)

	response, err := eventService.GetMetrics(req)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, uint64(500), response.TotalCount)
	assert.Equal(t, uint64(250), response.UniqueCount)
	assert.Equal(t, "hour", response.GroupBy)
	assert.Len(t, response.Groups, 3)
	assert.Equal(t, "2024-08-12 14:00:00", response.Groups[0].GroupValue)
	mockRepo.AssertExpectations(t)
}

func TestEventService_GetMetrics_GroupByDay(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	logger, _ := zap.NewDevelopment()
	eventService := NewEventService(mockPublisher, mockRepo, logger)

	req := &dto.GetMetricsRequest{
		EventName: "product_view",
		From:      1723475612,
		To:        1726067612, // ~30 days
		GroupBy:   "day",
	}

	expectedQuery := repository.MetricsQuery{
		EventName: "product_view",
		From:      1723475612,
		To:        1726067612,
		GroupBy:   "day",
	}

	expectedResult := &repository.MetricsResult{
		TotalCount:  5000,
		UniqueCount: 2500,
		Groups: []repository.MetricsGroupResult{
			{GroupValue: "2024-08-12", TotalCount: 1500},
			{GroupValue: "2024-08-13", TotalCount: 1800},
			{GroupValue: "2024-08-14", TotalCount: 1700},
		},
	}

	mockRepo.On("GetMetrics", mock.Anything, expectedQuery).Return(expectedResult, nil)

	response, err := eventService.GetMetrics(req)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, uint64(5000), response.TotalCount)
	assert.Equal(t, uint64(2500), response.UniqueCount)
	assert.Equal(t, "day", response.GroupBy)
	assert.Len(t, response.Groups, 3)
	assert.Equal(t, "2024-08-12", response.Groups[0].GroupValue)
	mockRepo.AssertExpectations(t)
}

func TestEventService_GetMetrics_InvalidGroupBy(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	logger, _ := zap.NewDevelopment()
	eventService := NewEventService(mockPublisher, mockRepo, logger)

	req := &dto.GetMetricsRequest{
		EventName: "product_view",
		From:      1723475612,
		To:        1723562012,
		GroupBy:   "week", // Invalid group_by
	}

	response, err := eventService.GetMetrics(req)
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid group_by value")
	assert.Contains(t, err.Error(), "week")
	mockRepo.AssertNotCalled(t, "GetMetrics")
}

func TestEventService_GetMetrics_HourlyGroupingTooLargeRange(t *testing.T) {
	mockPublisher := new(MockQueuePublisher)
	mockRepo := new(MockEventRepository)
	logger, _ := zap.NewDevelopment()
	eventService := NewEventService(mockPublisher, mockRepo, logger)

	req := &dto.GetMetricsRequest{
		EventName: "product_view",
		From:      1723475612,
		To:        1723475612 + 91*24*3600, // 91 days - too large
		GroupBy:   "hour",
	}

	response, err := eventService.GetMetrics(req)
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "time range too large for hourly grouping")
	assert.Contains(t, err.Error(), "91 days")
	mockRepo.AssertNotCalled(t, "GetMetrics")
}
