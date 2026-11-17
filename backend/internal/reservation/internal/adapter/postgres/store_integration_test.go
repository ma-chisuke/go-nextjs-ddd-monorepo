//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/postgres/postgrestest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/adapter/postgres"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app/apptest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// at is a moment far enough ahead that every rule about the past holds.
var at = time.Date(2030, time.June, 3, 9, 0, 0, 0, time.UTC)

// newStore returns the store over the real database, connected as the runtime
// role of Reservation, and a room of its own for the test. Other tests run at
// the same time and leave their rows, so a test looks only at its own room;
// the rows of the room are deleted when the test ends.
func newStore(t *testing.T) (store *postgres.Store, room domain.RoomID, pool *pgxpool.Pool) {
	t.Helper()
	pool = postgrestest.Open(t, "reservation")
	room = domain.RoomID(newUUID(t))
	t.Cleanup(func() {
		ctx := context.Background()
		for _, statement := range []string{
			"DELETE FROM idempotency_keys WHERE reservation_id IN (SELECT id FROM reservations WHERE room_id = $1)",
			"DELETE FROM reservations WHERE room_id = $1",
		} {
			if _, err := pool.Exec(ctx, statement, uuid.UUID(room)); err != nil {
				t.Errorf("cannot delete the rows of the test: %v", err)
			}
		}
	})
	return postgres.NewStore(pool), room, pool
}

func newUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// hours returns the period from h1:00 to h2:00 on the day of at.
func hours(h1, h2 int) domain.Period {
	day := at.Truncate(24 * time.Hour)
	return domain.Period{Start: day.Add(time.Duration(h1) * time.Hour), End: day.Add(time.Duration(h2) * time.Hour)}
}

// confirmed returns a new confirmed reservation of the room, with a key of its
// own.
func confirmed(t *testing.T, room domain.RoomID, period domain.Period) (domain.Reservation, app.IdempotencyKey) {
	t.Helper()
	id := domain.ReservationID(newUUID(t))
	r := domain.Restore(id, room, period, 4, "organizer@example.com", domain.StatusConfirmed)
	return r, app.IdempotencyKey{Key: "key-" + id.String(), Fingerprint: "fp-" + id.String(), Reservation: id}
}

func add(t *testing.T, store *postgres.Store, room domain.RoomID, period domain.Period) domain.Reservation {
	t.Helper()
	r, key := confirmed(t, room, period)
	if _, err := store.Add(t.Context(), r, key, at); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestStoreKeepsReservations(t *testing.T) {
	t.Parallel()
	store, room, _ := newStore(t)
	ctx := t.Context()

	r, key := confirmed(t, room, hours(10, 11))
	version, err := store.Add(ctx, r, key, at)
	if err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Errorf("a new reservation is at version %d, want 1", version)
	}
	got, err := store.Get(ctx, r.ID())
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || got.Reservation.ID() != r.ID() || got.Reservation.Room() != room ||
		!got.Reservation.Period().Start.Equal(r.Period().Start) || !got.Reservation.Period().End.Equal(r.Period().End) ||
		got.Reservation.Attendees() != 4 || got.Reservation.Organizer() != "organizer@example.com" ||
		got.Reservation.Status() != domain.StatusConfirmed {
		t.Errorf("got %+v, want the stored reservation %+v at version 1", got, r)
	}
	stored, err := store.FindKey(ctx, key.Key)
	if err != nil {
		t.Fatal(err)
	}
	if stored != key {
		t.Errorf("got the key %+v, want %+v", stored, key)
	}
	if _, err := store.FindKey(ctx, "no-such-key-"+r.ID().String()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("got %v for a key that does not exist, want app.ErrNotFound", err)
	}
	if _, err := store.Get(ctx, domain.ReservationID(newUUID(t))); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("got %v for a reservation that does not exist, want app.ErrNotFound", err)
	}

	// A change names the version it read, and counts.
	cancelled, _, err := got.Reservation.Cancel(at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(ctx, cancelled, 2, at); !errors.Is(err, app.ErrVersionConflict) {
		t.Errorf("got %v for a change of version 2, want app.ErrVersionConflict", err)
	}
	version, err = store.Update(ctx, cancelled, 1, at)
	if err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Errorf("the change led to version %d, want 2", version)
	}
	if got, _ := store.Get(ctx, r.ID()); got.Version != 2 || got.Reservation.Status() != domain.StatusCancelled {
		t.Errorf("got %+v, want the cancelled reservation at version 2", got)
	}
}

func TestStoreRefusesOverlaps(t *testing.T) {
	t.Parallel()
	store, room, _ := newStore(t)
	ctx := t.Context()
	first := add(t, store, room, hours(10, 12))

	overlapping, key := confirmed(t, room, hours(11, 13))
	if _, err := store.Add(ctx, overlapping, key, at); !errors.Is(err, app.ErrOverlap) {
		t.Errorf("got %v, want app.ErrOverlap", err)
	}
	// The transaction stored nothing: neither the reservation nor its key.
	if _, err := store.Get(ctx, overlapping.ID()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("got %v, want no reservation", err)
	}
	if _, err := store.FindKey(ctx, key.Key); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("got %v, want no idempotency key", err)
	}
	// One may end when the next starts.
	add(t, store, room, hours(12, 13))
	add(t, store, room, hours(9, 10))

	// The use case asks first; the database answers the same.
	overlaps, err := store.Overlaps(ctx, room, hours(11, 12), domain.ReservationID{})
	if err != nil || !overlaps {
		t.Errorf("got %v, %v, want an overlap from 11:00 to 12:00", overlaps, err)
	}
	overlaps, err = store.Overlaps(ctx, room, hours(11, 12), first.ID())
	if err != nil || overlaps {
		t.Errorf("got %v, %v, want no overlap when the reservation itself is left out", overlaps, err)
	}

	// A move onto another reservation is refused too.
	moved, _, err := first.Reschedule(domain.Room{ID: room, Capacity: 8, Hours: domain.OpeningHours{Closes: domain.NewTimeOfDay(23, 0)}}, hours(10, 13), 4, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(ctx, moved, 1, at); !errors.Is(err, app.ErrOverlap) {
		t.Errorf("got %v, want app.ErrOverlap", err)
	}

	// A cancelled reservation holds nothing.
	cancelled, _, err := first.Cancel(at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(ctx, cancelled, 1, at); err != nil {
		t.Fatal(err)
	}
	add(t, store, room, hours(10, 12))
}

func TestStoreRefusesATakenKey(t *testing.T) {
	t.Parallel()
	store, room, _ := newStore(t)
	ctx := t.Context()
	_, key := confirmed(t, room, hours(10, 11))
	first, _ := confirmed(t, room, hours(10, 11))
	key.Reservation = first.ID()
	if _, err := store.Add(ctx, first, key, at); err != nil {
		t.Fatal(err)
	}
	second, _ := confirmed(t, room, hours(14, 15))
	key.Reservation = second.ID()
	if _, err := store.Add(ctx, second, key, at); !errors.Is(err, app.ErrKeyTaken) {
		t.Errorf("got %v, want app.ErrKeyTaken", err)
	}
	if _, err := store.Get(ctx, second.ID()); !errors.Is(err, app.ErrNotFound) {
		t.Errorf("got %v, want no reservation: the key and the reservation are stored together or not at all", err)
	}
}

func TestStoreListsInPages(t *testing.T) {
	t.Parallel()
	store, room, pool := newStore(t)
	ctx := t.Context()
	for _, h := range [][2]int{{15, 16}, {9, 10}, {12, 13}, {10, 11}} {
		add(t, store, room, hours(h[0], h[1]))
	}
	other := domain.RoomID(newUUID(t))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM idempotency_keys WHERE reservation_id IN (SELECT id FROM reservations WHERE room_id = $1)", uuid.UUID(other))
		_, _ = pool.Exec(context.Background(), "DELETE FROM reservations WHERE room_id = $1", uuid.UUID(other))
	})
	add(t, store, other, hours(10, 11))

	list := func(q app.ListQuery) string {
		t.Helper()
		var starts []int
		for {
			page, err := store.List(ctx, q)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range page {
				starts = append(starts, r.Period().Start.UTC().Hour())
			}
			if len(page) < q.Limit {
				return fmt.Sprint(starts)
			}
			last := page[len(page)-1]
			q.AfterStart, q.AfterID = last.Period().Start, last.ID()
		}
	}
	from, to := hours(10, 15).Start, hours(10, 15).End
	if got := list(app.ListQuery{Room: &room, Limit: 2}); got != "[9 10 12 15]" {
		t.Errorf("got the starts %s, want [9 10 12 15]", got)
	}
	// The reservation from 9:00 ends when the window starts, and the one
	// from 15:00 starts when it ends.
	if got := list(app.ListQuery{Room: &room, From: &from, To: &to, Limit: 1}); got != "[10 12]" {
		t.Errorf("got the starts %s, want [10 12]", got)
	}
}

// Two requests book the same slot at the same moment. The checks of the use
// case cannot see each other's rows, so the exclusion constraint decides:
// exactly one of them succeeds.
func TestConcurrentBookingsOfOneSlot(t *testing.T) {
	t.Parallel()
	store, room, _ := newStore(t)
	const requests = 2
	errs := make([]error, requests)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range requests {
		r, key := confirmed(t, room, hours(10, 11))
		wg.Go(func() {
			<-start
			_, errs[i] = store.Add(t.Context(), r, key, at)
		})
	}
	close(start)
	wg.Wait()
	succeeded, refused := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, app.ErrOverlap):
			refused++
		default:
			t.Errorf("got the error %v, want success or app.ErrOverlap", err)
		}
	}
	t.Logf("%d concurrent bookings of one slot: %d succeeded, %d refused as overlapping", requests, succeeded, refused)
	if succeeded != 1 || refused != requests-1 {
		t.Errorf("%d succeeded and %d were refused, want exactly 1 success", succeeded, refused)
	}
}

// The same race through the use case: many clients book overlapping periods
// of one room at once, and exactly one gets the room. The others get the
// business error, whether the check of the use case or the constraint of the
// database stopped them.
func TestConcurrentBookingsThroughTheUseCase(t *testing.T) {
	t.Parallel()
	store, room, _ := newStore(t)
	rooms := apptest.NewRooms(domain.Room{ID: room, Capacity: 8, Hours: domain.OpeningHours{Closes: domain.NewTimeOfDay(23, 0)}})
	ids := app.IDGeneratorFunc(func() (domain.ReservationID, error) {
		id, err := uuid.NewV7()
		return domain.ReservationID(id), err
	})
	uc := app.New(store, rooms, apptest.NewClock(at.Add(-24*time.Hour)), ids)

	const requests = 8
	errs := make([]error, requests)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range requests {
		in := app.BookRoomInput{
			IdempotencyKey: fmt.Sprintf("race-%s-%d", room, i),
			Room:           room,
			Period:         hours(10+i%2, 12),
			Attendees:      2,
			Organizer:      "organizer@example.com",
		}
		wg.Go(func() {
			<-start
			_, errs[i] = uc.BookRoom(t.Context(), in)
		})
	}
	close(start)
	wg.Wait()
	succeeded := 0
	for _, err := range errs {
		var business *app.Error
		switch {
		case err == nil:
			succeeded++
		case errors.As(err, &business) && business.Kind == app.KindRoomAlreadyBooked:
		default:
			t.Errorf("got the error %v, want success or the business error KindRoomAlreadyBooked", err)
		}
	}
	t.Logf("%d concurrent bookings through the use case: %d succeeded", requests, succeeded)
	if succeeded != 1 {
		t.Errorf("%d bookings succeeded, want exactly 1", succeeded)
	}
}

// The constraints of the schema reject what the use cases would never send.
func TestReservationConstraints(t *testing.T) {
	t.Parallel()
	_, room, pool := newStore(t)
	insert := `INSERT INTO reservations (room_id, starts_at, ends_at, attendees, organizer_email, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'organizer@example.com', $5, now(), now())`
	tests := []struct {
		name       string
		start, end time.Time
		attendees  int
		status     string
	}{
		{"ends before it starts", hours(11, 10).Start, hours(11, 10).End, 4, "confirmed"},
		{"ends when it starts", hours(10, 10).Start, hours(10, 10).End, 4, "confirmed"},
		{"nobody attends", hours(10, 11).Start, hours(10, 11).End, 0, "confirmed"},
		{"an unknown status", hours(10, 11).Start, hours(10, 11).End, 4, "pending"},
	}
	for _, tt := range tests {
		if _, err := pool.Exec(t.Context(), insert, uuid.UUID(room), tt.start, tt.end, tt.attendees, tt.status); err == nil {
			t.Errorf("%s: the database accepted the row", tt.name)
		}
	}
}

// The runtime role works with the rows of its own schema, and with nothing
// else (TPL-0005, TPL-0009).
func TestRuntimeRolePrivileges(t *testing.T) {
	t.Parallel()
	pool := postgrestest.Open(t, "reservation")
	denied := map[string]string{
		"read the rooms of Facility":          "SELECT 1 FROM facility.rooms",
		"create a table in its schema":        "CREATE TABLE reservation.intruder (id integer)",
		"change a table":                      "ALTER TABLE reservations ADD COLUMN intruder integer",
		"drop a table":                        "DROP TABLE idempotency_keys",
		"empty a table":                       "TRUNCATE reservations",
		"create a table in the public schema": "CREATE TABLE public.intruder (id integer)",
	}
	for what, statement := range denied {
		if _, err := pool.Exec(t.Context(), statement); err == nil {
			t.Errorf("the runtime role of Reservation could %s", what)
		}
	}
}
