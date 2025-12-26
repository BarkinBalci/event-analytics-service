package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/BarkinBalci/event-analytics-service/internal/dto"
)

const (
	testTimestamp int64 = 1766702551
)

// MockEventService is a mock implementation of service.EventServicer
type MockEventService struct {
	mock.Mock
}

func (m *MockEventService) ProcessEvent(event *dto.PublishEventRequest) (string, error) {
	args := m.Called(event)
	return args.String(0), args.Error(1)
}

func (m *MockEventService) ProcessBulkEvents(events []dto.PublishEventRequest) ([]string, []string, error) {
	args := m.Called(events)
	return args.Get(0).([]string), args.Get(1).([]string), args.Error(2)
}

func (m *MockEventService) GetMetrics(req *dto.GetMetricsRequest) (*dto.GetMetricsResponse, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.GetMetricsResponse), args.Error(1)
}

func TestHandler_HealthCheck(t *testing.T) {
	mockService := new(MockEventService)
	log := zap.NewNop()

	handler := NewHandler(mockService, log)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
}

func TestHandler_PublishEvent_Success(t *testing.T) {
	mockService := new(MockEventService)
	log := zap.NewNop()

	handler := NewHandler(mockService, log)

	eventReq := dto.PublishEventRequest{
		EventName:  "test_event",
		Channel:    "web",
		UserID:     "user123",
		Timestamp:  testTimestamp,
		CampaignID: "campaign1",
	}

	mockService.On("ProcessEvent", &eventReq).Return("event-id-123", nil)

	body, _ := json.Marshal(eventReq)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response dto.PublishEventResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "event-id-123", response.EventID)
	assert.Equal(t, "accepted", response.Status)
	mockService.AssertExpectations(t)
}

func TestHandler_PublishEvent_InvalidJSON(t *testing.T) {
	mockService := new(MockEventService)
	log := zap.NewNop()

	handler := NewHandler(mockService, log)

	invalidJSON := []byte(`{"event_name": "test", invalid}`)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "validation_error", response.Error)
	mockService.AssertNotCalled(t, "ProcessEvent")
}

func TestHandler_PublishEvent_MissingRequiredFields(t *testing.T) {
	mockService := new(MockEventService)
	log := zap.NewNop()

	handler := NewHandler(mockService, log)

	eventReq := dto.PublishEventRequest{
		EventName: "test_event",
		// Missing required fields: Channel, UserID, Timestamp
	}

	body, _ := json.Marshal(eventReq)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "validation_error", response.Error)
	mockService.AssertNotCalled(t, "ProcessEvent")
}

func TestHandler_PublishEvent_ServiceError(t *testing.T) {
	mockService := new(MockEventService)
	log := zap.NewNop()

	handler := NewHandler(mockService, log)

	eventReq := dto.PublishEventRequest{
		EventName: "test_event",
		Channel:   "web",
		UserID:    "user123",
		Timestamp: testTimestamp,
	}

	serviceErr := errors.New("queue publish error")
	mockService.On("ProcessEvent", &eventReq).Return("", serviceErr)

	body, _ := json.Marshal(eventReq)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "internal_error", response.Error)
	assert.Contains(t, response.Message, "queue publish error")
	mockService.AssertExpectations(t)
}

func TestHandler_PublishEventsBulk_Success(t *testing.T) {
	mockService := new(MockEventService)
	log := zap.NewNop()

	handler := NewHandler(mockService, log)

	bulkReq := dto.PublishEventsBulkRequest{
		Events: []dto.PublishEventRequest{
			{
				EventName: "event1",
				Channel:   "web",
				UserID:    "user1",
				Timestamp: testTimestamp,
			},
			{
				EventName: "event2",
				Channel:   "mobile",
				UserID:    "user2",
				Timestamp: testTimestamp,
			},
		},
	}

	mockService.On("ProcessBulkEvents", bulkReq.Events).Return(
		[]string{"event-id-1", "event-id-2"},
		[]string{},
		nil,
	)

	body, _ := json.Marshal(bulkReq)
	req := httptest.NewRequest(http.MethodPost, "/events/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response dto.PublishBulkEventsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 2, response.Accepted)
	assert.Equal(t, 0, response.Rejected)
	assert.Len(t, response.EventIDs, 2)
	assert.Empty(t, response.Errors)
	mockService.AssertExpectations(t)
}

func TestHandler_PublishEventsBulk_PartialSuccess(t *testing.T) {
	mockService := new(MockEventService)
	log := zap.NewNop()

	handler := NewHandler(mockService, log)

	bulkReq := dto.PublishEventsBulkRequest{
		Events: []dto.PublishEventRequest{
			{
				EventName: "event1",
				Channel:   "web",
				UserID:    "user1",
				Timestamp: testTimestamp,
			},
			{
				EventName: "event2",
				Channel:   "mobile",
				UserID:    "user2",
				Timestamp: testTimestamp,
			},
		},
	}

	mockService.On("ProcessBulkEvents", bulkReq.Events).Return(
		[]string{"event-id-1"},
		[]string{"timestamp validation failed"},
		nil,
	)

	body, _ := json.Marshal(bulkReq)
	req := httptest.NewRequest(http.MethodPost, "/events/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response dto.PublishBulkEventsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 1, response.Accepted)
	assert.Equal(t, 1, response.Rejected)
	assert.Len(t, response.EventIDs, 1)
	assert.Len(t, response.Errors, 1)
	mockService.AssertExpectations(t)
}

func TestHandler_PublishEventsBulk_InvalidRequest(t *testing.T) {
	mockService := new(MockEventService)
	log := zap.NewNop()

	handler := NewHandler(mockService, log)

	invalidJSON := []byte(`{"events": [{"invalid"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/events/bulk", bytes.NewReader(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "validation_error", response.Error)
	mockService.AssertNotCalled(t, "ProcessBulkEvents")
}

func TestHandler_PublishEventsBulk_EmptyEvents(t *testing.T) {
	mockService := new(MockEventService)
	log := zap.NewNop()

	handler := NewHandler(mockService, log)

	bulkReq := dto.PublishEventsBulkRequest{
		Events: []dto.PublishEventRequest{},
	}

	body, _ := json.Marshal(bulkReq)
	req := httptest.NewRequest(http.MethodPost, "/events/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "validation_error", response.Error)
	mockService.AssertNotCalled(t, "ProcessBulkEvents")
}

func TestHandler_GetMetrics_Success(t *testing.T) {
	mockService := new(MockEventService)
	log := zap.NewNop()

	handler := NewHandler(mockService, log)

	expectedResponse := &dto.GetMetricsResponse{
		EventName:   "test_event",
		From:        1000,
		To:          2000,
		TotalCount:  100,
		UniqueCount: 50,
		Groups:      []dto.MetricsGroupData{},
	}

	mockService.On("GetMetrics", &dto.GetMetricsRequest{
		EventName: "test_event",
		From:      1000,
		To:        2000,
	}).Return(expectedResponse, nil)

	req := httptest.NewRequest(http.MethodGet, "/metrics?event_name=test_event&from=1000&to=2000", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.GetMetricsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test_event", response.EventName)
	assert.Equal(t, uint64(100), response.TotalCount)
	assert.Equal(t, uint64(50), response.UniqueCount)
	mockService.AssertExpectations(t)
}

func TestHandler_GetMetrics_InvalidQueryParams(t *testing.T) {
	mockService := new(MockEventService)
	log := zap.NewNop()

	handler := NewHandler(mockService, log)

	// Missing required query parameters
	req := httptest.NewRequest(http.MethodGet, "/metrics?event_name=test_event", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "validation_error", response.Error)
	mockService.AssertNotCalled(t, "GetMetrics")
}

func TestHandler_GetMetrics_ServiceError(t *testing.T) {
	mockService := new(MockEventService)
	log := zap.NewNop()

	handler := NewHandler(mockService, log)

	serviceErr := errors.New("database connection error")
	mockService.On("GetMetrics", &dto.GetMetricsRequest{
		EventName: "test_event",
		From:      1000,
		To:        2000,
	}).Return(nil, serviceErr)

	req := httptest.NewRequest(http.MethodGet, "/metrics?event_name=test_event&from=1000&to=2000", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response dto.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "internal_error", response.Error)
	assert.Contains(t, response.Message, "database connection error")
	mockService.AssertExpectations(t)
}

func TestHandler_GetMetrics_GroupByHour(t *testing.T) {
	mockService := new(MockEventService)
	log := zap.NewNop()
	handler := NewHandler(mockService, log)

	expectedResponse := &dto.GetMetricsResponse{
		EventName:   "product_view",
		From:        1766702551,
		To:          1780702551,
		TotalCount:  500,
		UniqueCount: 250,
		GroupBy:     "hour",
		Groups: []dto.MetricsGroupData{
			{GroupValue: "2025-12-27 14:00:00", TotalCount: 150},
			{GroupValue: "2025-12-27 15:00:00", TotalCount: 200},
			{GroupValue: "2025-12-27 16:00:00", TotalCount: 150},
		},
	}

	mockService.On("GetMetrics", mock.MatchedBy(func(req *dto.GetMetricsRequest) bool {
		return req.EventName == "product_view" &&
			req.From == 1766702551 &&
			req.To == 1780702551 &&
			req.GroupBy == "hour"
	})).Return(expectedResponse, nil)

	req, _ := http.NewRequest("GET", "/metrics?event_name=product_view&from=1766702551&to=1780702551&group_by=hour", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.GetMetricsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "product_view", response.EventName)
	assert.Equal(t, "hour", response.GroupBy)
	assert.Len(t, response.Groups, 3)
	assert.Equal(t, "2025-12-27 14:00:00", response.Groups[0].GroupValue)
	mockService.AssertExpectations(t)
}

func TestHandler_GetMetrics_GroupByDay(t *testing.T) {
	mockService := new(MockEventService)
	log := zap.NewNop()
	handler := NewHandler(mockService, log)

	expectedResponse := &dto.GetMetricsResponse{
		EventName:   "product_view",
		From:        1766702551,
		To:          1780702551,
		TotalCount:  5000,
		UniqueCount: 2500,
		GroupBy:     "day",
		Groups: []dto.MetricsGroupData{
			{GroupValue: "2025-12-27", TotalCount: 1500},
			{GroupValue: "2025-12-28", TotalCount: 1800},
			{GroupValue: "2025-12-29", TotalCount: 1700},
		},
	}

	mockService.On("GetMetrics", mock.MatchedBy(func(req *dto.GetMetricsRequest) bool {
		return req.EventName == "product_view" &&
			req.From == 1766702551 &&
			req.To == 1780702551 &&
			req.GroupBy == "day"
	})).Return(expectedResponse, nil)

	req, _ := http.NewRequest("GET", "/metrics?event_name=product_view&from=1766702551&to=1780702551&group_by=day", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.GetMetricsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "product_view", response.EventName)
	assert.Equal(t, "day", response.GroupBy)
	assert.Len(t, response.Groups, 3)
	assert.Equal(t, "2025-12-27", response.Groups[0].GroupValue)
	mockService.AssertExpectations(t)
}
