package domain

import "time"

// Event represents an event stored in ClickHouse
type Event struct {
	EventID     string    `ch:"event_id"`
	EventName   string    `ch:"event_name"`
	Channel     string    `ch:"channel"`
	CampaignID  string    `ch:"campaign_id"`
	UserID      string    `ch:"user_id"`
	Timestamp   int64     `ch:"timestamp"`
	Tags        []string  `ch:"tags"`
	Metadata    string    `ch:"metadata"`
	ProcessedAt time.Time `ch:"processed_at"`
	Version     uint64    `ch:"version"`
}
