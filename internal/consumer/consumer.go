package consumer

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.uber.org/zap"

	"github.com/BarkinBalci/event-analytics-service/internal/config"
	"github.com/BarkinBalci/event-analytics-service/internal/queue"
	"github.com/BarkinBalci/event-analytics-service/internal/repository"
)

// Consumer orchestrates a pipeline of stages to process SQS messages
type Consumer struct {
	receiver    *Receiver
	parser      *ParserStage
	batchWriter *BatchWriter
}

// NewConsumer creates a new consumer with a pipeline architecture
func NewConsumer(cfg *config.Config, queueConsumer queue.QueueConsumer, repo repository.EventRepository, log *zap.Logger) *Consumer {
	receiver := NewReceiver(queueConsumer, ReceiverConfig{
		MaxMessages:     10,
		WaitTimeSeconds: 20,
		BufferSize:      100,
	}, log)

	parser := NewParserStage(queueConsumer, NewJSONEventParser(), log)

	batchWriter := NewBatchWriter(repo, BatchWriterConfig{
		MaxBatchSize: cfg.Consumer.BatchSizeMax,
		FlushTimeout: time.Duration(cfg.Consumer.BatchTimeoutSec) * time.Second,
	}, log)

	return &Consumer{
		receiver:    receiver,
		parser:      parser,
		batchWriter: batchWriter,
	}
}

// Start begins the consumer pipeline
func (c *Consumer) Start(ctx context.Context) error {
	messageChan := make(chan types.Message, 100)
	envelopeChan := make(chan *Envelope, 100)

	var wg sync.WaitGroup

	// Start all pipeline stages
	wg.Add(3)

	// Stage 1: Receive messages from SQS
	go func() {
		defer wg.Done()
		c.receiver.Start(ctx, messageChan)
	}()

	// Stage 2: Parse messages into envelopes
	go func() {
		defer wg.Done()
		c.parser.Start(ctx, messageChan, envelopeChan)
	}()

	// Stage 3: Batch and write to the repository
	go func() {
		defer wg.Done()
		c.batchWriter.Start(ctx, envelopeChan)
	}()

	wg.Wait()
	return nil
}
