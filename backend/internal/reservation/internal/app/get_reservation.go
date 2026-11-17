package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// GetReservation returns the reservation with the id, with its version.
func (s *Reservations) GetReservation(ctx context.Context, id domain.ReservationID) (Versioned, error) {
	v, err := s.store.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Versioned{}, newError(KindReservationNotFound, fmt.Sprintf("No reservation has the id %s.", id))
	}
	if err != nil {
		return Versioned{}, fmt.Errorf("get reservation %s: %w", id, err)
	}
	return v, nil
}
