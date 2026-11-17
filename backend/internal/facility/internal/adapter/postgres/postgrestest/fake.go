// Package postgrestest provides an in-memory stand-in for the generated
// queries of the Facility context, for tests that must run without a
// database. It keeps the rules of the schema that the use cases rely on: the
// unique name, and the order of a listing. The tests against a real database
// (build tag integration) check the queries themselves.
package postgrestest

import (
	"cmp"
	"context"
	"slices"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/adapter/postgres"
)

// Fake implements postgres.Querier in memory.
type Fake struct {
	mu    sync.Mutex
	rooms []postgres.Room
}

var _ postgres.Querier = (*Fake)(nil)

// InsertRoom stores a room, and fails like the database when the name is taken.
func (f *Fake) InsertRoom(_ context.Context, arg postgres.InsertRoomParams) (postgres.Room, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, room := range f.rooms {
		if room.Name == arg.Name {
			return postgres.Room{}, &pgconn.PgError{Code: "23505", ConstraintName: "rooms_name_key"}
		}
	}
	room := postgres.Room(arg)
	f.rooms = append(f.rooms, room)
	return room, nil
}

// GetRoom returns the room with the id, or pgx.ErrNoRows.
func (f *Fake) GetRoom(_ context.Context, id uuid.UUID) (postgres.Room, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, room := range f.rooms {
		if room.ID == id {
			return room, nil
		}
	}
	return postgres.Room{}, pgx.ErrNoRows
}

// ListRooms returns the rooms after (AfterName, AfterID) in the order of
// (name, id), at most PageSize of them.
func (f *Fake) ListRooms(_ context.Context, arg postgres.ListRoomsParams) ([]postgres.Room, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sorted := slices.Clone(f.rooms)
	slices.SortFunc(sorted, func(a, b postgres.Room) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID.String(), b.ID.String()))
	})
	page := []postgres.Room{}
	for _, room := range sorted {
		after := cmp.Or(cmp.Compare(room.Name, arg.AfterName), cmp.Compare(room.ID.String(), arg.AfterID.String())) > 0
		if after && len(page) < int(arg.PageSize) {
			page = append(page, room)
		}
	}
	return page, nil
}
