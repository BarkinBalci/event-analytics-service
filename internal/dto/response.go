package dto

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error" example:"validation_error"`
	Message string `json:"message,omitempty" example:"event_name is required"`
}

// PublishEventResponse represents a successful event ingestion response
type PublishEventResponse struct {
	EventID string `json:"event_id" example:"evt_1a2b3c4d5e6f"`
	Status  string `json:"status" example:"accepted"`
}

// PublishBulkEventsResponse represents a successful bulk event ingestion response
type PublishBulkEventsResponse struct {
	Accepted int      `json:"accepted" example:"5"`
	Rejected int      `json:"rejected" example:"0"`
	EventIDs []string `json:"event_ids,omitempty" example:"evt_1,evt_2,evt_3"`
	Errors   []string `json:"errors,omitempty" example:"validation error on event 3"`
}

// MetricsGroupData represents aggregated metrics for a specific group
type MetricsGroupData struct {
	GroupValue string `json:"group_value" example:"web"`
	TotalCount uint64 `json:"total_count" example:"1500"`
}

// GetMetricsResponse represents the metrics query response
type GetMetricsResponse struct {
	EventName   string             `json:"event_name" example:"product_view"`
	From        int64              `json:"from" example:"1723475612"`
	To          int64              `json:"to" example:"1723562012"`
	TotalCount  uint64             `json:"total_count" example:"5000"`
	UniqueCount uint64             `json:"unique_count" example:"2500"`
	GroupBy     string             `json:"group_by,omitempty" example:"channel"`
	Groups      []MetricsGroupData `json:"groups,omitempty"`
}
