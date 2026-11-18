//go:build integration

// These tests run the adapters against AWS Emulator (`make aws-server`). They
// are also the tests of the emulator itself (TPL-0017): they fail when a
// version of it stops doing what the template relies on, which is that a rule
// of EventBridge delivers to an SQS queue, and that SQS moves a message that
// keeps failing to the dead-letter queue.
package awsevents_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/awsconfig/awstest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/awsevents"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
)

const (
	source        = "/contexts/reservation"
	confirmedType = "reservation.ReservationConfirmed.v1"
	cancelledType = "reservation.ReservationCancelled.v1"
	// maxReceives is how often the test queue shows a message before it
	// moves the message to the dead-letter queue.
	maxReceives = 2
)

// subscription is what the infrastructure of a consuming context creates: its
// queue with a dead-letter queue, and a rule on the bus that selects the
// events of one source and type into the queue.
type subscription struct {
	bus      string
	queueURL string
	deadURL  string
}

// subscribe creates a bus and the subscription of one consumer to the
// confirmed reservations, under names that no other test uses, and removes
// them when the test ends.
func subscribe(t *testing.T, cfg aws.Config) subscription {
	t.Helper()
	ctx := t.Context()
	name := "test-" + uuid.NewString()
	bus := eventbridge.NewFromConfig(cfg)
	queues := sqs.NewFromConfig(cfg)

	arnOf := func(queueURL *string) string {
		out, err := queues.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       queueURL,
			AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
		})
		if err != nil {
			t.Fatal(err)
		}
		return out.Attributes[string(sqstypes.QueueAttributeNameQueueArn)]
	}

	dead, err := queues.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String(name + "-dlq")})
	if err != nil {
		t.Fatalf("cannot create a queue in AWS Emulator; run `make aws-server` first: %v", err)
	}
	redrive, err := json.Marshal(map[string]any{"deadLetterTargetArn": arnOf(dead.QueueUrl), "maxReceiveCount": maxReceives})
	if err != nil {
		t.Fatal(err)
	}
	queue, err := queues.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(name),
		Attributes: map[string]string{
			string(sqstypes.QueueAttributeNameRedrivePolicy): string(redrive),
			// A failed message comes again after one second, not thirty.
			string(sqstypes.QueueAttributeNameVisibilityTimeout): "1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	queueARN := arnOf(queue.QueueUrl)

	if _, err := bus.CreateEventBus(ctx, &eventbridge.CreateEventBusInput{Name: aws.String(name)}); err != nil {
		t.Fatal(err)
	}
	pattern, err := json.Marshal(map[string][]string{"source": {source}, "detail-type": {confirmedType}})
	if err != nil {
		t.Fatal(err)
	}
	rule, err := bus.PutRule(ctx, &eventbridge.PutRuleInput{
		Name: aws.String(name), EventBusName: aws.String(name), EventPattern: aws.String(string(pattern)),
	})
	if err != nil {
		t.Fatal(err)
	}
	// EventBridge may send to the queue only when the policy of the queue
	// says so, in AWS and in the emulator alike.
	policy, err := json.Marshal(map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{{
			"Effect":    "Allow",
			"Principal": map[string]string{"Service": "events.amazonaws.com"},
			"Action":    "sqs:SendMessage",
			"Resource":  queueARN,
			"Condition": map[string]any{"ArnEquals": map[string]string{"aws:SourceArn": aws.ToString(rule.RuleArn)}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queues.SetQueueAttributes(ctx, &sqs.SetQueueAttributesInput{
		QueueUrl:   queue.QueueUrl,
		Attributes: map[string]string{string(sqstypes.QueueAttributeNamePolicy): string(policy)},
	}); err != nil {
		t.Fatal(err)
	}
	targets, err := bus.PutTargets(ctx, &eventbridge.PutTargetsInput{
		Rule: aws.String(name), EventBusName: aws.String(name),
		Targets: []ebtypes.Target{{Id: aws.String("queue"), Arn: aws.String(queueARN), InputPath: aws.String(awsevents.DetailInputPath)}},
	})
	if err != nil || targets.FailedEntryCount > 0 {
		t.Fatalf("PutTargets = %+v, %v", targets, err)
	}

	t.Cleanup(func() {
		// A fresh context: the context of the test is canceled before Cleanup
		// runs. Failures are of no interest: the emulator forgets everything
		// when it stops.
		ctx := context.Background()
		_, _ = bus.RemoveTargets(ctx, &eventbridge.RemoveTargetsInput{Rule: aws.String(name), EventBusName: aws.String(name), Ids: []string{"queue"}})
		_, _ = bus.DeleteRule(ctx, &eventbridge.DeleteRuleInput{Name: aws.String(name), EventBusName: aws.String(name)})
		_, _ = bus.DeleteEventBus(ctx, &eventbridge.DeleteEventBusInput{Name: aws.String(name)})
		_, _ = queues.DeleteQueue(ctx, &sqs.DeleteQueueInput{QueueUrl: queue.QueueUrl})
		_, _ = queues.DeleteQueue(ctx, &sqs.DeleteQueueInput{QueueUrl: dead.QueueUrl})
	})
	return subscription{bus: name, queueURL: aws.ToString(queue.QueueUrl), deadURL: aws.ToString(dead.QueueUrl)}
}

func envelope(t *testing.T, eventType string) event.Envelope {
	t.Helper()
	e, err := event.New(uuid.Must(uuid.NewV7()), source, eventType, uuid.NewString(),
		time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC), map[string]any{"attendees": 4, "organizerEmail": "organizer@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// consume runs a consumer of the queue with the handler until the test ends.
func consume(t *testing.T, cfg aws.Config, queueURL string, handle event.Handler) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	consumer := awsevents.NewConsumer(cfg, queueURL, slog.New(slog.DiscardHandler), awsevents.ConsumerConfig{WaitSeconds: 1})
	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx, handle) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Consumer.Run = %v, want nil after the cancellation", err)
		}
	})
}

// eventually waits until check holds.
func eventually(t *testing.T, what string, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !check() {
		if time.Now().After(deadline) {
			t.Fatalf("after 30 seconds, still not: %s", what)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// messages returns how many messages the queue holds, visible or in flight.
func messages(t *testing.T, cfg aws.Config, queueURL string) int {
	t.Helper()
	out, err := sqs.NewFromConfig(cfg).GetQueueAttributes(t.Context(), &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(queueURL),
		AttributeNames: []sqstypes.QueueAttributeName{
			sqstypes.QueueAttributeNameApproximateNumberOfMessages,
			sqstypes.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, value := range out.Attributes {
		var n int
		if _, err := fmt.Sscan(value, &n); err != nil {
			t.Fatal(err)
		}
		total += n
	}
	return total
}

// receiveOne returns the body of the one message that the queue shows.
func receiveOne(t *testing.T, cfg aws.Config, queueURL string) string {
	t.Helper()
	var body string
	eventually(t, "a message in the queue "+queueURL, func() bool {
		out, err := sqs.NewFromConfig(cfg).ReceiveMessage(t.Context(), &sqs.ReceiveMessageInput{
			QueueUrl: aws.String(queueURL), MaxNumberOfMessages: 10, WaitTimeSeconds: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Messages) > 1 {
			t.Fatalf("the queue shows %d messages, want one", len(out.Messages))
		}
		if len(out.Messages) == 1 {
			body = aws.ToString(out.Messages[0].Body)
		}
		return body != ""
	})
	return body
}

func TestAPublishedEventReachesTheHandlerOfASubscriber(t *testing.T) {
	cfg := awstest.Config(t)
	sub := subscribe(t, cfg)
	publisher := awsevents.NewPublisher(cfg, sub.bus)

	// The rule selects the confirmed reservations: the other event must not
	// arrive.
	other, wanted := envelope(t, cancelledType), envelope(t, confirmedType)
	for _, e := range []event.Envelope{other, wanted} {
		if err := publisher.Publish(t.Context(), e); err != nil {
			t.Fatal(err)
		}
	}

	var mu sync.Mutex
	var handled []event.Envelope
	consume(t, cfg, sub.queueURL, func(_ context.Context, e event.Envelope) error {
		mu.Lock()
		defer mu.Unlock()
		handled = append(handled, e)
		return nil
	})
	eventually(t, "the handler got the event", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(handled) > 0
	})
	// A handled event is deleted, so it does not come a second time.
	eventually(t, "the queue is empty", func() bool { return messages(t, cfg, sub.queueURL) == 0 })
	time.Sleep(1500 * time.Millisecond) // longer than the visibility timeout

	mu.Lock()
	defer mu.Unlock()
	if len(handled) != 1 {
		t.Fatalf("the handler got %d events, want the confirmed reservation alone: %+v", len(handled), handled)
	}
	got := handled[0]
	if got.ID != wanted.ID || got.Source != wanted.Source || got.Type != wanted.Type || got.Subject != wanted.Subject ||
		!got.Time.Equal(wanted.Time) || got.SpecVersion != wanted.SpecVersion || got.DataContentType != wanted.DataContentType {
		t.Errorf("the handler got %+v, want %+v", got, wanted)
	}
	var gotData, wantData map[string]any
	if err := json.Unmarshal(got.Data, &gotData); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wanted.Data, &wantData); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(gotData) != fmt.Sprint(wantData) {
		t.Errorf("the data is %v, want %v", gotData, wantData)
	}
	if n := messages(t, cfg, sub.deadURL); n != 0 {
		t.Errorf("the dead-letter queue holds %d messages, want none", n)
	}
}

func TestAnEventWhoseHandlerKeepsFailingMovesToTheDeadLetterQueue(t *testing.T) {
	cfg := awstest.Config(t)
	sub := subscribe(t, cfg)
	sent := envelope(t, confirmedType)
	if err := awsevents.NewPublisher(cfg, sub.bus).Publish(t.Context(), sent); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	attempts := 0
	consume(t, cfg, sub.queueURL, func(context.Context, event.Envelope) error {
		mu.Lock()
		defer mu.Unlock()
		attempts++
		return errors.New("the mail server is away")
	})

	set := receiveOne(t, cfg, sub.deadURL)
	e, err := event.Parse([]byte(set))
	if err != nil || e.ID != sent.ID {
		t.Errorf("the dead-letter queue holds %q (%v), want the event %s", set, err, sent.ID)
	}
	mu.Lock()
	defer mu.Unlock()
	if attempts != maxReceives {
		t.Errorf("the handler was called %d times, want %d before the queue gives up", attempts, maxReceives)
	}
}

func TestAMessageThatIsNoEventMovesToTheDeadLetterQueue(t *testing.T) {
	cfg := awstest.Config(t)
	sub := subscribe(t, cfg)
	if _, err := sqs.NewFromConfig(cfg).SendMessage(t.Context(), &sqs.SendMessageInput{
		QueueUrl: aws.String(sub.queueURL), MessageBody: aws.String("not an event"),
	}); err != nil {
		t.Fatal(err)
	}
	// An event behind it is still handled.
	sent := envelope(t, confirmedType)
	if err := awsevents.NewPublisher(cfg, sub.bus).Publish(t.Context(), sent); err != nil {
		t.Fatal(err)
	}

	handled := make(chan event.Envelope, 10)
	consume(t, cfg, sub.queueURL, func(_ context.Context, e event.Envelope) error {
		handled <- e
		return nil
	})

	select {
	case e := <-handled:
		if e.ID != sent.ID {
			t.Errorf("the handler got the event %s, want %s", e.ID, sent.ID)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the event behind the bad message was not handled")
	}
	if set := receiveOne(t, cfg, sub.deadURL); set != "not an event" {
		t.Errorf("the dead-letter queue holds %q, want the bad message", set)
	}
	if len(handled) != 0 {
		t.Errorf("the handler was called for the bad message")
	}
}

func TestPublishFailsWhenEventBridgeRefusesTheEntry(t *testing.T) {
	cfg := awstest.Config(t)
	sub := subscribe(t, cfg)
	// EventBridge answers 200 and refuses the entry: only AWS itself may
	// publish under a source that starts with "aws.".
	refused := envelope(t, confirmedType)
	refused.Source = "aws.reservation"
	err := awsevents.NewPublisher(cfg, sub.bus).Publish(t.Context(), refused)
	if err == nil || !strings.Contains(err.Error(), "EventBridge refused it") {
		t.Errorf("Publish = %v, want the refusal of EventBridge: the relay would mark the event as published", err)
	}
}

func TestReadyTellsWhetherTheBusExists(t *testing.T) {
	cfg := awstest.Config(t)
	sub := subscribe(t, cfg)
	if err := awsevents.NewPublisher(cfg, sub.bus).Ready(t.Context()); err != nil {
		t.Errorf("Ready = %v for a bus that exists", err)
	}
	if err := awsevents.NewPublisher(cfg, "no-such-bus-"+uuid.NewString()).Ready(t.Context()); err == nil {
		t.Error("Ready = nil for a bus that does not exist")
	}
}
