package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.uber.org/zap"

	"github.com/BarkinBalci/event-analytics-service/internal/domain"
	"github.com/BarkinBalci/event-analytics-service/internal/repository"
)

// Repository implements EventRepository for ClickHouse
type Repository struct {
	client *Client
	log    *zap.Logger
}

// NewRepository creates a new ClickHouse repository
func NewRepository(client *Client, log *zap.Logger) *Repository {
	return &Repository{
		client: client,
		log:    log,
	}
}

// InitSchema initializes the ClickHouse schema with ReplacingMergeTree engine
func (r *Repository) InitSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS events (
		event_id String,
		event_name LowCardinality(String),
		channel LowCardinality(String),
		campaign_id String,
		user_id String,
		timestamp Int64,
		tags Array(String),
		metadata String,
		processed_at DateTime64(3) DEFAULT now64(3),
		version UInt64
	) ENGINE = ReplacingMergeTree(version)
	PRIMARY KEY (event_id)
	ORDER BY (event_id, timestamp)
	PARTITION BY toYYYYMM(toDateTime(timestamp))
	SETTINGS index_granularity = 8192
	`

	if err := r.client.Conn().Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create events table: %w", err)
	}

	r.log.Info("ClickHouse schema initialized successfully")
	return nil
}

// InsertBatch inserts a batch of events into ClickHouse
func (r *Repository) InsertBatch(ctx context.Context, events []*domain.Event) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}

	batch, err := r.client.Conn().PrepareBatch(ctx, "INSERT INTO events")
	if err != nil {
		return 0, fmt.Errorf("failed to prepare batch: %w", err)
	}

	insertedCount := 0
	for _, event := range events {
		if event.Version == 0 {
			event.Version = uint64(time.Now().UnixNano())
		}

		metadataJSON := event.Metadata
		if metadataJSON == "" {
			metadataJSON = "{}"
		}

		tags := event.Tags
		if tags == nil {
			tags = []string{}
		}

		err := batch.Append(
			event.EventID,
			event.EventName,
			event.Channel,
			event.CampaignID,
			event.UserID,
			event.Timestamp,
			tags,
			metadataJSON,
			event.ProcessedAt,
			event.Version,
		)

		if err != nil {
			return 0, fmt.Errorf("failed to append event to batch: %w", err)
		}
		insertedCount++
	}

	if insertedCount == 0 {
		return 0, fmt.Errorf("no events could be appended to batch")
	}

	if err := batch.Send(); err != nil {
		return 0, fmt.Errorf("failed to send batch: %w", err)
	}

	return insertedCount, nil
}

// Ping checks if the ClickHouse connection is alive
func (r *Repository) Ping(ctx context.Context) error {
	return r.client.Conn().Ping(ctx)
}

// Close closes the ClickHouse connection
func (r *Repository) Close() error {
	return r.client.Close()
}

// GetMetrics retrieves aggregated metrics from ClickHouse
func (r *Repository) GetMetrics(ctx context.Context, query repository.MetricsQuery) (*repository.MetricsResult, error) {
	result := &repository.MetricsResult{
		Groups: []repository.MetricsGroupResult{},
	}

	// Build the WHERE clause
	whereClause := "WHERE event_name = ? AND timestamp >= ? AND timestamp <= ?"
	args := []interface{}{query.EventName, query.From, query.To}

	// Get overall metrics
	overallQuery := fmt.Sprintf(`
		SELECT
			count() as total_count,
			uniq(user_id) as unique_count
		FROM events FINAL
		%s
	`, whereClause)

	row := r.client.Conn().QueryRow(ctx, overallQuery, args...)
	if err := row.Scan(&result.TotalCount, &result.UniqueCount); err != nil {
		return nil, fmt.Errorf("failed to query overall metrics: %w", err)
	}

	// If groupBy is specified, get grouped metrics
	if query.GroupBy != "" {
		validGroupBy := map[string]bool{"channel": true, "hour": true, "day": true}
		if !validGroupBy[query.GroupBy] {
			return nil, fmt.Errorf("unsupported group_by value: %s (supported: channel, hour, day)", query.GroupBy)
		}

		var selectField string
		var groupByClause string
		var orderBy string

		switch query.GroupBy {
		case "channel":
			selectField = "channel"
			groupByClause = "GROUP BY channel"
			orderBy = "ORDER BY total_count DESC"
		case "hour":
			selectField = "formatDateTime(toStartOfHour(toDateTime(timestamp)), '%Y-%m-%d %H:00:00')"
			groupByClause = "GROUP BY toStartOfHour(toDateTime(timestamp))"
			orderBy = "ORDER BY group_value ASC"
		case "day":
			selectField = "formatDateTime(toStartOfDay(toDateTime(timestamp)), '%Y-%m-%d')"
			groupByClause = "GROUP BY toStartOfDay(toDateTime(timestamp))"
			orderBy = "ORDER BY group_value ASC"
		}

		groupedQuery := fmt.Sprintf(`
			SELECT
				%s as group_value,
				count() as total_count
			FROM events FINAL
			%s
			%s
			%s
		`, selectField, whereClause, groupByClause, orderBy)

		rows, err := r.client.Conn().Query(ctx, groupedQuery, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to query grouped metrics: %w", err)
		}
		defer func(rows driver.Rows) {
			err := rows.Close()
			if err != nil {
				r.log.Error("Failed to close grouped metrics rows", zap.Error(err))
			}
		}(rows)

		for rows.Next() {
			var group repository.MetricsGroupResult
			if err := rows.Scan(&group.GroupValue, &group.TotalCount); err != nil {
				return nil, fmt.Errorf("failed to scan grouped metrics row: %w", err)
			}
			result.Groups = append(result.Groups, group)
		}

		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating grouped metrics rows: %w", err)
		}
	}

	return result, nil
}
