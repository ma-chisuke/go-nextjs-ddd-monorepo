package app

import (
	"context"
	"errors"
	"time"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// The errors that the ports return. A use case turns each into a business
// error of its own, or passes it on wrapped.
var (
	// ErrNotFound: the store holds no such reservation or idempotency key.
	ErrNotFound = errors.New("not found")
	// ErrOverlap: the store refused a confirmed reservation because another
	// confirmed reservation of the room overlaps its period.
	ErrOverlap = errors.New("another confirmed reservation of the room overlaps the period")
	// ErrVersionConflict: the reservation is no longer at the version that
	// the change read.
	ErrVersionConflict = errors.New("the reservation changed since it was read")
	// ErrKeyTaken: the store holds the idempotency key already.
	ErrKeyTaken = errors.New("the idempotency key is stored already")
	// ErrUnknownRoom: the directory of rooms knows no room with the id.
	ErrUnknownRoom = errors.New("the room does not exist")
	// ErrRoomsUnavailable: the directory of rooms did not answer in time,
	// or gave an answer that cannot be used.
	ErrRoomsUnavailable = errors.New("the directory of rooms does not answer")
)

// Store keeps reservations, with their versions and the idempotency keys of
// the requests that booked them. It refuses two confirmed reservations of one
// room whose periods overlap, even when two requests store them at the same
// time.
type Store interface {
	// Add stores a new reservation at version 1, together with the
	// idempotency key of the request that booked it, all or nothing. It
	// returns ErrOverlap or ErrKeyTaken when it refuses the reservation.
	Add(ctx context.Context, r domain.Reservation, key IdempotencyKey, at time.Time) (Version, error)
	// Get returns the reservation with the id, or ErrNotFound.
	Get(ctx context.Context, id domain.ReservationID) (Versioned, error)
	// Update replaces the reservation if the store holds it at version read,
	// and returns the new version. It returns ErrVersionConflict when the
	// reservation is at another version, and ErrOverlap when it refuses the
	// new period.
	Update(ctx context.Context, r domain.Reservation, read Version, at time.Time) (Version, error)
	// Overlaps reports whether a confirmed reservation of the room, other
	// than except, overlaps the period.
	Overlaps(ctx context.Context, room domain.RoomID, period domain.Period, except domain.ReservationID) (bool, error)
	// List returns the reservations that q selects, in the order of their
	// start and then of their id.
	List(ctx context.Context, q ListQuery) ([]domain.Reservation, error)
	// FindKey returns the idempotency key, or ErrNotFound.
	FindKey(ctx context.Context, key string) (IdempotencyKey, error)
}

// IdempotencyKey is the key that a client sent with a booking, stored with
// the fingerprint of the request and the reservation that it booked. A
// request that comes again with the key gets that reservation, and books
// nothing new.
type IdempotencyKey struct {
	Key string
	// Fingerprint identifies the content of the request, so that a key that
	// comes again with another request can be told apart from a repeat.
	Fingerprint string
	Reservation domain.ReservationID
}

// ListQuery selects reservations. A field that is nil selects everything.
type ListQuery struct {
	// Room selects the reservations of one room.
	Room *domain.RoomID
	// From selects the reservations that end after it.
	From *time.Time
	// To selects the reservations that start before it.
	To *time.Time
	// AfterStart and AfterID select the reservations that come after this
	// position in the order of the list. Their zero values select from the
	// beginning.
	AfterStart time.Time
	AfterID    domain.ReservationID
	// Limit is the largest number of reservations to return.
	Limit int
}

// RoomDirectory tells what Reservation needs to know about a room. The
// Facility context owns rooms, and an adapter asks its API.
type RoomDirectory interface {
	// Room returns the room with the id. It returns ErrUnknownRoom when no
	// room has the id, and ErrRoomsUnavailable when it cannot tell.
	Room(ctx context.Context, id domain.RoomID) (domain.Room, error)
}

// Clock tells the current time.
type Clock interface {
	Now() time.Time
}

// ClockFunc makes a function a Clock.
type ClockFunc func() time.Time

// Now returns f().
func (f ClockFunc) Now() time.Time { return f() }

// IDGenerator makes the ids of new reservations: UUIDs of version 7
// (TPL-0012).
type IDGenerator interface {
	NewReservationID() (domain.ReservationID, error)
}

// IDGeneratorFunc makes a function an IDGenerator.
type IDGeneratorFunc func() (domain.ReservationID, error)

// NewReservationID returns f().
func (f IDGeneratorFunc) NewReservationID() (domain.ReservationID, error) { return f() }
