package dto

// PublishEventRequest represents a publish event request
type PublishEventRequest struct {
	EventName  string                 `json:"event_name" binding:"required" example:"product_view"`
	Channel    string                 `json:"channel" binding:"required" example:"web"`
	CampaignID string                 `json:"campaign_id" example:"cmp_987"`
	UserID     string                 `json:"user_id" binding:"required" example:"user_123"`
	Timestamp  int64                  `json:"timestamp" binding:"required" example:"1723475612"`
	Tags       []string               `json:"tags" example:"electronics,homepage,flash_sale"`
	Metadata   map[string]interface{} `json:"metadata" swaggertype:"object,string" example:"product_id:prod-789,price:129.99"`
}

// PublishEventsBulkRequest represents a publish bulk event request
type PublishEventsBulkRequest struct {
	Events []PublishEventRequest `json:"events" binding:"required,min=1,max=1000,dive"`
}

// GetMetricsRequest represents a metrics query request
type GetMetricsRequest struct {
	EventName string `form:"event_name" binding:"required" example:"product_view"`
	From      int64  `form:"from" binding:"required" example:"1723475612"`
	To        int64  `form:"to" binding:"required" example:"1723562012"`
	GroupBy   string `form:"group_by" example:"channel"`
}
