//go:build integration

package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/adapter/postgres"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/postgres/postgrestest"
)

// newRooms returns the use cases over the real database, connected as the
// runtime role of Facility, and a prefix for the names of the rooms of this
// test. The rooms with that prefix are deleted when the test ends.
func newRooms(t *testing.T) (rooms *app.Rooms, prefix string, pool *pgxpool.Pool) {
	t.Helper()
	pool = postgrestest.Open(t, "facility")
	prefix = fmt.Sprintf("it-%d-", time.Now().UnixNano())
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM rooms WHERE name LIKE $1", prefix+"%"); err != nil {
			t.Errorf("cannot delete the rooms of the test: %v", err)
		}
	})
	now := func() time.Time { return time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC) }
	return app.NewRooms(postgres.New(pool), now), prefix, pool
}

func input(name string) app.RegisterRoomInput {
	return app.RegisterRoomInput{Name: name, Capacity: 8, OpenFrom: 9 * 60, OpenUntil: 18 * 60, TimeZone: "Asia/Tokyo"}
}

func wantKind(t *testing.T, err error, kind app.Kind) {
	t.Helper()
	var business *app.Error
	if !errors.As(err, &business) || business.Kind != kind {
		t.Errorf("got the error %v, want a business error of kind %d", err, kind)
	}
}

func TestRoomsInTheDatabase(t *testing.T) {
	t.Parallel()
	rooms, prefix, _ := newRooms(t)
	ctx := t.Context()

	created, err := rooms.RegisterRoom(ctx, input(prefix+"Aoba"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := rooms.GetRoom(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got != created {
		t.Errorf("got %+v, want the registered room %+v", got, created)
	}
	if got.OpenFrom.String() != "09:00" || got.OpenUntil.String() != "18:00" {
		t.Errorf("the opening hours changed in the database: %s to %s", got.OpenFrom, got.OpenUntil)
	}

	// The unique constraint of the database decides that a name is taken.
	_, err = rooms.RegisterRoom(ctx, input(prefix+"Aoba"))
	wantKind(t, err, app.KindRoomNameTaken)
}

func TestListRoomsInTheDatabase(t *testing.T) {
	t.Parallel()
	rooms, prefix, _ := newRooms(t)
	ctx := t.Context()
	for _, name := range []string{"Cedar", "Aoba", "Birch"} {
		if _, err := rooms.RegisterRoom(ctx, input(prefix+name)); err != nil {
			t.Fatal(err)
		}
	}

	// Other tests add rooms at the same time, so read every page and keep the
	// rooms of this test.
	var names []string
	cursor := ""
	for {
		page, next, err := rooms.ListRooms(ctx, 2, cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, room := range page {
			if len(room.Name) > len(prefix) && room.Name[:len(prefix)] == prefix {
				names = append(names, room.Name[len(prefix):])
			}
		}
		if next == "" {
			break
		}
		cursor = next
	}
	if got := fmt.Sprint(names); got != "[Aoba Birch Cedar]" {
		t.Errorf("got the rooms %s, want [Aoba Birch Cedar], each once and in the order of their names", got)
	}
}

// The constraints of the schema reject what the use case would never send.
func TestRoomConstraints(t *testing.T) {
	t.Parallel()
	_, prefix, pool := newRooms(t)
	insert := `INSERT INTO rooms (name, capacity, open_from, open_until, time_zone, created_at)
		VALUES ($1, $2, $3::time, $4::time, 'Asia/Tokyo', now())`
	tests := []struct {
		name      string
		capacity  int
		openFrom  string
		openUntil string
	}{
		{"zero-capacity", 0, "09:00", "18:00"},
		{"huge-capacity", 1001, "09:00", "18:00"},
		{"closes-before-it-opens", 8, "18:00", "09:00"},
	}
	for _, tt := range tests {
		if _, err := pool.Exec(t.Context(), insert, prefix+tt.name, tt.capacity, tt.openFrom, tt.openUntil); err == nil {
			t.Errorf("%s: the database accepted the row", tt.name)
		}
	}
}

// The runtime role works with the rows of its own schema, and with nothing
// else (TPL-0005, TPL-0009).
func TestRuntimeRolePrivileges(t *testing.T) {
	t.Parallel()
	pool := postgrestest.Open(t, "facility")
	denied := map[string]string{
		"create a table in its schema":        "CREATE TABLE facility.intruder (id integer)",
		"change a table":                      "ALTER TABLE rooms ADD COLUMN intruder integer",
		"drop a table":                        "DROP TABLE rooms",
		"empty a table":                       "TRUNCATE rooms",
		"read the schema of another context":  "SELECT 1 FROM reservation.reservations",
		"create a table in the public schema": "CREATE TABLE public.intruder (id integer)",
	}
	for what, statement := range denied {
		if _, err := pool.Exec(t.Context(), statement); err == nil {
			t.Errorf("the runtime role of Facility could %s", what)
		}
	}
}
