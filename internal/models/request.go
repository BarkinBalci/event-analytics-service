package models

// PublishEventRequest represents a publish event request
type PublishEventRequest struct {
	EventName  string                 `json:"event_name" binding:"required"`
	Channel    string                 `json:"channel" binding:"required"`
	CampaignID string                 `json:"campaign_id"`
	UserID     string                 `json:"user_id" binding:"required"`
	Timestamp  int64                  `json:"timestamp" binding:"required"`
	Tags       []string               `json:"tags"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// PublishEventsBulkRequest represents a publish bulk event request
type PublishEventsBulkRequest struct {
	Events []PublishEventRequest `json:"events" binding:"required,min=1,max=1000,dive"`
}
