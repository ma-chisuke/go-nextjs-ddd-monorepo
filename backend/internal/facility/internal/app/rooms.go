// Package app holds the use cases of the Facility context.
//
// Facility is a supporting subdomain: its rules are few and simple. So each
// use case is a transaction script that checks its input and calls the
// generated queries directly, and the context has no domain layer (TPL-0003).
package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/adapter/postgres"
)

// Room is a meeting room that can be booked.
type Room struct {
	ID       uuid.UUID
	Name     string
	Capacity int
	// OpenFrom and OpenUntil are the times of day between which the room can
	// be used, read in TimeZone.
	OpenFrom  TimeOfDay
	OpenUntil TimeOfDay
	// TimeZone is the name of an IANA time zone.
	TimeZone  string
	CreatedAt time.Time
}

// Rooms holds the use cases about rooms.
type Rooms struct {
	queries postgres.Querier
	now     func() time.Time
	newID   func() (uuid.UUID, error)
}

// NewRooms returns the use cases over queries. now gives the current time.
func NewRooms(queries postgres.Querier, now func() time.Time) *Rooms {
	return &Rooms{queries: queries, now: now, newID: uuid.NewV7}
}

// RegisterRoomInput is what RegisterRoom needs.
type RegisterRoomInput struct {
	Name      string
	Capacity  int
	OpenFrom  TimeOfDay
	OpenUntil TimeOfDay
	TimeZone  string
}

// RegisterRoom registers a room. The room must open before it closes, its
// time zone must exist, and its name must not be taken.
func (r *Rooms) RegisterRoom(ctx context.Context, in RegisterRoomInput) (Room, error) {
	if !in.OpenFrom.Before(in.OpenUntil) {
		return Room{}, newError(KindInvalidOpeningHours, fmt.Sprintf("openFrom %s is not before openUntil %s", in.OpenFrom, in.OpenUntil))
	}
	if _, err := time.LoadLocation(in.TimeZone); err != nil || in.TimeZone == "Local" {
		return Room{}, newError(KindUnknownTimeZone, fmt.Sprintf("%q is not the name of an IANA time zone", in.TimeZone))
	}
	id, err := r.newID()
	if err != nil {
		return Room{}, fmt.Errorf("register room: new id: %w", err)
	}
	row, err := r.queries.InsertRoom(ctx, postgres.InsertRoomParams{
		ID:        id,
		Name:      in.Name,
		Capacity:  int32(in.Capacity), //nolint:gosec // The API document limits capacity to 1000.
		OpenFrom:  in.OpenFrom.pg(),
		OpenUntil: in.OpenUntil.pg(),
		TimeZone:  in.TimeZone,
		CreatedAt: r.now(),
	})
	if postgres.IsUniqueViolation(err, "rooms_name_key") {
		return Room{}, newError(KindRoomNameTaken, fmt.Sprintf("a room named %q exists", in.Name))
	}
	if err != nil {
		return Room{}, fmt.Errorf("register room: %w", err)
	}
	return roomFrom(row), nil
}

// GetRoom returns the room with the given id.
func (r *Rooms) GetRoom(ctx context.Context, id uuid.UUID) (Room, error) {
	row, err := r.queries.GetRoom(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Room{}, newError(KindRoomNotFound, fmt.Sprintf("no room has the id %s", id))
	}
	if err != nil {
		return Room{}, fmt.Errorf("get room: %w", err)
	}
	return roomFrom(row), nil
}

// cursor is the position after the last room of a page: rooms are listed in
// the order of (name, id).
type cursor struct {
	Name string    `json:"n"`
	ID   uuid.UUID `json:"i"`
}

// ListRooms returns at most limit rooms after cursor, in the order of their
// names, and the cursor of the next page. The next cursor is empty on the
// last page. An empty cursor asks for the first page.
func (r *Rooms) ListRooms(ctx context.Context, limit int, after string) (rooms []Room, next string, err error) {
	var c cursor
	if after != "" {
		raw, err := base64.RawURLEncoding.DecodeString(after)
		if err != nil || json.Unmarshal(raw, &c) != nil {
			return nil, "", newError(KindInvalidCursor, "the cursor is not one that this API returned")
		}
	}
	// One row more than the page, to learn whether another page follows.
	rows, err := r.queries.ListRooms(ctx, postgres.ListRoomsParams{
		AfterName: c.Name,
		AfterID:   c.ID,
		PageSize:  int32(limit) + 1, //nolint:gosec // The API document limits limit to 100.
	})
	if err != nil {
		return nil, "", fmt.Errorf("list rooms: %w", err)
	}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		raw, err := json.Marshal(cursor{Name: last.Name, ID: last.ID})
		if err != nil {
			return nil, "", fmt.Errorf("list rooms: cursor: %w", err)
		}
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	rooms = make([]Room, 0, len(rows))
	for _, row := range rows {
		rooms = append(rooms, roomFrom(row))
	}
	return rooms, next, nil
}

func roomFrom(row postgres.Room) Room {
	return Room{
		ID:        row.ID,
		Name:      row.Name,
		Capacity:  int(row.Capacity),
		OpenFrom:  timeOfDayFrom(row.OpenFrom),
		OpenUntil: timeOfDayFrom(row.OpenUntil),
		TimeZone:  row.TimeZone,
		CreatedAt: row.CreatedAt.UTC(),
	}
}

// TimeOfDay is a time of day in minutes after midnight.
type TimeOfDay int

// ParseTimeOfDay reads a time of day written as "HH:MM" on a 24-hour clock.
func ParseTimeOfDay(s string) (TimeOfDay, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("%q is not a time of day in the form HH:MM", s)
	}
	return TimeOfDay(t.Hour()*60 + t.Minute()), nil
}

// String writes the time of day as "HH:MM".
func (t TimeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d", int(t)/60, int(t)%60)
}

// Before reports whether t is earlier in the day than u.
func (t TimeOfDay) Before(u TimeOfDay) bool {
	return t < u
}

func (t TimeOfDay) pg() pgtype.Time {
	return pgtype.Time{Microseconds: int64(t) * int64(time.Minute/time.Microsecond), Valid: true}
}

func timeOfDayFrom(t pgtype.Time) TimeOfDay {
	return TimeOfDay(t.Microseconds / int64(time.Minute/time.Microsecond))
}
