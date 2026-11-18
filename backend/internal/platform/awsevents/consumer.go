package awsevents

import (
	"context"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
)

// ConsumerConfig sets the pace of a Consumer. A zero field takes its default.
type ConsumerConfig struct {
	// WaitSeconds is how long one request waits for messages when the queue
	// is empty (long polling), up to 20 seconds. Default: 20.
	WaitSeconds int32
	// HandleTimeout bounds the handling of one event. Keep it below the
	// visibility timeout of the queue, or the event comes again while it is
	// still being handled. Default: 25 seconds.
	HandleTimeout time.Duration
	// RetryDelay is the pause after a request to SQS failed. Default: one
	// second.
	RetryDelay time.Duration
}

// The defaults of ConsumerConfig, and the largest number of messages that SQS
// gives in one request.
const (
	DefaultWaitSeconds   = 20
	DefaultHandleTimeout = 25 * time.Second
	DefaultRetryDelay    = time.Second
	maxMessages          = 10
)

func (c ConsumerConfig) withDefaults() ConsumerConfig {
	if c.WaitSeconds <= 0 || c.WaitSeconds > DefaultWaitSeconds {
		c.WaitSeconds = DefaultWaitSeconds
	}
	if c.HandleTimeout <= 0 {
		c.HandleTimeout = DefaultHandleTimeout
	}
	if c.RetryDelay <= 0 {
		c.RetryDelay = DefaultRetryDelay
	}
	return c
}

// Consumer reads the events of one SQS queue, the queue of one consuming
// context, and hands each to a handler.
//
// A message is deleted only after the handler returned nil. An event whose
// handler fails, and a message that is no event, stay in the queue: SQS shows
// them again after the visibility timeout, and moves them to the dead-letter
// queue after the number of receives that the queue allows. So one bad
// message does not hold up the rest, and nothing is lost.
type Consumer struct {
	client   *sqs.Client
	queueURL string
	logger   *slog.Logger
	config   ConsumerConfig
}

// NewConsumer returns the consumer of the queue with the URL.
func NewConsumer(cfg aws.Config, queueURL string, logger *slog.Logger, config ConsumerConfig) *Consumer {
	return &Consumer{client: sqs.NewFromConfig(cfg), queueURL: queueURL, logger: logger, config: config.withDefaults()}
}

// Run receives events and calls handle for each, one at a time, until ctx is
// canceled, and then returns nil. The event in hand is finished first: its
// handler gets a context that the cancellation does not reach. A failure of
// SQS is logged, and the consumer tries again after a pause.
func (c *Consumer) Run(ctx context.Context, handle event.Handler) error {
	c.logger.InfoContext(ctx, "the consumer of the queue runs", slog.String("queue", c.queueURL))
	for ctx.Err() == nil {
		for _, message := range c.receive(ctx) {
			if ctx.Err() != nil {
				// The rest of the batch becomes visible again by itself.
				break
			}
			c.handle(ctx, message, handle)
		}
	}
	c.logger.InfoContext(context.WithoutCancel(ctx), "the consumer of the queue stops")
	return nil
}

// receive waits for messages. When SQS fails, it logs the failure, pauses,
// and returns none.
func (c *Consumer) receive(ctx context.Context) []types.Message {
	out, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(c.queueURL),
		MaxNumberOfMessages: maxMessages,
		WaitTimeSeconds:     c.config.WaitSeconds,
	})
	if err == nil {
		return out.Messages
	}
	if ctx.Err() == nil {
		c.logger.ErrorContext(ctx, "the consumer could not receive from the queue", slog.Any("error", err))
		select {
		case <-ctx.Done():
		case <-time.After(c.config.RetryDelay):
		}
	}
	return nil
}

// handle hands one message to the handler, and deletes it when the handler
// took it.
func (c *Consumer) handle(ctx context.Context, message types.Message, handle event.Handler) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.config.HandleTimeout)
	defer cancel()
	messageID := slog.String("message_id", aws.ToString(message.MessageId))

	e, err := event.Parse([]byte(aws.ToString(message.Body)))
	if err != nil {
		c.logger.ErrorContext(ctx, "a message of the queue is no event; it stays for the dead-letter queue",
			messageID, slog.Any("error", err))
		return
	}
	attrs := []any{messageID, slog.String("event_id", e.ID.String()), slog.String("event_type", e.Type)}
	if err := handle(ctx, e); err != nil {
		c.logger.WarnContext(ctx, "an event was not handled and comes again", append(attrs, slog.Any("error", err))...)
		return
	}
	if _, err := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: message.ReceiptHandle,
	}); err != nil {
		// The handler is idempotent, so the second delivery does no harm.
		c.logger.WarnContext(ctx, "a handled event could not be deleted from the queue and comes again",
			append(attrs, slog.Any("error", err))...)
		return
	}
	c.logger.InfoContext(ctx, "an event was handled", attrs...)
}
