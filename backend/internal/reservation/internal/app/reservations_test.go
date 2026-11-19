package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app/apptest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

var (
	tokyo  = time.FixedZone("Asia/Tokyo", 9*60*60)
	roomA  = domain.Room{ID: domain.RoomID{0xa}, Capacity: 8, Hours: hours(9, 18), TimeZone: tokyo}
	roomB  = domain.Room{ID: domain.RoomID{0xb}, Capacity: 4, Hours: hours(9, 18), TimeZone: tokyo}
	nobody = domain.RoomID{0xff}
	// now is 10:00 in Tokyo on 18 November 2026.
	now = at(18, 10, 0)
)

func hours(opens, closes int) domain.OpeningHours {
	return domain.OpeningHours{Opens: domain.NewTimeOfDay(opens, 0), Closes: domain.NewTimeOfDay(closes, 0)}
}

// at returns hour:minute in Tokyo on the given day of November 2026.
func at(day, hour, minute int) time.Time {
	return time.Date(2026, time.November, day, hour, minute, 0, 0, tokyo)
}

// on returns the period from h1:00 to h2:00 in Tokyo on 20 November 2026.
func on(h1, h2 int) domain.Period {
	return domain.Period{Start: at(20, h1, 0), End: at(20, h2, 0)}
}

type fixture struct {
	store *apptest.Store
	rooms *apptest.Rooms
	clock *apptest.Clock
	uc    *app.Reservations
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	f := fixture{store: &apptest.Store{}, rooms: apptest.NewRooms(roomA, roomB), clock: apptest.NewClock(now)}
	f.uc = app.New(f.store, f.rooms, f.clock, &apptest.IDs{})
	return f
}

// booking returns the input of a booking of room A for the period, under a
// key of its own.
func booking(key string, period domain.Period) app.BookRoomInput {
	return app.BookRoomInput{IdempotencyKey: key, Room: roomA.ID, Period: period, Attendees: 4, Organizer: "organizer@example.com"}
}

func (f fixture) book(t *testing.T, key string, period domain.Period) app.Versioned {
	t.Helper()
	v, err := f.uc.BookRoom(t.Context(), booking(key, period))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// confirmedEvent returns the event ReservationConfirmed of a reservation of
// room A by the organizer of booking, at now.
func confirmedEvent(id domain.ReservationID, period domain.Period, attendees int) domain.Event {
	return domain.ReservationConfirmed{
		Reservation: id, Room: roomA.ID, Period: period, Attendees: attendees,
		Organizer: "organizer@example.com", OccurredAt: now,
	}
}

// wantEvents fails the test unless the store keeps exactly the events, in
// the order of their changes: those of the changes that it accepted, and none
// of those that it refused or that a use case never tried.
func wantEvents(t *testing.T, store *apptest.Store, want ...domain.Event) {
	t.Helper()
	if got := store.Events(); !slices.Equal(got, want) {
		t.Errorf("the store keeps the events\n%+v\nwant\n%+v", got, want)
	}
}

func wantAppKind(t *testing.T, err error, kind app.Kind) {
	t.Helper()
	var business *app.Error
	if !errors.As(err, &business) || business.Kind != kind {
		t.Errorf("got the error %v, want a business error of the use case of kind %d", err, kind)
	}
}

func wantDomainKind(t *testing.T, err error, kind domain.Kind) {
	t.Helper()
	var business *domain.Error
	if !errors.As(err, &business) || business.Kind != kind {
		t.Errorf("got the error %v, want a business error of the domain of kind %d", err, kind)
	}
}

func TestBookRoom(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	booked := f.book(t, "k1", on(10, 11))
	if booked.Version != 1 || booked.Reservation.ID() != apptest.ID(1) || booked.Reservation.Room() != roomA.ID ||
		booked.Reservation.Period() != on(10, 11) || booked.Reservation.Status() != domain.StatusConfirmed {
		t.Errorf("the booking is not the requested one: %+v", booked)
	}
	got, err := f.uc.GetReservation(t.Context(), booked.Reservation.ID())
	if err != nil {
		t.Fatal(err)
	}
	if got != booked {
		t.Errorf("got %+v, want the booked reservation %+v", got, booked)
	}
	wantEvents(t, f.store, confirmedEvent(apptest.ID(1), on(10, 11), 4))
}

func TestBookRoomAppliesTheRulesOfTheDomain(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	in := booking("k1", on(10, 11))
	in.Attendees = 9
	_, err := f.uc.BookRoom(t.Context(), in)
	wantDomainKind(t, err, domain.KindOverCapacity)
	wantEvents(t, f.store)
}

func TestBookRoomIdempotently(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	first := f.book(t, "k1", on(10, 11))

	// The same request with the same key gets the same reservation, and
	// books nothing new: the reservation does not stand in its own way.
	if again := f.book(t, "k1", on(10, 11)); again != first {
		t.Errorf("got %+v, want the reservation of the first request, %+v", again, first)
	}

	// The same holds after the reservation changed.
	cancelled, err := f.uc.CancelReservation(t.Context(), first.Reservation.ID(), first.Version)
	if err != nil {
		t.Fatal(err)
	}
	again := f.book(t, "k1", on(10, 11))
	if again != cancelled {
		t.Errorf("got %+v, want the reservation of the first request as it is now, %+v", again, cancelled)
	}
	page, _, err := f.uc.ListReservations(t.Context(), app.ListReservationsInput{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 {
		t.Errorf("the store holds %d reservations, want 1", len(page))
	}

	// The same instants written in another time zone are the same request.
	utc := on(10, 11)
	utc.Start, utc.End = utc.Start.UTC(), utc.End.UTC()
	if v := f.book(t, "k1", utc); v.Reservation.ID() != first.Reservation.ID() {
		t.Errorf("got the reservation %s, want %s", v.Reservation.ID(), first.Reservation.ID())
	}

	// Another request with the same key is refused.
	_, err = f.uc.BookRoom(t.Context(), booking("k1", on(12, 13)))
	wantAppKind(t, err, app.KindIdempotencyKeyReused)

	// Only the booking and the cancellation are events: a repeat changes
	// nothing.
	wantEvents(t, f.store,
		confirmedEvent(first.Reservation.ID(), on(10, 11), 4),
		domain.ReservationCancelled{
			Reservation: first.Reservation.ID(), Room: roomA.ID, Period: on(10, 11),
			Organizer: "organizer@example.com", OccurredAt: now,
		})
}

func TestBookRoomThatIsBooked(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	first := f.book(t, "k1", on(10, 12))

	_, err := f.uc.BookRoom(t.Context(), booking("k2", on(11, 13)))
	wantAppKind(t, err, app.KindRoomAlreadyBooked)
	wantEvents(t, f.store, confirmedEvent(first.Reservation.ID(), on(10, 12), 4))
	// One may end when the next starts.
	f.book(t, "k3", on(12, 13))
	f.book(t, "k4", on(9, 10))
	// Another room is free.
	other := booking("k5", on(10, 12))
	other.Room = roomB.ID
	if _, err := f.uc.BookRoom(t.Context(), other); err != nil {
		t.Errorf("got %v, want a booking of another room for the same period", err)
	}
}

// racingStore stands for a store in which another request stores its
// reservation between the checks of the use case and its own write: the
// checks see nothing, and the write is refused.
type racingStore struct {
	*apptest.Store
	findKeyCalls int
}

func (s *racingStore) Overlaps(context.Context, domain.RoomID, domain.Period, domain.ReservationID) (bool, error) {
	return false, nil
}

func (s *racingStore) FindKey(ctx context.Context, key string) (app.IdempotencyKey, error) {
	s.findKeyCalls++
	if s.findKeyCalls == 1 {
		return app.IdempotencyKey{}, app.ErrNotFound
	}
	return s.Store.FindKey(ctx, key)
}

func TestBookRoomInARace(t *testing.T) {
	t.Parallel()
	store := &apptest.Store{}
	ids := &apptest.IDs{}
	winner, err := app.New(store, apptest.NewRooms(roomA), apptest.NewClock(now), ids).BookRoom(t.Context(), booking("k1", on(10, 11)))
	if err != nil {
		t.Fatal(err)
	}

	// The store refuses an overlapping reservation that the check missed.
	racing := app.New(&racingStore{Store: store}, apptest.NewRooms(roomA), apptest.NewClock(now), ids)
	_, err = racing.BookRoom(t.Context(), booking("k2", on(10, 11)))
	wantAppKind(t, err, app.KindRoomAlreadyBooked)
	wantEvents(t, store, confirmedEvent(winner.Reservation.ID(), on(10, 11), 4))

	// A repeat of the winner that lost the race gets the reservation of the
	// winner.
	racing = app.New(&racingStore{Store: store}, apptest.NewRooms(roomA), apptest.NewClock(now), ids)
	got, err := racing.BookRoom(t.Context(), booking("k1", on(10, 11)))
	if err != nil {
		t.Fatal(err)
	}
	if got != winner {
		t.Errorf("got %+v, want the reservation of the request that won, %+v", got, winner)
	}

	// Another request with the key of the winner that lost the race is
	// refused, as it would be without the race.
	racing = app.New(&racingStore{Store: store}, apptest.NewRooms(roomA), apptest.NewClock(now), ids)
	_, err = racing.BookRoom(t.Context(), booking("k1", on(12, 13)))
	wantAppKind(t, err, app.KindIdempotencyKeyReused)
	wantEvents(t, store, confirmedEvent(winner.Reservation.ID(), on(10, 11), 4))
}

func TestBookRoomWhenFacilitySaysNo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	in := booking("k1", on(10, 11))
	in.Room = nobody
	_, err := f.uc.BookRoom(t.Context(), in)
	wantAppKind(t, err, app.KindUnknownRoom)
	if want := "No room has the id ff000000-0000-0000-0000-000000000000."; err == nil || err.Error() != want {
		t.Errorf("got the error %v, want %q", err, want)
	}

	cause := fmt.Errorf("%w: connection refused", app.ErrRoomsUnavailable)
	f.rooms.Fail(cause)
	_, err = f.uc.BookRoom(t.Context(), booking("k2", on(10, 11)))
	wantAppKind(t, err, app.KindFacilityUnavailable)
	if !errors.Is(err, cause) {
		t.Errorf("the error %v does not keep its cause, which the log needs", err)
	}
	want := "The rooms cannot be checked now. Try again later. (cause: the directory of rooms does not answer: connection refused)"
	if err == nil || err.Error() != want {
		t.Errorf("got the error %v, want %q: the log reads the cause after the detail", err, want)
	}

	// Any other failure of the directory is not a business error, and the
	// log keeps it.
	broke := errors.New("the directory broke")
	f.rooms.Fail(broke)
	_, err = f.uc.BookRoom(t.Context(), booking("k3", on(10, 11)))
	var business *app.Error
	if !errors.Is(err, broke) || errors.As(err, &business) {
		t.Errorf("got %v, want the failure of the directory, not a business error", err)
	}
	wantEvents(t, f.store)
}

func TestGetReservationThatDoesNotExist(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	_, err := f.uc.GetReservation(t.Context(), apptest.ID(42))
	wantAppKind(t, err, app.KindReservationNotFound)
}

func TestRescheduleReservation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	booked := f.book(t, "k1", on(10, 11))
	other := f.book(t, "k2", on(14, 15))
	id := booked.Reservation.ID()
	start, end, attendees := at(20, 12, 0), at(20, 14, 0), 6

	// A longer period that overlaps the old one: a reservation never stands
	// in its own way.
	moved, err := f.uc.RescheduleReservation(t.Context(), app.RescheduleReservationInput{
		Reservation: id, Read: 1, End: &start,
	})
	if err != nil {
		t.Fatal(err)
	}
	if moved.Version != 2 || moved.Reservation.Period() != on(10, 12) || moved.Reservation.Attendees() != 4 {
		t.Errorf("got %+v, want version 2, from 10:00 to 12:00, with 4 attendees", moved)
	}

	moved, err = f.uc.RescheduleReservation(t.Context(), app.RescheduleReservationInput{
		Reservation: id, Read: 2, Start: &start, End: &end, Attendees: &attendees,
	})
	if err != nil {
		t.Fatal(err)
	}
	if moved.Version != 3 || moved.Reservation.Period() != on(12, 14) || moved.Reservation.Attendees() != 6 {
		t.Errorf("got %+v, want version 3, from 12:00 to 14:00, with 6 attendees", moved)
	}

	// The version that the client read is the current one.
	_, err = f.uc.RescheduleReservation(t.Context(), app.RescheduleReservationInput{Reservation: id, Read: 2, Attendees: &attendees})
	wantAppKind(t, err, app.KindVersionMismatch)
	// Another reservation holds 14:00 to 15:00.
	later := at(20, 15, 0)
	_, err = f.uc.RescheduleReservation(t.Context(), app.RescheduleReservationInput{Reservation: id, Read: 3, End: &later})
	wantAppKind(t, err, app.KindRoomAlreadyBooked)
	// The rules of a booking apply.
	tooMany := 9
	_, err = f.uc.RescheduleReservation(t.Context(), app.RescheduleReservationInput{Reservation: id, Read: 3, Attendees: &tooMany})
	wantDomainKind(t, err, domain.KindOverCapacity)
	// A reservation that does not exist.
	_, err = f.uc.RescheduleReservation(t.Context(), app.RescheduleReservationInput{Reservation: apptest.ID(42), Read: 1})
	wantAppKind(t, err, app.KindReservationNotFound)
	// Facility does not answer.
	f.rooms.Fail(app.ErrRoomsUnavailable)
	_, err = f.uc.RescheduleReservation(t.Context(), app.RescheduleReservationInput{Reservation: id, Read: 3, Attendees: &attendees})
	wantAppKind(t, err, app.KindFacilityUnavailable)

	// Each rescheduling confirms the reservation again, with its new period
	// and attendees; a refused one is no event.
	wantEvents(t, f.store,
		confirmedEvent(id, on(10, 11), 4),
		confirmedEvent(other.Reservation.ID(), on(14, 15), 4),
		confirmedEvent(id, on(10, 12), 4),
		confirmedEvent(id, on(12, 14), 6))
}

// conflictingStore stands for a store in which another request changes the
// reservation between the read of the use case and its write.
type conflictingStore struct{ *apptest.Store }

func (s conflictingStore) Update(context.Context, domain.Reservation, []domain.Event, app.Version, time.Time) (app.Version, error) {
	return 0, app.ErrVersionConflict
}

func TestChangeInARace(t *testing.T) {
	t.Parallel()
	store := &apptest.Store{}
	ids := &apptest.IDs{}
	booked, err := app.New(store, apptest.NewRooms(roomA), apptest.NewClock(now), ids).BookRoom(t.Context(), booking("k1", on(10, 11)))
	if err != nil {
		t.Fatal(err)
	}
	racing := app.New(conflictingStore{store}, apptest.NewRooms(roomA), apptest.NewClock(now), ids)
	_, err = racing.CancelReservation(t.Context(), booked.Reservation.ID(), 1)
	wantAppKind(t, err, app.KindVersionMismatch)
	attendees := 2
	_, err = racing.RescheduleReservation(t.Context(), app.RescheduleReservationInput{Reservation: booked.Reservation.ID(), Read: 1, Attendees: &attendees})
	wantAppKind(t, err, app.KindVersionMismatch)
	wantEvents(t, store, confirmedEvent(booked.Reservation.ID(), on(10, 11), 4))
}

func TestCancelReservation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	booked := f.book(t, "k1", on(10, 11))
	id := booked.Reservation.ID()

	_, err := f.uc.CancelReservation(t.Context(), id, 2)
	wantAppKind(t, err, app.KindVersionMismatch)

	cancelled, err := f.uc.CancelReservation(t.Context(), id, 1)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Version != 2 || cancelled.Reservation.Status() != domain.StatusCancelled {
		t.Errorf("got %+v, want a cancelled reservation at version 2", cancelled)
	}
	// The period is free again.
	f.book(t, "k2", on(10, 11))

	_, err = f.uc.CancelReservation(t.Context(), id, 2)
	wantDomainKind(t, err, domain.KindReservationCancelled)
	attendees := 2
	_, err = f.uc.RescheduleReservation(t.Context(), app.RescheduleReservationInput{Reservation: id, Read: 2, Attendees: &attendees})
	wantDomainKind(t, err, domain.KindReservationCancelled)
	_, err = f.uc.CancelReservation(t.Context(), apptest.ID(42), 1)
	wantAppKind(t, err, app.KindReservationNotFound)

	wantEvents(t, f.store,
		confirmedEvent(id, on(10, 11), 4),
		domain.ReservationCancelled{
			Reservation: id, Room: roomA.ID, Period: on(10, 11),
			Organizer: "organizer@example.com", OccurredAt: now,
		},
		confirmedEvent(apptest.ID(2), on(10, 11), 4))
}

func TestCancelReservationTooLate(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	booked := f.book(t, "k1", on(10, 11))
	f.clock.Set(at(20, 9, 31))
	_, err := f.uc.CancelReservation(t.Context(), booked.Reservation.ID(), 1)
	wantDomainKind(t, err, domain.KindCancellationTooLate)
	wantEvents(t, f.store, confirmedEvent(booked.Reservation.ID(), on(10, 11), 4))
}

func TestListReservations(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	// Booked out of order: the list is in the order of the start.
	for i, hour := range []int{15, 9, 12, 10} {
		f.book(t, fmt.Sprintf("k%d", i), on(hour, hour+1))
	}
	inB := booking("kb", on(9, 10))
	inB.Room = roomB.ID
	if _, err := f.uc.BookRoom(t.Context(), inB); err != nil {
		t.Fatal(err)
	}

	list := func(in app.ListReservationsInput) []string {
		t.Helper()
		var starts []string
		for {
			page, next, err := f.uc.ListReservations(t.Context(), in)
			if err != nil {
				t.Fatal(err)
			}
			if len(page) > in.Limit {
				t.Errorf("a page holds %d reservations, and the limit is %d", len(page), in.Limit)
			}
			for _, r := range page {
				starts = append(starts, fmt.Sprintf("%s@%02d", r.Room().String()[:2], r.Period().Start.In(tokyo).Hour()))
			}
			if next == "" {
				return starts
			}
			in.Cursor = next
		}
	}
	room := roomA.ID
	from, to := at(20, 10, 0), at(20, 15, 0)
	tests := []struct {
		name string
		in   app.ListReservationsInput
		want string
	}{
		{"all, two at a time", app.ListReservationsInput{Limit: 2}, "[0a@09 0b@09 0a@10 0a@12 0a@15]"},
		{"one room", app.ListReservationsInput{Room: &room, Limit: 3}, "[0a@09 0a@10 0a@12 0a@15]"},
		// The reservation from 9:00 to 10:00 ends when the window starts,
		// and the one from 15:00 starts when it ends.
		{"a window", app.ListReservationsInput{From: &from, To: &to, Limit: 1}, "[0a@10 0a@12]"},
	}
	for _, tt := range tests {
		if got := fmt.Sprint(list(tt.in)); got != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, got, tt.want)
		}
	}

	_, _, err := f.uc.ListReservations(t.Context(), app.ListReservationsInput{Limit: 2, Cursor: "not-a-cursor"})
	wantAppKind(t, err, app.KindInvalidCursor)
}

// brokenStore fails in the method named broken, as a store whose database
// goes down in the middle of a use case.
type brokenStore struct {
	*apptest.Store
	broken string
}

var errDown = errors.New("the database is down")

func (s brokenStore) Add(ctx context.Context, r domain.Reservation, events []domain.Event, key app.IdempotencyKey, at time.Time) (app.Version, error) {
	if s.broken == "Add" {
		return 0, errDown
	}
	return s.Store.Add(ctx, r, events, key, at)
}

func (s brokenStore) Get(ctx context.Context, id domain.ReservationID) (app.Versioned, error) {
	if s.broken == "Get" {
		return app.Versioned{}, errDown
	}
	return s.Store.Get(ctx, id)
}

func (s brokenStore) Update(ctx context.Context, r domain.Reservation, events []domain.Event, read app.Version, at time.Time) (app.Version, error) {
	if s.broken == "Update" {
		return 0, errDown
	}
	return s.Store.Update(ctx, r, events, read, at)
}

func (s brokenStore) Overlaps(ctx context.Context, room domain.RoomID, period domain.Period, except domain.ReservationID) (bool, error) {
	if s.broken == "Overlaps" {
		return false, errDown
	}
	return s.Store.Overlaps(ctx, room, period, except)
}

func (s brokenStore) List(ctx context.Context, q app.ListQuery) ([]domain.Reservation, error) {
	if s.broken == "List" {
		return nil, errDown
	}
	return s.Store.List(ctx, q)
}

func (s brokenStore) FindKey(ctx context.Context, key string) (app.IdempotencyKey, error) {
	if s.broken == "FindKey" {
		return app.IdempotencyKey{}, errDown
	}
	return s.Store.FindKey(ctx, key)
}

// A failure of the store is not a business error: it reaches the client as
// an internal error, and the log keeps it. Each use case passes on the
// failure of each method that it calls.
func TestFailuresOfTheStore(t *testing.T) {
	t.Parallel()
	attendees := 2
	tests := []struct {
		name   string
		broken string
		call   func(ctx context.Context, uc *app.Reservations, booked app.Versioned) error
	}{
		{"get", "Get", func(ctx context.Context, uc *app.Reservations, booked app.Versioned) error {
			_, err := uc.GetReservation(ctx, booked.Reservation.ID())
			return err
		}},
		{"look for the key of a booking", "FindKey", func(ctx context.Context, uc *app.Reservations, _ app.Versioned) error {
			_, err := uc.BookRoom(ctx, booking("k2", on(12, 13)))
			return err
		}},
		{"get the reservation of a repeated booking", "Get", func(ctx context.Context, uc *app.Reservations, _ app.Versioned) error {
			_, err := uc.BookRoom(ctx, booking("k1", on(10, 11)))
			return err
		}},
		{"check a booking for overlaps", "Overlaps", func(ctx context.Context, uc *app.Reservations, _ app.Versioned) error {
			_, err := uc.BookRoom(ctx, booking("k2", on(12, 13)))
			return err
		}},
		{"add a booking", "Add", func(ctx context.Context, uc *app.Reservations, _ app.Versioned) error {
			_, err := uc.BookRoom(ctx, booking("k2", on(12, 13)))
			return err
		}},
		{"check a rescheduling for overlaps", "Overlaps", func(ctx context.Context, uc *app.Reservations, booked app.Versioned) error {
			_, err := uc.RescheduleReservation(ctx, app.RescheduleReservationInput{Reservation: booked.Reservation.ID(), Read: 1, Attendees: &attendees})
			return err
		}},
		{"update a rescheduling", "Update", func(ctx context.Context, uc *app.Reservations, booked app.Versioned) error {
			_, err := uc.RescheduleReservation(ctx, app.RescheduleReservationInput{Reservation: booked.Reservation.ID(), Read: 1, Attendees: &attendees})
			return err
		}},
		{"load a cancellation", "Get", func(ctx context.Context, uc *app.Reservations, booked app.Versioned) error {
			_, err := uc.CancelReservation(ctx, booked.Reservation.ID(), 1)
			return err
		}},
		{"update a cancellation", "Update", func(ctx context.Context, uc *app.Reservations, booked app.Versioned) error {
			_, err := uc.CancelReservation(ctx, booked.Reservation.ID(), 1)
			return err
		}},
		{"list", "List", func(ctx context.Context, uc *app.Reservations, _ app.Versioned) error {
			_, _, err := uc.ListReservations(ctx, app.ListReservationsInput{Limit: 1})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			booked := f.book(t, "k1", on(10, 11))
			uc := app.New(brokenStore{Store: f.store, broken: tt.broken}, f.rooms, f.clock, &apptest.IDs{})
			err := tt.call(t.Context(), uc, booked)
			var business *app.Error
			if !errors.Is(err, errDown) || errors.As(err, &business) {
				t.Errorf("got %v, want the failure of the store, not a business error", err)
			}
			wantEvents(t, f.store, confirmedEvent(booked.Reservation.ID(), on(10, 11), 4))
		})
	}
}

// The clock and the generator of ids can be plain functions, and a booking
// for which no id can be made books nothing.
func TestPortsAsFunctions(t *testing.T) {
	t.Parallel()
	store := &apptest.Store{}
	clock := app.ClockFunc(func() time.Time { return now })
	ids := app.IDGeneratorFunc(func() (domain.ReservationID, error) { return apptest.ID(7), nil })
	booked, err := app.New(store, apptest.NewRooms(roomA), clock, ids).BookRoom(t.Context(), booking("k1", on(10, 11)))
	if err != nil {
		t.Fatal(err)
	}
	if booked.Reservation.ID() != apptest.ID(7) {
		t.Errorf("got the id %s, want the one of the function, %s", booked.Reservation.ID(), apptest.ID(7))
	}

	errNoID := errors.New("no randomness")
	ids = app.IDGeneratorFunc(func() (domain.ReservationID, error) { return domain.ReservationID{}, errNoID })
	_, err = app.New(store, apptest.NewRooms(roomA), clock, ids).BookRoom(t.Context(), booking("k2", on(12, 13)))
	var business *app.Error
	if !errors.Is(err, errNoID) || errors.As(err, &business) {
		t.Errorf("got %v, want the failure of the generator of ids, not a business error", err)
	}
	wantEvents(t, store, confirmedEvent(apptest.ID(7), on(10, 11), 4))
}

// JSON writes no time after the year 9999, so neither the fingerprint of a
// booking nor the cursor of a list can hold one. Such a time is a failure,
// not a business error, and the log keeps what JSON said; the API never
// sends one.
func TestTimesAfterTheYear9999(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	far := func(day int) domain.Period {
		return domain.Period{Start: time.Date(10000, time.January, day, 10, 0, 0, 0, tokyo), End: time.Date(10000, time.January, day, 11, 0, 0, 0, tokyo)}
	}
	wantJSONFailure := func(err error) {
		t.Helper()
		var business *app.Error
		var marshal *json.MarshalerError
		if !errors.As(err, &marshal) || errors.As(err, &business) {
			t.Errorf("got %v, want the failure of JSON, not a business error", err)
		}
	}
	_, err := f.uc.BookRoom(t.Context(), booking("k1", far(3)))
	wantJSONFailure(err)

	for i, key := range []string{"k1", "k2"} {
		r := domain.Restore(apptest.ID(uint64(i+1)), roomA.ID, far(3+i), 4, "organizer@example.com", domain.StatusConfirmed)
		if _, err := f.store.Add(t.Context(), r, nil, app.IdempotencyKey{Key: key, Reservation: r.ID()}, now); err != nil {
			t.Fatal(err)
		}
	}
	_, _, err = f.uc.ListReservations(t.Context(), app.ListReservationsInput{Limit: 1})
	wantJSONFailure(err)
	wantEvents(t, f.store)
}

// Every kind is listed once, so that the tests that walk Kinds see each.
func TestKinds(t *testing.T) {
	t.Parallel()
	kinds := app.Kinds()
	seen := map[app.Kind]bool{}
	for i, kind := range kinds {
		if kind != app.Kind(i+1) || seen[kind] {
			t.Errorf("Kinds()[%d] is %d: list every kind once, in the order of their values", i, kind)
		}
		seen[kind] = true
	}
	// The list ends with the last kind. A new kind goes after it, and takes
	// its place here.
	if last := app.KindInvalidCursor; len(kinds) != int(last) {
		t.Errorf("Kinds() lists %d kinds, want every kind up to %d", len(kinds), last)
	}
}
