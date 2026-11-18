// Package apptest provides in-memory stand-ins for the ports of the
// Reservation context, for tests that run without a database or the
// Facility API. The store keeps the rules of the database that the use cases
// rely on: versions, the exclusion of overlapping confirmed reservations,
// unique idempotency keys, and the order of a listing. The tests against a
// real database (build tag integration) check the database itself. The store
// also keeps the events of each change that it accepted, so that a test can
// see which events a use case stored.
package apptest

import (
	"cmp"
	"context"
	"encoding/binary"
	"slices"
	"sync"
	"time"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// Store implements app.Store in memory. The zero value is an empty store.
type Store struct {
	mu     sync.Mutex
	rows   []row
	keys   map[string]app.IdempotencyKey
	events []domain.Event
}

type row struct {
	reservation domain.Reservation
	version     app.Version
}

var _ app.Store = (*Store)(nil)

// Add stores a new reservation at version 1 with its events and its
// idempotency key, and refuses it as the database would.
func (s *Store) Add(_ context.Context, r domain.Reservation, events []domain.Event, key app.IdempotencyKey, _ time.Time) (app.Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.overlaps(r.Room(), r.Period(), r.ID()) && r.Status() == domain.StatusConfirmed {
		return 0, app.ErrOverlap
	}
	if _, taken := s.keys[key.Key]; taken {
		return 0, app.ErrKeyTaken
	}
	if s.keys == nil {
		s.keys = map[string]app.IdempotencyKey{}
	}
	s.keys[key.Key] = key
	s.rows = append(s.rows, row{reservation: r, version: 1})
	s.events = append(s.events, events...)
	return 1, nil
}

// Get returns the reservation with the id, or app.ErrNotFound.
func (s *Store) Get(_ context.Context, id domain.ReservationID) (app.Versioned, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, stored := range s.rows {
		if stored.reservation.ID() == id {
			return app.Versioned{Reservation: stored.reservation, Version: stored.version}, nil
		}
	}
	return app.Versioned{}, app.ErrNotFound
}

// Update replaces the reservation at version read and stores the events of
// the change, and refuses it as the database would.
func (s *Store) Update(_ context.Context, r domain.Reservation, events []domain.Event, read app.Version, _ time.Time) (app.Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, stored := range s.rows {
		if stored.reservation.ID() != r.ID() {
			continue
		}
		if stored.version != read {
			return 0, app.ErrVersionConflict
		}
		if r.Status() == domain.StatusConfirmed && s.overlaps(r.Room(), r.Period(), r.ID()) {
			return 0, app.ErrOverlap
		}
		s.rows[i] = row{reservation: r, version: read + 1}
		s.events = append(s.events, events...)
		return read + 1, nil
	}
	return 0, app.ErrVersionConflict
}

// Events returns the events that the store keeps, in the order in which the
// changes that they report were stored.
func (s *Store) Events() []domain.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.events)
}

// Overlaps reports whether a confirmed reservation of the room, other than
// except, overlaps the period.
func (s *Store) Overlaps(_ context.Context, room domain.RoomID, period domain.Period, except domain.ReservationID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.overlaps(room, period, except), nil
}

func (s *Store) overlaps(room domain.RoomID, period domain.Period, except domain.ReservationID) bool {
	for _, stored := range s.rows {
		r := stored.reservation
		if r.ID() != except && r.Room() == room && r.Status() == domain.StatusConfirmed && r.Period().Overlaps(period) {
			return true
		}
	}
	return false
}

// List returns the reservations that q selects, in the order of (start, id).
func (s *Store) List(_ context.Context, q app.ListQuery) ([]domain.Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sorted := make([]domain.Reservation, 0, len(s.rows))
	for _, stored := range s.rows {
		sorted = append(sorted, stored.reservation)
	}
	slices.SortFunc(sorted, compare)
	after := domain.Restore(q.AfterID, domain.RoomID{}, domain.Period{Start: q.AfterStart}, 0, "", 0)
	page := []domain.Reservation{}
	for _, r := range sorted {
		selected := compare(r, after) > 0 &&
			(q.Room == nil || r.Room() == *q.Room) &&
			(q.From == nil || r.Period().End.After(*q.From)) &&
			(q.To == nil || r.Period().Start.Before(*q.To))
		if selected && len(page) < q.Limit {
			page = append(page, r)
		}
	}
	return page, nil
}

// compare orders reservations by (start, id), as the database does.
func compare(a, b domain.Reservation) int {
	ida, idb := a.ID(), b.ID()
	return cmp.Or(a.Period().Start.Compare(b.Period().Start), slices.Compare(ida[:], idb[:]))
}

// FindKey returns the idempotency key, or app.ErrNotFound.
func (s *Store) FindKey(_ context.Context, key string) (app.IdempotencyKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.keys[key]
	if !ok {
		return app.IdempotencyKey{}, app.ErrNotFound
	}
	return stored, nil
}

// Rooms implements app.RoomDirectory over a fixed set of rooms. Set Err to
// make every lookup fail with it, as when Facility does not answer.
type Rooms struct {
	mu    sync.Mutex
	rooms map[domain.RoomID]domain.Room
	err   error
}

var _ app.RoomDirectory = (*Rooms)(nil)

// NewRooms returns a directory of the given rooms.
func NewRooms(rooms ...domain.Room) *Rooms {
	d := &Rooms{rooms: map[domain.RoomID]domain.Room{}}
	for _, room := range rooms {
		d.rooms[room.ID] = room
	}
	return d
}

// Fail makes every lookup from now on return err; nil ends the failure.
func (d *Rooms) Fail(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.err = err
}

// Room returns the room with the id, or app.ErrUnknownRoom.
func (d *Rooms) Room(_ context.Context, id domain.RoomID) (domain.Room, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.err != nil {
		return domain.Room{}, d.err
	}
	room, ok := d.rooms[id]
	if !ok {
		return domain.Room{}, app.ErrUnknownRoom
	}
	return room, nil
}

// Clock implements app.Clock with a time that only the test moves.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

var _ app.Clock = (*Clock)(nil)

// NewClock returns a clock that shows now.
func NewClock(now time.Time) *Clock {
	return &Clock{now: now}
}

// Now returns the time that the clock shows.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Set moves the clock to now.
func (c *Clock) Set(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = now
}

// IDs implements app.IDGenerator with ids that count up from 1, in the
// layout of a UUID version 7, so that a test can tell them apart and knows
// their order.
type IDs struct {
	mu   sync.Mutex
	last uint64
}

var _ app.IDGenerator = (*IDs)(nil)

// NewReservationID returns the next id.
func (g *IDs) NewReservationID() (domain.ReservationID, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.last++
	return ID(g.last), nil
}

// ID returns the n-th id that IDs makes.
func ID(n uint64) domain.ReservationID {
	var id domain.ReservationID
	id[6] = 0x70                                // version 7
	binary.BigEndian.PutUint64(id[8:], n|1<<63) // variant of RFC 9562
	return id
}
