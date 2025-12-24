package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/BarkinBalci/event-analytics-service/internal/models"
	"github.com/BarkinBalci/event-analytics-service/internal/service"
)

type Handler struct {
	eventService *service.EventService
	router       *gin.Engine
}

func NewHandler() *Handler {
	h := &Handler{
		eventService: service.NewEventService(),
		router:       gin.Default(),
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
}

// healthCheck handles health check requests
func (h *Handler) healthCheck(c *gin.Context) {
	// TODO: add a more sophisticated health check
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// publishEvent handles POST /events
func (h *Handler) publishEvent(c *gin.Context) {
	var req models.PublishEventRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	eventID, err := h.eventService.ProcessEvent(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, models.PublishEventResponse{
		EventID: eventID,
		Status:  "accepted",
	})
}

// publishEventsBulk handles POST /events/bulk
func (h *Handler) publishEventsBulk(c *gin.Context) {
	var bulkRequest models.PublishEventsBulkRequest

	if err := c.ShouldBindJSON(&bulkRequest); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	eventIDs, errors, err := h.eventService.ProcessBulkEvents(bulkRequest.Events)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Message: err.Error(),
		})
		return
	}

	accepted := len(eventIDs)
	rejected := len(errors)

	c.JSON(http.StatusAccepted, models.PublishBulkEventsResponse{
		Accepted: accepted,
		Rejected: rejected,
		EventIDs: eventIDs,
		Errors:   errors,
	})
}
