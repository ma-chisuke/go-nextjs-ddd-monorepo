// Package postgres is the database side of the Reservation context: its
// schema (schema.sql), its queries (queries/), the code that sqlc generates
// from them (*.gen.go), the store that implements the port of the use cases
// over them, and the outbox from which the relay publishes the events of the
// context (outbox.go). Generated types do not leave this package: the store
// turns rows into domain objects and back (TPL-0011), and domain events into
// integration events (events.go).
package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// The SQLSTATEs and the constraints that the store turns into the errors of
// the port.
const (
	exclusionViolation = "23P01"
	uniqueViolation    = "23505"
	noOverlap          = "reservations_no_overlap"
	uniqueKey          = "idempotency_keys_key_key"
)

// The values of the column status.
const (
	statusConfirmed = "confirmed"
	statusCancelled = "cancelled"
)

// Store implements app.Store over PostgreSQL, connected as the runtime role
// of Reservation.
type Store struct {
	pool    *pgxpool.Pool
	queries *Queries
}

var _ app.Store = (*Store)(nil)

// NewStore returns the store over pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, queries: New(pool)}
}

// Add stores the reservation, its events, and its idempotency key in one
// transaction. The exclusion constraint refuses an overlapping confirmed
// reservation, and the unique constraint a key that is taken; when two
// requests race, the database makes the second wait for the first and then
// refuses it.
func (s *Store) Add(ctx context.Context, r domain.Reservation, events []domain.Event, key app.IdempotencyKey, at time.Time) (app.Version, error) {
	params, err := insertParams(r, at)
	if err != nil {
		return 0, err
	}
	outboxed, err := outboxEvents(events, at)
	if err != nil {
		return 0, err
	}
	var version int32
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		if version, err = q.InsertReservation(ctx, params); err != nil {
			return err
		}
		err = q.InsertIdempotencyKey(ctx, InsertIdempotencyKeyParams{
			ReservationID: uuid.UUID(key.Reservation),
			Key:           key.Key,
			Fingerprint:   key.Fingerprint,
			CreatedAt:     at,
		})
		if err != nil {
			return err
		}
		return insertEvents(ctx, q, outboxed)
	})
	if err != nil {
		return 0, portError(err)
	}
	return app.Version(version), nil
}

// Get returns the reservation with the id.
func (s *Store) Get(ctx context.Context, id domain.ReservationID) (app.Versioned, error) {
	row, err := s.queries.GetReservation(ctx, uuid.UUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Versioned{}, app.ErrNotFound
	}
	if err != nil {
		return app.Versioned{}, fmt.Errorf("get reservation: %w", err)
	}
	return versioned(row)
}

// Update replaces the reservation if it is still at version read, and stores
// its events, in one transaction.
func (s *Store) Update(ctx context.Context, r domain.Reservation, events []domain.Event, read app.Version, at time.Time) (app.Version, error) {
	readVersion, err := int32Of(int(read))
	if err != nil {
		return 0, err
	}
	status, err := statusText(r.Status())
	if err != nil {
		return 0, err
	}
	outboxed, err := outboxEvents(events, at)
	if err != nil {
		return 0, err
	}
	var version int32
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		var err error
		version, err = q.UpdateReservation(ctx, UpdateReservationParams{
			StartsAt:    r.Period().Start,
			EndsAt:      r.Period().End,
			Attendees:   r.Attendees(),
			Status:      status,
			At:          at,
			ID:          uuid.UUID(r.ID()),
			ReadVersion: readVersion,
		})
		if err != nil {
			return err
		}
		return insertEvents(ctx, q, outboxed)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// The reservation exists, because the use case read it: another
		// change came first.
		return 0, app.ErrVersionConflict
	}
	if err != nil {
		return 0, portError(err)
	}
	return app.Version(version), nil
}

// Overlaps reports whether a confirmed reservation of the room, other than
// except, overlaps the period.
func (s *Store) Overlaps(ctx context.Context, room domain.RoomID, period domain.Period, except domain.ReservationID) (bool, error) {
	overlaps, err := s.queries.Overlaps(ctx, OverlapsParams{
		RoomID:   uuid.UUID(room),
		ExceptID: uuid.UUID(except),
		StartsAt: period.Start,
		EndsAt:   period.End,
	})
	if err != nil {
		return false, fmt.Errorf("overlaps: %w", err)
	}
	return overlaps, nil
}

// List returns the reservations that q selects, in the order of (start, id).
func (s *Store) List(ctx context.Context, q app.ListQuery) ([]domain.Reservation, error) {
	limit, err := int32Of(q.Limit)
	if err != nil {
		return nil, err
	}
	params := ListReservationsParams{
		AfterStartsAt: q.AfterStart,
		AfterID:       uuid.UUID(q.AfterID),
		WindowFrom:    q.From,
		WindowTo:      q.To,
		PageSize:      limit,
	}
	if q.Room != nil {
		room := uuid.UUID(*q.Room)
		params.RoomID = &room
	}
	rows, err := s.queries.ListReservations(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list reservations: %w", err)
	}
	page := make([]domain.Reservation, 0, len(rows))
	for _, row := range rows {
		v, err := versioned(GetReservationRow(row))
		if err != nil {
			return nil, err
		}
		page = append(page, v.Reservation)
	}
	return page, nil
}

// FindKey returns the idempotency key.
func (s *Store) FindKey(ctx context.Context, key string) (app.IdempotencyKey, error) {
	row, err := s.queries.GetIdempotencyKey(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.IdempotencyKey{}, app.ErrNotFound
	}
	if err != nil {
		return app.IdempotencyKey{}, fmt.Errorf("find idempotency key: %w", err)
	}
	return app.IdempotencyKey{
		Key:         row.Key,
		Fingerprint: row.Fingerprint,
		Reservation: domain.ReservationID(row.ReservationID),
	}, nil
}

// portError turns the refusals of the constraints into the errors of the
// port.
func portError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == exclusionViolation && pgErr.ConstraintName == noOverlap:
			return app.ErrOverlap
		case pgErr.Code == uniqueViolation && pgErr.ConstraintName == uniqueKey:
			return app.ErrKeyTaken
		}
	}
	return fmt.Errorf("store reservation: %w", err)
}

func insertParams(r domain.Reservation, at time.Time) (InsertReservationParams, error) {
	status, err := statusText(r.Status())
	if err != nil {
		return InsertReservationParams{}, err
	}
	return InsertReservationParams{
		ID:             uuid.UUID(r.ID()),
		RoomID:         uuid.UUID(r.Room()),
		StartsAt:       r.Period().Start,
		EndsAt:         r.Period().End,
		Attendees:      r.Attendees(),
		OrganizerEmail: r.Organizer(),
		Status:         status,
		At:             at,
	}, nil
}

// versioned turns a row into a reservation and its version.
func versioned(row GetReservationRow) (app.Versioned, error) {
	status, err := statusFrom(row.Status)
	if err != nil {
		return app.Versioned{}, err
	}
	r := domain.Restore(
		domain.ReservationID(row.ID),
		domain.RoomID(row.RoomID),
		domain.Period{Start: row.StartsAt, End: row.EndsAt},
		row.Attendees,
		row.OrganizerEmail,
		status,
	)
	return app.Versioned{Reservation: r, Version: app.Version(row.Version)}, nil
}

func statusText(s domain.Status) (string, error) {
	switch s {
	case domain.StatusConfirmed:
		return statusConfirmed, nil
	case domain.StatusCancelled:
		return statusCancelled, nil
	}
	return "", fmt.Errorf("store reservation: the status %s has no column value", s)
}

func statusFrom(text string) (domain.Status, error) {
	switch text {
	case statusConfirmed:
		return domain.StatusConfirmed, nil
	case statusCancelled:
		return domain.StatusCancelled, nil
	}
	return 0, fmt.Errorf("read reservation: the status %q is not known", text)
}

// int32Of returns n as an int32, the type of an integer column.
func int32Of(n int) (int32, error) {
	if n < math.MinInt32 || n > math.MaxInt32 {
		return 0, fmt.Errorf("store reservation: %d does not fit in an integer column", n)
	}
	return int32(n), nil
}
