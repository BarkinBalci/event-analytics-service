package config

import (
	"fmt"

	"github.com/BarkinBalci/envconfig"
)

type Config struct {
	ServiceEnvironment           string `envconfig:"SERVICE_ENVIRONMENT" required:"true"`
	ServiceAPIPort               string `envconfig:"SERVICE_API_PORT" default:"8080"`
	ServiceHost                  string `envconfig:"SERVICE_HOST" default:"localhost:8080"`
	ValkeyHost                   string `envconfig:"VALKEY_HOST" required:"true"`
	ValkeyPort                   string `envconfig:"VALKEY_PORT" required:"true"`
	SQSEndpoint                  string `envconfig:"SQS_ENDPOINT"`
	SQSQueueURL                  string `envconfig:"SQS_QUEUE_URL" required:"true"`
	SQSRegion                    string `envconfig:"SQS_REGION" required:"true"`
	ClickHouseHost               string `envconfig:"CLICKHOUSE_HOST" required:"true"`
	ClickHousePort               string `envconfig:"CLICKHOUSE_PORT" required:"true"`
	ClickHouseDB                 string `envconfig:"CLICKHOUSE_DB" required:"true"`
	ClickHouseUser               string `envconfig:"CLICKHOUSE_USER" default:""`
	ClickHousePassword           string `envconfig:"CLICKHOUSE_PASSWORD" default:""`
	ClickHouseMaxOpenConns       int    `envconfig:"CLICKHOUSE_MAX_OPEN_CONNS" default:"5"`
	ClickHouseMaxIdleConns       int    `envconfig:"CLICKHOUSE_MAX_IDLE_CONNS" default:"2"`
	ClickHouseConnMaxLifetimeSec int    `envconfig:"CLICKHOUSE_CONN_MAX_LIFETIME_SEC" default:"3600"`
	ConsumerBatchSizeMin         int    `envconfig:"CONSUMER_BATCH_SIZE_MIN" default:"100"`
	ConsumerBatchSizeMax         int    `envconfig:"CONSUMER_BATCH_SIZE_MAX" default:"2000"`
	ConsumerBatchTimeoutSec      int    `envconfig:"CONSUMER_BATCH_TIMEOUT_SEC" default:"10"`
	ConsumerHealthCheckPort      string `envconfig:"CONSUMER_HEALTH_CHECK_PORT" default:"8081"`
	ValkeyIdempotencyEnabled     bool   `envconfig:"VALKEY_IDEMPOTENCY_ENABLED" default:"true"`
	ValkeyIdempotencyFailOpen    bool   `envconfig:"VALKEY_IDEMPOTENCY_FAIL_OPEN" default:"true"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to process config: %w", err)
	}

	return &cfg, nil
}
