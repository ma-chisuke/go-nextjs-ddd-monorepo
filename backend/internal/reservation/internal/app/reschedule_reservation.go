package app

import (
	"context"
	"time"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// RescheduleReservationInput is what RescheduleReservation needs. A change
// that is nil keeps the value of the reservation.
type RescheduleReservationInput struct {
	Reservation domain.ReservationID
	// Read is the version of the reservation that the client read.
	Read      Version
	Start     *time.Time
	End       *time.Time
	Attendees *int32
}

// RescheduleReservation changes the period or the number of attendees of a
// confirmed reservation, under the same rules as a booking, provided that the
// reservation did not change since the client read it. The change is stored
// with the event ReservationConfirmed, which carries the new period and
// attendees.
func (s *Reservations) RescheduleReservation(ctx context.Context, in RescheduleReservationInput) (Versioned, error) {
	current, err := s.load(ctx, in.Reservation, in.Read)
	if err != nil {
		return Versioned{}, err
	}
	r := current.Reservation
	period := r.Period()
	if in.Start != nil {
		period.Start = *in.Start
	}
	if in.End != nil {
		period.End = *in.End
	}
	attendees := r.Attendees()
	if in.Attendees != nil {
		attendees = *in.Attendees
	}
	room, err := s.room(ctx, r.Room())
	if err != nil {
		return Versioned{}, err
	}
	now := s.clock.Now()
	changed, events, err := r.Reschedule(room, period, attendees, now)
	if err != nil {
		return Versioned{}, err
	}
	if err := s.checkFree(ctx, changed); err != nil {
		return Versioned{}, err
	}
	return s.update(ctx, changed, events, in.Read, now)
}
