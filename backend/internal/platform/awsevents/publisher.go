// Package awsevents carries integration events over AWS (TPL-0013): a
// Publisher that hands the events of an outbox to an EventBridge event bus,
// and a Consumer that reads the SQS queue of a consuming context and calls
// its handler. The contexts see neither: they write to their outbox and
// implement event.Handler.
//
// On the bus, an event is an EventBridge entry whose `source` and
// `detail-type` are the `source` and the `type` of the CloudEvents envelope,
// and whose `detail` is the envelope itself. A rule of the consuming context
// selects events by those two fields and sends the `detail` alone to the
// queue (the input path `$.detail`), so the body of a message is the
// envelope as the publisher wrote it.
package awsevents

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/outbox"
)

// DetailInputPath is the input path that the target of a rule sets, so that
// the queue receives the envelope without the wrapping of EventBridge.
const DetailInputPath = "$.detail"

// Publisher publishes events to one EventBridge event bus.
type Publisher struct {
	client *eventbridge.Client
	bus    string
}

var _ outbox.Publisher = (*Publisher)(nil)

// NewPublisher returns the publisher to the event bus with the name or ARN.
func NewPublisher(cfg aws.Config, bus string) *Publisher {
	return &Publisher{client: eventbridge.NewFromConfig(cfg), bus: bus}
}

// Ready reports whether the bus exists. The process that runs a relay calls
// it once before the relay starts: an event for a bus that does not exist is
// not refused everywhere (AWS Emulator accepts it and drops it), and the relay
// would mark it as published.
func (p *Publisher) Ready(ctx context.Context) error {
	if _, err := p.client.DescribeEventBus(ctx, &eventbridge.DescribeEventBusInput{Name: aws.String(p.bus)}); err != nil {
		return fmt.Errorf("the event bus %s: %w", p.bus, err)
	}
	return nil
}

// Publish hands the event to the bus. It returns nil only when EventBridge
// has accepted the entry. One call carries one event: the relay marks each
// event by the answer for it, and the outbox of the sample is far from the
// volume where batches of ten would matter.
func (p *Publisher) Publish(ctx context.Context, e event.Envelope) error {
	detail, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("publish event %s: %w", e.ID, err)
	}
	out, err := p.client.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{{
			EventBusName: aws.String(p.bus),
			Source:       aws.String(e.Source),
			DetailType:   aws.String(e.Type),
			Detail:       aws.String(string(detail)),
			Time:         aws.Time(e.Time),
		}},
	})
	if err != nil {
		return fmt.Errorf("publish event %s to the bus %s: %w", e.ID, p.bus, err)
	}
	// EventBridge answers 200 also when it refused an entry.
	if out.FailedEntryCount > 0 || len(out.Entries) != 1 {
		code, message := "unknown", "EventBridge gave no reason"
		if len(out.Entries) == 1 {
			code, message = aws.ToString(out.Entries[0].ErrorCode), aws.ToString(out.Entries[0].ErrorMessage)
		}
		return fmt.Errorf("publish event %s to the bus %s: EventBridge refused it: %s: %s", e.ID, p.bus, code, message)
	}
	return nil
}
