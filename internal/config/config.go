package config

import (
	"fmt"

	"github.com/BarkinBalci/envconfig"
)

type Service struct {
	Environment string `envconfig:"SERVICE_ENVIRONMENT" required:"true"`
	APIPort     string `envconfig:"SERVICE_API_PORT" default:"8080"`
	Host        string `envconfig:"SERVICE_HOST" default:"localhost:8080"`
}

type SQS struct {
	Endpoint string `envconfig:"SQS_ENDPOINT"`
	QueueURL string `envconfig:"SQS_QUEUE_URL" required:"true"`
	Region   string `envconfig:"SQS_REGION" required:"true"`
}

type ClickHouse struct {
	Host            string `envconfig:"CLICKHOUSE_HOST" required:"true"`
	Port            string `envconfig:"CLICKHOUSE_PORT" required:"true"`
	Database        string `envconfig:"CLICKHOUSE_DATABASE" required:"true"`
	User            string `envconfig:"CLICKHOUSE_USER" default:""`
	Password        string `envconfig:"CLICKHOUSE_PASSWORD" default:""`
	UseTLS          bool   `envconfig:"CLICKHOUSE_USE_TLS" default:"true"`
	MaxOpenConns    int    `envconfig:"CLICKHOUSE_MAX_OPEN_CONNS" default:"5"`
	MaxIdleConns    int    `envconfig:"CLICKHOUSE_MAX_IDLE_CONNS" default:"2"`
	ConnMaxLifetime int    `envconfig:"CLICKHOUSE_CONN_MAX_LIFETIME" default:"10"`
}

type Consumer struct {
	BatchSizeMin    int    `envconfig:"CONSUMER_BATCH_SIZE_MIN" default:"100"`
	BatchSizeMax    int    `envconfig:"CONSUMER_BATCH_SIZE_MAX" default:"2000"`
	BatchTimeoutSec int    `envconfig:"CONSUMER_BATCH_TIMEOUT_SEC" default:"10"`
	HealthCheckPort string `envconfig:"CONSUMER_HEALTH_CHECK_PORT" default:"8081"`
}

type Config struct {
	Service    Service
	SQS        SQS
	ClickHouse ClickHouse
	Consumer   Consumer
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to process config: %w", err)
	}

	return &cfg, nil
}
