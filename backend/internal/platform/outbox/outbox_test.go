package outbox_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/outbox"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/outbox/outboxtest"
)

func events(t *testing.T, n int) []event.Envelope {
	t.Helper()
	made := make([]event.Envelope, n)
	for i := range made {
		e, err := event.New(uuid.Must(uuid.NewV7()), event.Source("reservation"),
			event.Type("reservation", "ReservationConfirmed", 1), "", time.Now(), map[string]int{"n": i})
		if err != nil {
			t.Fatal(err)
		}
		made[i] = e
	}
	return made
}

func ids(events []event.Envelope) []uuid.UUID {
	out := make([]uuid.UUID, len(events))
	for i, e := range events {
		out[i] = e.ID
	}
	return out
}

func relay(source outbox.Source, publisher outbox.Publisher, log *bytes.Buffer, now func() time.Time, config outbox.Config) *outbox.Relay {
	return outbox.NewRelay(source, publisher, slog.New(slog.NewJSONHandler(log, nil)), now, config)
}

func TestFlushPublishesEveryWaitingEventInOrder(t *testing.T) {
	source, publisher := &outboxtest.Source{}, &outboxtest.Publisher{}
	waiting := events(t, 7)
	source.Add(waiting...)
	var log bytes.Buffer
	// A batch smaller than the outbox, so that Flush needs several.
	n, err := relay(source, publisher, &log, time.Now, outbox.Config{Batch: 3}).Flush(t.Context())
	if err != nil || n != 7 {
		t.Fatalf("Flush = %d, %v, want 7, nil", n, err)
	}
	if got := ids(publisher.Published()); !slices.Equal(got, ids(waiting)) {
		t.Errorf("published %v, want %v", got, ids(waiting))
	}
	if pending := source.Pending(); len(pending) > 0 {
		t.Errorf("%d events still wait", len(pending))
	}
	if n, err := relay(source, publisher, &log, time.Now, outbox.Config{}).Flush(t.Context()); err != nil || n != 0 {
		t.Errorf("a second Flush = %d, %v, want 0, nil: an event must not be published twice", n, err)
	}
}

func TestFlushLeavesARefusedEventInTheOutbox(t *testing.T) {
	source, publisher := &outboxtest.Source{}, &outboxtest.Publisher{}
	waiting := events(t, 3)
	source.Add(waiting...)
	publisher.Refuse = func(e event.Envelope) bool { return e.ID == waiting[1].ID }
	var log bytes.Buffer
	r := relay(source, publisher, &log, time.Now, outbox.Config{})

	n, err := r.Flush(t.Context())
	if n != 2 || !errors.Is(err, outboxtest.ErrRefused) {
		t.Fatalf("Flush = %d, %v, want 2 and the error of the broker", n, err)
	}
	if pending := source.Pending(); !slices.Equal(pending, []uuid.UUID{waiting[1].ID}) {
		t.Errorf("pending = %v, want the refused event alone", pending)
	}
	if !strings.Contains(log.String(), waiting[1].ID.String()) {
		t.Errorf("the log does not name the refused event: %s", log.String())
	}

	// The broker takes it later: the event is published then, once.
	publisher.Refuse = nil
	if n, err := r.Flush(t.Context()); err != nil || n != 1 {
		t.Fatalf("the next Flush = %d, %v, want 1, nil", n, err)
	}
	want := []uuid.UUID{waiting[0].ID, waiting[2].ID, waiting[1].ID}
	if got := ids(publisher.Published()); !slices.Equal(got, want) {
		t.Errorf("published %v, want %v", got, want)
	}
}

func TestFlushStopsWhenABatchIsFullOfRefusedEvents(t *testing.T) {
	source, publisher := &outboxtest.Source{}, &outboxtest.Publisher{}
	source.Add(events(t, 4)...)
	publisher.Refuse = func(event.Envelope) bool { return true }
	var log bytes.Buffer
	n, err := relay(source, publisher, &log, time.Now, outbox.Config{Batch: 2}).Flush(t.Context())
	if n != 0 || err == nil {
		t.Fatalf("Flush = %d, %v, want 0 and an error", n, err)
	}
	if len(source.Pending()) != 4 {
		t.Errorf("pending = %d, want 4", len(source.Pending()))
	}
}

func TestFlushReturnsTheErrorOfTheOutbox(t *testing.T) {
	away := errors.New("the database is away")
	source, publisher := &outboxtest.Source{Err: away}, &outboxtest.Publisher{}
	var log bytes.Buffer
	if _, err := relay(source, publisher, &log, time.Now, outbox.Config{}).Flush(t.Context()); !errors.Is(err, away) {
		t.Errorf("Flush = %v, want the error of the outbox", err)
	}
}

func TestRunPublishesWhatArrivesAndRemovesOldEvents(t *testing.T) {
	now := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	source, publisher := &outboxtest.Source{}, &outboxtest.Publisher{}
	// One event was published eight days ago, one yesterday.
	old := events(t, 2)
	source.Now = func() time.Time { return now.Add(-8 * 24 * time.Hour) }
	source.Add(old[0])
	var log bytes.Buffer
	if _, err := relay(source, publisher, &log, time.Now, outbox.Config{}).Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	source.Now = func() time.Time { return now.Add(-24 * time.Hour) }
	source.Add(old[1])
	if _, err := relay(source, publisher, &log, time.Now, outbox.Config{}).Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	source.Now = func() time.Time { return now }

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	r := relay(source, publisher, &log, func() time.Time { return now }, outbox.Config{Interval: time.Millisecond})
	go func() { done <- r.Run(ctx) }()

	arriving := events(t, 2)
	source.Add(arriving...)
	// The relay publishes the two events, and removes the event of eight days
	// ago; the three younger ones stay. It removes old events after it
	// published what waited, so the test waits for both before it stops the
	// relay: a relay that is stopped right after publishing removes nothing.
	deadline := time.After(5 * time.Second)
	for len(publisher.Published()) < 4 || source.Len() != 3 {
		select {
		case <-deadline:
			t.Fatalf("the relay published %d events, want 4, and the outbox holds %d, want 3",
				len(publisher.Published()), source.Len())
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("Run = %v, want nil after the cancellation", err)
	}
	if source.Len() != 3 {
		t.Errorf("the outbox holds %d events after the relay stopped, want 3", source.Len())
	}
}

func TestRunKeepsGoingWhenTheOutboxFails(t *testing.T) {
	source, publisher := &outboxtest.Source{Err: errors.New("the database is away")}, &outboxtest.Publisher{}
	var log bytes.Buffer
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := relay(source, publisher, &log, time.Now, outbox.Config{Interval: time.Millisecond}).Run(ctx); err != nil {
		t.Errorf("Run = %v, want nil", err)
	}
	if !strings.Contains(log.String(), "the database is away") {
		t.Errorf("the failure was not logged: %s", log.String())
	}
}
