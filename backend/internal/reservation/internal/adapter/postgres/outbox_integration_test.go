//go:build integration

package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/event/eventtest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/outbox"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/outbox/outboxtest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/adapter/postgres"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app/apptest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// The outbox is one queue for the whole database, so the tests of this file
// do not run in parallel: none of them runs while the tests of the store
// look at their events. The database may still hold events of others, such
// as those of a service that shares it, or of a test that runs in another
// copy of the repository. A test hands only its own events to the relay, and
// refuses the others (errNotMine), which leaves them in the outbox as they
// were.

// batch is larger than the events of others that the outbox may hold, so
// that one Drain reaches the events of the test.
const batch = 1000

// errNotMine is what a test answers for an event of others.
var errNotMine = errors.New("the event is not one of this test")

// newOutbox returns the outbox over pool, which marks events at the instant
// that now returns, and logs to the returned buffer.
func newOutbox(pool *pgxpool.Pool, now func() time.Time) (*postgres.Outbox, *syncBuffer) {
	log := &syncBuffer{}
	return postgres.NewOutbox(pool, now, slog.New(slog.NewJSONHandler(log, nil))), log
}

// syncBuffer is a log that two goroutines can write.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// mine reports whether an event is about one of the reservations.
func mine(rs ...domain.Reservation) func(event.Envelope) bool {
	subjects := map[string]bool{}
	for _, r := range rs {
		subjects[r.ID().String()] = true
	}
	return func(e event.Envelope) bool { return subjects[e.Subject] }
}

// drained is what one Drain handed to publish.
type drained struct {
	// all holds the ids of every event that Drain handed to publish, in
	// order, those of others included.
	all []uuid.UUID
	// published holds the events of the test that publish accepted.
	published []event.Envelope
}

// drain runs one Drain that publishes the events that accept selects, and
// refuses the others.
func drain(ctx context.Context, t *testing.T, o *postgres.Outbox, limit int, accept func(event.Envelope) bool) (drained, int) {
	t.Helper()
	d, n, err := tryDrain(ctx, o, limit, accept)
	if err != nil {
		t.Fatal(err)
	}
	return d, n
}

// tryDrain is drain, which returns the error of Drain.
func tryDrain(ctx context.Context, o *postgres.Outbox, limit int, accept func(event.Envelope) bool) (drained, int, error) {
	var d drained
	n, err := o.Drain(ctx, limit, func(_ context.Context, e event.Envelope) error {
		d.all = append(d.all, e.ID)
		if !accept(e) {
			return errNotMine
		}
		d.published = append(d.published, e)
		return nil
	})
	return d, n, err
}

// idsOf returns the ids of the events.
func idsOf(events []event.Envelope) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(events))
	for _, e := range events {
		out = append(out, e.ID)
	}
	return out
}

// eventIDs returns the ids of the events about the reservations that the
// outbox holds, each reservation after the other.
func eventIDs(t *testing.T, pool *pgxpool.Pool, rs ...domain.Reservation) []uuid.UUID {
	t.Helper()
	var out []uuid.UUID
	for _, r := range rs {
		for _, e := range storedEvents(t, pool, r.ID()) {
			out = append(out, e.ID)
		}
	}
	return out
}

// isPublished reports whether the outbox marks the event of the reservation
// as published. The reservation has one event.
func isPublished(t *testing.T, pool *pgxpool.Pool, r domain.Reservation) bool {
	t.Helper()
	events := storedEvents(t, pool, r.ID())
	if len(events) != 1 {
		t.Fatalf("the outbox holds %d events about the reservation %s, want 1", len(events), r.ID())
	}
	return events[0].published
}

// ascending reports whether each id is greater than the one before it.
func ascending(ids []uuid.UUID) bool {
	return slices.IsSortedFunc(ids, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) }) &&
		len(slices.Compact(slices.Clone(ids))) == len(ids)
}

func TestDrainPublishesTheOldestEventsFirst(t *testing.T) {
	store, room, pool := newStore(t)
	o, _ := newOutbox(pool, time.Now)
	ctx := t.Context()
	first := add(t, store, room, hours(9, 10))
	second := add(t, store, room, hours(14, 15))
	third := add(t, store, room, hours(11, 12))
	all := mine(first, second, third)
	want := eventIDs(t, pool, first, second, third)
	if !ascending(want) {
		t.Fatalf("the ids of the events %v do not follow the order of the changes", want)
	}

	// Drain takes no more than its limit. Every event is refused here, so
	// none is marked.
	d, n := drain(ctx, t, o, 1, func(event.Envelope) bool { return false })
	if len(d.all) != 1 || n != 0 {
		t.Errorf("a Drain of 1 event handed %d events to publish and published %d, want 1 and 0", len(d.all), n)
	}

	// The second event is refused: it stays, and the others are marked.
	d, n = drain(ctx, t, o, batch, func(e event.Envelope) bool { return all(e) && e.Subject != second.ID().String() })
	if !ascending(d.all) {
		t.Errorf("Drain handed the events %v to publish, want them in the order of their ids", d.all)
	}
	if got, wantPublished := idsOf(d.published), []uuid.UUID{want[0], want[2]}; !slices.Equal(got, wantPublished) {
		t.Errorf("Drain published %v, want the first and the third events %v", got, wantPublished)
	}
	if n != 2 {
		t.Errorf("Drain returned %d, want the 2 events that it published", n)
	}
	for _, e := range d.published {
		eventtest.AssertMatchesContract(t, e)
	}
	if !isPublished(t, pool, first) || isPublished(t, pool, second) || !isPublished(t, pool, third) {
		t.Error("the outbox does not mark exactly the first and the third events as published")
	}

	// The refused event comes again, and nothing else of the test.
	d, n = drain(ctx, t, o, batch, all)
	if got := idsOf(d.published); !slices.Equal(got, []uuid.UUID{want[1]}) || n != 1 {
		t.Errorf("the next Drain published %v (%d), want only the refused event %s", got, n, want[1])
	}
	d, n = drain(ctx, t, o, batch, all)
	if len(d.published) != 0 || n != 0 {
		t.Errorf("a Drain after every event was published published %v (%d), want nothing", idsOf(d.published), n)
	}
	if !isPublished(t, pool, second) {
		t.Error("the outbox does not mark the second event as published")
	}
}

// Two relays drain the outbox at the same time: the first one holds its
// events inside publish while the second one runs. The second one does not
// wait for the first, and takes none of its events, so no event is published
// twice.
func TestOverlappingDrainsPublishEachEventOnce(t *testing.T) {
	store, room, pool := newStore(t)
	o, _ := newOutbox(pool, time.Now)
	var rs []domain.Reservation
	for h := 9; h < 13; h++ {
		rs = append(rs, add(t, store, room, hours(h, h+1)))
	}
	all := mine(rs...)

	held, release := make(chan struct{}), make(chan struct{})
	var first drained
	done := make(chan error, 1)
	go func() {
		var hold sync.Once
		_, err := o.Drain(t.Context(), 2, func(_ context.Context, e event.Envelope) error {
			hold.Do(func() {
				close(held)
				<-release
			})
			first.all = append(first.all, e.ID)
			if !all(e) {
				return errNotMine
			}
			first.published = append(first.published, e)
			return nil
		})
		done <- err
	}()
	select {
	case <-held:
	case err := <-done:
		t.Fatalf("the first relay returned before it took an event: %v", err)
	}
	// A second relay that waited for the locks of the first one would wait
	// forever here, because the first one goes on only after the second one
	// returns; the deadline turns that into a failure.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	second, _, secondErr := tryDrain(ctx, o, batch, all)
	cancel()
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if secondErr != nil {
		t.Fatalf("the second relay: %v", secondErr)
	}

	for _, id := range second.all {
		if slices.Contains(first.all, id) {
			t.Errorf("both relays took the event %s", id)
		}
	}
	published := append(idsOf(first.published), idsOf(second.published)...)
	slices.SortFunc(published, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	if want := eventIDs(t, pool, rs...); !slices.Equal(published, want) {
		t.Errorf("the two relays published %v, want each of the events %v once", published, want)
	}
	t.Logf("the first relay published %d events of the test, and the second one %d", len(first.published), len(second.published))
}

// An event that cannot be read is a bug of the code that wrote it: it is
// logged and stays in the outbox, and the other events are published.
func TestDrainGoesOnPastAnEventThatCannotBeRead(t *testing.T) {
	store, room, pool := newStore(t)
	o, log := newOutbox(pool, time.Now)
	ctx := t.Context()

	// Two bad rows, older than the event of the test: one that is not an
	// envelope, and one that holds an envelope under another id. Each names
	// the room of the test, so that the test deletes it at the end.
	notAnEnvelope, otherID := newUUID(t), newUUID(t)
	valid, err := event.New(newUUID(t), event.Source("reservation"), "reservation.Broken.v1", newUUID(t).String(), at,
		map[string]string{"roomId": uuid.UUID(room).String()})
	if err != nil {
		t.Fatal(err)
	}
	insert := "INSERT INTO outbox (id, event_type, payload, created_at) VALUES ($1, 'reservation.Broken.v1', $2, $3)"
	if _, err := pool.Exec(ctx, insert, notAnEnvelope, fmt.Sprintf(`{"data": {"roomId": %q}}`, uuid.UUID(room)), at); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, insert, otherID, valid, at); err != nil {
		t.Fatal(err)
	}
	r := add(t, store, room, hours(10, 11))

	d, n := drain(ctx, t, o, batch, mine(r))
	if got, want := idsOf(d.published), eventIDs(t, pool, r); !slices.Equal(got, want) || n != 1 {
		t.Errorf("Drain published %v (%d), want only the event of the test %v", got, n, want)
	}
	if slices.Contains(d.all, notAnEnvelope) || slices.Contains(d.all, otherID) || slices.Contains(d.all, valid.ID) {
		t.Errorf("Drain handed a row that cannot be read to publish: %v", d.all)
	}
	for _, id := range []uuid.UUID{notAnEnvelope, otherID} {
		var published bool
		if err := pool.QueryRow(ctx, "SELECT published_at IS NOT NULL FROM outbox WHERE id = $1", id).Scan(&published); err != nil {
			t.Fatal(err)
		}
		if published {
			t.Errorf("the row %s that cannot be read is marked as published", id)
		}
		if !strings.Contains(log.String(), `"level":"ERROR","msg":"an event of the outbox cannot be read, and stays in the outbox","event_id":"`+id.String()+`"`) {
			t.Errorf("the log does not report the row %s that cannot be read:\n%s", id, log)
		}
	}
}

func TestPruneRemovesOnlyOldPublishedEvents(t *testing.T) {
	store, room, pool := newStore(t)
	ctx := t.Context()
	old, recent, waiting := add(t, store, room, hours(9, 10)), add(t, store, room, hours(10, 11)), add(t, store, room, hours(11, 12))

	// The instants are long past, so that Prune removes no event that
	// another test or a service published.
	day1 := time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC)
	day2 := day1.Add(24 * time.Hour)
	now := day1
	o, _ := newOutbox(pool, func() time.Time { return now })
	drain(ctx, t, o, batch, mine(old))
	now = day2
	drain(ctx, t, o, batch, mine(recent))

	n, err := o.Prune(ctx, day2)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Errorf("Prune removed %d events, want at least the old one", n)
	}
	if got := storedEvents(t, pool, old.ID()); len(got) != 0 {
		t.Errorf("the event published before the instant is still there: %+v", got)
	}
	if !isPublished(t, pool, recent) {
		t.Error("the event published at the instant is gone, or not marked")
	}
	if isPublished(t, pool, waiting) {
		t.Error("the event that waits is marked as published")
	}
}

// The whole path: a booking through the use case stores its event, and the
// relay of the platform publishes it once, as its contract says.
func TestTheRelayPublishesABooking(t *testing.T) {
	store, room, pool := newStore(t)
	ctx := t.Context()
	rooms := apptest.NewRooms(domain.Room{ID: room, Capacity: 8, Hours: domain.OpeningHours{Closes: domain.NewTimeOfDay(23, 0)}})
	newID := app.IDGeneratorFunc(func() (domain.ReservationID, error) {
		id, err := uuid.NewV7()
		return domain.ReservationID(id), err
	})
	booked, err := app.New(store, rooms, apptest.NewClock(at.Add(-24*time.Hour)), newID).BookRoom(ctx, app.BookRoomInput{
		IdempotencyKey: "relay-" + uuid.UUID(room).String(),
		Room:           room,
		Period:         hours(10, 11),
		Attendees:      3,
		Organizer:      "organizer@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	ours := mine(booked.Reservation)
	publisher := &outboxtest.Publisher{Refuse: func(e event.Envelope) bool { return !ours(e) }}
	o, _ := newOutbox(pool, time.Now)
	relay := outbox.NewRelay(o, publisher, slog.New(slog.DiscardHandler), time.Now, outbox.Config{Batch: batch})
	flush := func() {
		t.Helper()
		if _, err := relay.Flush(ctx); err != nil && !errors.Is(err, outboxtest.ErrRefused) {
			t.Fatal(err)
		}
	}
	flush()
	flush()

	var published []event.Envelope
	for _, e := range publisher.Published() {
		if ours(e) {
			published = append(published, e)
		}
	}
	if len(published) != 1 {
		t.Fatalf("the relay published %d events of the booking, want 1", len(published))
	}
	e := published[0]
	eventtest.AssertMatchesContract(t, e)
	if e.Type != typeConfirmed || e.Source != "/contexts/reservation" {
		t.Errorf("the relay published the event %s from %s, want %s from /contexts/reservation", e.Type, e.Source, typeConfirmed)
	}
	// PostgreSQL keeps jsonb in a form of its own, so the data comes back
	// with the same members in another order.
	wantData := fmt.Sprintf(`{"reservationId":%q,"roomId":%q,"startsAt":"2030-06-03T10:00:00Z","endsAt":"2030-06-03T11:00:00Z","attendees":3,"organizerEmail":"organizer@example.com"}`,
		booked.Reservation.ID(), uuid.UUID(room))
	var got, want map[string]any
	if err := json.Unmarshal(e.Data, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(wantData), &want); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(got, want) {
		t.Errorf("the relay published the data\n%s\nwant\n%s", e.Data, wantData)
	}
	if !isPublished(t, pool, booked.Reservation) {
		t.Error("the outbox does not mark the event as published")
	}
}
