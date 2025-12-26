package consumer

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/BarkinBalci/event-analytics-service/internal/domain"
)

// JSONEventParser implements MessageParser for JSON-formatted event messages
type JSONEventParser struct{}

// NewJSONEventParser creates a new JSON event parser
func NewJSONEventParser() *JSONEventParser {
	return &JSONEventParser{}
}

// Parse parses a JSON message body into an Event
func (p *JSONEventParser) Parse(body []byte) (*domain.Event, error) {
	var msgBody map[string]interface{}
	if err := json.Unmarshal(body, &msgBody); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message body: %w", err)
	}

	metadataJSON := "{}"
	if metadata, ok := msgBody["metadata"].(map[string]interface{}); ok && len(metadata) > 0 {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = string(metadataBytes)
	}

	var tags []string
	if tagsInterface, ok := msgBody["tags"].([]interface{}); ok {
		for _, t := range tagsInterface {
			if tagStr, ok := t.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	}

	event := &domain.Event{
		EventID:     getStringField(msgBody, "event_id"),
		EventName:   getStringField(msgBody, "event_name"),
		Channel:     getStringField(msgBody, "channel"),
		CampaignID:  getStringField(msgBody, "campaign_id"),
		UserID:      getStringField(msgBody, "user_id"),
		Timestamp:   getInt64Field(msgBody, "timestamp"),
		Tags:        tags,
		Metadata:    metadataJSON,
		ProcessedAt: time.Now(),
		Version:     uint64(time.Now().UnixNano()),
	}

	return event, nil
}

// Helper functions for extracting fields from parsed JSON
func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getInt64Field(m map[string]interface{}, key string) int64 {
	if val, ok := m[key].(float64); ok {
		return int64(val)
	}
	return 0
}
