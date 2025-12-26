package clickhouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/BarkinBalci/event-analytics-service/internal/config"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Client wraps the ClickHouse connection
type Client struct {
	connection driver.Conn
	config     *config.ClickHouse
	log        *zap.Logger
}

// NewClient creates a new ClickHouse client with the given configuration
func NewClient(ctx context.Context, config *config.ClickHouse, log *zap.Logger) (*Client, error) {
	addr := fmt.Sprintf("%s:%s", config.Host, config.Port)

	log.Info("Connecting to ClickHouse",
		zap.String("host", config.Host),
		zap.String("port", config.Port),
		zap.String("database", config.Database),
		zap.Bool("useTLS", config.UseTLS))

	// Configure TLS based on environment
	var tlsConfig *tls.Config
	if config.UseTLS {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: false,
		}
	}

	connection, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.User,
			Password: config.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		TLS:              tlsConfig,
		DialTimeout:      5 * time.Second,
		MaxOpenConns:     config.MaxOpenConns,
		MaxIdleConns:     config.MaxIdleConns,
		ConnMaxLifetime:  time.Duration(config.ConnMaxLifetime) * time.Second,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
		BlockBufferSize:  10,
	})

	if err != nil {
		log.Error("Failed to connect to ClickHouse", zap.Error(err))
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	// Verify connection
	if err := connection.Ping(ctx); err != nil {
		log.Error("Failed to ping ClickHouse", zap.Error(err))
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	log.Info("ClickHouse connection established successfully")

	return &Client{connection: connection, config: config, log: log}, nil
}

// Conn returns the underlying ClickHouse connection
func (c *Client) Conn() driver.Conn {
	return c.connection
}

// Close closes the ClickHouse connection
func (c *Client) Close() error {
	c.log.Info("Closing ClickHouse connection")
	if err := c.connection.Close(); err != nil {
		c.log.Error("Error closing ClickHouse connection", zap.Error(err))
		return err
	}
	c.log.Info("ClickHouse connection closed successfully")
	return nil
}
