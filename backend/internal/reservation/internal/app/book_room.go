package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// BookRoomInput is what BookRoom needs.
type BookRoomInput struct {
	// IdempotencyKey is chosen by the client once for each booking. A
	// request that comes again with the key gets the reservation of the
	// first one.
	IdempotencyKey string
	Room           domain.RoomID
	Period         domain.Period
	Attendees      int
	// Organizer is the email address of the person who books the room.
	Organizer string
}

// BookRoom books a room. The domain applies the rules of a booking to the
// room as Facility describes it, and the store refuses a period that another
// confirmed reservation of the room overlaps. The new reservation is stored
// with the event ReservationConfirmed.
//
// A request whose idempotency key was stored before gets the reservation of
// that key, provided that it is the same request; nothing new is booked, and
// no event is stored.
func (s *Reservations) BookRoom(ctx context.Context, in BookRoomInput) (Versioned, error) {
	fingerprint, err := in.fingerprint()
	if err != nil {
		return Versioned{}, fmt.Errorf("book room: %w", err)
	}
	if v, found, err := s.repeated(ctx, in.IdempotencyKey, fingerprint); found || err != nil {
		return v, err
	}
	room, err := s.room(ctx, in.Room)
	if err != nil {
		return Versioned{}, err
	}
	id, err := s.ids.NewReservationID()
	if err != nil {
		return Versioned{}, fmt.Errorf("book room: new id: %w", err)
	}
	now := s.clock.Now()
	r, events, err := domain.Book(id, room, in.Period, in.Attendees, in.Organizer, now)
	if err != nil {
		return Versioned{}, err
	}
	if err := s.checkFree(ctx, r); err != nil {
		return Versioned{}, err
	}
	key := IdempotencyKey{Key: in.IdempotencyKey, Fingerprint: fingerprint, Reservation: id}
	version, err := s.store.Add(ctx, r, events, key, now)
	if errors.Is(err, ErrOverlap) || errors.Is(err, ErrKeyTaken) {
		// Another request was stored between the checks above and this one.
		// If it came with the same key, this request repeats it.
		v, found, repeatErr := s.repeated(ctx, in.IdempotencyKey, fingerprint)
		switch {
		case found || repeatErr != nil:
			return v, repeatErr
		case errors.Is(err, ErrOverlap):
			return Versioned{}, alreadyBooked(r)
		}
	}
	if err != nil {
		return Versioned{}, fmt.Errorf("book room: %w", err)
	}
	return Versioned{Reservation: r, Version: version}, nil
}

// repeated looks for the idempotency key. found is false when the key is new.
// When the key came before with the same request, v is the reservation that
// it booked; with another request, err says that the key was reused.
func (s *Reservations) repeated(ctx context.Context, key, fingerprint string) (v Versioned, found bool, err error) {
	stored, err := s.store.FindKey(ctx, key)
	if errors.Is(err, ErrNotFound) {
		return Versioned{}, false, nil
	}
	if err != nil {
		return Versioned{}, true, fmt.Errorf("book room: idempotency key: %w", err)
	}
	if stored.Fingerprint != fingerprint {
		return Versioned{}, true, newError(KindIdempotencyKeyReused,
			"The Idempotency-Key came before with another request. Use a new key for each booking.")
	}
	v, err = s.store.Get(ctx, stored.Reservation)
	if err != nil {
		return Versioned{}, true, fmt.Errorf("book room: the reservation of the idempotency key: %w", err)
	}
	return v, true, nil
}

// fingerprint identifies the content of the request: two requests with the
// same fingerprint book the same room, for the same instants, for the same
// attendees and organizer.
func (in BookRoomInput) fingerprint() (string, error) {
	content, err := json.Marshal(struct {
		Room      uuid.UUID `json:"room"`
		Start     time.Time `json:"start"`
		End       time.Time `json:"end"`
		Attendees int       `json:"attendees"`
		Organizer string    `json:"organizer"`
	}{uuid.UUID(in.Room), in.Period.Start.UTC(), in.Period.End.UTC(), in.Attendees, in.Organizer})
	if err != nil {
		return "", fmt.Errorf("fingerprint: %w", err)
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}
