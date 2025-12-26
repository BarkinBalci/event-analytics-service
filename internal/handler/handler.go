package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"

	_ "github.com/BarkinBalci/event-analytics-service/docs"
	"github.com/BarkinBalci/event-analytics-service/internal/dto"
	"github.com/BarkinBalci/event-analytics-service/internal/service"
)

type Handler struct {
	eventService service.EventServicer
	router       *gin.Engine
	log          *zap.Logger
}

func NewHandler(eventService service.EventServicer, log *zap.Logger) *Handler {
	h := &Handler{
		eventService: eventService,
		router:       gin.Default(),
		log:          log,
	}

	h.registerRoutes()

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	h.router.GET("/health", h.healthCheck)
	h.router.POST("/events", h.publishEvent)
	h.router.POST("/events/bulk", h.publishEventsBulk)
	h.router.GET("/metrics", h.getMetrics)
	h.router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

// healthCheck handles health check requests
// @Summary Health check
// @Description Check if the service is running
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func (h *Handler) healthCheck(c *gin.Context) {
	// TODO: add a more sophisticated health check
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// publishEvent handles POST /events
// @Summary Publish a single event
// @Description Publish a single analytics event to the queue
// @Tags events
// @Accept json
// @Produce json
// @Param event body dto.PublishEventRequest true "Event data"
// @Success 202 {object} dto.PublishEventResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /events [post]
func (h *Handler) publishEvent(c *gin.Context) {
	var req dto.PublishEventRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid event request",
			zap.Error(err),
			zap.String("event_name", req.EventName))
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	eventID, err := h.eventService.ProcessEvent(&req)
	if err != nil {
		h.log.Error("Failed to process event",
			zap.Error(err),
			zap.String("event_name", req.EventName),
			zap.String("user_id", req.UserID))
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: err.Error(),
		})
		return
	}

	h.log.Info("Event accepted",
		zap.String("event_id", eventID),
		zap.String("event_name", req.EventName))

	c.JSON(http.StatusAccepted, dto.PublishEventResponse{
		EventID: eventID,
		Status:  "accepted",
	})
}

// publishEventsBulk handles POST /events/bulk
// @Summary Publish multiple events
// @Description Publish multiple analytics events in bulk to the queue
// @Tags events
// @Accept json
// @Produce json
// @Param events body dto.PublishEventsBulkRequest true "Bulk events data"
// @Success 202 {object} dto.PublishBulkEventsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /events/bulk [post]
func (h *Handler) publishEventsBulk(c *gin.Context) {
	var bulkRequest dto.PublishEventsBulkRequest

	if err := c.ShouldBindJSON(&bulkRequest); err != nil {
		h.log.Warn("Invalid bulk event request", zap.Error(err))
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	eventIDs, errors, err := h.eventService.ProcessBulkEvents(bulkRequest.Events)
	if err != nil {
		h.log.Error("Failed to process bulk events",
			zap.Error(err),
			zap.Int("event_count", len(bulkRequest.Events)))
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: err.Error(),
		})
		return
	}

	accepted := len(eventIDs)
	rejected := len(errors)

	h.log.Info("Bulk events processed",
		zap.Int("accepted", accepted),
		zap.Int("rejected", rejected),
		zap.Int("total", len(bulkRequest.Events)))

	c.JSON(http.StatusAccepted, dto.PublishBulkEventsResponse{
		Accepted: accepted,
		Rejected: rejected,
		EventIDs: eventIDs,
		Errors:   errors,
	})
}

// getMetrics handles GET /metrics
// @Summary Get aggregated metrics
// @Description Retrieve aggregated event metrics with optional grouping by channel, hour, or day
// @Tags metrics
// @Produce json
// @Param event_name query string true "Event name to filter by" example:"product_view"
// @Param from query int true "Start timestamp (Unix epoch)" example:"1723475612"
// @Param to query int true "End timestamp (Unix epoch)" example:"1723562012"
// @Param group_by query string false "Field to group by (channel, hour, day)" Enums(channel, hour, day) example:"channel"
// @Success 200 {object} dto.GetMetricsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /metrics [get]
func (h *Handler) getMetrics(c *gin.Context) {
	var req dto.GetMetricsRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		h.log.Warn("Invalid metrics request", zap.Error(err))
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	response, err := h.eventService.GetMetrics(&req)
	if err != nil {
		h.log.Error("Failed to get metrics",
			zap.Error(err),
			zap.String("event_name", req.EventName),
			zap.Int64("from", req.From),
			zap.Int64("to", req.To))
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: err.Error(),
		})
		return
	}

	h.log.Info("Metrics retrieved",
		zap.String("event_name", req.EventName),
		zap.Uint64("total_count", response.TotalCount),
		zap.Uint64("unique_count", response.UniqueCount))

	c.JSON(http.StatusOK, response)
}
