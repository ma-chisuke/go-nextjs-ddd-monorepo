// Package app holds the use cases of the Reservation context, one file for
// each business operation, and the ports through which they reach the world:
// the store of reservations, the directory of rooms, the clock, and the
// generator of ids (TPL-0004).
//
// A use case loads what the domain needs, asks the domain to apply the rules,
// and stores the result together with the events that the domain returned;
// other contexts learn of a change through these events. The rules
// themselves are in the domain package; the errors that belong to a use case,
// such as a reservation that does not exist, are the kinds in errors.go.
package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// Version counts the changes of a reservation, starting at 1. A change names
// the version that it read, and fails when the reservation changed since, so
// that no change overwrites another one unseen.
type Version int

// Versioned is a reservation as the store holds it, with its version.
type Versioned struct {
	Reservation domain.Reservation
	Version     Version
}

// Reservations holds the use cases of the context.
type Reservations struct {
	store Store
	rooms RoomDirectory
	clock Clock
	ids   IDGenerator
}

// New returns the use cases over the given ports.
func New(store Store, rooms RoomDirectory, clock Clock, ids IDGenerator) *Reservations {
	return &Reservations{store: store, rooms: rooms, clock: clock, ids: ids}
}

// room asks the directory for the room with the id.
func (s *Reservations) room(ctx context.Context, id domain.RoomID) (domain.Room, error) {
	room, err := s.rooms.Room(ctx, id)
	switch {
	case errors.Is(err, ErrUnknownRoom):
		return domain.Room{}, newError(KindUnknownRoom, fmt.Sprintf("No room has the id %s.", id))
	case errors.Is(err, ErrRoomsUnavailable):
		return domain.Room{}, &Error{
			Kind:   KindFacilityUnavailable,
			Detail: "The rooms cannot be checked now. Try again later.",
			Err:    err,
		}
	case err != nil:
		return domain.Room{}, fmt.Errorf("room %s: %w", id, err)
	}
	return room, nil
}

// checkFree checks that no other confirmed reservation of the room overlaps
// the period of r. The store checks again when it stores r, because another
// request may store an overlapping reservation in between; this check gives
// the common case a clear answer early.
func (s *Reservations) checkFree(ctx context.Context, r domain.Reservation) error {
	taken, err := s.store.Overlaps(ctx, r.Room(), r.Period(), r.ID())
	if err != nil {
		return fmt.Errorf("overlap: %w", err)
	}
	if taken {
		return alreadyBooked(r)
	}
	return nil
}

func alreadyBooked(r domain.Reservation) *Error {
	return newError(KindRoomAlreadyBooked, fmt.Sprintf(
		"Another reservation of the room overlaps the period from %s to %s.",
		r.Period().Start.UTC().Format(time.RFC3339), r.Period().End.UTC().Format(time.RFC3339)))
}

// load returns the reservation with the id, provided that it is still at the
// version that the client read.
func (s *Reservations) load(ctx context.Context, id domain.ReservationID, read Version) (Versioned, error) {
	v, err := s.GetReservation(ctx, id)
	if err != nil {
		return Versioned{}, err
	}
	if v.Version != read {
		return Versioned{}, versionMismatch(v.Version, read)
	}
	return v, nil
}

// update stores the changed reservation and the events of the change,
// provided that the reservation is still at the version that the client read.
func (s *Reservations) update(ctx context.Context, r domain.Reservation, events []domain.Event, read Version, now time.Time) (Versioned, error) {
	version, err := s.store.Update(ctx, r, events, read, now)
	switch {
	case errors.Is(err, ErrVersionConflict):
		return Versioned{}, newError(KindVersionMismatch, "The reservation changed since it was read. Read it again.")
	case errors.Is(err, ErrOverlap):
		return Versioned{}, alreadyBooked(r)
	case err != nil:
		return Versioned{}, fmt.Errorf("update %s: %w", r.ID(), err)
	}
	return Versioned{Reservation: r, Version: version}, nil
}

func versionMismatch(current, read Version) *Error {
	return newError(KindVersionMismatch, fmt.Sprintf(
		"The reservation is at version %d, and the request names version %d. Read it again.", current, read))
}
