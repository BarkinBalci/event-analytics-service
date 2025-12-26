package consumer

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/BarkinBalci/event-analytics-service/internal/domain"
	"github.com/BarkinBalci/event-analytics-service/internal/repository"
)

// BatchWriterConfig configures the batch writer
type BatchWriterConfig struct {
	MaxBatchSize int
	FlushTimeout time.Duration
}

// BatchWriter handles batching and writing events to the repository
type BatchWriter struct {
	repository repository.EventRepository
	config     BatchWriterConfig
	log        *zap.Logger
}

// NewBatchWriter creates a new batch writer
func NewBatchWriter(repo repository.EventRepository, config BatchWriterConfig, log *zap.Logger) *BatchWriter {
	return &BatchWriter{
		repository: repo,
		config:     config,
		log:        log,
	}
}

// Start begins processing envelopes, batching, and writing to the repository
func (w *BatchWriter) Start(ctx context.Context, in <-chan *Envelope) {
	ticker := time.NewTicker(w.config.FlushTimeout)
	defer ticker.Stop()

	batch := make([]*Envelope, 0, w.config.MaxBatchSize)

	for {
		select {
		case <-ctx.Done():
			w.log.Info("Batch writer shutting down")
			if len(batch) > 0 {
				w.log.Info("Flushing final batch", zap.Int("envelope_count", len(batch)))
				w.processBatch(ctx, batch)
			}
			return

		case envelope, ok := <-in:
			if !ok {
				w.log.Info("Batch writer input channel closed")
				if len(batch) > 0 {
					w.log.Info("Flushing final batch", zap.Int("envelope_count", len(batch)))
					w.processBatch(ctx, batch)
				}
				return
			}

			batch = append(batch, envelope)

			if len(batch) >= w.config.MaxBatchSize {
				w.log.Info("Batch size threshold reached", zap.Int("batch_size", len(batch)))
				w.processBatch(ctx, batch)
				batch = make([]*Envelope, 0, w.config.MaxBatchSize)
				ticker.Reset(w.config.FlushTimeout)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				w.log.Info("Batch timeout reached", zap.Int("envelope_count", len(batch)))
				w.processBatch(ctx, batch)
				batch = make([]*Envelope, 0, w.config.MaxBatchSize)
			}
		}
	}
}

// processBatch handles the atomic transaction: insert + ack/nack
func (w *BatchWriter) processBatch(ctx context.Context, envelopes []*Envelope) {
	if len(envelopes) == 0 {
		return
	}

	events := make([]*domain.Event, len(envelopes))
	for i, env := range envelopes {
		events[i] = env.Event
	}

	insertedCount, err := w.repository.InsertBatch(ctx, events)

	if err != nil {
		w.log.Error("Failed to insert batch",
			zap.Error(err),
			zap.Int("event_count", len(events)))
		w.nackAll(ctx, envelopes)
		return
	}

	if insertedCount != len(events) {
		w.log.Warn("Partial insert success",
			zap.Int("inserted", insertedCount),
			zap.Int("expected", len(events)))
		w.nackAll(ctx, envelopes)
		return
	}

	w.log.Info("Successfully inserted events",
		zap.Int("count", insertedCount))
	w.ackAll(ctx, envelopes)
}

// ackAll acknowledges all envelopes (deletes from SQS)
func (w *BatchWriter) ackAll(ctx context.Context, envelopes []*Envelope) {
	for _, env := range envelopes {
		if err := env.Ack(ctx); err != nil {
			w.log.Error("Failed to ack envelope", zap.Error(err))
		}
	}
}

// nackAll negatively acknowledges all envelopes (leaves in SQS for retry)
func (w *BatchWriter) nackAll(ctx context.Context, envelopes []*Envelope) {
	for _, env := range envelopes {
		if err := env.Nack(ctx); err != nil {
			w.log.Error("Failed to nack envelope", zap.Error(err))
		}
	}
}

// AckBatch is a helper for batch acknowledgment to SQS
func (w *BatchWriter) AckBatch(ctx context.Context, envelopes []*Envelope) error {
	if len(envelopes) == 0 {
		return nil
	}

	var lastErr error
	for _, env := range envelopes {
		if err := env.Ack(ctx); err != nil {
			w.log.Error("Failed to ack envelope", zap.Error(err))
			lastErr = err
		}
	}

	if lastErr != nil {
		return fmt.Errorf("some acknowledgments failed: %w", lastErr)
	}

	return nil
}
