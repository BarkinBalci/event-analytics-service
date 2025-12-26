package sqs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.uber.org/zap"

	envConfig "github.com/BarkinBalci/event-analytics-service/internal/config"
	"github.com/BarkinBalci/event-analytics-service/internal/dto"
)

// Client represents an SQS client
type Client struct {
	client *sqs.Client
	config envConfig.SQS
	log    *zap.Logger
}

// NewClient creates a new SQS client
func NewClient(ctx context.Context, SQSConfig envConfig.SQS, log *zap.Logger) (*Client, error) {
	configOpts := []func(*config.LoadOptions) error{
		config.WithRegion(SQSConfig.Region),
	}

	var clientOpts []func(*sqs.Options)

	// Configure for local development with ElasticMQ
	if SQSConfig.Endpoint != "" {
		log.Info("Configuring SQS for local development",
			zap.String("endpoint", SQSConfig.Endpoint))
		configOpts = append(configOpts,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("dummy", "dummy", "")))

		clientOpts = append(clientOpts, func(o *sqs.Options) {
			o.BaseEndpoint = aws.String(SQSConfig.Endpoint)
		})
	}

	cfg, err := config.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	sqsClient := sqs.NewFromConfig(cfg, clientOpts...)

	log.Info("SQS client created",
		zap.String("region", SQSConfig.Region),
		zap.String("queue_url", SQSConfig.QueueURL))

	return &Client{
		client: sqsClient,
		config: SQSConfig,
		log:    log,
	}, nil
}

// ReceiveMessages receives messages from SQS
func (c *Client) ReceiveMessages(ctx context.Context, input *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
	return c.client.ReceiveMessage(ctx, input)
}

// DeleteMessage deletes a message from SQS
func (c *Client) DeleteMessage(ctx context.Context, input *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error) {
	return c.client.DeleteMessage(ctx, input)
}

// Client returns the underlying SQS client
func (c *Client) Client() *sqs.Client {
	return c.client
}

// QueueURL returns the configured queue URL
func (c *Client) QueueURL() string {
	return c.config.QueueURL
}

// PublishEvent publishes an event to SQS
func (c *Client) PublishEvent(ctx context.Context, event *dto.PublishEventRequest, eventID string) error {
	messageBody := map[string]interface{}{
		"event_id":    eventID,
		"event_name":  event.EventName,
		"channel":     event.Channel,
		"campaign_id": event.CampaignID,
		"user_id":     event.UserID,
		"timestamp":   event.Timestamp,
		"tags":        event.Tags,
		"metadata":    event.Metadata,
	}

	bodyJSON, err := json.Marshal(messageBody)
	if err != nil {
		c.log.Error("Failed to marshal event",
			zap.String("event_id", eventID),
			zap.String("event_name", event.EventName),
			zap.Error(err))
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = c.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(c.config.QueueURL),
		MessageBody: aws.String(string(bodyJSON)),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"EventName": {
				DataType:    aws.String("String"),
				StringValue: aws.String(event.EventName),
			},
			"Channel": {
				DataType:    aws.String("String"),
				StringValue: aws.String(event.Channel),
			},
		},
	})
	if err != nil {
		c.log.Error("Failed to send message to SQS",
			zap.String("event_id", eventID),
			zap.String("event_name", event.EventName),
			zap.Error(err))
		return fmt.Errorf("failed to send message to SQS: %w", err)
	}

	c.log.Info("Event published to SQS",
		zap.String("event_id", eventID),
		zap.String("event_name", event.EventName))

	return nil
}
