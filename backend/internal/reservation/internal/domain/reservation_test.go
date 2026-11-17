package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// The room of most tests is in Tokyo (UTC+9, no daylight saving time), so
// every test also proves that the rules read the clock of the room, not UTC.
var (
	tokyo         = time.FixedZone("Asia/Tokyo", 9*60*60)
	roomID        = domain.RoomID{0x01, 0x93, 0xa6, 0xf0, 0x7b, 0x5c, 0x7e, 0x2a, 0x9c, 0x3d, 0x1f, 0x2e, 0x3d, 0x4c, 0x5b, 0x6a}
	reservationID = domain.ReservationID{0x01, 0x93, 0xa6, 0xf1, 0, 0, 0x70, 0, 0x80, 0, 0, 0, 0, 0, 0, 0x01}
	room          = domain.Room{
		ID:       roomID,
		Capacity: 8,
		Hours:    domain.OpeningHours{Opens: domain.NewTimeOfDay(9, 0), Closes: domain.NewTimeOfDay(18, 0)},
		TimeZone: tokyo,
	}
	// now is 10:00 in Tokyo on 18 November 2026.
	now = at(18, 10, 0)
)

// at returns hour:minute in Tokyo on the given day of November 2026.
func at(day, hour, minute int) time.Time {
	return time.Date(2026, time.November, day, hour, minute, 0, 0, tokyo)
}

// on returns the period from h1:m1 to h2:m2 in Tokyo on the given day.
func on(day, h1, m1, h2, m2 int) domain.Period {
	return domain.Period{Start: at(day, h1, m1), End: at(day, h2, m2)}
}

func wantKind(t *testing.T, err error, kind domain.Kind) {
	t.Helper()
	var business *domain.Error
	if !errors.As(err, &business) || business.Kind != kind {
		t.Errorf("got the error %v, want a business error of kind %d", err, kind)
	}
}

func book(t *testing.T, period domain.Period) domain.Reservation {
	t.Helper()
	r, _, err := domain.Book(reservationID, room, period, 4, "organizer@example.com", now)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestBook(t *testing.T) {
	t.Parallel()
	// A room that is open almost all day, for the periods around midnight.
	lateRoom := room
	lateRoom.Hours = domain.OpeningHours{Opens: domain.NewTimeOfDay(0, 0), Closes: domain.NewTimeOfDay(23, 45)}

	tests := []struct {
		name      string
		room      domain.Room
		period    domain.Period
		attendees int
		kind      domain.Kind // 0: the booking succeeds
	}{
		{"one hour inside the opening hours", room, on(20, 10, 0, 11, 0), 4, 0},
		{"exactly the longest period", room, on(20, 10, 0, 14, 0), 4, 0},
		{"one step longer than the longest period", room, on(20, 10, 0, 14, 15), 4, domain.KindPeriodTooLong},
		{"ends when it starts", room, on(20, 10, 0, 10, 0), 4, domain.KindPeriodOutOfOrder},
		{"ends before it starts", room, on(20, 11, 0, 10, 0), 4, domain.KindPeriodOutOfOrder},
		{"starts off the grid", room, on(20, 10, 5, 11, 0), 4, domain.KindPeriodMisaligned},
		{"ends off the grid", room, on(20, 10, 0, 10, 50), 4, domain.KindPeriodMisaligned},
		{"starts one second off the grid", room, domain.Period{Start: at(20, 10, 0).Add(time.Second), End: at(20, 11, 0)}, 4, domain.KindPeriodMisaligned},
		{"starts now", room, on(18, 10, 0, 11, 0), 4, 0},
		{"started one step ago", room, on(18, 9, 45, 11, 0), 4, domain.KindPeriodInPast},
		{"started yesterday", room, on(17, 10, 0, 11, 0), 4, domain.KindPeriodInPast},
		{"nobody attends", room, on(20, 10, 0, 11, 0), 0, domain.KindNoAttendees},
		{"the attendees fill the room", room, on(20, 10, 0, 11, 0), 8, 0},
		{"one attendee too many", room, on(20, 10, 0, 11, 0), 9, domain.KindOverCapacity},
		{"starts when the room opens", room, on(20, 9, 0, 10, 0), 4, 0},
		{"ends when the room closes", room, on(20, 17, 0, 18, 0), 4, 0},
		{"starts before the room opens", room, on(20, 8, 45, 10, 0), 4, domain.KindOutsideOpeningHours},
		{"ends after the room closes", room, on(20, 17, 0, 18, 15), 4, domain.KindOutsideOpeningHours},
		{
			"inside the opening hours in UTC, after them in Tokyo", room,
			domain.Period{Start: time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC), End: time.Date(2026, 11, 20, 11, 0, 0, 0, time.UTC)},
			4, domain.KindOutsideOpeningHours,
		},
		{"ends when a late room closes", lateRoom, on(20, 23, 0, 23, 45), 4, 0},
		{"runs into the next day", lateRoom, domain.Period{Start: at(20, 23, 30), End: at(21, 0, 15)}, 4, domain.KindOutsideOpeningHours},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, events, err := domain.Book(reservationID, tt.room, tt.period, tt.attendees, "organizer@example.com", now)
			if tt.kind != 0 {
				wantKind(t, err, tt.kind)
				if len(events) != 0 {
					t.Errorf("a rejected booking has the events %v", events)
				}
				return
			}
			if err != nil {
				t.Fatalf("got the error %v, want a booking", err)
			}
			if r.ID() != reservationID || r.Room() != roomID || r.Period() != tt.period ||
				r.Attendees() != tt.attendees || r.Organizer() != "organizer@example.com" || r.Status() != domain.StatusConfirmed {
				t.Errorf("the reservation is not the one that was booked: %+v", r)
			}
			want := domain.ReservationConfirmed{
				Reservation: reservationID, Room: roomID, Period: tt.period,
				Attendees: tt.attendees, Organizer: "organizer@example.com", OccurredAt: now,
			}
			if len(events) != 1 || events[0] != want {
				t.Errorf("got the events %+v, want one %+v", events, want)
			}
		})
	}
}

func TestReschedule(t *testing.T) {
	t.Parallel()
	original := book(t, on(20, 10, 0, 11, 0))

	moved, events, err := original.Reschedule(room, on(20, 14, 0, 16, 0), 6, now)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Period() != on(20, 14, 0, 16, 0) || moved.Attendees() != 6 || moved.ID() != reservationID || moved.Status() != domain.StatusConfirmed {
		t.Errorf("the reservation did not move: %+v", moved)
	}
	if original.Period() != on(20, 10, 0, 11, 0) || original.Attendees() != 4 {
		t.Errorf("rescheduling changed the reservation it was called on: %+v", original)
	}
	want := domain.ReservationConfirmed{
		Reservation: reservationID, Room: roomID, Period: on(20, 14, 0, 16, 0),
		Attendees: 6, Organizer: "organizer@example.com", OccurredAt: now,
	}
	if len(events) != 1 || events[0] != want {
		t.Errorf("got the events %+v, want one %+v", events, want)
	}

	// Rescheduling follows the rules of booking.
	_, _, err = original.Reschedule(room, on(20, 10, 0, 14, 15), 4, now)
	wantKind(t, err, domain.KindPeriodTooLong)
	_, _, err = original.Reschedule(room, on(20, 10, 0, 11, 0), 9, now)
	wantKind(t, err, domain.KindOverCapacity)
	_, _, err = original.Reschedule(room, on(18, 9, 45, 11, 0), 4, now)
	wantKind(t, err, domain.KindPeriodInPast)

	// The room cannot change.
	other := room
	other.ID = domain.RoomID{0xff}
	_, _, err = original.Reschedule(other, on(20, 14, 0, 16, 0), 4, now)
	var business *domain.Error
	if err == nil || errors.As(err, &business) {
		t.Errorf("got %v, want an error that is not a business error: the caller passed the wrong room", err)
	}
}

func TestCancel(t *testing.T) {
	t.Parallel()
	r := book(t, on(20, 10, 0, 11, 0))
	deadline := at(20, 9, 30)

	cancelled, events, err := r.Cancel(deadline)
	if err != nil {
		t.Fatalf("got the error %v, want a cancellation exactly 30 minutes before the start", err)
	}
	if cancelled.Status() != domain.StatusCancelled || r.Status() != domain.StatusConfirmed {
		t.Errorf("got the statuses %s and %s, want cancelled for the result and confirmed for the original", cancelled.Status(), r.Status())
	}
	want := domain.ReservationCancelled{
		Reservation: reservationID, Room: roomID, Period: r.Period(),
		Organizer: "organizer@example.com", OccurredAt: deadline,
	}
	if len(events) != 1 || events[0] != want {
		t.Errorf("got the events %+v, want one %+v", events, want)
	}

	_, _, err = r.Cancel(deadline.Add(time.Nanosecond))
	wantKind(t, err, domain.KindCancellationTooLate)
	_, _, err = r.Cancel(at(20, 10, 30))
	wantKind(t, err, domain.KindCancellationTooLate)

	// A cancelled reservation stays as it is.
	_, _, err = cancelled.Cancel(now)
	wantKind(t, err, domain.KindReservationCancelled)
	_, _, err = cancelled.Reschedule(room, on(20, 14, 0, 15, 0), 4, now)
	wantKind(t, err, domain.KindReservationCancelled)
}

func TestRestore(t *testing.T) {
	t.Parallel()
	// The store may hold a reservation that the rules would reject today,
	// such as one in the past.
	period := on(1, 10, 0, 11, 0)
	r := domain.Restore(reservationID, roomID, period, 3, "organizer@example.com", domain.StatusConfirmed)
	if r.ID() != reservationID || r.Room() != roomID || r.Period() != period || r.Attendees() != 3 || r.Status() != domain.StatusConfirmed {
		t.Errorf("the restored reservation is not the stored one: %+v", r)
	}
	_, _, err := r.Cancel(now)
	wantKind(t, err, domain.KindCancellationTooLate)
}

func TestPeriodOverlaps(t *testing.T) {
	t.Parallel()
	base := on(20, 10, 0, 11, 0)
	tests := []struct {
		name  string
		other domain.Period
		want  bool
	}{
		{"ends when the other starts", on(20, 9, 0, 10, 0), false},
		{"starts when the other ends", on(20, 11, 0, 12, 0), false},
		{"shares one step at the start", on(20, 9, 0, 10, 15), true},
		{"shares one step at the end", on(20, 10, 45, 12, 0), true},
		{"lies inside the other", on(20, 10, 15, 10, 45), true},
		{"holds the other", on(20, 9, 0, 12, 0), true},
		{"is the same", base, true},
		{"is on another day", on(21, 10, 0, 11, 0), false},
	}
	for _, tt := range tests {
		if got := base.Overlaps(tt.other); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
		if got := tt.other.Overlaps(base); got != tt.want {
			t.Errorf("%s, the other way round: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestTimeOfDay(t *testing.T) {
	t.Parallel()
	tod := domain.NewTimeOfDay(9, 5)
	if tod.Hour() != 9 || tod.Minute() != 5 || tod.String() != "09:05" {
		t.Errorf("got %d:%d (%s), want 9:5 (09:05)", tod.Hour(), tod.Minute(), tod)
	}
}

func TestIDText(t *testing.T) {
	t.Parallel()
	if got, want := roomID.String(), "0193a6f0-7b5c-7e2a-9c3d-1f2e3d4c5b6a"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if got, want := reservationID.String(), "0193a6f1-0000-7000-8000-000000000001"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestStatusText(t *testing.T) {
	t.Parallel()
	for status, want := range map[domain.Status]string{
		domain.StatusConfirmed: "confirmed",
		domain.StatusCancelled: "cancelled",
		domain.Status(0):       "Status(0)",
	} {
		if got := status.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

// Every kind is listed once, so that the tests that walk Kinds see each.
func TestKinds(t *testing.T) {
	t.Parallel()
	seen := map[domain.Kind]bool{}
	for i, kind := range domain.Kinds() {
		if kind != domain.Kind(i+1) || seen[kind] {
			t.Errorf("Kinds()[%d] is %d: list every kind once, in the order of their values", i, kind)
		}
		seen[kind] = true
	}
}
