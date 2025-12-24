package models

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// PublishEventResponse represents a successful event ingestion response
type PublishEventResponse struct {
	EventID string `json:"event_id"`
	Status  string `json:"status"`
}

// PublishBulkEventsResponse represents a successful bulk event ingestion response
type PublishBulkEventsResponse struct {
	Accepted int      `json:"accepted"`
	Rejected int      `json:"rejected"`
	EventIDs []string `json:"event_ids,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}
