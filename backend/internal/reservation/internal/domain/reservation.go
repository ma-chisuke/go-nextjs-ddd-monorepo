package domain

import (
	"errors"
	"fmt"
	"time"
)

// Status says whether a reservation holds its room.
type Status int

// The statuses of a reservation.
const (
	// StatusConfirmed: the reservation holds its room for its period.
	StatusConfirmed Status = iota + 1
	// StatusCancelled: the reservation was cancelled, and holds nothing.
	StatusCancelled
)

// String writes the status in one lowercase word.
func (s Status) String() string {
	switch s {
	case StatusConfirmed:
		return "confirmed"
	case StatusCancelled:
		return "cancelled"
	}
	return fmt.Sprintf("Status(%d)", int(s))
}

// Reservation is the booking of a meeting room for a period, for a number of
// attendees, made by an organizer. It is the aggregate of this context: every
// change goes through its methods, which apply the rules and return the
// changed reservation, without changing the one they are called on.
//
// The store adds one rule that a single reservation cannot check: two
// confirmed reservations of one room never overlap.
type Reservation struct {
	id        ReservationID
	room      RoomID
	period    Period
	attendees int32
	organizer string
	status    Status
}

// ID returns the id of the reservation.
func (r Reservation) ID() ReservationID { return r.id }

// Room returns the id of the reserved room.
func (r Reservation) Room() RoomID { return r.room }

// Period returns when the reservation holds the room.
func (r Reservation) Period() Period { return r.period }

// Attendees returns how many people attend, the organizer included.
func (r Reservation) Attendees() int32 { return r.attendees }

// Organizer returns the email address of the person who booked the room.
func (r Reservation) Organizer() string { return r.organizer }

// Status returns whether the reservation holds its room.
func (r Reservation) Status() Status { return r.status }

// Book books room for period, for attendees people, on behalf of organizer.
// id is the id of the new reservation, and now is the current time.
//
// The period starts and ends on a quarter hour, lasts at most LongestPeriod,
// and does not start before now. The attendees fit the capacity of the room,
// and the room is open for the whole period. Book does not know the other
// reservations of the room; the use case checks that none overlaps.
func Book(id ReservationID, room Room, period Period, attendees int32, organizer string, now time.Time) (Reservation, []Event, error) {
	if err := checkTerms(room, period, attendees, now); err != nil {
		return Reservation{}, nil, err
	}
	r := Reservation{
		id:        id,
		room:      room.ID,
		period:    period,
		attendees: attendees,
		organizer: organizer,
		status:    StatusConfirmed,
	}
	return r, []Event{r.confirmed(now)}, nil
}

// Reschedule moves the reservation to period and changes the number of its
// attendees, under the same rules as Book. room is the room of the
// reservation. A cancelled reservation cannot be rescheduled.
func (r Reservation) Reschedule(room Room, period Period, attendees int32, now time.Time) (Reservation, []Event, error) {
	if r.status == StatusCancelled {
		return Reservation{}, nil, newError(KindReservationCancelled, "The reservation is cancelled, so it cannot be rescheduled.")
	}
	if room.ID != r.room {
		return Reservation{}, nil, errors.New("domain: reschedule: the room is not the room of the reservation")
	}
	if err := checkTerms(room, period, attendees, now); err != nil {
		return Reservation{}, nil, err
	}
	changed := r
	changed.period = period
	changed.attendees = attendees
	return changed, []Event{changed.confirmed(now)}, nil
}

// Cancel cancels the reservation at now, which is at the latest
// CancellationNotice before the reservation starts. The room is free for the
// period from then on. A cancelled reservation cannot be cancelled again.
func (r Reservation) Cancel(now time.Time) (Reservation, []Event, error) {
	if r.status == StatusCancelled {
		return Reservation{}, nil, newError(KindReservationCancelled, "The reservation is cancelled already.")
	}
	if deadline := r.period.Start.Add(-CancellationNotice); now.After(deadline) {
		return Reservation{}, nil, newError(KindCancellationTooLate, fmt.Sprintf(
			"A reservation can be cancelled until %s before it starts, and that was at %s.",
			describe(CancellationNotice), deadline.UTC().Format(time.RFC3339)))
	}
	cancelled := r
	cancelled.status = StatusCancelled
	return cancelled, []Event{ReservationCancelled{
		Reservation: r.id,
		Room:        r.room,
		Period:      r.period,
		Organizer:   r.organizer,
		OccurredAt:  now,
	}}, nil
}

// Restore rebuilds a reservation that was booked before, from what the store
// holds. It checks no rule: the rules held when the reservation was booked or
// last changed, and a rule that changed since does not undo a booking.
func Restore(id ReservationID, room RoomID, period Period, attendees int32, organizer string, status Status) Reservation {
	return Reservation{
		id:        id,
		room:      room,
		period:    period,
		attendees: attendees,
		organizer: organizer,
		status:    status,
	}
}

func (r Reservation) confirmed(now time.Time) ReservationConfirmed {
	return ReservationConfirmed{
		Reservation: r.id,
		Room:        r.room,
		Period:      r.period,
		Attendees:   r.attendees,
		Organizer:   r.organizer,
		OccurredAt:  now,
	}
}

// checkTerms applies the rules that a booking and a rescheduling share, and
// returns the first rule that the terms break.
func checkTerms(room Room, period Period, attendees int32, now time.Time) error {
	switch {
	case !period.Start.Before(period.End):
		return newError(KindPeriodOutOfOrder, "The period must end after it starts.")
	case !onStep(period.Start) || !onStep(period.End):
		return newError(KindPeriodMisaligned, fmt.Sprintf(
			"The period must start and end on a multiple of %s, such as 10:00, 10:15, or 10:30.", describe(PeriodStep)))
	case period.Length() > LongestPeriod:
		return newError(KindPeriodTooLong, fmt.Sprintf(
			"The period lasts %s, and a reservation lasts at most %s.", describe(period.Length()), describe(LongestPeriod)))
	case period.Start.Before(now):
		return newError(KindPeriodInPast, "The period starts in the past.")
	case attendees < 1:
		return newError(KindNoAttendees, "A reservation has at least 1 attendee.")
	case attendees > room.Capacity:
		return newError(KindOverCapacity, fmt.Sprintf(
			"The room holds %d people, and %d attend.", room.Capacity, attendees))
	case !room.isOpenDuring(period):
		return newError(KindOutsideOpeningHours, fmt.Sprintf(
			"The room is open from %s, and the period must lie inside these hours on one day.", room.describeHours()))
	}
	return nil
}
