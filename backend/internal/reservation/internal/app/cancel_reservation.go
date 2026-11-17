package app

import (
	"context"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// CancelReservation cancels a confirmed reservation, at the latest
// domain.CancellationNotice before it starts, provided that it did not change
// since the client read it at version read. The room is free for the period
// from then on.
func (s *Reservations) CancelReservation(ctx context.Context, id domain.ReservationID, read Version) (Versioned, error) {
	current, err := s.load(ctx, id, read)
	if err != nil {
		return Versioned{}, err
	}
	now := s.clock.Now()
	// The events reach the outbox from the milestone that adds events
	// (TPL-0013). Until then nothing publishes them.
	cancelled, _, err := current.Reservation.Cancel(now)
	if err != nil {
		return Versioned{}, err
	}
	return s.update(ctx, cancelled, read, now)
}
