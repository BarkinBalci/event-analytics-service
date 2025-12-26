package consumer

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.uber.org/zap"

	"github.com/BarkinBalci/event-analytics-service/internal/queue"
)

// ParserStage handles parsing SQS messages into domain envelopes
type ParserStage struct {
	consumer queue.QueueConsumer
	parser   MessageParser
	log      *zap.Logger
}

// NewParserStage creates a new parser stage
func NewParserStage(consumer queue.QueueConsumer, parser MessageParser, log *zap.Logger) *ParserStage {
	return &ParserStage{
		consumer: consumer,
		parser:   parser,
		log:      log,
	}
}

// Start begins parsing messages and outputs envelopes
func (p *ParserStage) Start(ctx context.Context, in <-chan types.Message, out chan<- *Envelope) {
	defer close(out)

	for {
		select {
		case <-ctx.Done():
			p.log.Info("Parser stage shutting down")
			return
		case msg, ok := <-in:
			if !ok {
				p.log.Info("Parser stage input channel closed")
				return
			}

			envelope := p.parseMessage(ctx, msg)
			if envelope == nil {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case out <- envelope:
				// Envelope sent to next stage
			}
		}
	}
}

// parseMessage parses a single SQS message into an envelope
func (p *ParserStage) parseMessage(ctx context.Context, msg types.Message) *Envelope {
	body := aws.ToString(msg.Body)
	event, err := p.parser.Parse([]byte(body))

	if err != nil {
		p.log.Warn("Failed to parse message",
			zap.String("message_id", aws.ToString(msg.MessageId)),
			zap.Error(err))
		if err := p.deleteMessage(ctx, msg); err != nil {
			p.log.Error("Failed to delete malformed message",
				zap.String("message_id", aws.ToString(msg.MessageId)),
				zap.Error(err))
		}
		return nil
	}

	ack := func(ctx context.Context) error {
		return p.deleteMessage(ctx, msg)
	}

	nack := func(ctx context.Context) error {
		// TODO: While the messages would become visible on their own maybe something could be done to reduce the time in here.
		return nil
	}

	return NewEnvelope(event, ack, nack)
}

// deleteMessage deletes a message from SQS
func (p *ParserStage) deleteMessage(ctx context.Context, msg types.Message) error {
	_, err := p.consumer.DeleteMessage(ctx, &awssqs.DeleteMessageInput{
		QueueUrl:      aws.String(p.consumer.QueueURL()),
		ReceiptHandle: msg.ReceiptHandle,
	})
	if err != nil {
		p.log.Error("Failed to delete message",
			zap.String("message_id", aws.ToString(msg.MessageId)),
			zap.Error(err))
		return err
	}
	p.log.Info("Deleted malformed message from SQS",
		zap.String("message_id", aws.ToString(msg.MessageId)))
	return nil
}
