package domain

import "time"

// Event is something that happened to a reservation and that other parts of
// the business may act on, such as sending a notice to the organizer. Only
// this package makes events.
type Event interface {
	event()
}

// ReservationConfirmed says that a reservation holds its room for its period,
// with its attendees. It happens when a room is booked, and again when the
// reservation is rescheduled, with the new period and attendees.
type ReservationConfirmed struct {
	Reservation ReservationID
	Room        RoomID
	Period      Period
	Attendees   int
	Organizer   string
	// OccurredAt is when the reservation was confirmed.
	OccurredAt time.Time
}

// ReservationCancelled says that a reservation was cancelled, and that its
// room is free for its period.
type ReservationCancelled struct {
	Reservation ReservationID
	Room        RoomID
	Period      Period
	Organizer   string
	// OccurredAt is when the reservation was cancelled.
	OccurredAt time.Time
}

func (ReservationConfirmed) event() {}

func (ReservationCancelled) event() {}
